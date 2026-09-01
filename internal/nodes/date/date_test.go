package date

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

// fixed is the deterministic instant (Saturday, June 15 2024, 12:30:45.123 UTC)
// used by every timestamp-based assertion.
var fixed = time.Date(2024, 6, 15, 12, 30, 45, 123000000, time.UTC)

var allNodeTypes = []string{
	"date:now", "date:create", "date:extract", "date:format", "date:parse",
	"date:compare", "date:add", "date:subtract", "date:to_unix", "date:to_unix_ms",
}

func invoke(nodeType string, config, inputs map[string]any) nodes.Invocation {
	module, _ := Find(nodeType)
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: nodeType, Type: nodeType, Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func execute(t *testing.T, nodeType string, config, inputs map[string]any) (map[string]any, error) {
	t.Helper()
	module, ok := Find(nodeType)
	if !ok {
		t.Fatalf("%s is not a registered date node", nodeType)
	}
	result, err := module.Execute(context.Background(), invoke(nodeType, config, inputs), nil)
	if err != nil {
		return nil, err
	}
	if result.Ports != nil || result.Loop != nil {
		t.Fatalf("%s returned ports/loop on a pure node: %#v", nodeType, result)
	}
	return result.Outputs, nil
}

func TestRegisterExposesEveryNode(t *testing.T) {
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got := len(All()); got != len(allNodeTypes) {
		t.Fatalf("All() = %d nodes, want %d", got, len(allNodeTypes))
	}
	for _, nodeType := range allNodeTypes {
		module, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s was not registered", nodeType)
		}
		definition := module.Definition()
		if definition.Category != "Date" || definition.Mode != domain.NodePure || definition.Source != "builtin" {
			t.Fatalf("%s definition = %#v", nodeType, definition)
		}
		if _, found := Find(nodeType); !found {
			t.Fatalf("Find(%q) = false", nodeType)
		}
	}
	if _, found := Find("date:unknown"); found {
		t.Fatal("Find() accepted an unknown node type")
	}
}

func TestNowProducesFiniteTimestampAndStrings(t *testing.T) {
	for _, timezone := range []string{"local", "utc"} {
		outputs, err := execute(t, "date:now", map[string]any{"timezone": timezone}, map[string]any{})
		if err != nil {
			t.Fatalf("now(%s) error = %v", timezone, err)
		}
		timestamp, ok := outputs["timestamp"].(float64)
		if !ok || math.IsNaN(timestamp) || math.IsInf(timestamp, 0) {
			t.Fatalf("now(%s) timestamp = %#v", timezone, outputs["timestamp"])
		}
		iso, _ := outputs["iso"].(string)
		local, _ := outputs["local"].(string)
		if iso == "" || local == "" {
			t.Fatalf("now(%s) strings = %q / %q", timezone, iso, local)
		}
		if _, err := time.Parse(time.RFC3339Nano, iso); err != nil {
			t.Fatalf("now(%s) iso %q is not RFC3339: %v", timezone, iso, err)
		}
	}
	local, _ := execute(t, "date:now", map[string]any{"timezone": "local"}, map[string]any{})
	utc, _ := execute(t, "date:now", map[string]any{"timezone": "utc"}, map[string]any{})
	drift := math.Abs(local["timestamp"].(float64) - utc["timestamp"].(float64))
	if drift > 24*60*60*1000 {
		t.Fatalf("local and utc now() differ by %v ms", drift)
	}
}

func TestCreateFromComponents(t *testing.T) {
	outputs, err := execute(t, "date:create", map[string]any{"timezone": "utc"}, map[string]any{
		"year": 2024.0, "month": 6.0, "day": 15.0,
		"hour": 12.0, "minute": 30.0, "second": 45.0, "millisecond": 123.0,
	})
	if err != nil {
		t.Fatalf("create() error = %v", err)
	}
	if outputs["timestamp"] != float64(fixed.UnixMilli()) {
		t.Fatalf("timestamp = %#v, want %v", outputs["timestamp"], fixed.UnixMilli())
	}
	if outputs["iso"] != "2024-06-15T12:30:45.123Z" {
		t.Fatalf("iso = %#v", outputs["iso"])
	}
}

func TestCreateFallsBackToConfigAndDefaults(t *testing.T) {
	// Config values feed unwired pins; month/day default to January 1st.
	outputs, err := execute(t, "date:create", map[string]any{
		"year": 2024.0, "month": 6.0, "day": 15.0, "timezone": "utc",
	}, map[string]any{})
	if err != nil {
		t.Fatalf("create() error = %v", err)
	}
	want := time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC)
	if outputs["timestamp"] != float64(want.UnixMilli()) {
		t.Fatalf("timestamp = %#v, want %v", outputs["timestamp"], want.UnixMilli())
	}

	defaults, err := execute(t, "date:create", map[string]any{"timezone": "utc"}, map[string]any{})
	if err != nil {
		t.Fatalf("create(defaults) error = %v", err)
	}
	wantDefault := time.Date(time.Now().Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	if defaults["timestamp"] != float64(wantDefault.UnixMilli()) {
		t.Fatalf("default timestamp = %#v, want %v", defaults["timestamp"], wantDefault.UnixMilli())
	}
}

func TestCreateInputsOverrideConfig(t *testing.T) {
	outputs, err := execute(t, "date:create",
		map[string]any{"year": 2024.0, "month": 5.0, "day": 10.0, "timezone": "utc"},
		map[string]any{"month": 6.0, "day": 15.0},
	)
	if err != nil {
		t.Fatalf("create() error = %v", err)
	}
	want := time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC)
	if outputs["timestamp"] != float64(want.UnixMilli()) {
		t.Fatalf("timestamp = %#v, want %v", outputs["timestamp"], want.UnixMilli())
	}
}

func TestCreateRejectsInvalidMonth(t *testing.T) {
	for _, month := range []float64{0, 13, -1} {
		if _, err := execute(t, "date:create", map[string]any{"timezone": "utc"}, map[string]any{"month": month}); err == nil || !strings.Contains(err.Error(), "month must be between 1 and 12") {
			t.Fatalf("month %v error = %v", month, err)
		}
	}
}

func TestCreateUnknownTimezoneFallsBackToLocal(t *testing.T) {
	// Anything that is not "utc" resolves to the machine's local zone.
	unknown, err := execute(t, "date:create", map[string]any{
		"year": 2024.0, "month": 6.0, "day": 15.0, "timezone": "mars/olympus",
	}, map[string]any{})
	if err != nil {
		t.Fatalf("create(unknown timezone) error = %v", err)
	}
	local, err := execute(t, "date:create", map[string]any{
		"year": 2024.0, "month": 6.0, "day": 15.0, "timezone": "local",
	}, map[string]any{})
	if err != nil {
		t.Fatalf("create(local) error = %v", err)
	}
	if unknown["timestamp"] != local["timestamp"] {
		t.Fatalf("unknown timezone timestamp = %#v, local = %#v", unknown["timestamp"], local["timestamp"])
	}
}

func TestExtractAllComponents(t *testing.T) {
	outputs, err := execute(t, "date:extract", map[string]any{"timezone": "utc"}, map[string]any{
		"timestamp": float64(fixed.UnixMilli()),
	})
	if err != nil {
		t.Fatalf("extract() error = %v", err)
	}
	want := map[string]any{
		"year": 2024.0, "month": 6.0, "day": 15.0,
		"hour": 12.0, "minute": 30.0, "second": 45.0, "millisecond": 123.0,
		"weekday": 6.0, "dayOfYear": 167.0, "weekOfYear": 24.0,
		"iso":  "2024-06-15T12:30:45.123Z",
		"unix": float64(fixed.Unix()), "unixMs": float64(fixed.UnixMilli()),
	}
	if len(outputs) != len(want) {
		t.Fatalf("extract outputs = %#v", outputs)
	}
	for key, value := range want {
		if outputs[key] != value {
			t.Errorf("extract %s = %#v, want %#v", key, outputs[key], value)
		}
	}
}

func TestExtractTimezoneDifference(t *testing.T) {
	timestamp := float64(fixed.UnixMilli())
	localDisplay := fixed.In(time.Local)
	local, err := execute(t, "date:extract", map[string]any{"timezone": "local"}, map[string]any{"timestamp": timestamp})
	if err != nil {
		t.Fatalf("extract(local) error = %v", err)
	}
	if local["hour"] != float64(localDisplay.Hour()) {
		t.Fatalf("local hour = %#v, want %v", local["hour"], localDisplay.Hour())
	}
	utc, err := execute(t, "date:extract", map[string]any{"timezone": "utc"}, map[string]any{"timestamp": timestamp})
	if err != nil {
		t.Fatalf("extract(utc) error = %v", err)
	}
	if utc["hour"] != 12.0 {
		t.Fatalf("utc hour = %#v", utc["hour"])
	}
}

func TestExtractRejectsInvalidTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp any
		want      string
	}{
		{"missing", nil, "extract requires numeric timestamp input"},
		{"text", "1718459445123", "extract requires numeric timestamp input"},
		{"NaN", math.NaN(), "timestamp must be a finite number"},
		{"infinite", math.Inf(1), "timestamp must be a finite number"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := execute(t, "date:extract", map[string]any{"timezone": "utc"}, map[string]any{"timestamp": test.timestamp})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestFormatUsesConfigInputAndDefaultLayouts(t *testing.T) {
	timestamp := float64(fixed.UnixMilli())
	tests := []struct {
		name   string
		inputs map[string]any
		config map[string]any
		want   string
	}{
		{"config layout", map[string]any{"timestamp": timestamp}, map[string]any{"format": "2006-01-02", "timezone": "utc"}, "2024-06-15"},
		{"input layout overrides config", map[string]any{"timestamp": timestamp, "format": " 15:04:05 "}, map[string]any{"format": "2006-01-02", "timezone": "utc"}, "12:30:45"},
		{"default layout", map[string]any{"timestamp": timestamp}, map[string]any{"timezone": "utc"}, "2024-06-15 12:30:45"},
		{"millisecond layout", map[string]any{"timestamp": timestamp}, map[string]any{"format": "15:04:05.000", "timezone": "utc"}, "12:30:45.123"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputs, err := execute(t, "date:format", test.config, test.inputs)
			if err != nil {
				t.Fatalf("format() error = %v", err)
			}
			if outputs["text"] != test.want {
				t.Fatalf("text = %#v, want %q", outputs["text"], test.want)
			}
		})
	}
}

func TestFormatRejectsInvalidTimestamp(t *testing.T) {
	if _, err := execute(t, "date:format", map[string]any{"timezone": "utc"}, map[string]any{}); err == nil || !strings.Contains(err.Error(), "format requires numeric timestamp input") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseCustomLayout(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		layout string
		from   string // "input" or "config"
		want   time.Time
	}{
		{"input layout", "2024-06-15 14:30:45", "2006-01-02 15:04:05", "input", time.Date(2024, time.June, 15, 14, 30, 45, 0, time.UTC)},
		{"config layout", "15/06/2024", "02/01/2006", "config", time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputs := map[string]any{"text": test.text}
			config := map[string]any{"timezone": "utc"}
			if test.from == "input" {
				inputs["format"] = test.layout
			} else {
				config["format"] = test.layout
			}
			outputs, err := execute(t, "date:parse", config, inputs)
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}
			if outputs["timestamp"] != float64(test.want.UnixMilli()) {
				t.Fatalf("timestamp = %#v, want %v", outputs["timestamp"], test.want.UnixMilli())
			}
			if outputs["iso"] != test.want.Format(time.RFC3339Nano) {
				t.Fatalf("iso = %#v", outputs["iso"])
			}
		})
	}
}

func TestParseCommonFormats(t *testing.T) {
	midnight := time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		text string
		want time.Time
	}{
		{"2024-06-15T14:30:45Z", time.Date(2024, time.June, 15, 14, 30, 45, 0, time.UTC)},
		{"2024-06-15T14:30:45.123Z", time.Date(2024, time.June, 15, 14, 30, 45, 123000000, time.UTC)},
		{"2024-06-15 14:30:45", time.Date(2024, time.June, 15, 14, 30, 45, 0, time.UTC)},
		{"2024-06-15", midnight},
		{"06/15/2024", midnight},
		{"15/06/2024", midnight},
		{"2024/06/15", midnight},
		{"Jun 15, 2024", midnight},
		{"June 15, 2024", midnight},
		{"15 Jun 2024", midnight},
		{"15 June 2024", midnight},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			outputs, err := execute(t, "date:parse", map[string]any{"timezone": "utc"}, map[string]any{"text": test.text})
			if err != nil {
				t.Fatalf("parse(%q) error = %v", test.text, err)
			}
			if outputs["timestamp"] != float64(test.want.UnixMilli()) {
				t.Fatalf("parse(%q) timestamp = %#v, want %v", test.text, outputs["timestamp"], test.want.UnixMilli())
			}
		})
	}
}

func TestParseFormatRoundTrip(t *testing.T) {
	parsed, err := execute(t, "date:parse", map[string]any{"timezone": "utc"}, map[string]any{"text": "2024-06-15T12:30:45.123Z"})
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	formatted, err := execute(t, "date:format", map[string]any{"format": time.RFC3339Nano, "timezone": "utc"}, map[string]any{"timestamp": parsed["timestamp"]})
	if err != nil {
		t.Fatalf("format() error = %v", err)
	}
	if formatted["text"] != "2024-06-15T12:30:45.123Z" {
		t.Fatalf("round trip = %#v", formatted["text"])
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	tests := []struct {
		name   string
		inputs map[string]any
		want   string
	}{
		{"empty text", map[string]any{"text": "  "}, "parse requires non-empty text input"},
		{"unparseable", map[string]any{"text": "not a date"}, "parse date:"},
		{"wrong layout", map[string]any{"text": "15/06/2024", "format": "2006-01-02"}, "parse date:"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := execute(t, "date:parse", map[string]any{"timezone": "utc"}, test.inputs)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestCompareOutputsBooleansAndDifferences(t *testing.T) {
	tests := []struct {
		name        string
		left, right float64
		want        map[string]any
	}{
		{
			name: "before", left: 1000, right: 2000,
			want: map[string]any{"before": true, "after": false, "equal": false, "diffMs": -1000.0, "diffSeconds": -1.0, "diffMinutes": -1.0 / 60, "diffHours": -1.0 / 3600, "diffDays": -1.0 / 86400},
		},
		{
			name: "after", left: 3000, right: 2000,
			want: map[string]any{"before": false, "after": true, "equal": false, "diffMs": 1000.0, "diffSeconds": 1.0, "diffMinutes": 1.0 / 60, "diffHours": 1.0 / 3600, "diffDays": 1.0 / 86400},
		},
		{
			name: "equal", left: 1500, right: 1500,
			want: map[string]any{"before": false, "after": false, "equal": true, "diffMs": 0.0, "diffSeconds": 0.0, "diffMinutes": 0.0, "diffHours": 0.0, "diffDays": 0.0},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputs, err := execute(t, "date:compare", map[string]any{}, map[string]any{"left": test.left, "right": test.right})
			if err != nil {
				t.Fatalf("compare() error = %v", err)
			}
			for key, value := range test.want {
				if outputs[key] != value {
					t.Errorf("compare %s = %#v, want %#v", key, outputs[key], value)
				}
			}
		})
	}
}

func TestCompareRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name        string
		left, right any
		want        string
	}{
		{"text left", "yesterday", 2000.0, "compare requires numeric left and right inputs"},
		{"missing right", 1000.0, nil, "compare requires numeric left and right inputs"},
		{"NaN", math.NaN(), 1.0, "inputs must be finite numbers"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := execute(t, "date:compare", map[string]any{}, map[string]any{"left": test.left, "right": test.right})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAddKnownDurations(t *testing.T) {
	base := time.Date(2024, time.June, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		inputs map[string]any
		want   time.Time
	}{
		{"days and hours", map[string]any{"timestamp": float64(base.UnixMilli()), "days": 5.0, "hours": 3.0}, base.AddDate(0, 0, 5).Add(3 * time.Hour)},
		{"minutes seconds milliseconds", map[string]any{"timestamp": float64(base.UnixMilli()), "minutes": 90.0, "seconds": 30.0, "milliseconds": 250.0}, base.Add(90*time.Minute + 30*time.Second + 250*time.Millisecond)},
		{"month clamp to month end", map[string]any{"timestamp": float64(time.Date(2024, time.January, 31, 12, 0, 0, 0, time.UTC).UnixMilli()), "months": 1.0}, time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC)},
		{"leap day plus one year", map[string]any{"timestamp": float64(time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC).UnixMilli()), "years": 1.0}, time.Date(2025, time.February, 28, 12, 0, 0, 0, time.UTC)},
		{"no duration is identity", map[string]any{"timestamp": float64(base.UnixMilli())}, base},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputs, err := execute(t, "date:add", map[string]any{"timezone": "utc"}, test.inputs)
			if err != nil {
				t.Fatalf("add() error = %v", err)
			}
			if outputs["timestamp"] != float64(test.want.UnixMilli()) {
				t.Fatalf("timestamp = %#v, want %v", outputs["timestamp"], test.want.UnixMilli())
			}
		})
	}
}

func TestSubtractKnownDurations(t *testing.T) {
	base := time.Date(2024, time.June, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		inputs map[string]any
		want   time.Time
	}{
		{"days", map[string]any{"timestamp": float64(base.UnixMilli()), "days": 2.0}, base.AddDate(0, 0, -2)},
		{"hours and minutes", map[string]any{"timestamp": float64(base.UnixMilli()), "hours": 1.0, "minutes": 30.0}, base.Add(-90 * time.Minute)},
		{"month clamp", map[string]any{"timestamp": float64(time.Date(2024, time.May, 31, 12, 0, 0, 0, time.UTC).UnixMilli()), "months": 1.0}, time.Date(2024, time.April, 30, 12, 0, 0, 0, time.UTC)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputs, err := execute(t, "date:subtract", map[string]any{"timezone": "utc"}, test.inputs)
			if err != nil {
				t.Fatalf("subtract() error = %v", err)
			}
			if outputs["timestamp"] != float64(test.want.UnixMilli()) {
				t.Fatalf("timestamp = %#v, want %v", outputs["timestamp"], test.want.UnixMilli())
			}
		})
	}
}

func TestAddInputsOverrideConfigDurations(t *testing.T) {
	base := time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC)
	outputs, err := execute(t, "date:add",
		map[string]any{"days": 1.0, "hours": 2.0, "timezone": "utc"},
		map[string]any{"timestamp": float64(base.UnixMilli()), "days": 5.0},
	)
	if err != nil {
		t.Fatalf("add() error = %v", err)
	}
	want := base.AddDate(0, 0, 5).Add(2 * time.Hour)
	if outputs["timestamp"] != float64(want.UnixMilli()) {
		t.Fatalf("timestamp = %#v, want %v", outputs["timestamp"], want.UnixMilli())
	}
}

func TestAddRejectsInvalidTimestamp(t *testing.T) {
	for _, nodeType := range []string{"date:add", "date:subtract"} {
		if _, err := execute(t, nodeType, map[string]any{"timezone": "utc"}, map[string]any{"days": 5.0}); err == nil || !strings.Contains(err.Error(), "date math requires numeric timestamp input") {
			t.Fatalf("%s error = %v", nodeType, err)
		}
	}
}

func TestToUnixConversions(t *testing.T) {
	tests := []struct {
		nodeType string
		input    float64
		want     float64
	}{
		{"date:to_unix", 1718461845123.0, 1718461845.123},
		{"date:to_unix_ms", 1718461845123.0, 1718461845123.0},
	}
	for _, test := range tests {
		t.Run(test.nodeType, func(t *testing.T) {
			outputs, err := execute(t, test.nodeType, map[string]any{}, map[string]any{"timestamp": test.input})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if outputs["value"] != test.want {
				t.Fatalf("value = %#v, want %v", outputs["value"], test.want)
			}
		})
	}
}

func TestToUnixRejectsInvalidTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp any
		want      string
	}{
		{"missing", nil, "to_unix requires numeric timestamp input"},
		{"text", "1718461845123", "to_unix requires numeric timestamp input"},
		{"NaN", math.NaN(), "timestamp must be a finite number"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := execute(t, "date:to_unix", map[string]any{}, map[string]any{"timestamp": test.timestamp})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestDefinitionShapes(t *testing.T) {
	now, _ := Find("date:now")
	nowDefinition := now.Definition()
	if len(nowDefinition.Outputs) != 3 || nowDefinition.Outputs[0].ID != "timestamp" || nowDefinition.Outputs[1].ID != "iso" || nowDefinition.Outputs[2].ID != "local" {
		t.Fatalf("now outputs = %#v", nowDefinition.Outputs)
	}
	if len(nowDefinition.Fields) != 1 || nowDefinition.Fields[0].Name != "timezone" || len(nowDefinition.Fields[0].Options) != 2 {
		t.Fatalf("now fields = %#v", nowDefinition.Fields)
	}
	if nowDefinition.DefaultConfig["timezone"] != "local" {
		t.Fatalf("now defaults = %#v", nowDefinition.DefaultConfig)
	}

	extract, _ := Find("date:extract")
	extractDefinition := extract.Definition()
	if len(extractDefinition.Outputs) != 13 || len(extractDefinition.Inputs) != 1 {
		t.Fatalf("extract ports = %d inputs / %d outputs", len(extractDefinition.Inputs), len(extractDefinition.Outputs))
	}

	compare, _ := Find("date:compare")
	if definition := compare.Definition(); len(definition.Outputs) != 8 || definition.Fields != nil {
		t.Fatalf("compare definition = %#v", definition)
	}
}

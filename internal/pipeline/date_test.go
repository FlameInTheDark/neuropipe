package pipeline

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestEvaluateDate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		nodeType string
		inputs   map[string]any
		config   map[string]any
		want     map[string]any
		wantErr  string
	}{
		{
			name:     "now local",
			nodeType: "date:now",
			inputs:   map[string]any{},
			config:   map[string]any{"timezone": "local"},
			want:     nil,
			wantErr:  "",
		},
		{
			name:     "now utc",
			nodeType: "date:now",
			inputs:   map[string]any{},
			config:   map[string]any{"timezone": "utc"},
			want:     nil,
			wantErr:  "",
		},
		{
			name:     "create full",
			nodeType: "date:create",
			inputs:   map[string]any{},
			config:   map[string]any{"year": 2024.0, "month": 6.0, "day": 15.0, "hour": 14.0, "minute": 30.0, "second": 45.0, "millisecond": 123.0, "timezone": "utc"},
			want:     nil,
			wantErr:  "",
		},
		{
			name:     "create defaults",
			nodeType: "date:create",
			inputs:   map[string]any{},
			config:   map[string]any{"timezone": "utc"},
			want:     nil,
			wantErr:  "",
		},
		{
			name:     "create invalid month",
			nodeType: "date:create",
			inputs:   map[string]any{},
			config:   map[string]any{"month": 13.0, "timezone": "utc"},
			want:     nil,
			wantErr:  "month must be between 1 and 12",
		},
		{
			name:     "extract",
			nodeType: "date:extract",
			inputs:   map[string]any{"timestamp": float64(time.Date(2024, 6, 15, 14, 30, 45, 123000000, time.UTC).UnixMilli())},
			config:   map[string]any{"timezone": "utc"},
			want: map[string]any{
				"year":        2024.0,
				"month":       6.0,
				"day":         15.0,
				"hour":        14.0,
				"minute":      30.0,
				"second":      45.0,
				"millisecond": 123.0,
				"weekday":     6.0,
				"dayOfYear":   167.0,
				"weekOfYear":  24.0,
				"unix":        float64(time.Date(2024, 6, 15, 14, 30, 45, 123000000, time.UTC).Unix()),
				"unixMs":      float64(time.Date(2024, 6, 15, 14, 30, 45, 123000000, time.UTC).UnixMilli()),
			},
			wantErr: "",
		},
		{
			name:     "extract missing timestamp",
			nodeType: "date:extract",
			inputs:   map[string]any{},
			config:   map[string]any{"timezone": "utc"},
			want:     nil,
			wantErr:  "extract requires numeric timestamp input",
		},
		{
			name:     "format with config",
			nodeType: "date:format",
			inputs:   map[string]any{"timestamp": float64(time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC).UnixMilli())},
			config:   map[string]any{"format": "2006-01-02", "timezone": "utc"},
			want:     map[string]any{"text": "2024-06-15"},
			wantErr:  "",
		},
		{
			name:     "format with input override",
			nodeType: "date:format",
			inputs:   map[string]any{"timestamp": float64(time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC).UnixMilli()), "format": "15:04:05"},
			config:   map[string]any{"format": "2006-01-02", "timezone": "utc"},
			want:     map[string]any{"text": "14:30:45"},
			wantErr:  "",
		},
		{
			name:     "format missing timestamp",
			nodeType: "date:format",
			inputs:   map[string]any{},
			config:   map[string]any{"timezone": "utc"},
			want:     nil,
			wantErr:  "format requires numeric timestamp input",
		},
		{
			name:     "parse with format",
			nodeType: "date:parse",
			inputs:   map[string]any{"text": "2024-06-15 14:30:45", "format": "2006-01-02 15:04:05"},
			config:   map[string]any{"timezone": "utc"},
			want:     nil,
			wantErr:  "",
		},
		{
			name:     "parse auto RFC3339",
			nodeType: "date:parse",
			inputs:   map[string]any{"text": "2024-06-15T14:30:45Z"},
			config:   map[string]any{"timezone": "utc"},
			want:     nil,
			wantErr:  "",
		},
		{
			name:     "parse auto ISO",
			nodeType: "date:parse",
			inputs:   map[string]any{"text": "2024-06-15T14:30:45.123456789Z"},
			config:   map[string]any{"timezone": "utc"},
			want:     nil,
			wantErr:  "",
		},
		{
			name:     "parse missing text",
			nodeType: "date:parse",
			inputs:   map[string]any{"text": ""},
			config:   map[string]any{"timezone": "utc"},
			want:     nil,
			wantErr:  "parse requires non-empty text input",
		},
		{
			name:     "compare before",
			nodeType: "date:compare",
			inputs:   map[string]any{"left": 1000.0, "right": 2000.0},
			config:   map[string]any{},
			want: map[string]any{
				"before":      true,
				"after":       false,
				"equal":       false,
				"diffMs":      -1000.0,
				"diffSeconds": -1.0,
				"diffMinutes": -1.0 / 60.0,
				"diffHours":   -1.0 / 3600.0,
				"diffDays":    -1.0 / 86400.0,
			},
			wantErr: "",
		},
		{
			name:     "compare after",
			nodeType: "date:compare",
			inputs:   map[string]any{"left": 3000.0, "right": 2000.0},
			config:   map[string]any{},
			want: map[string]any{
				"before":      false,
				"after":       true,
				"equal":       false,
				"diffMs":      1000.0,
				"diffSeconds": 1.0,
				"diffMinutes": 1.0 / 60.0,
				"diffHours":   1.0 / 3600.0,
				"diffDays":    1.0 / 86400.0,
			},
			wantErr: "",
		},
		{
			name:     "compare equal",
			nodeType: "date:compare",
			inputs:   map[string]any{"left": 1500.0, "right": 1500.0},
			config:   map[string]any{},
			want: map[string]any{
				"before":      false,
				"after":       false,
				"equal":       true,
				"diffMs":      0.0,
				"diffSeconds": 0.0,
				"diffMinutes": 0.0,
				"diffHours":   0.0,
				"diffDays":    0.0,
			},
			wantErr: "",
		},
		{
			name:     "compare invalid input",
			nodeType: "date:compare",
			inputs:   map[string]any{"left": "invalid", "right": 2000.0},
			config:   map[string]any{},
			want:     nil,
			wantErr:  "compare requires numeric left and right inputs",
		},
		{
			name:     "add duration",
			nodeType: "date:add",
			inputs:   map[string]any{"timestamp": float64(time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC).UnixMilli()), "days": 5.0, "hours": 3.0},
			config:   map[string]any{"timezone": "utc"},
			want:     nil,
			wantErr:  "",
		},
		{
			name:     "subtract duration",
			nodeType: "date:subtract",
			inputs:   map[string]any{"timestamp": float64(time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC).UnixMilli()), "days": 2.0},
			config:   map[string]any{"timezone": "utc"},
			want:     nil,
			wantErr:  "",
		},
		{
			name:     "add missing timestamp",
			nodeType: "date:add",
			inputs:   map[string]any{"days": 5.0},
			config:   map[string]any{"timezone": "utc"},
			want:     nil,
			wantErr:  "date math requires numeric timestamp input",
		},
		{
			name:     "to_unix",
			nodeType: "date:to_unix",
			inputs:   map[string]any{"timestamp": 1718461845123.0},
			config:   map[string]any{},
			want:     map[string]any{"value": 1718461845.123},
			wantErr:  "",
		},
		{
			name:     "to_unix_ms",
			nodeType: "date:to_unix_ms",
			inputs:   map[string]any{"timestamp": 1718461845123.0},
			config:   map[string]any{},
			want:     map[string]any{"value": 1718461845123.0},
			wantErr:  "",
		},
		{
			name:     "to_unix missing timestamp",
			nodeType: "date:to_unix",
			inputs:   map[string]any{},
			config:   map[string]any{},
			want:     nil,
			wantErr:  "to_unix requires numeric timestamp input",
		},
		{
			name:     "unknown node type",
			nodeType: "date:unknown",
			inputs:   map[string]any{},
			config:   map[string]any{},
			want:     nil,
			wantErr:  "unsupported date node",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputs, err := evaluateDate(test.nodeType, test.inputs, test.config)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("evaluateDate() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("evaluateDate() error = %v", err)
			}
			if test.want != nil {
				for k, v := range test.want {
					got := outputs[k]
					if gv, ok := got.(float64); ok {
						wantFloat, ok := v.(float64)
						if !ok {
							t.Errorf("output %s want value is not float64: %T", k, v)
							continue
						}
						if math.Abs(gv-wantFloat) > 0.001 {
							t.Errorf("output %s = %v, want %v", k, gv, wantFloat)
						}
					} else if gv, ok := got.(bool); ok {
						wantBool, ok := v.(bool)
						if !ok {
							t.Errorf("output %s want value is not bool: %T", k, v)
							continue
						}
						if gv != wantBool {
							t.Errorf("output %s = %v, want %v", k, gv, wantBool)
						}
					} else if gv, ok := got.(string); ok {
						wantStr, ok := v.(string)
						if !ok {
							t.Errorf("output %s want value is not string: %T", k, v)
							continue
						}
						if gv != wantStr {
							t.Errorf("output %s = %q, want %q", k, gv, wantStr)
						}
					}
				}
			}
			if outputs == nil {
				t.Fatalf("expected outputs, got nil")
			}
		})
	}
}

func TestEvaluateNowTimezone(t *testing.T) {
	t.Parallel()
	localOut, _ := evaluateDate("date:now", map[string]any{}, map[string]any{"timezone": "local"})
	utcOut, _ := evaluateDate("date:now", map[string]any{}, map[string]any{"timezone": "utc"})

	localTs := localOut["timestamp"].(float64)
	utcTs := utcOut["timestamp"].(float64)

	diff := math.Abs(localTs - utcTs)
	if diff > 24*60*60*1000 {
		t.Errorf("local and utc timestamps differ by more than a day: %v vs %v", localTs, utcTs)
	}
}

func TestEvaluateCreateTimezone(t *testing.T) {
	t.Parallel()
	localWall := time.Date(2024, 6, 15, 12, 0, 0, 0, time.Local)
	utcWall := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	want := float64(localWall.UnixMilli() - utcWall.UnixMilli())

	localOut, _ := evaluateDate("date:create", map[string]any{}, map[string]any{
		"year": 2024.0, "month": 6.0, "day": 15.0, "hour": 12.0, "timezone": "local",
	})
	utcOut, _ := evaluateDate("date:create", map[string]any{}, map[string]any{
		"year": 2024.0, "month": 6.0, "day": 15.0, "hour": 12.0, "timezone": "utc",
	})

	localTs := localOut["timestamp"].(float64)
	utcTs := utcOut["timestamp"].(float64)

	if got := localTs - utcTs; got != want {
		t.Errorf("local and utc timestamp difference = %v, want %v (local offset in ms)", got, want)
	}
}

func TestEvaluateExtractTimezone(t *testing.T) {
	t.Parallel()
	localDisplay := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC).In(time.Local)
	ts := float64(time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC).UnixMilli())
	localOut, _ := evaluateDate("date:extract", map[string]any{"timestamp": ts}, map[string]any{"timezone": "local"})
	utcOut, _ := evaluateDate("date:extract", map[string]any{"timestamp": ts}, map[string]any{"timezone": "utc"})

	if got := localOut["hour"].(float64); got != float64(localDisplay.Hour()) {
		t.Errorf("local hour = %v, want %v", got, localDisplay.Hour())
	}
	if got := utcOut["hour"].(float64); got != 12 {
		t.Errorf("utc hour = %v, want 12", got)
	}
}

func TestEvaluateFormatTimezone(t *testing.T) {
	t.Parallel()
	localDisplay := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC).In(time.Local)
	ts := float64(time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC).UnixMilli())
	localOut, _ := evaluateDate("date:format", map[string]any{"timestamp": ts}, map[string]any{"format": "15:04", "timezone": "local"})
	utcOut, _ := evaluateDate("date:format", map[string]any{"timestamp": ts}, map[string]any{"format": "15:04", "timezone": "utc"})

	if got := localOut["text"]; got != localDisplay.Format("15:04") {
		t.Errorf("local formatted time = %q, want %q", got, localDisplay.Format("15:04"))
	}
	if got := utcOut["text"]; got != "12:00" {
		t.Errorf("utc formatted time = %q, want %q", got, "12:00")
	}
}

func TestEvaluateParseCommonFormats(t *testing.T) {
	t.Parallel()
	formats := []string{
		"2024-06-15T14:30:45Z",
		"2024-06-15T14:30:45.123Z",
		"2024-06-15 14:30:45",
		"2024-06-15",
		"06/15/2024",
		"15/06/2024",
		"2024/06/15",
		"Jun 15, 2024",
		"June 15, 2024",
		"15 Jun 2024",
		"15 June 2024",
	}

	for _, f := range formats {
		t.Run(f, func(t *testing.T) {
			_, err := evaluateDate("date:parse", map[string]any{"text": f}, map[string]any{"timezone": "utc"})
			if err != nil {
				t.Errorf("parse %q failed: %v", f, err)
			}
		})
	}
}

func TestEvaluateDateMathCalendarArithmetic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		inputs map[string]any
		want   time.Time
	}{
		{
			name:   "Jan 31 plus one month clamps to Feb 29 in a leap year",
			inputs: map[string]any{"timestamp": float64(time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC).UnixMilli()), "months": 1.0},
			want:   time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
		},
		{
			name:   "Mar 31 plus one month clamps to Apr 30",
			inputs: map[string]any{"timestamp": float64(time.Date(2024, 3, 31, 12, 0, 0, 0, time.UTC).UnixMilli()), "months": 1.0},
			want:   time.Date(2024, 4, 30, 12, 0, 0, 0, time.UTC),
		},
		{
			name:   "Feb 29 plus one year clamps to Feb 28",
			inputs: map[string]any{"timestamp": float64(time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC).UnixMilli()), "years": 1.0},
			want:   time.Date(2025, 2, 28, 12, 0, 0, 0, time.UTC),
		},
		{
			name:   "Jan 31 minus one month stays on Dec 31",
			inputs: map[string]any{"timestamp": float64(time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC).UnixMilli()), "months": -1.0},
			want:   time.Date(2023, 12, 31, 12, 0, 0, 0, time.UTC),
		},
		{
			name:   "May 31 minus one month clamps to Apr 30",
			inputs: map[string]any{"timestamp": float64(time.Date(2024, 5, 31, 12, 0, 0, 0, time.UTC).UnixMilli()), "months": -1.0},
			want:   time.Date(2024, 4, 30, 12, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, err := evaluateDate("date:add", test.inputs, map[string]any{"timezone": "utc"})
			if err != nil {
				t.Fatalf("add failed: %v", err)
			}
			result := time.UnixMilli(int64(out["timestamp"].(float64))).In(time.UTC)
			if !result.Equal(test.want) {
				t.Errorf("got %v, want %v", result, test.want)
			}
		})
	}
}

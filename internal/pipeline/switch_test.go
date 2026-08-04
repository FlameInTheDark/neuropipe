package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestMatchSwitchCase(t *testing.T) {
	tests := []struct {
		name       string
		comparator switchComparator
		valueType  domain.DataType
		input      any
		literal    any
		want       bool
	}{
		{name: "strict text equality", comparator: switchEquals, valueType: domain.DataText, input: "ready", literal: "ready", want: true},
		{name: "different types do not equal", comparator: switchEquals, valueType: domain.DataText, input: 1, literal: "1", want: false},
		{name: "boolean inequality", comparator: switchNotEquals, valueType: domain.DataBoolean, input: false, literal: true, want: true},
		{name: "text contains", comparator: switchContains, valueType: domain.DataText, input: "needs-review", literal: "review", want: true},
		{name: "text starts with", comparator: switchStartsWith, valueType: domain.DataText, input: "urgent-ticket", literal: "urgent", want: true},
		{name: "text ends with", comparator: switchEndsWith, valueType: domain.DataText, input: "report.md", literal: ".md", want: true},
		{name: "number greater than", comparator: switchGreaterThan, valueType: domain.DataNumber, input: 12, literal: 10, want: true},
		{name: "number greater than or equal", comparator: switchGreaterThanOrEqual, valueType: domain.DataNumber, input: 10, literal: 10, want: true},
		{name: "number less than", comparator: switchLessThan, valueType: domain.DataNumber, input: 9, literal: 10, want: true},
		{name: "number less than or equal", comparator: switchLessThanOrEqual, valueType: domain.DataNumber, input: 10, literal: 10, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			literal, err := switchLiteral(test.literal, test.valueType)
			if err != nil {
				t.Fatalf("switchLiteral() error = %v", err)
			}
			matched, err := matchSwitchCase(test.input, switchConfiguration{Comparator: test.comparator}, switchCase{ValueType: test.valueType, Value: literal})
			if err != nil {
				t.Fatalf("matchSwitchCase() error = %v", err)
			}
			if matched != test.want {
				t.Fatalf("matchSwitchCase() = %v, want %v", matched, test.want)
			}
		})
	}
}

func TestSwitchRoutesFirstMatchAndDefault(t *testing.T) {
	configuration := map[string]any{"switch": map[string]any{"comparator": "contains", "cases": []any{
		map[string]any{"id": "first", "label": "First", "valueType": "text", "value": "review"},
		map[string]any{"id": "second", "label": "Second", "valueType": "text", "value": "needs"},
	}}}
	for _, test := range []struct {
		name     string
		input    string
		outputID string
	}{
		{name: "first matching case wins", input: "needs-review", outputID: "first"},
		{name: "unmatched value follows default", input: "done", outputID: "default"},
	} {
		t.Run(test.name, func(t *testing.T) {
			flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
				v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
				v2Node("value", "data:constant", map[string]any{"value": test.input}),
				v2Node("switch", "flow:switch", configuration),
				v2Node("selected", "flow:reroute", nil),
			}, Edges: []domain.FlowEdge{
				execEdge("start-switch", "start", "out", "switch", "in"),
				dataEdge("value-switch", "value", "value", "switch", "selection"),
				execEdge("switch-selected", "switch", test.outputID, "selected", "in"),
			}}
			result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got := result.NodeRuns[len(result.NodeRuns)-1].NodeID; got != "selected" {
				t.Fatalf("last executed node = %q, want selected", got)
			}
		})
	}
}

func TestLegacySwitchUsesCaseInsensitiveOptions(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("value", "data:constant", map[string]any{"value": "APPROVED"}),
		v2Node("switch", "flow:switch", map[string]any{"options": []any{map[string]any{"id": "approved", "label": "Approved"}}}),
		v2Node("selected", "flow:reroute", nil),
	}, Edges: []domain.FlowEdge{
		execEdge("start-switch", "start", "out", "switch", "in"),
		dataEdge("value-switch", "value", "value", "switch", "selection"),
		execEdge("switch-selected", "switch", "approved", "selected", "in"),
	}}
	if _, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("legacy Switch Execute() error = %v", err)
	}
}

func TestSwitchRejectsIncompatibleRuntimeTextValue(t *testing.T) {
	_, err := matchSwitchCase(10, switchConfiguration{Comparator: switchContains}, switchCase{ValueType: domain.DataText, Value: "1"})
	if err == nil || !strings.Contains(err.Error(), "requires a Text value") {
		t.Fatalf("matchSwitchCase() error = %v, want text type error", err)
	}
}

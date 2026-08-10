package pipeline

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func constantOutput(t *testing.T, config map[string]any) (map[string]any, error) {
	t.Helper()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("const", "data:constant", config),
		v2Node("store", "flow:set_variable", map[string]any{"name": "Probe"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("const-store", "const", "value", "store", "value"),
	}}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		return nil, err
	}
	for _, run := range result.NodeRuns {
		if run.NodeID == "const" && run.Status == domain.RunCompleted {
			outputs, ok := run.Output.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("constant output is not a map: %#v", run.Output)
			}
			return outputs, nil
		}
	}
	return nil, fmt.Errorf("constant node was not evaluated")
}

func TestDataConstantTypedValues(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		want   map[string]any
	}{
		{name: "default type passes text", config: map[string]any{"value": "hello"}, want: map[string]any{"value": "hello"}},
		{name: "explicit text", config: map[string]any{"value": "5", "type": "text"}, want: map[string]any{"value": "5"}},
		{name: "number", config: map[string]any{"value": "7", "type": "number"}, want: map[string]any{"value": 7.0}},
		{name: "boolean true", config: map[string]any{"value": "true", "type": "boolean"}, want: map[string]any{"value": true}},
		{name: "boolean false", config: map[string]any{"value": "false", "type": "boolean"}, want: map[string]any{"value": false}},
		{name: "legacy typed value unaffected", config: map[string]any{"value": 3.0}, want: map[string]any{"value": 3.0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputs, err := constantOutput(t, test.config)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(outputs, test.want) {
				t.Fatalf("constant output = %#v, want %#v", outputs, test.want)
			}
		})
	}
}

func TestDataConstantInvalidNumberFails(t *testing.T) {
	_, err := constantOutput(t, map[string]any{"value": "abc", "type": "number"})
	if err == nil || !strings.Contains(err.Error(), "cannot cast") {
		t.Fatalf("Execute() error = %v, want cast failure", err)
	}
}

func TestDataConstantV3RejectsLegacyTextCoercion(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("constant", "data:constant", map[string]any{"value": "7", "type": "number"}),
		v2Node("store", "flow:set_variable", map[string]any{"name": "Probe"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("constant-store", "constant", "value", "store", "value"),
	}}
	if _, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{}); err == nil || !strings.Contains(err.Error(), "constant number value") {
		t.Fatalf("Execute() error = %v, want canonical-number error", err)
	}
}

func TestDataConstantOutputPinTypeFollowsConfig(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		want   domain.DataType
	}{
		{name: "explicit text", config: map[string]any{"type": "text"}, want: domain.DataText},
		{name: "number", config: map[string]any{"type": "number"}, want: domain.DataNumber},
		{name: "boolean", config: map[string]any{"type": "boolean"}, want: domain.DataBoolean},
		{name: "legacy type-less stays any", config: map[string]any{"value": "x"}, want: domain.DataAny},
		{name: "typed legacy value stays any", config: map[string]any{"value": true}, want: domain.DataAny},
	}
	registry := catalog.New()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition, ok := registry.Get("data:constant")
			if !ok {
				t.Fatalf("registry is missing data:constant")
			}
			resolved, err := definitionForNode(definition, v2Node("const", "data:constant", test.config))
			if err != nil {
				t.Fatalf("definitionForNode() error = %v", err)
			}
			value, ok := findOutput(resolved, "value")
			if !ok {
				t.Fatalf("data:constant has no value output")
			}
			if value.DataType != test.want {
				t.Fatalf("value output DataType = %q, want %q", value.DataType, test.want)
			}
		})
	}
}

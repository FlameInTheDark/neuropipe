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
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("const", "data:constant", config),
		cfgNode("store", "flow:set_variable", map[string]any{"name": "Probe"}),
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
		{name: "number", config: map[string]any{"value": 7.0, "type": "number"}, want: map[string]any{"value": 7.0}},
		{name: "boolean true", config: map[string]any{"value": true, "type": "boolean"}, want: map[string]any{"value": true}},
		{name: "boolean false", config: map[string]any{"value": false, "type": "boolean"}, want: map[string]any{"value": false}},
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

func TestDataConstantRejectsNonCanonicalValues(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		want   string
	}{
		{name: "text value must be a string", config: map[string]any{"value": 7.0, "type": "text"}, want: "constant text value"},
		{name: "number value must be numeric", config: map[string]any{"value": "abc", "type": "number"}, want: "constant number value"},
		{name: "boolean value must be a bool", config: map[string]any{"value": "true", "type": "boolean"}, want: "constant Boolean value"},
		{name: "unknown type", config: map[string]any{"value": "x", "type": "colour"}, want: "unknown constant type"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := constantOutput(t, test.config)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Execute() error = %v, want %q failure", err, test.want)
			}
		})
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
	}
	registry := catalog.New()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition, ok := registry.Get("data:constant")
			if !ok {
				t.Fatalf("registry is missing data:constant")
			}
			module, ok := registry.Node("data:constant")
			if !ok {
				t.Fatalf("registry is missing the constant module")
			}
			resolved, err := module.Resolve(cfgNode("const", "data:constant", test.config))
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			value, ok := findOutput(resolved, "value")
			if !ok {
				t.Fatalf("data:constant has no value output")
			}
			if value.DataType != test.want {
				t.Fatalf("value output DataType = %q, want %q", value.DataType, test.want)
			}
			_ = definition
		})
	}
}

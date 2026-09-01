package arrayget

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	module, ok := registry.Get("data:array_get")
	if !ok {
		t.Fatal("data:array_get was not registered")
	}
	return module
}

func invocation(module nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:array_get", Data: map[string]any{"config": map[string]any{}}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:array_get" || definition.Mode != domain.NodePure || definition.Category != "Arrays" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "array" || got.DataType != domain.DataList || got.Direction != domain.PinInput {
		t.Fatalf("array input = %#v", got)
	}
	index := definition.Inputs[1]
	if index.ID != "index" || index.DataType != domain.DataNumber || index.Direction != domain.PinInput {
		t.Fatalf("index input = %#v", index)
	}
	if index.Default != 0.0 {
		t.Fatalf("index default = %#v, want 0", index.Default)
	}
	if got := definition.Outputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinOutput {
		t.Fatalf("value output = %#v", got)
	}
	field := definition.Fields[0]
	if field.Name != "index" || field.Kind != "number" || field.Placeholder != "0" {
		t.Fatalf("index field = %#v", field)
	}
	if !reflect.DeepEqual(definition.DefaultConfig, map[string]any{"index": 0.0}) {
		t.Fatalf("default config = %#v, want index 0", definition.DefaultConfig)
	}
}

func TestEvaluatePicksZeroBasedIndex(t *testing.T) {
	module := registeredModule(t)
	items := []any{"a", map[string]any{"name": "Ada"}, true}
	tests := []struct {
		name  string
		index any
		want  any
	}{
		{"float64 index", 0.0, "a"},
		{"second element", 1.0, map[string]any{"name": "Ada"}},
		{"boolean element", 2.0, true},
		{"int index", 1, map[string]any{"name": "Ada"}},
		{"int64 index", int64(2), true},
		{"float32 index", float32(0), "a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, map[string]any{"array": items, "index": test.index}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs, map[string]any{"value": test.want}) {
				t.Fatalf("outputs = %#v, want value %#v", result.Outputs, test.want)
			}
		})
	}
}

func TestEvaluateRejectsBadInputs(t *testing.T) {
	module := registeredModule(t)
	items := []any{"a", "b"}
	tests := []struct {
		name    string
		inputs  map[string]any
		message string
	}{
		{"missing array", map[string]any{"index": 0.0}, "pick requires an Array list"},
		{"nil array", map[string]any{"array": nil, "index": 0.0}, "pick requires an Array list"},
		{"typed slice array", map[string]any{"array": []string{"a"}, "index": 0.0}, "pick requires an Array list"},
		{"missing index", map[string]any{"array": items}, "pick requires a numeric Index"},
		{"nil index", map[string]any{"array": items, "index": nil}, "pick requires a numeric Index"},
		{"text index", map[string]any{"array": items, "index": "1"}, "pick requires a numeric Index"},
		{"fractional index", map[string]any{"array": items, "index": 1.5}, "pick requires a numeric Index"},
		{"boolean index", map[string]any{"array": items, "index": true}, "pick requires a numeric Index"},
		{"negative index", map[string]any{"array": items, "index": -1.0}, "array index -1 is out of range (length 2)"},
		{"out of range index", map[string]any{"array": items, "index": 5.0}, "array index 5 is out of range (length 2)"},
		{"out of range on empty list", map[string]any{"array": []any{}, "index": 0.0}, "array index 0 is out of range (length 0)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, test.inputs), nil)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Execute() error = %v, want it to contain %q", err, test.message)
			}
		})
	}
}

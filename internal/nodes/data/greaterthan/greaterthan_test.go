package greaterthan

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
	module, ok := registry.Get("data:greater_than")
	if !ok {
		t.Fatal("data:greater_than was not registered")
	}
	return module
}

func invocation(module nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:greater_than", Data: map[string]any{"config": map[string]any{}}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:greater_than" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "left" || got.DataType != domain.DataNumber || got.Direction != domain.PinInput {
		t.Fatalf("left input = %#v", got)
	}
	if got := definition.Inputs[1]; got.ID != "right" || got.DataType != domain.DataNumber || got.Direction != domain.PinInput {
		t.Fatalf("right input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "value" || got.DataType != domain.DataBoolean || got.Direction != domain.PinOutput {
		t.Fatalf("value output = %#v", got)
	}
	if !reflect.DeepEqual(definition.DefaultConfig, map[string]any{}) {
		t.Fatalf("default config = %#v, want empty", definition.DefaultConfig)
	}
}

func TestEvaluateComparesNumbers(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name        string
		left, right any
		want        bool
	}{
		{"float greater", 2.0, 1.0, true},
		{"float smaller", 1.0, 2.0, false},
		{"equal values are not greater", 2.0, 2.0, false},
		{"int left beats float right", 3, 2.5, true},
		{"int64 left loses to float right", int64(1), 2.0, false},
		{"float32 left wins", float32(2.5), 2.0, true},
		{"negative comparison", -1.0, -2.0, true},
		{"fractional boundary", 1.5, 1.5, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, map[string]any{"left": test.left, "right": test.right}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs, map[string]any{"value": test.want}) {
				t.Fatalf("outputs = %#v, want value %v", result.Outputs, test.want)
			}
		})
	}
}

func TestEvaluateRequiresNumericInputs(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name   string
		inputs map[string]any
	}{
		{"missing left", map[string]any{"right": 1.0}},
		{"missing right", map[string]any{"left": 1.0}},
		{"nil left", map[string]any{"left": nil, "right": 1.0}},
		{"text left", map[string]any{"left": "2", "right": 1.0}},
		{"text right", map[string]any{"left": 2.0, "right": "1"}},
		{"boolean left", map[string]any{"left": true, "right": 1.0}},
		{"list right", map[string]any{"left": 2.0, "right": []any{1.0}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, test.inputs), nil)
			if err == nil || !strings.Contains(err.Error(), "greater than requires numeric inputs") {
				t.Fatalf("Execute() error = %v, want a numeric-input requirement", err)
			}
		})
	}
}

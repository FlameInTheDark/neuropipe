package length

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
	module, ok := registry.Get("data:length")
	if !ok {
		t.Fatal("data:length was not registered")
	}
	return module
}

func invocation(module nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:length", Data: map[string]any{"config": map[string]any{}}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:length" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinInput {
		t.Fatalf("value input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "length" || got.DataType != domain.DataNumber || got.Direction != domain.PinOutput {
		t.Fatalf("length output = %#v", got)
	}
	if len(definition.Fields) != 0 {
		t.Fatalf("fields = %#v, want none", definition.Fields)
	}
}

func TestEvaluateCountsSupportedValues(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name  string
		value any
		want  float64
	}{
		{"list elements", []any{1.0, "a", true}, 3.0},
		{"empty list", []any{}, 0.0},
		{"ascii text characters", "neuro", 5.0},
		{"utf8 text characters", "héllo", 5.0},
		{"empty text", "", 0.0},
		{"object keys", map[string]any{"a": 1.0, "b": 2.0}, 2.0},
		{"typed string map keys", map[string]int{"a": 1, "b": 2, "c": 3}, 3.0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, map[string]any{"value": test.value}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs, map[string]any{"length": test.want}) {
				t.Fatalf("outputs = %#v, want length %v", result.Outputs, test.want)
			}
		})
	}
}

func TestEvaluateDereferencesPointers(t *testing.T) {
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"value": &map[string]any{"a": 1.0}}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(result.Outputs, map[string]any{"length": 1.0}) {
		t.Fatalf("outputs = %#v, want the dereferenced map length", result.Outputs)
	}
}

func TestEvaluateRejectsUnsupportedValues(t *testing.T) {
	module := registeredModule(t)
	structValue := struct{ Name string }{Name: "Ada"}
	for _, value := range []any{42.0, true, nil, structValue, []string{"a"}, []byte("ab"), &structValue} {
		_, err := module.Execute(context.Background(), invocation(module, map[string]any{"value": value}), nil)
		if err == nil || !strings.Contains(err.Error(), "length requires a list, text, or object value") {
			t.Fatalf("Execute(%#v) error = %v, want the unsupported-value failure", value, err)
		}
	}
}

func TestEvaluateMissingInputFails(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module, map[string]any{}), nil)
	if err == nil || !strings.Contains(err.Error(), "length requires a list, text, or object value, received <nil>") {
		t.Fatalf("Execute() error = %v, want the <nil> report", err)
	}
}

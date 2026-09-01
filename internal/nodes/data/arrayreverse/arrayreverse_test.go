package arrayreverse

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
	module, ok := registry.Get("data:array_reverse")
	if !ok {
		t.Fatal("data:array_reverse was not registered")
	}
	return module
}

func invocation(module nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:array_reverse", Data: map[string]any{"config": map[string]any{}}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:array_reverse" || definition.Mode != domain.NodePure || definition.Category != "Arrays" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "array" || got.DataType != domain.DataList || got.Direction != domain.PinInput {
		t.Fatalf("array input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "array" || got.DataType != domain.DataList || got.Direction != domain.PinOutput {
		t.Fatalf("array output = %#v", got)
	}
	if len(definition.Fields) != 0 {
		t.Fatalf("fields = %#v, want none", definition.Fields)
	}
}

func TestEvaluateReversesList(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name  string
		array []any
		want  []any
	}{
		{"text", []any{"a", "b", "c"}, []any{"c", "b", "a"}},
		{"numbers", []any{1.0, 2.0}, []any{2.0, 1.0}},
		{"single element", []any{"only"}, []any{"only"}},
		{"empty list", []any{}, []any{}},
		{"nested elements", []any{1.0, []any{"x", "y"}}, []any{[]any{"x", "y"}, 1.0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, map[string]any{"array": test.array}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			want := map[string]any{"array": test.want}
			if !reflect.DeepEqual(result.Outputs, want) {
				t.Fatalf("outputs = %#v, want %#v", result.Outputs, want)
			}
		})
	}
}

func TestEvaluateLeavesInputUnchanged(t *testing.T) {
	module := registeredModule(t)
	original := []any{"a", "b", "c"}
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"array": original}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(original, []any{"a", "b", "c"}) {
		t.Fatalf("input list mutated to %#v", original)
	}
	if got := result.Outputs["array"]; !reflect.DeepEqual(got, []any{"c", "b", "a"}) {
		t.Fatalf("outputs = %#v, want the reversed copy", result.Outputs)
	}
}

func TestEvaluateRequiresArrayList(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name   string
		inputs map[string]any
	}{
		{"missing array", map[string]any{}},
		{"nil array", map[string]any{"array": nil}},
		{"text array", map[string]any{"array": "abc"}},
		{"object array", map[string]any{"array": map[string]any{"a": 1.0}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, test.inputs), nil)
			if err == nil || !strings.Contains(err.Error(), "reverse array requires an Array list") {
				t.Fatalf("Execute() error = %v, want an Array list requirement", err)
			}
		})
	}
}

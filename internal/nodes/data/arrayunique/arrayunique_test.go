package arrayunique

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
	module, ok := registry.Get("data:array_unique")
	if !ok {
		t.Fatal("data:array_unique was not registered")
	}
	return module
}

func invocation(module nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:array_unique", Data: map[string]any{"config": map[string]any{}}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:array_unique" || definition.Mode != domain.NodePure || definition.Category != "Arrays" {
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

func TestEvaluateRemovesDuplicates(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name  string
		array []any
		want  []any
	}{
		{"text duplicates", []any{"a", "b", "a", "c", "b"}, []any{"a", "b", "c"}},
		{"number duplicates", []any{1.0, 2.0, 1.0, 1.0}, []any{1.0, 2.0}},
		{"mixed scalars", []any{1.0, "1", true, 1.0, "1"}, []any{1.0, "1", true}},
		{"number kinds collapse", []any{1.0, 2.0, 1}, []any{1.0, 2.0}},
		{"duplicate objects", []any{map[string]any{"a": 1.0}, map[string]any{"a": 1.0}, map[string]any{"a": 2.0}}, []any{map[string]any{"a": 1.0}, map[string]any{"a": 2.0}}},
		{"duplicate nested lists", []any{[]any{"x"}, []any{"x"}, []any{"y"}}, []any{[]any{"x"}, []any{"y"}}},
		{"nulls deduplicate", []any{nil, nil}, []any{nil}},
		{"no duplicates unchanged", []any{"a", "b"}, []any{"a", "b"}},
		{"empty list", []any{}, []any{}},
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

func TestEvaluateKeepsFirstOccurrenceAndOrder(t *testing.T) {
	module := registeredModule(t)
	original := []any{"late", "first", "first", "late", "only"}
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"array": original}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["array"]; !reflect.DeepEqual(got, []any{"late", "first", "only"}) {
		t.Fatalf("outputs = %#v, want first occurrences in arrival order", result.Outputs)
	}
	if !reflect.DeepEqual(original, []any{"late", "first", "first", "late", "only"}) {
		t.Fatalf("input list mutated to %#v", original)
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
			if err == nil || !strings.Contains(err.Error(), "unique array requires an Array list") {
				t.Fatalf("Execute() error = %v, want an Array list requirement", err)
			}
		})
	}
}

package arraysort

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
	module, ok := registry.Get("data:array_sort")
	if !ok {
		t.Fatal("data:array_sort was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	if config == nil {
		config = map[string]any{}
	}
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:array_sort", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:array_sort" || definition.Mode != domain.NodePure || definition.Category != "Arrays" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "array" || got.DataType != domain.DataList || got.Direction != domain.PinInput {
		t.Fatalf("array input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "array" || got.DataType != domain.DataList || got.Direction != domain.PinOutput {
		t.Fatalf("array output = %#v", got)
	}
	if len(definition.Outputs) != 1 {
		t.Fatalf("outputs = %#v, want exactly the array pin", definition.Outputs)
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Name != "order" || definition.Fields[0].Kind != "select" {
		t.Fatalf("fields = %#v, want the single order select", definition.Fields)
	}
	if got := definition.DefaultConfig["order"]; got != "ascending" {
		t.Fatalf("default order = %#v, want ascending", got)
	}
}

func TestEvaluateSortsAscendingByDefault(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name  string
		array []any
		want  []any
	}{
		{"numbers", []any{3.0, 1.0, 2.0}, []any{1.0, 2.0, 3.0}},
		{"text", []any{"pear", "apple", "fig"}, []any{"apple", "fig", "pear"}},
		{"booleans", []any{true, false, true}, []any{false, true, true}},
		{"mixed scalars rank numbers text booleans", []any{"b", true, 2.0, "a", false, 1.0}, []any{1.0, 2.0, "a", "b", false, true}},
		{"empty list", []any{}, []any{}},
		{"single element", []any{"only"}, []any{"only"}},
		{"already sorted", []any{1.0, 2.0, 3.0}, []any{1.0, 2.0, 3.0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"array": test.array}), nil)
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

func TestEvaluateSortsDescending(t *testing.T) {
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"order": "descending"}, map[string]any{"array": []any{1.0, 3.0, 2.0}}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["array"]; !reflect.DeepEqual(got, []any{3.0, 2.0, 1.0}) {
		t.Fatalf("outputs = %#v, want descending order", result.Outputs)
	}
}

func TestEvaluateKeepsEqualElementsStable(t *testing.T) {
	module := registeredModule(t)
	// Equal numbers keep their arrival order, which distinguishes a stable
	// sort from a plain one on this shuffled input.
	array := []any{2.0, 1.0, 2.0, 1.0, 2.0}
	result, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"array": array}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["array"]; !reflect.DeepEqual(got, []any{1.0, 1.0, 2.0, 2.0, 2.0}) {
		t.Fatalf("outputs = %#v, want a stable ascending sort", result.Outputs)
	}
	if !reflect.DeepEqual(array, []any{2.0, 1.0, 2.0, 1.0, 2.0}) {
		t.Fatalf("input list mutated to %#v", array)
	}
}

func TestEvaluateRejectsUnorderableElements(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name  string
		array []any
		want  string
	}{
		{"object element", []any{map[string]any{"a": 1.0}}, "object"},
		{"nested list element", []any{1.0, []any{"x"}}, "list"},
		{"null element", []any{"a", nil}, "null"},
		{"bytes element", []any{"a", []byte("b")}, "bytes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"array": test.array}), nil)
			if err == nil || !strings.Contains(err.Error(), "sort array cannot order "+test.want+" elements") {
				t.Fatalf("Execute() error = %v, want an orderable failure naming %s", err, test.want)
			}
		})
	}
}

func TestEvaluateRejectsNonListAndUnknownOrder(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"array": "abc"}), nil)
	if err == nil || !strings.Contains(err.Error(), "sort array requires an Array list") {
		t.Fatalf("Execute() error = %v, want an Array list requirement", err)
	}
	_, err = module.Execute(context.Background(), invocation(module, map[string]any{"order": "sideways"}, map[string]any{"array": []any{1.0}}), nil)
	if err == nil || !strings.Contains(err.Error(), "not ascending or descending") {
		t.Fatalf("Execute() error = %v, want an order failure", err)
	}
}

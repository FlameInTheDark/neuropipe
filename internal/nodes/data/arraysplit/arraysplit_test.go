package arraysplit

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
	module, ok := registry.Get("data:array_split")
	if !ok {
		t.Fatal("data:array_split was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	if config == nil {
		config = map[string]any{}
	}
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:array_split", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:array_split" || definition.Mode != domain.NodePure || definition.Category != "Arrays" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "array" || got.DataType != domain.DataList || got.Direction != domain.PinInput {
		t.Fatalf("array input = %#v", got)
	}
	if got := definition.Inputs[1]; got.ID != "size" || got.DataType != domain.DataNumber || got.Direction != domain.PinInput {
		t.Fatalf("size input = %#v", got)
	}
	if got := definition.Inputs[1].Default; got != 10.0 {
		t.Fatalf("size default = %#v, want ten", got)
	}
	if got := definition.Outputs[0]; got.ID != "arrays" || got.DataType != domain.DataList || got.Direction != domain.PinOutput {
		t.Fatalf("arrays output = %#v", got)
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Name != "size" || definition.Fields[0].Kind != "number" {
		t.Fatalf("fields = %#v, want the single size number field", definition.Fields)
	}
	if got := definition.DefaultConfig["size"]; got != 10.0 {
		t.Fatalf("default size = %#v, want ten", got)
	}
}

func TestEvaluateSplitsIntoBatches(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name  string
		array []any
		size  any
		want  []any
	}{
		{"even batches", []any{1.0, 2.0, 3.0, 4.0}, 2.0, []any{[]any{1.0, 2.0}, []any{3.0, 4.0}}},
		{"short final batch", []any{"a", "b", "c", "d", "e"}, 2.0, []any{[]any{"a", "b"}, []any{"c", "d"}, []any{"e"}}},
		{"size larger than list", []any{1.0, 2.0}, 5.0, []any{[]any{1.0, 2.0}}},
		{"size of one", []any{1.0, 2.0}, 1.0, []any{[]any{1.0}, []any{2.0}}},
		{"empty list", []any{}, 3.0, []any{}},
		{"integer input kind", []any{1.0, 2.0, 3.0}, 3, []any{[]any{1.0, 2.0, 3.0}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"array": test.array, "size": test.size}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			want := map[string]any{"arrays": test.want}
			if !reflect.DeepEqual(result.Outputs, want) {
				t.Fatalf("outputs = %#v, want %#v", result.Outputs, want)
			}
		})
	}
}

func TestEvaluateUsesConfiguredSizeWithoutWire(t *testing.T) {
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"size": 2.0}, map[string]any{"array": []any{1.0, 2.0, 3.0}}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["arrays"]; !reflect.DeepEqual(got, []any{[]any{1.0, 2.0}, []any{3.0}}) {
		t.Fatalf("outputs = %#v, want batches of two from the config size", result.Outputs)
	}
}

func TestEvaluateWireOverridesConfiguredSize(t *testing.T) {
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"size": 5.0}, map[string]any{"array": []any{1.0, 2.0, 3.0}, "size": 1.0}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["arrays"]; !reflect.DeepEqual(got, []any{[]any{1.0}, []any{2.0}, []any{3.0}}) {
		t.Fatalf("outputs = %#v, want the wired size to win", result.Outputs)
	}
}

func TestEvaluateDefaultsToTenWithoutSize(t *testing.T) {
	module := registeredModule(t)
	array := []any{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0, 11.0, 12.0}
	result, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"array": array}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["arrays"]; !reflect.DeepEqual(got, []any{array[:10], []any{11.0, 12.0}}) {
		t.Fatalf("outputs = %#v, want a ten-element batch plus the remainder", result.Outputs)
	}
}

func TestEvaluateRejectsBadSize(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name string
		size any
		want string
	}{
		{"zero size", 0.0, "at least one element"},
		{"negative size", -2.0, "at least one element"},
		{"fractional size", 1.5, "whole-number Size"},
		{"text size", "two", "numeric Size"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"array": []any{1.0, 2.0}, "size": test.size}), nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Execute() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestEvaluateRequiresArrayList(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name   string
		inputs map[string]any
	}{
		{"missing array", map[string]any{"size": 2.0}},
		{"nil array", map[string]any{"array": nil, "size": 2.0}},
		{"text array", map[string]any{"array": "abc", "size": 2.0}},
		{"typed slice", map[string]any{"array": []string{"a"}, "size": 2.0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, nil, test.inputs), nil)
			if err == nil || !strings.Contains(err.Error(), "split array requires an Array list") {
				t.Fatalf("Execute() error = %v, want an Array list requirement", err)
			}
		})
	}
}

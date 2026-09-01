package arrayappend

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
	module, ok := registry.Get("data:array_append")
	if !ok {
		t.Fatal("data:array_append was not registered")
	}
	return module
}

func invocation(module nodes.Node, inputs map[string]any) nodes.Invocation {
	return invocationWithConfig(module, map[string]any{}, inputs)
}

func invocationWithConfig(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:array_append", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:array_append" || definition.Mode != domain.NodePure || definition.Category != "Arrays" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "array" || got.DataType != domain.DataList || got.Direction != domain.PinInput {
		t.Fatalf("array input = %#v", got)
	}
	if got := definition.Inputs[1]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinInput {
		t.Fatalf("value input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "array" || got.DataType != domain.DataList || got.Direction != domain.PinOutput {
		t.Fatalf("array output = %#v", got)
	}
	if len(definition.Outputs) != 1 {
		t.Fatalf("outputs = %#v, want exactly the array pin", definition.Outputs)
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Name != "mode" || definition.Fields[0].Kind != "select" {
		t.Fatalf("fields = %#v, want the single mode select", definition.Fields)
	}
	if len(definition.Fields[0].Options) != 2 || definition.Fields[0].Options[0].Value != "item" || definition.Fields[0].Options[1].Value != "array" {
		t.Fatalf("mode options = %#v", definition.Fields[0].Options)
	}
	if got := definition.DefaultConfig["mode"]; got != "item" {
		t.Fatalf("default mode = %#v, want item", got)
	}
}

func TestEvaluateAppendsValue(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name  string
		array []any
		value any
		want  []any
	}{
		{"text to text list", []any{"a", "b"}, "c", []any{"a", "b", "c"}},
		{"number to empty list", []any{}, 1.0, []any{1.0}},
		{"nil value is appended", []any{"a"}, nil, []any{"a", nil}},
		{"object value", []any{}, map[string]any{"ok": true}, []any{map[string]any{"ok": true}}},
		{"nested list value", []any{1.0}, []any{"x"}, []any{1.0, []any{"x"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, map[string]any{"array": test.array, "value": test.value}), nil)
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

func TestEvaluateConcatenatesInArrayMode(t *testing.T) {
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocationWithConfig(module,
		map[string]any{"mode": "array"},
		map[string]any{"array": []any{"a", "b"}, "value": []any{"c", 1.0}}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["array"]; !reflect.DeepEqual(got, []any{"a", "b", "c", 1.0}) {
		t.Fatalf("outputs = %#v, want the concatenated four items", result.Outputs)
	}
}

func TestEvaluateConcatenationKeepsInputsUnchanged(t *testing.T) {
	module := registeredModule(t)
	base := []any{"a"}
	extra := []any{"b", "c"}
	result, err := module.Execute(context.Background(), invocationWithConfig(module,
		map[string]any{"mode": "array"},
		map[string]any{"array": base, "value": extra}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(base, []any{"a"}) || !reflect.DeepEqual(extra, []any{"b", "c"}) {
		t.Fatalf("inputs mutated: base %#v extra %#v", base, extra)
	}
	if got := result.Outputs["array"]; !reflect.DeepEqual(got, []any{"a", "b", "c"}) {
		t.Fatalf("outputs = %#v, want the concatenated three items", result.Outputs)
	}
}

func TestEvaluateArrayModeRequiresListValue(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name  string
		value any
	}{
		{"text value", "c"},
		{"missing value", nil},
		{"object value", map[string]any{"a": 1.0}},
		{"typed slice value", []string{"c"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocationWithConfig(module,
				map[string]any{"mode": "array"},
				map[string]any{"array": []any{"a"}, "value": test.value}), nil)
			if err == nil || !strings.Contains(err.Error(), "Value input to be an Array list") {
				t.Fatalf("Execute() error = %v, want a list Value requirement", err)
			}
		})
	}
}

func TestEvaluateRejectsUnknownMode(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocationWithConfig(module,
		map[string]any{"mode": "both"},
		map[string]any{"array": []any{"a"}, "value": "b"}), nil)
	if err == nil || !strings.Contains(err.Error(), "Single item, Array elements") {
		t.Fatalf("Execute() error = %v, want an unknown-mode failure", err)
	}
}

func TestEvaluateLeavesInputListUnchanged(t *testing.T) {
	module := registeredModule(t)
	original := []any{"a", "b"}
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"array": original, "value": "c"}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(original, []any{"a", "b"}) {
		t.Fatalf("input list mutated to %#v, want the original two items", original)
	}
	if got := result.Outputs["array"]; !reflect.DeepEqual(got, []any{"a", "b", "c"}) {
		t.Fatalf("outputs = %#v, want the three-item copy", result.Outputs)
	}
}

func TestEvaluateRequiresArrayList(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name   string
		inputs map[string]any
	}{
		{"missing array", map[string]any{"value": "c"}},
		{"nil array", map[string]any{"array": nil, "value": "c"}},
		{"text array", map[string]any{"array": "abc", "value": "c"}},
		{"typed string slice", map[string]any{"array": []string{"a"}, "value": "c"}},
		{"object array", map[string]any{"array": map[string]any{"a": 1.0}, "value": "c"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, test.inputs), nil)
			if err == nil || !strings.Contains(err.Error(), "append to array requires an Array list") {
				t.Fatalf("Execute() error = %v, want an Array list requirement", err)
			}
		})
	}
}

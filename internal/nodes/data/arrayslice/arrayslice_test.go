package arrayslice

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
	module, ok := registry.Get("data:array_slice")
	if !ok {
		t.Fatal("data:array_slice was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	if config == nil {
		config = map[string]any{}
	}
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:array_slice", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:array_slice" || definition.Mode != domain.NodePure || definition.Category != "Arrays" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "array" || got.DataType != domain.DataList || got.Direction != domain.PinInput {
		t.Fatalf("array input = %#v", got)
	}
	if got := definition.Inputs[1]; got.ID != "start" || got.DataType != domain.DataNumber || got.Direction != domain.PinInput {
		t.Fatalf("start input = %#v", got)
	}
	if got := definition.Inputs[2]; got.ID != "count" || got.DataType != domain.DataNumber || got.Direction != domain.PinInput {
		t.Fatalf("count input = %#v", got)
	}
	if got := definition.Inputs[1].Default; got != 0.0 {
		t.Fatalf("start default = %#v, want zero", got)
	}
	if got := definition.Outputs[0]; got.ID != "array" || got.DataType != domain.DataList || got.Direction != domain.PinOutput {
		t.Fatalf("array output = %#v", got)
	}
	if len(definition.Fields) != 2 || definition.Fields[0].Name != "start" || definition.Fields[1].Name != "count" {
		t.Fatalf("fields = %#v, want start and count", definition.Fields)
	}
}

func TestEvaluateSlicesList(t *testing.T) {
	module := registeredModule(t)
	source := []any{"a", "b", "c", "d", "e"}
	tests := []struct {
		name  string
		start any
		count any
		want  []any
	}{
		{"middle section", 1.0, 2.0, []any{"b", "c"}},
		{"no count runs to the end", 3.0, nil, []any{"d", "e"}},
		{"start zero takes the head", 0.0, 2.0, []any{"a", "b"}},
		{"start zero without count copies all", 0.0, nil, []any{"a", "b", "c", "d", "e"}},
		{"count past the end clamps", 4.0, 10.0, []any{"e"}},
		{"start past the end is empty", 9.0, 2.0, []any{}},
		{"count of zero is empty", 1.0, 0.0, []any{}},
		{"integer input kinds", 1, 2, []any{"b", "c"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"array": source, "start": test.start, "count": test.count}), nil)
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

func TestEvaluateUsesConfiguredSettingsWithoutWires(t *testing.T) {
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"start": 1.0, "count": 2.0}, map[string]any{"array": []any{"a", "b", "c", "d"}}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["array"]; !reflect.DeepEqual(got, []any{"b", "c"}) {
		t.Fatalf("outputs = %#v, want the configured section", result.Outputs)
	}
}

func TestEvaluateDefaultsToWholeList(t *testing.T) {
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"array": []any{"a", "b"}}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["array"]; !reflect.DeepEqual(got, []any{"a", "b"}) {
		t.Fatalf("outputs = %#v, want the whole list with the zero start default", result.Outputs)
	}
}

func TestEvaluateRejectsBadSettings(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name   string
		inputs map[string]any
		want   string
	}{
		{"negative start", map[string]any{"array": []any{"a"}, "start": -1.0}, "Start of zero or more"},
		{"fractional start", map[string]any{"array": []any{"a"}, "start": 0.5}, "whole-number start"},
		{"negative count", map[string]any{"array": []any{"a"}, "start": 0.0, "count": -1.0}, "Count of zero or more"},
		{"fractional count", map[string]any{"array": []any{"a"}, "start": 0.0, "count": 1.5}, "whole-number count"},
		{"text start", map[string]any{"array": []any{"a"}, "start": "one"}, "numeric start"},
		{"text count", map[string]any{"array": []any{"a"}, "count": "two"}, "numeric count"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, nil, test.inputs), nil)
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
		{"missing array", map[string]any{"start": 0.0}},
		{"nil array", map[string]any{"array": nil, "start": 0.0}},
		{"text array", map[string]any{"array": "abc", "start": 0.0}},
		{"typed slice", map[string]any{"array": []string{"a"}, "start": 0.0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, nil, test.inputs), nil)
			if err == nil || !strings.Contains(err.Error(), "slice array requires an Array list") {
				t.Fatalf("Execute() error = %v, want an Array list requirement", err)
			}
		})
	}
}

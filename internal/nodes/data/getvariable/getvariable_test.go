package getvariable

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type variableRuntime struct{ vars map[string]any }

func (r variableRuntime) LookupVariable(name string) (any, bool) {
	value, ok := r.vars[name]
	return value, ok
}

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	module, ok := registry.Get("data:get_variable")
	if !ok {
		t.Fatal("data:get_variable was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:get_variable", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          config,
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:get_variable" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if len(definition.Inputs) != 0 {
		t.Fatalf("inputs = %#v, want none", definition.Inputs)
	}
	if got := definition.Outputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinOutput {
		t.Fatalf("value output = %#v", got)
	}
	field := definition.Fields[0]
	if field.Name != "name" || field.Kind != "string" || !field.Required || field.Placeholder != "Result" {
		t.Fatalf("name field = %#v", field)
	}
}

func TestEvaluateReadsExecutionVariable(t *testing.T) {
	module := registeredModule(t)
	runtime := variableRuntime{vars: map[string]any{
		"Result":  "done",
		"payload": map[string]any{"name": "Ada"},
		"items":   []any{"a", "b"},
		"count":   42.0,
	}}
	tests := []struct {
		name     string
		variable string
		value    any
	}{
		{"text value", "Result", "done"},
		{"object value", "payload", map[string]any{"name": "Ada"}},
		{"list value", "items", []any{"a", "b"}},
		{"number value", "count", 42.0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, map[string]any{"name": test.variable}), runtime)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs, map[string]any{"value": test.value}) {
				t.Fatalf("outputs = %#v, want value %#v", result.Outputs, test.value)
			}
		})
	}
}

func TestEvaluateTrimsConfiguredName(t *testing.T) {
	module := registeredModule(t)
	runtime := variableRuntime{vars: map[string]any{"Result": "done"}}
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"name": "  Result "}), runtime)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["value"] != "done" {
		t.Fatalf("outputs = %#v, want the trimmed-name lookup", result.Outputs)
	}
}

func TestEvaluateFailsWhenVariableWasNeverSet(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module, map[string]any{"name": "missing"}), variableRuntime{vars: map[string]any{}})
	if err == nil || !strings.Contains(err.Error(), `variable "missing" has not been set in this execution`) {
		t.Fatalf("Execute() error = %v, want the unset-variable failure", err)
	}
}

func TestEvaluateFailsOnEmptyName(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module, map[string]any{}), variableRuntime{vars: map[string]any{}})
	if err == nil || !strings.Contains(err.Error(), `variable "" has not been set in this execution`) {
		t.Fatalf("Execute() error = %v, want the empty-name lookup failure", err)
	}
}

func TestEvaluateRequiresVariableRuntime(t *testing.T) {
	module := registeredModule(t)
	invocation := invocation(module, map[string]any{"name": "Result"})
	_, err := module.Execute(context.Background(), invocation, nil)
	if err == nil || !strings.Contains(err.Error(), "get variable runtime is unavailable") {
		t.Fatalf("Execute(nil runtime) error = %v, want the unavailable-runtime failure", err)
	}
	_, err = module.Execute(context.Background(), invocation, struct{}{})
	if err == nil || !strings.Contains(err.Error(), "get variable runtime is unavailable") {
		t.Fatalf("Execute(unrelated runtime) error = %v, want the unavailable-runtime failure", err)
	}
}

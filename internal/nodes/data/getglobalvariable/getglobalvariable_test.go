package getglobalvariable

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type globalsRuntime struct{ vars map[string]any }

func (r globalsRuntime) ReadGlobalVariable(name string) (any, bool) {
	value, ok := r.vars[name]
	return value, ok
}

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	module, ok := registry.Get("data:get_global_variable")
	if !ok {
		t.Fatal("data:get_global_variable was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:get_global_variable", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          config,
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:get_global_variable" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if len(definition.Inputs) != 0 {
		t.Fatalf("inputs = %#v, want none", definition.Inputs)
	}
	if got := definition.Outputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinOutput {
		t.Fatalf("value output = %#v", got)
	}
	field := definition.Fields[0]
	if field.Name != "name" || field.Kind != "select" || !field.Required {
		t.Fatalf("name field = %#v", field)
	}
	if got, want := definition.DefaultConfig["name"], ""; got != want {
		t.Fatalf("default name = %#v, want %q", got, want)
	}
}

func TestEvaluateReadsDeclaredVariable(t *testing.T) {
	module := registeredModule(t)
	runtime := globalsRuntime{vars: map[string]any{"apiToken": "secret", "limit": 42.0}}
	tests := []struct {
		name     string
		variable string
		value    any
	}{
		{"text value", "apiToken", "secret"},
		{"number value", "limit", 42.0},
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
	runtime := globalsRuntime{vars: map[string]any{"apiToken": "secret"}}
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"name": "  apiToken\t"}), runtime)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["value"] != "secret" {
		t.Fatalf("outputs = %#v, want the trimmed-name lookup", result.Outputs)
	}
}

func TestEvaluateRequiresSelectedVariable(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module, map[string]any{"name": "  "}), globalsRuntime{})
	if err == nil || !strings.Contains(err.Error(), "select a variable to read") {
		t.Fatalf("Execute() error = %v, want select a variable to read", err)
	}
}

func TestEvaluateFailsOnDeletedVariable(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module, map[string]any{"name": "gone"}), globalsRuntime{vars: map[string]any{}})
	if err == nil || !strings.Contains(err.Error(), `global variable "gone" is not declared or has been deleted`) {
		t.Fatalf("Execute() error = %v, want the deleted-variable failure", err)
	}
}

func TestEvaluateRequiresGlobalVariableRuntime(t *testing.T) {
	module := registeredModule(t)
	invocation := invocation(module, map[string]any{"name": "apiToken"})
	_, err := module.Execute(context.Background(), invocation, nil)
	if err == nil || !strings.Contains(err.Error(), "global variable runtime is unavailable") {
		t.Fatalf("Execute(nil runtime) error = %v, want the unavailable-runtime failure", err)
	}
	_, err = module.Execute(context.Background(), invocation, struct{}{})
	if err == nil || !strings.Contains(err.Error(), "global variable runtime is unavailable") {
		t.Fatalf("Execute(unrelated runtime) error = %v, want the unavailable-runtime failure", err)
	}
}

func TestResolveWithoutHooksKeepsGenericPin(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": "counter"}}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	output := resolved.Outputs[0]
	if output.DataType != domain.DataAny || output.Type == nil || output.Type.Kind != domain.TypeAny {
		t.Fatalf("unresolved output = %#v, want the generic any pin", output)
	}
	if options := resolved.Fields[0].Options; len(options) != 0 {
		t.Fatalf("name field options = %#v, want none without declarations", options)
	}
}

func TestResolveRewritesOutputToDeclaredType(t *testing.T) {
	defer SetDeclaredType(nil)
	SetDeclaredType(func(name string) (domain.DataType, bool) {
		if name == "counter" {
			return domain.DataNumber, true
		}
		return "", false
	})
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": "counter"}}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	output := resolved.Outputs[0]
	if output.DataType != domain.DataNumber || output.Type == nil || output.Type.Kind != domain.TypeFloat {
		t.Fatalf("declared output = %#v, want a number/float pin", output)
	}
	// A name the host cannot declare keeps the generic Any pin.
	resolved, err = module.Resolve(domain.FlowNode{Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": "unknown"}}})
	if err != nil {
		t.Fatalf("Resolve(unknown) error = %v", err)
	}
	if output := resolved.Outputs[0]; output.DataType != domain.DataAny {
		t.Fatalf("unknown output = %#v, want the generic any pin", output)
	}
}

func TestResolveKeepsAnyPinWithoutName(t *testing.T) {
	defer SetDeclaredType(nil)
	SetDeclaredType(func(name string) (domain.DataType, bool) {
		return domain.DataText, true
	})
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": ""}}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if output := resolved.Outputs[0]; output.DataType != domain.DataAny {
		t.Fatalf("unnamed output = %#v, want the generic any pin", output)
	}
}

func TestResolveInjectsDeclaredNameOptions(t *testing.T) {
	defer SetDeclaredOptions(nil)
	SetDeclaredOptions(func() []domain.Option {
		return []domain.Option{{Value: "counter", Label: "Counter"}}
	})
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": "counter"}}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	options := resolved.Fields[0].Options
	if len(options) != 1 || options[0].Value != "counter" || options[0].Label != "Counter" {
		t.Fatalf("name field options = %#v, want the declared picklist", options)
	}
}

package setvariable_test

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/setvariable"
)

type variableStoreStub struct {
	stored map[string]any
}

func (s *variableStoreStub) StoreVariable(name string, value any) {
	if s.stored == nil {
		s.stored = map[string]any{}
	}
	s.stored[name] = value
}

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := setvariable.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:set_variable")
	if !ok {
		t.Fatal("flow:set_variable was not registered")
	}
	return registered
}

func invocation(registered nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "set-1", Type: "flow:set_variable", Data: map[string]any{"config": config}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:set_variable" || definition.Mode != domain.NodeImpure || definition.Category != "Data" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != "value" {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if len(definition.Outputs) != 2 || definition.Outputs[0].ID != "out" || definition.Outputs[1].ID != "result" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Name != "name" || !definition.Fields[0].Required {
		t.Fatalf("fields = %#v", definition.Fields)
	}
}

func TestExecuteStoresValueAndRoutesOut(t *testing.T) {
	registered := module(t)
	store := &variableStoreStub{}
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{"name": "answer"}, map[string]any{"value": float64(42)}), store)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if store.stored["answer"] != float64(42) {
		t.Fatalf("stored = %#v", store.stored)
	}
	if result.Outputs["result"] != float64(42) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
}

func TestExecuteAcceptsIdentifierNames(t *testing.T) {
	names := []string{"Result", "_private", "a1_b", "Zz9"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			registered := module(t)
			store := &variableStoreStub{}
			if _, err := registered.Execute(context.Background(), invocation(registered, map[string]any{"name": name}, map[string]any{"value": "v"}), store); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if _, stored := store.stored[name]; !stored {
				t.Fatalf("variable %q was not stored: %#v", name, store.stored)
			}
		})
	}
}

func TestExecuteRejectsInvalidNames(t *testing.T) {
	names := []string{"", "   ", "1abc", "has space", "with-dash", "dot.name"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			registered := module(t)
			store := &variableStoreStub{}
			_, err := registered.Execute(context.Background(), invocation(registered, map[string]any{"name": name}, map[string]any{"value": "v"}), store)
			if err == nil {
				t.Fatalf("Execute() accepted invalid variable name %q", name)
			}
			if len(store.stored) != 0 {
				t.Fatalf("nothing must be stored for %q: %#v", name, store.stored)
			}
		})
	}
}

func TestExecuteTrimsSurroundingWhitespace(t *testing.T) {
	registered := module(t)
	store := &variableStoreStub{}
	if _, err := registered.Execute(context.Background(), invocation(registered, map[string]any{"name": "  answer  "}, map[string]any{"value": 1}), store); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, stored := store.stored["answer"]; !stored {
		t.Fatalf("stored = %#v, want trimmed name", store.stored)
	}
}

func TestExecuteStoresNilValue(t *testing.T) {
	registered := module(t)
	store := &variableStoreStub{}
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{"name": "empty"}, map[string]any{}), store)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	value, stored := store.stored["empty"]
	if !stored || value != nil {
		t.Fatalf("stored = %#v", store.stored)
	}
	if result.Outputs["result"] != nil {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestExecuteRequiresVariableWriter(t *testing.T) {
	registered := module(t)
	if _, err := registered.Execute(context.Background(), invocation(registered, map[string]any{"name": "answer"}, map[string]any{}), nil); err == nil {
		t.Fatal("Execute() accepted a runtime without a variable writer")
	}
}

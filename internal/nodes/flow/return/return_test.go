package returnnode_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	returnnode "github.com/FlameInTheDark/neuropipe/internal/nodes/flow/return"
)

type returnSignalerStub struct {
	values  map[string]any
	signals int
}

func (s *returnSignalerStub) Return(values map[string]any) {
	s.signals++
	s.values = values
}

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := returnnode.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:return")
	if !ok {
		t.Fatal("flow:return was not registered")
	}
	return registered
}

func invocation(registered nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "return-1", Type: "flow:return", Data: map[string]any{"config": map[string]any{}}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:return" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 1 || definition.Inputs[0].ID != "in" || definition.Inputs[0].Kind != domain.PinExec {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if len(definition.Outputs) != 0 {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
}

func TestExecuteSignalsReturnWithInputs(t *testing.T) {
	registered := module(t)
	signaler := &returnSignalerStub{}
	inputs := map[string]any{"value": float64(42), "text": "done"}
	result, err := registered.Execute(context.Background(), invocation(registered, inputs), signaler)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if signaler.signals != 1 {
		t.Fatalf("return signals = %d, want 1", signaler.signals)
	}
	if !reflect.DeepEqual(signaler.values, inputs) {
		t.Fatalf("signaled values = %#v, want %#v", signaler.values, inputs)
	}
	if !reflect.DeepEqual(result.Outputs["result"], inputs) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if len(result.Ports) != 0 {
		t.Fatalf("ports = %#v", result.Ports)
	}
}

func TestExecuteSignalsReturnWithoutInputs(t *testing.T) {
	registered := module(t)
	signaler := &returnSignalerStub{}
	result, err := registered.Execute(context.Background(), invocation(registered, nil), signaler)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if signaler.signals != 1 || len(signaler.values) != 0 {
		t.Fatalf("signaled values = %#v", signaler.values)
	}
	reported, ok := result.Outputs["result"].(map[string]any)
	if !ok || len(reported) != 0 {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestExecuteRequiresReturnSignaler(t *testing.T) {
	registered := module(t)
	if _, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}), nil); err == nil {
		t.Fatal("Execute() accepted a runtime without a return signaler")
	}
}

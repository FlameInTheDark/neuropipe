package sequence_test

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/sequence"
)

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := sequence.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:sequence")
	if !ok {
		t.Fatal("flow:sequence was not registered")
	}
	return registered
}

func invocation(registered nodes.Node) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "seq-1", Type: "flow:sequence", Data: map[string]any{"config": map[string]any{}}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:sequence" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 1 || definition.Inputs[0].ID != "in" || definition.Inputs[0].Kind != domain.PinExec {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if len(definition.Outputs) != 2 || definition.Outputs[0].ID != "then_0" || definition.Outputs[1].ID != "then_1" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
}

func TestExecuteEmitsPortsInDeclaredOrder(t *testing.T) {
	registered := module(t)
	result, err := registered.Execute(context.Background(), invocation(registered), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Ports) != 2 || result.Ports[0] != "then_0" || result.Ports[1] != "then_1" {
		t.Fatalf("ports = %#v, want [then_0 then_1]", result.Ports)
	}
	reported, ok := result.Outputs["result"].(map[string]any)
	if !ok || len(reported) != 0 {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if result.Loop != nil {
		t.Fatalf("loop plan = %#v", result.Loop)
	}
}

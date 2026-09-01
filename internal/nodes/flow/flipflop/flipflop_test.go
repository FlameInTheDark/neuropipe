package flipflop_test

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/flipflop"
)

type flipFlopStoreStub struct{ next bool }

func (s *flipFlopStoreStub) NextFlipFlop(string) bool { return s.next }

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := flipflop.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:flip_flop")
	if !ok {
		t.Fatal("flow:flip_flop was not registered")
	}
	return registered
}

func invocation(registered nodes.Node) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "flip-1", Type: "flow:flip_flop", Data: map[string]any{"config": map[string]any{}}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:flip_flop" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 1 || definition.Inputs[0].ID != "in" || definition.Inputs[0].Kind != domain.PinExec {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if len(definition.Outputs) != 2 || definition.Outputs[0].ID != "a" || definition.Outputs[1].ID != "b" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
}

func TestExecuteSelectsPortFromStoreState(t *testing.T) {
	tests := []struct {
		name       string
		next       bool
		wantPort   string
		wantOutput string
	}{
		{"store true routes A", true, "a", "a"},
		{"store false routes B", false, "b", "b"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			result, err := registered.Execute(context.Background(), invocation(registered), &flipFlopStoreStub{next: test.next})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(result.Ports) != 1 || result.Ports[0] != test.wantPort {
				t.Fatalf("ports = %#v, want [%s]", result.Ports, test.wantPort)
			}
			reported, ok := result.Outputs["result"].(map[string]any)
			if !ok || reported["output"] != test.wantOutput {
				t.Fatalf("outputs = %#v", result.Outputs)
			}
		})
	}
}

func TestExecuteRequiresFlipFlopStore(t *testing.T) {
	if _, err := module(t).Execute(context.Background(), invocation(module(t)), nil); err == nil {
		t.Fatal("Execute() accepted a runtime without a flip flop store")
	}
}

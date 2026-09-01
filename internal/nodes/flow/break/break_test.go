package breaknode_test

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	breaknode "github.com/FlameInTheDark/neuropipe/internal/nodes/flow/break"
)

type loopRuntimeStub struct {
	inLoop        bool
	breakRequests int
}

func (l *loopRuntimeStub) InLoop() bool { return l.inLoop }

func (l *loopRuntimeStub) RequestBreak() { l.breakRequests++ }

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := breaknode.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:break")
	if !ok {
		t.Fatal("flow:break was not registered")
	}
	return registered
}

func invocation(registered nodes.Node) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "break-1", Type: "flow:break", Data: map[string]any{"config": map[string]any{}}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:break" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 1 || definition.Inputs[0].ID != "in" || definition.Inputs[0].Kind != domain.PinExec {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if len(definition.Outputs) != 0 {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
}

func TestExecuteRequestsBreakInsideLoop(t *testing.T) {
	runtime := &loopRuntimeStub{inLoop: true}
	result, err := module(t).Execute(context.Background(), invocation(module(t)), runtime)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runtime.breakRequests != 1 {
		t.Fatalf("break requests = %d, want 1", runtime.breakRequests)
	}
	if len(result.Ports) != 0 {
		t.Fatalf("ports = %#v", result.Ports)
	}
	reported, ok := result.Outputs["result"].(map[string]any)
	if !ok || reported["break"] != true {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestExecuteFailsOutsideLoop(t *testing.T) {
	runtime := &loopRuntimeStub{inLoop: false}
	if _, err := module(t).Execute(context.Background(), invocation(module(t)), runtime); err == nil {
		t.Fatal("Execute() ran break outside a loop body")
	}
	if runtime.breakRequests != 0 {
		t.Fatalf("break requests = %d, want 0", runtime.breakRequests)
	}
}

func TestExecuteRequiresLoopController(t *testing.T) {
	if _, err := module(t).Execute(context.Background(), invocation(module(t)), nil); err == nil {
		t.Fatal("Execute() accepted a runtime without a loop controller")
	}
}

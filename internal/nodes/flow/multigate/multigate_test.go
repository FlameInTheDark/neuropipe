package multigate_test

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/multigate"
)

type multiGateStoreStub struct {
	index int
	sets  []int
}

func (s *multiGateStoreStub) MultiGateIndex(string) int { return s.index }

func (s *multiGateStoreStub) SetMultiGateIndex(_ string, index int) {
	s.sets = append(s.sets, index)
	s.index = index
}

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := multigate.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:multi_gate")
	if !ok {
		t.Fatal("flow:multi_gate was not registered")
	}
	return registered
}

func invocation(registered nodes.Node, config map[string]any, execInput string) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "multi-1", Type: "flow:multi_gate", Data: map[string]any{"config": config}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		ExecInput:       execInput,
		Config:          config,
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:multi_gate" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != "reset" {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	ids := make([]string, 0, len(definition.Outputs))
	for _, port := range definition.Outputs {
		ids = append(ids, port.ID)
	}
	if len(ids) != 3 || ids[0] != "a" || ids[1] != "b" || ids[2] != "c" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Name != "loop" || definition.Fields[0].Kind != "boolean" {
		t.Fatalf("fields = %#v", definition.Fields)
	}
}

func TestExecuteCyclesThroughPorts(t *testing.T) {
	registered := module(t)
	store := &multiGateStoreStub{}
	want := []string{"a", "b", "c"}
	for index, port := range want {
		result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}, "in"), store)
		if err != nil {
			t.Fatalf("Execute() %d error = %v", index, err)
		}
		if len(result.Ports) != 1 || result.Ports[0] != port {
			t.Fatalf("pulse %d ports = %#v, want [%s]", index, result.Ports, port)
		}
		reported, ok := result.Outputs["result"].(map[string]any)
		if !ok || reported["index"] != index {
			t.Fatalf("pulse %d outputs = %#v", index, result.Outputs)
		}
	}
	if store.index != 3 {
		t.Fatalf("stored index = %d, want 3", store.index)
	}
}

func TestExecuteCompletesWhenExhaustedWithoutLoop(t *testing.T) {
	registered := module(t)
	store := &multiGateStoreStub{index: 3}
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}, "in"), store)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Ports) != 0 {
		t.Fatalf("ports = %#v, exhausted gate must not route", result.Ports)
	}
	reported, ok := result.Outputs["result"].(map[string]any)
	if !ok || reported["complete"] != true {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if len(store.sets) != 0 {
		t.Fatalf("exhausted gate must not rewrite its index: %#v", store.sets)
	}
}

func TestExecuteLoopsWhenConfigured(t *testing.T) {
	tests := []struct {
		name  string
		index int
	}{
		{"restarts from exhausted index", 3},
		{"wraps an out-of-range index", 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			store := &multiGateStoreStub{index: test.index}
			result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{"loop": true}, "in"), store)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(result.Ports) != 1 || result.Ports[0] != "a" {
				t.Fatalf("ports = %#v, want [a]", result.Ports)
			}
			reported, ok := result.Outputs["result"].(map[string]any)
			if !ok || reported["index"] != 0 {
				t.Fatalf("outputs = %#v", result.Outputs)
			}
			if len(store.sets) != 1 || store.sets[0] != 1 {
				t.Fatalf("sets = %#v, want [1]", store.sets)
			}
		})
	}
}

func TestExecuteResetRestartsCycle(t *testing.T) {
	registered := module(t)
	store := &multiGateStoreStub{index: 2}
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}, "reset"), store)
	if err != nil {
		t.Fatalf("reset Execute() error = %v", err)
	}
	if len(store.sets) != 1 || store.sets[0] != 0 {
		t.Fatalf("sets = %#v, want [0]", store.sets)
	}
	if len(result.Ports) != 0 {
		t.Fatalf("reset ports = %#v", result.Ports)
	}
	reported, ok := result.Outputs["result"].(map[string]any)
	if !ok || reported["reset"] != true {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	next, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}, "in"), store)
	if err != nil {
		t.Fatalf("post-reset Execute() error = %v", err)
	}
	if len(next.Ports) != 1 || next.Ports[0] != "a" {
		t.Fatalf("post-reset ports = %#v, want [a]", next.Ports)
	}
}

func TestExecuteRequiresMultiGateStore(t *testing.T) {
	registered := module(t)
	if _, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}, "in"), nil); err == nil {
		t.Fatal("Execute() accepted a runtime without a multi gate store")
	}
}

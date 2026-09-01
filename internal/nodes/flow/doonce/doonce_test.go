package doonce_test

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/doonce"
)

type onceStoreStub struct {
	claimed bool
	resets  int
}

func (s *onceStoreStub) ClaimOnce(string) bool {
	if s.claimed {
		return false
	}
	s.claimed = true
	return true
}

func (s *onceStoreStub) ResetOnce(string) {
	s.resets++
	s.claimed = false
}

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := doonce.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:do_once")
	if !ok {
		t.Fatal("flow:do_once was not registered")
	}
	return registered
}

func invocation(registered nodes.Node, execInput string) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "once-1", Type: "flow:do_once", Data: map[string]any{"config": map[string]any{}}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		ExecInput:       execInput,
		Config:          map[string]any{},
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:do_once" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != "reset" {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if definition.Inputs[0].Kind != domain.PinExec || definition.Inputs[1].Kind != domain.PinExec {
		t.Fatalf("inputs must both be exec pins: %#v", definition.Inputs)
	}
	if len(definition.Outputs) != 1 || definition.Outputs[0].ID != "out" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
}

func TestExecutePassesFirstPulseThenBlocks(t *testing.T) {
	registered := module(t)
	store := &onceStoreStub{}
	first, err := registered.Execute(context.Background(), invocation(registered, "in"), store)
	if err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	if len(first.Ports) != 1 || first.Ports[0] != "out" {
		t.Fatalf("first ports = %#v", first.Ports)
	}
	reported, ok := first.Outputs["result"].(map[string]any)
	if !ok || reported["first"] != true {
		t.Fatalf("first outputs = %#v", first.Outputs)
	}
	second, err := registered.Execute(context.Background(), invocation(registered, "in"), store)
	if err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}
	if len(second.Ports) != 0 {
		t.Fatalf("second ports = %#v", second.Ports)
	}
	reported, ok = second.Outputs["result"].(map[string]any)
	if !ok || reported["alreadyDone"] != true {
		t.Fatalf("second outputs = %#v", second.Outputs)
	}
}

func TestExecuteResetClearsClaimAndEmitsNoPort(t *testing.T) {
	registered := module(t)
	store := &onceStoreStub{}
	if _, err := registered.Execute(context.Background(), invocation(registered, "in"), store); err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	reset, err := registered.Execute(context.Background(), invocation(registered, "reset"), store)
	if err != nil {
		t.Fatalf("reset Execute() error = %v", err)
	}
	if store.resets != 1 {
		t.Fatalf("resets = %d, want 1", store.resets)
	}
	if len(reset.Ports) != 0 {
		t.Fatalf("reset ports = %#v", reset.Ports)
	}
	reported, ok := reset.Outputs["result"].(map[string]any)
	if !ok || reported["reset"] != true {
		t.Fatalf("reset outputs = %#v", reset.Outputs)
	}
	again, err := registered.Execute(context.Background(), invocation(registered, "in"), store)
	if err != nil {
		t.Fatalf("post-reset Execute() error = %v", err)
	}
	if len(again.Ports) != 1 || again.Ports[0] != "out" {
		t.Fatalf("post-reset ports = %#v", again.Ports)
	}
}

func TestExecuteRequiresOnceStore(t *testing.T) {
	if _, err := module(t).Execute(context.Background(), invocation(module(t), "in"), nil); err == nil {
		t.Fatal("Execute() accepted a runtime without a once store")
	}
}

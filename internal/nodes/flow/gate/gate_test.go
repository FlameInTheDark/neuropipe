package gate_test

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/gate"
)

// gateStoreStub records every persisted transition. The first SetGateOpen
// marks the node configured, mirroring the host store's semantics.
type gateStoreStub struct {
	open       bool
	configured bool
	sets       []bool
}

func (s *gateStoreStub) GateOpen(string) (bool, bool) { return s.open, s.configured }

func (s *gateStoreStub) SetGateOpen(_ string, open bool) {
	s.sets = append(s.sets, open)
	s.open = open
	s.configured = true
}

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := gate.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:gate")
	if !ok {
		t.Fatal("flow:gate was not registered")
	}
	return registered
}

func invocation(registered nodes.Node, config map[string]any, execInput string) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "gate-1", Type: "flow:gate", Data: map[string]any{"config": config}},
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
	if definition.Type != "flow:gate" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	ids := make([]string, 0, len(definition.Inputs))
	for _, port := range definition.Inputs {
		ids = append(ids, port.ID)
	}
	if len(ids) != 4 || ids[0] != "in" || ids[1] != "open" || ids[2] != "close" || ids[3] != "toggle" {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if len(definition.Outputs) != 1 || definition.Outputs[0].ID != "out" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Name != "startOpen" || definition.Fields[0].Kind != "boolean" {
		t.Fatalf("fields = %#v", definition.Fields)
	}
	if definition.DefaultConfig["startOpen"] != true {
		t.Fatalf("default config = %#v", definition.DefaultConfig)
	}
}

func TestExecuteControlTransitions(t *testing.T) {
	tests := []struct {
		name     string
		exec     string
		open     bool
		wantSets []bool
		wantOpen bool
	}{
		{"open persists true", "open", false, []bool{true}, true},
		{"close persists false", "close", true, []bool{false}, false},
		{"toggle flips closed to open", "toggle", false, []bool{true}, true},
		{"toggle flips open to closed", "toggle", true, []bool{false}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			store := &gateStoreStub{open: test.open, configured: true}
			result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}, test.exec), store)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(store.sets) != 1 || store.sets[0] != test.wantSets[0] {
				t.Fatalf("sets = %#v, want %v", store.sets, test.wantSets)
			}
			if len(result.Ports) != 0 {
				t.Fatalf("control transitions must not route the out port: %#v", result.Ports)
			}
			reported, ok := result.Outputs["result"].(map[string]any)
			if !ok || reported["open"] != test.wantOpen {
				t.Fatalf("outputs = %#v", result.Outputs)
			}
		})
	}
}

func TestExecuteEnterRoutesOnlyWhileOpen(t *testing.T) {
	tests := []struct {
		name     string
		open     bool
		wantPort bool
	}{
		{"open gate passes execution", true, true},
		{"closed gate blocks execution", false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			store := &gateStoreStub{open: test.open, configured: true}
			result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}, "in"), store)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			// The resolved state is persisted even on a plain enter pulse.
			if len(store.sets) != 1 || store.sets[0] != test.open {
				t.Fatalf("sets = %#v, want [%v]", store.sets, test.open)
			}
			if test.wantPort {
				if len(result.Ports) != 1 || result.Ports[0] != "out" {
					t.Fatalf("ports = %#v, want [out]", result.Ports)
				}
			} else if len(result.Ports) != 0 {
				t.Fatalf("ports = %#v, want none", result.Ports)
			}
			reported, ok := result.Outputs["result"].(map[string]any)
			if !ok || reported["open"] != test.open {
				t.Fatalf("outputs = %#v", result.Outputs)
			}
		})
	}
}

func TestExecuteUnconfiguredGateReadsStartOpenConfig(t *testing.T) {
	tests := []struct {
		name       string
		startOpen  any
		wantPassed bool
	}{
		{"startOpen true passes", true, true},
		{"startOpen false blocks", false, false},
		// The inspector default is Start open: an unset or non-boolean value
		// keeps the gate open rather than silently closing it.
		{"unset startOpen resolves open", nil, true},
		{"text startOpen resolves open", "true", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			config := map[string]any{}
			if test.startOpen != nil {
				config["startOpen"] = test.startOpen
			}
			store := &gateStoreStub{}
			result, err := registered.Execute(context.Background(), invocation(registered, config, "in"), store)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if test.wantPassed && (len(result.Ports) != 1 || result.Ports[0] != "out") {
				t.Fatalf("ports = %#v, want [out]", result.Ports)
			}
			if !test.wantPassed && len(result.Ports) != 0 {
				t.Fatalf("ports = %#v, want none", result.Ports)
			}
		})
	}
}

func TestExecuteConfiguredStoreOverridesStartOpen(t *testing.T) {
	registered := module(t)
	store := &gateStoreStub{open: true, configured: true}
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{"startOpen": false}, "in"), store)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %#v, configured store state must win", result.Ports)
	}
}

func TestExecuteRequiresGateStore(t *testing.T) {
	registered := module(t)
	if _, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}, "in"), nil); err == nil {
		t.Fatal("Execute() accepted a runtime without a gate store")
	}
}

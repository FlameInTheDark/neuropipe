package branch_test

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/branch"
)

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := branch.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:branch")
	if !ok {
		t.Fatal("flow:branch was not registered")
	}
	return registered
}

func invocation(registered nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "branch-1", Type: "flow:branch", Data: map[string]any{"config": map[string]any{}}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:branch" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "in" || definition.Inputs[0].Kind != domain.PinExec {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if definition.Inputs[1].ID != "condition" || definition.Inputs[1].Kind != domain.PinData || definition.Inputs[1].DataType != domain.DataBoolean {
		t.Fatalf("condition pin = %#v", definition.Inputs[1])
	}
	if len(definition.Outputs) != 2 || definition.Outputs[0].ID != "true" || definition.Outputs[1].ID != "false" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
}

func TestExecuteRoutesBooleanCondition(t *testing.T) {
	tests := []struct {
		name      string
		condition any
		wantPort  string
		wantOpen  bool
	}{
		{"true routes to true port", true, "true", true},
		{"false routes to false port", false, "false", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module(t).Execute(context.Background(), invocation(module(t), map[string]any{"condition": test.condition}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(result.Ports) != 1 || result.Ports[0] != test.wantPort {
				t.Fatalf("ports = %#v, want [%s]", result.Ports, test.wantPort)
			}
			reported, ok := result.Outputs["result"].(map[string]any)
			if !ok || reported["condition"] != test.wantOpen {
				t.Fatalf("outputs = %#v", result.Outputs)
			}
		})
	}
}

func TestExecuteRejectsNonBooleanCondition(t *testing.T) {
	tests := []struct {
		name      string
		condition any
	}{
		{"missing condition", nil},
		{"text condition", "yes"},
		{"number condition", float64(1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputs := map[string]any{}
			if test.condition != nil {
				inputs["condition"] = test.condition
			}
			_, err := module(t).Execute(context.Background(), invocation(module(t), inputs), nil)
			if err == nil {
				t.Fatal("Execute() accepted a non-Boolean condition")
			}
		})
	}
}

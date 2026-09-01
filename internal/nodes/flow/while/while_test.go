package while_test

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/while"
)

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := while.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:while")
	if !ok {
		t.Fatal("flow:while was not registered")
	}
	return registered
}

func invocation(registered nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "while-1", Type: "flow:while", Data: map[string]any{"config": map[string]any{}}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:while" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != "condition" {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if definition.Inputs[1].DataType != domain.DataBoolean {
		t.Fatalf("condition pin dataType = %q", definition.Inputs[1].DataType)
	}
	if len(definition.Outputs) != 2 || definition.Outputs[0].ID != "loop" || definition.Outputs[1].ID != "completed" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
}

func TestExecuteReturnsConditionDrivenPlan(t *testing.T) {
	registered := module(t)
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	plan := result.Loop
	if plan == nil {
		t.Fatal("Execute() returned no loop plan")
	}
	if plan.Continue == nil {
		t.Fatal("while must provide a Continue function")
	}
	if len(plan.Iterations) != 0 {
		t.Fatalf("iterations = %#v", plan.Iterations)
	}
	if plan.ReportedCount != -1 {
		t.Fatalf("reported count = %d, want -1 (host counts iterations)", plan.ReportedCount)
	}
	if len(result.Ports) != 0 || result.Outputs != nil {
		t.Fatalf("result = %#v", result)
	}
}

func TestContinueEvaluatesBooleanCondition(t *testing.T) {
	tests := []struct {
		name      string
		condition any
		want      bool
	}{
		{"true keeps looping", true, true},
		{"false stops the loop", false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			continueLoop, err := result.Loop.Continue(map[string]any{"condition": test.condition})
			if err != nil {
				t.Fatalf("Continue() error = %v", err)
			}
			if continueLoop != test.want {
				t.Fatalf("Continue() = %v, want %v", continueLoop, test.want)
			}
		})
	}
}

func TestContinueRejectsNonBooleanCondition(t *testing.T) {
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
			registered := module(t)
			result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			inputs := map[string]any{}
			if test.condition != nil {
				inputs["condition"] = test.condition
			}
			if _, err := result.Loop.Continue(inputs); err == nil {
				t.Fatal("Continue() accepted a non-Boolean condition")
			}
		})
	}
}

package forloop_test

import (
	"context"
	"math"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/forloop"
)

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := forloop.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:for_loop")
	if !ok {
		t.Fatal("flow:for_loop was not registered")
	}
	return registered
}

func invocation(registered nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "for-1", Type: "flow:for_loop", Data: map[string]any{"config": map[string]any{}}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:for_loop" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 3 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != "first" || definition.Inputs[2].ID != "last" {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	ids := make([]string, 0, len(definition.Outputs))
	for _, port := range definition.Outputs {
		ids = append(ids, port.ID)
	}
	if len(ids) != 3 || ids[0] != "loop" || ids[1] != "completed" || ids[2] != "index" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
}

func TestExecuteBuildsInclusiveRange(t *testing.T) {
	registered := module(t)
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{
		"first": float64(2), "last": float64(4),
	}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	plan := result.Loop
	if plan == nil {
		t.Fatal("Execute() returned no loop plan")
	}
	if plan.ReportedCount != -1 {
		t.Fatalf("reported count = %d, want -1 (host counts iterations)", plan.ReportedCount)
	}
	if plan.Continue != nil {
		t.Fatal("for loop must not provide a Continue function")
	}
	if len(result.Ports) != 0 || result.Outputs != nil {
		t.Fatalf("result = %#v", result)
	}
	if len(plan.Iterations) != 3 {
		t.Fatalf("iterations = %#v", plan.Iterations)
	}
	for offset, iteration := range plan.Iterations {
		want := float64(2 + offset)
		if iteration["index"] != want {
			t.Fatalf("iteration %d index = %#v, want %v", offset, iteration["index"], want)
		}
		reported, ok := iteration["result"].(map[string]any)
		if !ok || reported["index"] != want {
			t.Fatalf("iteration %d result = %#v", offset, iteration)
		}
	}
}

func TestExecuteAcceptsEmptyAndReversedRanges(t *testing.T) {
	tests := []struct {
		name       string
		first      any
		last       any
		wantBounds int
	}{
		{"equal bounds run once", float64(3), float64(3), 1},
		{"reversed bounds run zero times", float64(5), float64(2), 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{
				"first": test.first, "last": test.last,
			}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(result.Loop.Iterations) != test.wantBounds {
				t.Fatalf("iterations = %d, want %d", len(result.Loop.Iterations), test.wantBounds)
			}
		})
	}
}

func TestExecuteAcceptsIntegerInputTypes(t *testing.T) {
	registered := module(t)
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{
		"first": 1, "last": int64(3),
	}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Loop.Iterations) != 3 {
		t.Fatalf("iterations = %#v", result.Loop.Iterations)
	}
}

func TestExecuteRejectsNonIntegerBounds(t *testing.T) {
	tests := []struct {
		name  string
		first any
		last  any
	}{
		{"fractional first", 1.5, float64(3)},
		{"fractional last", float64(1), 3.5},
		{"text first", "1", float64(3)},
		{"missing first", nil, float64(3)},
		{"missing last", float64(1), nil},
		{"NaN first", math.NaN(), float64(3)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputs := map[string]any{"last": test.last}
			if test.first != nil {
				inputs["first"] = test.first
			}
			registered := module(t)
			if _, err := registered.Execute(context.Background(), invocation(registered, inputs), nil); err == nil {
				t.Fatal("Execute() accepted non-integer loop bounds")
			}
		})
	}
}

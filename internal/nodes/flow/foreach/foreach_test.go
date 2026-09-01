package foreach_test

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/foreach"
)

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := foreach.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:for_each")
	if !ok {
		t.Fatal("flow:for_each was not registered")
	}
	return registered
}

func invocation(registered nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "each-1", Type: "flow:for_each", Data: map[string]any{"config": map[string]any{}}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:for_each" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != "items" {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if definition.Inputs[1].DataType != domain.DataList {
		t.Fatalf("items pin dataType = %q", definition.Inputs[1].DataType)
	}
	ids := make([]string, 0, len(definition.Outputs))
	for _, port := range definition.Outputs {
		ids = append(ids, port.ID)
	}
	want := []string{"loop", "completed", "item", "index"}
	for index, id := range want {
		if ids[index] != id {
			t.Fatalf("outputs = %#v, want %v", definition.Outputs, want)
		}
	}
}

func TestExecuteBuildsIterationPlan(t *testing.T) {
	registered := module(t)
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{
		"items": []any{"alpha", 2, true},
	}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	plan := result.Loop
	if plan == nil {
		t.Fatal("Execute() returned no loop plan")
	}
	if plan.ReportedCount != 3 {
		t.Fatalf("reported count = %d, want 3", plan.ReportedCount)
	}
	if plan.Continue != nil {
		t.Fatal("for-each must not provide a Continue function")
	}
	if len(result.Ports) != 0 || result.Outputs != nil {
		t.Fatalf("result = %#v", result)
	}
	if len(plan.Iterations) != 3 {
		t.Fatalf("iterations = %#v", plan.Iterations)
	}
	for index, iteration := range plan.Iterations {
		if iteration["item"] == nil || iteration["index"] != float64(index) {
			t.Fatalf("iteration %d = %#v", index, iteration)
		}
		reported, ok := iteration["result"].(map[string]any)
		if !ok || reported["item"] != iteration["item"] || reported["index"] != float64(index) {
			t.Fatalf("iteration %d result = %#v", index, iteration)
		}
	}
	if plan.Iterations[0]["item"] != "alpha" || plan.Iterations[1]["item"] != 2 || plan.Iterations[2]["item"] != true {
		t.Fatalf("iteration items = %#v", plan.Iterations)
	}
}

func TestExecuteAcceptsEmptyList(t *testing.T) {
	registered := module(t)
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{"items": []any{}}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Loop == nil || len(result.Loop.Iterations) != 0 || result.Loop.ReportedCount != 0 {
		t.Fatalf("loop plan = %#v", result.Loop)
	}
}

func TestExecuteRejectsNonListItems(t *testing.T) {
	tests := []struct {
		name  string
		items any
	}{
		{"missing items", nil},
		{"text items", "alpha,beta"},
		{"single value", 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputs := map[string]any{}
			if test.items != nil {
				inputs["items"] = test.items
			}
			registered := module(t)
			if _, err := registered.Execute(context.Background(), invocation(registered, inputs), nil); err == nil {
				t.Fatal("Execute() accepted a non-list items value")
			}
		})
	}
}

func TestExecuteCapsIterationCount(t *testing.T) {
	items := make([]any, 10_005)
	for index := range items {
		items[index] = index
	}
	registered := module(t)
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{"items": items}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Loop.Iterations) != 10_001 {
		t.Fatalf("iterations = %d, want the 10_001 safety cap", len(result.Loop.Iterations))
	}
	if result.Loop.ReportedCount != 10_005 {
		t.Fatalf("reported count = %d, want 10005", result.Loop.ReportedCount)
	}
}

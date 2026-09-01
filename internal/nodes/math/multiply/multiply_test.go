package multiply_test

import (
	"context"
	"math"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/multiply"
)

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := multiply.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("math:multiply")
	if !ok {
		t.Fatal("math:multiply was not registered")
	}
	return registered
}

func invocation(inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "mul-1", Type: "math:multiply", Data: map[string]any{"config": map[string]any{}}},
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "math:multiply" || definition.Mode != domain.NodePure || definition.Category != "Math" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "a" || definition.Inputs[1].ID != "b" {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if len(definition.Outputs) != 1 || definition.Outputs[0].ID != "result" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
}

func TestExecuteMultipliesNumbers(t *testing.T) {
	tests := []struct {
		name string
		a, b any
		want float64
	}{
		{"floats", float64(2), float64(3), 6},
		{"int inputs", 4, 5, 20},
		{"negative product", float64(-2), float64(3), -6},
		{"fraction", float64(0.5), float64(8), 4},
		{"zero product", float64(2.5), float64(0), 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module(t).Execute(context.Background(), invocation(map[string]any{"a": test.a, "b": test.b}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got, ok := result.Outputs["result"].(float64); !ok || got != test.want {
				t.Fatalf("result = %#v, want %v", result.Outputs["result"], test.want)
			}
		})
	}
}

func TestExecuteRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name   string
		inputs map[string]any
	}{
		{"missing a", map[string]any{"b": float64(1)}},
		{"missing b", map[string]any{"a": float64(1)}},
		{"text a", map[string]any{"a": "2", "b": float64(1)}},
		{"NaN b", map[string]any{"a": float64(1), "b": math.NaN()}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := module(t).Execute(context.Background(), invocation(test.inputs), nil); err == nil {
				t.Fatal("Execute() accepted an invalid input")
			}
		})
	}
}

func TestExecuteRejectsNonFiniteResult(t *testing.T) {
	_, err := module(t).Execute(context.Background(), invocation(map[string]any{
		"a": math.MaxFloat64, "b": float64(2),
	}), nil)
	if err == nil {
		t.Fatal("Execute() accepted an overflowing product")
	}
}

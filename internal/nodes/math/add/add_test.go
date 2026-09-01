package add_test

import (
	"context"
	"math"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/add"
)

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := add.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("math:add")
	if !ok {
		t.Fatal("math:add was not registered")
	}
	return registered
}

func invocation(inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "add-1", Type: "math:add", Data: map[string]any{"config": map[string]any{}}},
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "math:add" || definition.Mode != domain.NodePure || definition.Category != "Math" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "a" || definition.Inputs[1].ID != "b" {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	for _, input := range definition.Inputs {
		if input.DataType != domain.DataNumber || input.Default != 0.0 {
			t.Fatalf("input %q = %#v", input.ID, input)
		}
	}
	if len(definition.Outputs) != 1 || definition.Outputs[0].ID != "result" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
	if definition.DefaultConfig["a"] != 0.0 || definition.DefaultConfig["b"] != 0.0 {
		t.Fatalf("default config = %#v", definition.DefaultConfig)
	}
}

func TestExecuteAddsNumbers(t *testing.T) {
	tests := []struct {
		name string
		a, b any
		want float64
	}{
		{"floats", float64(2), float64(3), 5},
		{"int inputs", 2, 3, 5},
		{"mixed int and float", 2, float64(3.5), 5.5},
		{"negative result", float64(-1.5), float64(0.5), -1},
		{"zero identity", float64(0), float64(0), 0},
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
		want   string
	}{
		{"missing a", map[string]any{"b": float64(1)}, "math input A must be a finite number"},
		{"missing b", map[string]any{"a": float64(1)}, "math input B must be a finite number"},
		{"text a", map[string]any{"a": "1", "b": float64(1)}, "math input A must be a finite number"},
		{"NaN a", map[string]any{"a": math.NaN(), "b": float64(1)}, "math input A must be a finite number"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module(t).Execute(context.Background(), invocation(test.inputs), nil)
			if err == nil {
				t.Fatal("Execute() accepted an invalid input")
			}
		})
	}
}

func TestExecuteRejectsNonFiniteResult(t *testing.T) {
	_, err := module(t).Execute(context.Background(), invocation(map[string]any{
		"a": math.MaxFloat64, "b": math.MaxFloat64,
	}), nil)
	if err == nil {
		t.Fatal("Execute() accepted an overflowing sum")
	}
}

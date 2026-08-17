package randomnumber

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestRandomNumberFloat(t *testing.T) {
	module := New()
	invocation := nodes.Invocation{
		Node:       domain.FlowNode{Type: "data:random_number"},
		Definition: module.Definition(),
		Config:     map[string]any{"type": "float"},
		Inputs:     map[string]any{},
	}
	result, err := module.Execute(context.Background(), invocation, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	value, ok := result.Outputs["value"].(float64)
	if !ok {
		t.Fatalf("expected float64 value, got %T", result.Outputs["value"])
	}
	if value < 0 || value >= 1 {
		t.Fatalf("expected value in [0, 1), got %v", value)
	}
}

func TestRandomNumberIntegerInRange(t *testing.T) {
	module := New()
	for range 100 {
		invocation := nodes.Invocation{
			Node:       domain.FlowNode{Type: "data:random_number"},
			Definition: module.Definition(),
			Config:     map[string]any{"type": "integer", "useRange": true, "from": 1.0, "to": 6.0},
			Inputs:     map[string]any{},
		}
		result, err := module.Execute(context.Background(), invocation, nil)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}
		value, ok := result.Outputs["value"].(int64)
		if !ok {
			t.Fatalf("expected int64 value, got %T", result.Outputs["value"])
		}
		if value < 1 || value > 6 {
			t.Fatalf("expected value in [1, 6], got %d", value)
		}
	}
}

func TestRandomNumberFloatInRange(t *testing.T) {
	module := New()
	for range 100 {
		invocation := nodes.Invocation{
			Node:       domain.FlowNode{Type: "data:random_number"},
			Definition: module.Definition(),
			Config:     map[string]any{"type": "float", "useRange": true, "from": 10.0, "to": 20.0},
			Inputs:     map[string]any{},
		}
		result, err := module.Execute(context.Background(), invocation, nil)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}
		value, ok := result.Outputs["value"].(float64)
		if !ok {
			t.Fatalf("expected float64 value, got %T", result.Outputs["value"])
		}
		if value < 10 || value > 20 {
			t.Fatalf("expected value in [10, 20], got %v", value)
		}
	}
}

func TestRandomNumberPinsOverrideFields(t *testing.T) {
	module := New()
	invocation := nodes.Invocation{
		Node:       domain.FlowNode{Type: "data:random_number"},
		Definition: module.Definition(),
		Config:     map[string]any{"type": "integer", "useRange": true, "from": 0.0, "to": 1.0},
		// Inputs override the inspector fields: range becomes [5, 5] which
		// forces the value to 5 regardless of the field configuration.
		Inputs: map[string]any{"from": 5.0, "to": 5.0},
	}
	result, err := module.Execute(context.Background(), invocation, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	value, ok := result.Outputs["value"].(int64)
	if !ok {
		t.Fatalf("expected int64 value, got %T", result.Outputs["value"])
	}
	if value != 5 {
		t.Fatalf("expected value 5 (pin override), got %d", value)
	}
}

func TestRandomNumberRangeInvertedFails(t *testing.T) {
	module := New()
	invocation := nodes.Invocation{
		Node:       domain.FlowNode{Type: "data:random_number"},
		Definition: module.Definition(),
		Config:     map[string]any{"type": "integer", "useRange": true, "from": 10.0, "to": 1.0},
		Inputs:     map[string]any{},
	}
	if _, err := module.Execute(context.Background(), invocation, nil); err == nil {
		t.Fatalf("expected error for inverted range, got nil")
	}
}

func TestRandomNumberResolveAdaptsOutputType(t *testing.T) {
	module := New()
	definition, err := module.Resolve(domain.FlowNode{Type: "data:random_number", Data: map[string]any{"config": map[string]any{"type": "integer"}}})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	for _, output := range definition.Outputs {
		if output.ID == "value" && output.Type != nil && output.Type.Kind != domain.TypeInt {
			t.Fatalf("expected value pin to be int, got %s", output.Type.Kind)
		}
	}
}

// TestRandomNumberResolveAdaptsInputPinsForInteger guards against the
// regression where selecting `integer` left the from/to pins as float, which
// made the strict type checker reject int sources connected to them.
func TestRandomNumberResolveAdaptsInputPinsForInteger(t *testing.T) {
	module := New()
	definition, err := module.Resolve(domain.FlowNode{Type: "data:random_number", Data: map[string]any{"config": map[string]any{"type": "integer"}}})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	for _, input := range definition.Inputs {
		if input.ID != "from" && input.ID != "to" {
			continue
		}
		if input.Type == nil || input.Type.Kind != domain.TypeInt {
			t.Fatalf("expected %s pin to be int, got %v", input.ID, input.Type)
		}
	}
}

// TestRandomNumberResolveKeepsFloatPinsByDefault confirms the default float
// contract is preserved when no type is configured.
func TestRandomNumberResolveKeepsFloatPinsByDefault(t *testing.T) {
	module := New()
	definition, err := module.Resolve(domain.FlowNode{Type: "data:random_number", Data: map[string]any{"config": map[string]any{"type": "float"}}})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	for _, input := range definition.Inputs {
		if input.ID != "from" && input.ID != "to" {
			continue
		}
		if input.Type == nil || input.Type.Kind != domain.TypeFloat {
			t.Fatalf("expected %s pin to be float, got %v", input.ID, input.Type)
		}
	}
	for _, output := range definition.Outputs {
		if output.ID == "value" && output.Type != nil && output.Type.Kind != domain.TypeFloat {
			t.Fatalf("expected value pin to be float, got %s", output.Type.Kind)
		}
	}
}

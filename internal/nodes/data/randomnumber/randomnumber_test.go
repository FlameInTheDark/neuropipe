package randomnumber

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestRegisterReportsImpureSourceContract(t *testing.T) {
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	module, ok := registry.Get("data:random_number")
	if !ok {
		t.Fatal("data:random_number was not registered")
	}
	definition := module.Definition()
	if definition.Mode != domain.NodeImpure || !definition.PortContractOwned {
		t.Fatalf("definition header = %#v", definition)
	}
	inputs := map[string]domain.PinKind{}
	for _, input := range definition.Inputs {
		inputs[input.ID] = input.Kind
	}
	if inputs["in"] != domain.PinExec || inputs["from"] != domain.PinData || inputs["to"] != domain.PinData {
		t.Fatalf("inputs = %#v", inputs)
	}
	outputs := map[string]domain.PinKind{}
	for _, output := range definition.Outputs {
		outputs[output.ID] = output.Kind
	}
	if outputs["out"] != domain.PinExec || outputs["value"] != domain.PinData {
		t.Fatalf("outputs = %#v", outputs)
	}
	want := map[string]any{"type": "float", "useRange": false, "from": 0.0, "to": 1.0}
	if !reflect.DeepEqual(definition.DefaultConfig, want) {
		t.Fatalf("default config = %#v, want %#v", definition.DefaultConfig, want)
	}
}

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

func TestRandomNumberDefaultsToFloatWithoutType(t *testing.T) {
	module := New()
	invocation := nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:random_number"},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          map[string]any{},
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
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

func TestRandomNumberExecuteEmitsExecPortAndResultRecord(t *testing.T) {
	module := New()
	invocation := nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:random_number", Data: map[string]any{"config": map[string]any{"type": "integer"}}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          map[string]any{"type": "integer"},
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
	}
	result, err := module.Execute(context.Background(), invocation, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !reflect.DeepEqual(result.Ports, []string{"out"}) {
		t.Fatalf("ports = %#v, want the single out exec port", result.Ports)
	}
	value, ok := result.Outputs["value"].(int64)
	if !ok {
		t.Fatalf("expected int64 value, got %T", result.Outputs["value"])
	}
	record, ok := result.Outputs["result"].(map[string]any)
	if !ok || record["value"] != value {
		t.Fatalf("result record = %#v, want it to mirror the value output", result.Outputs["result"])
	}
}

func TestRandomNumberParsesStringBoundsFromConfig(t *testing.T) {
	module := New()
	// Inspector number fields may persist as text; the bounds parser accepts them.
	invocation := nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:random_number", Data: map[string]any{"config": map[string]any{"type": "integer", "useRange": true, "from": "5", "to": "5"}}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          map[string]any{"type": "integer", "useRange": true, "from": "5", "to": "5"},
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
	}
	result, err := module.Execute(context.Background(), invocation, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Outputs["value"] != int64(5) {
		t.Fatalf("value = %#v, want 5 from the string bounds", result.Outputs["value"])
	}
}

func TestRandomNumberCancelledContext(t *testing.T) {
	module := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	invocation := nodes.Invocation{
		Node:       domain.FlowNode{Type: "data:random_number"},
		Definition: module.Definition(),
		Config:     map[string]any{"type": "float"},
		Inputs:     map[string]any{},
	}
	_, err := module.Execute(ctx, invocation, nil)
	if err == nil || !strings.Contains(err.Error(), "random number cancelled") {
		t.Fatalf("Execute(cancelled) error = %v, want the cancellation failure", err)
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

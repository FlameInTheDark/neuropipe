package gettype

import (
	"context"
	"reflect"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	module, ok := registry.Get("data:get_type")
	if !ok {
		t.Fatal("data:get_type was not registered")
	}
	return module
}

func invocation(module nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:get_type", Data: map[string]any{"config": map[string]any{}}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

type sampleRecord struct {
	Name string
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:get_type" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinInput {
		t.Fatalf("value input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "type" || got.DataType != domain.DataText {
		t.Fatalf("type output = %#v", got)
	}
	if got := definition.Outputs[1]; got.ID != "elementType" || got.DataType != domain.DataText {
		t.Fatalf("elementType output = %#v", got)
	}
}

func TestEvaluateReportsValueKinds(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name        string
		value       any
		wantType    string
		wantElement string
	}{
		{"nil is null", nil, "null", ""},
		{"text", "hello", "text", ""},
		{"boolean", true, "boolean", ""},
		{"float64 number", 3.14, "number", ""},
		{"float32 number", float32(1.5), "number", ""},
		{"int number", 7, "number", ""},
		{"int64 number", int64(7), "number", ""},
		{"empty list", []any{}, "list", "any"},
		{"homogeneous number list", []any{1.0, 2.0}, "list", "number"},
		{"homogeneous text list", []any{"a", "b"}, "list", "text"},
		{"mixed list", []any{1.0, "a"}, "list", "mixed"},
		{"object map", map[string]any{"a": 1.0}, "object", ""},
		{"struct", sampleRecord{Name: "Ada"}, "object", ""},
		{"integer-keyed map is unknown", map[int]string{1: "a"}, "unknown", ""},
		{"typed string slice is unknown", []string{"a"}, "unknown", ""},
		{"bytes are unknown", []byte("ab"), "unknown", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, map[string]any{"value": test.value}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			want := map[string]any{"type": test.wantType, "elementType": test.wantElement}
			if !reflect.DeepEqual(result.Outputs, want) {
				t.Fatalf("outputs = %#v, want %#v", result.Outputs, want)
			}
		})
	}
}

func TestEvaluateDereferencesPointers(t *testing.T) {
	module := registeredModule(t)
	record := sampleRecord{Name: "Ada"}
	tests := []struct {
		name     string
		value    any
		wantType string
	}{
		{"pointer to struct", &record, "object"},
		{"pointer to map", &map[string]any{"a": 1.0}, "object"},
		{"nil pointer stays unknown", (*sampleRecord)(nil), "unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, map[string]any{"value": test.value}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if result.Outputs["type"] != test.wantType {
				t.Fatalf("type = %#v, want %q", result.Outputs["type"], test.wantType)
			}
		})
	}
}

func TestEvaluateReportsMissingInputAsNull(t *testing.T) {
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(result.Outputs, map[string]any{"type": "null", "elementType": ""}) {
		t.Fatalf("outputs = %#v, want the null kind", result.Outputs)
	}
}

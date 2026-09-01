package typeassert

import (
	"context"
	"reflect"
	"strings"
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
	module, ok := registry.Get("data:type_assert")
	if !ok {
		t.Fatal("data:type_assert was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, value any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:type_assert", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          config,
		Inputs:          map[string]any{"value": value},
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:type_assert" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinInput {
		t.Fatalf("value input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinOutput {
		t.Fatalf("value output = %#v", got)
	}
	field := definition.Fields[0]
	if field.Name != "typeSpec" || field.Kind != "type-spec" || !field.Required {
		t.Fatalf("typeSpec field = %#v", field)
	}
	if got, want := definition.DefaultConfig["typeSpec"], map[string]any{"kind": "any"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("default typeSpec = %#v, want %#v", got, want)
	}
}

func TestEvaluateDefaultContractAcceptsAnything(t *testing.T) {
	module := registeredModule(t)
	for _, value := range []any{nil, "text", 42.0, true, map[string]any{"a": 1.0}, []any{"a"}} {
		result, err := module.Execute(context.Background(), invocation(module, nil, value), nil)
		if err != nil {
			t.Fatalf("Execute(%#v) error = %v", value, err)
		}
		if !reflect.DeepEqual(result.Outputs, map[string]any{"value": value}) {
			t.Fatalf("outputs = %#v, want the original value %#v", result.Outputs, value)
		}
	}
}

func TestEvaluateHonorsConfiguredContract(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name     string
		typeSpec map[string]any
		value    any
	}{
		{"int accepts int", map[string]any{"kind": "int"}, 7},
		{"float accepts int", map[string]any{"kind": "float"}, 7},
		{"float accepts float", map[string]any{"kind": "float"}, 2.5},
		{"string accepts text", map[string]any{"kind": "string"}, "hello"},
		{"bool accepts boolean", map[string]any{"kind": "bool"}, true},
		{"bytes accepts bytes", map[string]any{"kind": "bytes"}, []byte("raw")},
		{"list of string accepts text list", map[string]any{"kind": "list", "element": map[string]any{"kind": "string"}}, []any{"a", "b"}},
		{"map accepts string-keyed map", map[string]any{"kind": "map", "key": map[string]any{"kind": "string"}, "value": map[string]any{"kind": "any"}}, map[string]any{"a": 1.0}},
		{"record accepts matching object", map[string]any{"kind": "record", "fields": []any{map[string]any{"id": "name", "name": "name", "type": map[string]any{"kind": "string"}}}}, map[string]any{"name": "Ada"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, map[string]any{"typeSpec": test.typeSpec}, test.value), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs, map[string]any{"value": test.value}) {
				t.Fatalf("outputs = %#v, want the original value %#v", result.Outputs, test.value)
			}
		})
	}
}

func TestEvaluateRejectsContractViolations(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name     string
		typeSpec map[string]any
		value    any
	}{
		{"int rejects float", map[string]any{"kind": "int"}, 7.0},
		{"int rejects text", map[string]any{"kind": "int"}, "7"},
		{"string rejects number", map[string]any{"kind": "string"}, 42.0},
		{"string rejects nil", map[string]any{"kind": "string"}, nil},
		{"bool rejects text", map[string]any{"kind": "bool"}, "true"},
		{"string list rejects mixed list", map[string]any{"kind": "list", "element": map[string]any{"kind": "string"}}, []any{"a", 1}},
		{"record rejects missing field", map[string]any{"kind": "record", "fields": []any{map[string]any{"id": "name", "name": "name", "type": map[string]any{"kind": "string"}}}}, map[string]any{"other": 1.0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, map[string]any{"typeSpec": test.typeSpec}, test.value), nil)
			if err == nil || !strings.Contains(err.Error(), "type assertion failed") {
				t.Fatalf("Execute() error = %v, want a type assertion failure", err)
			}
		})
	}
}

func TestEvaluateRejectsInvalidContract(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name     string
		typeSpec any
		message  string
	}{
		{"unsupported kind", map[string]any{"kind": "bogus"}, "type contract is invalid"},
		{"list without element", map[string]any{"kind": "list"}, "type contract is invalid"},
		{"map without key and value", map[string]any{"kind": "map"}, "type contract is invalid"},
		{"scalar typeSpec", "string", "type contract is invalid"},
		{"unmarshalable typeSpec", map[string]any{"kind": make(chan int)}, "type contract must be JSON data"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, map[string]any{"typeSpec": test.typeSpec}, "value"), nil)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Execute() error = %v, want it to contain %q", err, test.message)
			}
		})
	}
}

func TestResolveTypesOutputFromContract(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name     string
		typeSpec map[string]any
		dataType domain.DataType
		kind     domain.TypeKind
		color    string
	}{
		{"string", map[string]any{"kind": "string"}, domain.DataText, domain.TypeString, "#e879f9"},
		{"int", map[string]any{"kind": "int"}, domain.DataNumber, domain.TypeInt, "#86efac"},
		{"float", map[string]any{"kind": "float"}, domain.DataNumber, domain.TypeFloat, "#86efac"},
		{"bool", map[string]any{"kind": "bool"}, domain.DataBoolean, domain.TypeBool, "#f87171"},
		{"list", map[string]any{"kind": "list", "element": map[string]any{"kind": "string"}}, domain.DataList, domain.TypeList, "#facc15"},
		{"map", map[string]any{"kind": "map", "key": map[string]any{"kind": "string"}, "value": map[string]any{"kind": "any"}}, domain.DataObject, domain.TypeMap, "#60a5fa"},
		{"record", map[string]any{"kind": "record", "fields": []any{map[string]any{"id": "name", "name": "name", "type": map[string]any{"kind": "string"}}}}, domain.DataObject, domain.TypeRecord, "#60a5fa"},
		{"any", map[string]any{"kind": "any"}, domain.DataAny, domain.TypeAny, "#a1a1aa"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := module.Resolve(domain.FlowNode{Type: "data:type_assert", Data: map[string]any{"config": map[string]any{"typeSpec": test.typeSpec}}})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			output := resolved.Outputs[0]
			if output.DataType != test.dataType || output.Color != test.color {
				t.Fatalf("output = %#v, want data type %s and color %s", output, test.dataType, test.color)
			}
			if output.Type == nil || output.Type.Kind != test.kind {
				t.Fatalf("output type = %#v, want kind %s", output.Type, test.kind)
			}
		})
	}
}

func TestResolveKeepsNestedContractShape(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:type_assert", Data: map[string]any{"config": map[string]any{"typeSpec": map[string]any{"kind": "list", "element": map[string]any{"kind": "string"}}}}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := domain.TypeSpec{Kind: domain.TypeList, Element: &domain.TypeSpec{Kind: domain.TypeString}}
	if got := resolved.Outputs[0].Type; got == nil || !reflect.DeepEqual(*got, want) {
		t.Fatalf("resolved contract = %#v, want %#v", got, want)
	}
}

func TestResolveFallsBackToDefaultAny(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:type_assert", Data: map[string]any{"config": map[string]any{}}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if output := resolved.Outputs[0]; output.DataType != domain.DataAny || output.Type == nil || output.Type.Kind != domain.TypeAny {
		t.Fatalf("output = %#v, want the default any contract", output)
	}
}

func TestResolveRejectsInvalidContract(t *testing.T) {
	module := registeredModule(t)
	if _, err := module.Resolve(domain.FlowNode{Type: "data:type_assert", Data: map[string]any{"config": map[string]any{"typeSpec": map[string]any{"kind": "bogus"}}}}); err == nil || !strings.Contains(err.Error(), "type contract is invalid") {
		t.Fatalf("Resolve() error = %v, want the invalid-contract failure", err)
	}
}

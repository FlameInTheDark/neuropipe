package constant

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
	module, ok := registry.Get("data:constant")
	if !ok {
		t.Fatal("data:constant was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:constant", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:constant" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if len(definition.Inputs) != 0 {
		t.Fatalf("inputs = %#v, want none for a literal source node", definition.Inputs)
	}
	if got := definition.Outputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinOutput {
		t.Fatalf("value output = %#v", got)
	}
	valueField := definition.Fields[0]
	if valueField.Name != "value" || valueField.Kind != "string" || valueField.Required {
		t.Fatalf("value field = %#v", valueField)
	}
	typeField := definition.Fields[1]
	if typeField.Name != "type" || typeField.Kind != "select" || !typeField.Required {
		t.Fatalf("type field = %#v", typeField)
	}
	values := make([]string, 0, len(typeField.Options))
	for _, option := range typeField.Options {
		values = append(values, option.Value)
	}
	if want := []string{"text", "number", "boolean"}; !reflect.DeepEqual(values, want) {
		t.Fatalf("type options = %v, want %v", values, want)
	}
	if !reflect.DeepEqual(definition.DefaultConfig, map[string]any{"type": "text"}) {
		t.Fatalf("default config = %#v, want type text", definition.DefaultConfig)
	}
}

func TestResolveTypesOutputPin(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name     string
		config   map[string]any
		dataType domain.DataType
		kind     domain.TypeKind
		color    string
	}{
		{"text", map[string]any{"type": "text"}, domain.DataText, domain.TypeString, "#e879f9"},
		{"number", map[string]any{"type": "number"}, domain.DataNumber, domain.TypeFloat, "#86efac"},
		{"boolean", map[string]any{"type": "boolean"}, domain.DataBoolean, domain.TypeBool, "#f87171"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := module.Resolve(domain.FlowNode{Type: "data:constant", Data: map[string]any{"config": test.config}})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			output := resolved.Outputs[0]
			if output.DataType != test.dataType || output.Type == nil || output.Type.Kind != test.kind || output.Color != test.color {
				t.Fatalf("resolved output = %#v, want %s/%s/%s", output, test.dataType, test.kind, test.color)
			}
		})
	}
}

func TestResolveKeepsAnyForMissingOrUnknownType(t *testing.T) {
	module := registeredModule(t)
	for name, config := range map[string]any{
		"no config":       nil,
		"empty type":      map[string]any{"type": ""},
		"unknown type":    map[string]any{"type": "object"},
		"non-string type": map[string]any{"type": 5.0},
	} {
		t.Run(name, func(t *testing.T) {
			resolved, err := module.Resolve(domain.FlowNode{Type: "data:constant", Data: map[string]any{"config": config}})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if output := resolved.Outputs[0]; output.DataType != domain.DataAny {
				t.Fatalf("resolved output data type = %q, want any fallback", output.DataType)
			}
		})
	}
}

func TestEvaluateReturnsStrictlyTypedLiterals(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name   string
		config map[string]any
		want   any
	}{
		{"text literal", map[string]any{"type": "text", "value": "hello"}, "hello"},
		{"text default type", map[string]any{"value": "hello"}, "hello"},
		{"number float literal", map[string]any{"type": "number", "value": 3.5}, 3.5},
		{"number int literal stays int", map[string]any{"type": "number", "value": 7}, 7},
		{"boolean literal", map[string]any{"type": "boolean", "value": true}, true},
		{"boolean false literal", map[string]any{"type": "boolean", "value": false}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, test.config, nil), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs, map[string]any{"value": test.want}) {
				t.Fatalf("outputs = %#v, want value %#v", result.Outputs, test.want)
			}
		})
	}
}

func TestEvaluateInputOverridesConfiguredLiteral(t *testing.T) {
	module := registeredModule(t)
	config := map[string]any{"type": "text", "value": "from-config"}
	result, err := module.Execute(context.Background(), invocation(module, config, map[string]any{"value": "from-input"}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(result.Outputs, map[string]any{"value": "from-input"}) {
		t.Fatalf("outputs = %#v, want the connected input literal", result.Outputs)
	}
}

func TestEvaluateRejectsTypeMismatches(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name    string
		config  map[string]any
		inputs  map[string]any
		message string
	}{
		{"text needs a string", map[string]any{"type": "text", "value": 12.0}, nil, "constant text value must be a string"},
		{"text rejects nil", map[string]any{"type": "text"}, nil, "constant text value must be a string"},
		{"text rejects connected number", map[string]any{"type": "text"}, map[string]any{"value": 12.0}, "constant text value must be a string"},
		{"number rejects text", map[string]any{"type": "number", "value": "5"}, nil, "constant number value"},
		{"number rejects nil", map[string]any{"type": "number"}, nil, "constant number value"},
		{"number rejects boolean", map[string]any{"type": "number", "value": true}, nil, "constant number value"},
		{"boolean rejects text", map[string]any{"type": "boolean", "value": "true"}, nil, "constant Boolean value must be a bool"},
		{"boolean rejects number", map[string]any{"type": "boolean", "value": 1.0}, nil, "constant Boolean value must be a bool"},
		{"unknown type", map[string]any{"type": "object", "value": "x"}, nil, `unknown constant type "object"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, test.config, test.inputs), nil)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Execute() error = %v, want it to contain %q", err, test.message)
			}
		})
	}
}

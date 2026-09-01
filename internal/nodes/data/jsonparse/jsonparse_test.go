package jsonparse

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
	module, ok := registry.Get("data:json_parse")
	if !ok {
		t.Fatal("data:json_parse was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	if config == nil {
		config = map[string]any{}
	}
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:json_parse", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:json_parse" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "text" || got.DataType != domain.DataText || got.Direction != domain.PinInput {
		t.Fatalf("text input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinOutput {
		t.Fatalf("value output = %#v", got)
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Name != "type" || definition.Fields[0].Kind != "select" {
		t.Fatalf("fields = %#v, want the root type select", definition.Fields)
	}
	if len(definition.Fields[0].Options) != len(rootTypes) {
		t.Fatalf("root type options = %#v, want %#v", definition.Fields[0].Options, rootTypes)
	}
	if got := definition.DefaultConfig["type"]; got != "object" {
		t.Fatalf("default root type = %#v, want object so new nodes parse objects without a Cast", got)
	}
}

func TestResolveTypesValueOutputByRootType(t *testing.T) {
	module := registeredModule(t)
	keyType := domain.TypeSpec{Kind: domain.TypeString}
	anyType := domain.TypeSpec{Kind: domain.TypeAny}
	mapType := domain.TypeSpec{Kind: domain.TypeMap, Key: &keyType, Value: &anyType}
	listType := domain.TypeSpec{Kind: domain.TypeList, Element: &anyType}
	tests := []struct {
		name     string
		config   map[string]any
		dataType domain.DataType
		typeSpec domain.TypeSpec
	}{
		{"object", map[string]any{"type": "object"}, domain.DataObject, mapType},
		{"list", map[string]any{"type": "list"}, domain.DataList, listType},
		{"text", map[string]any{"type": "text"}, domain.DataText, domain.TypeSpec{Kind: domain.TypeString}},
		{"number", map[string]any{"type": "number"}, domain.DataNumber, domain.TypeSpec{Kind: domain.TypeFloat}},
		{"boolean", map[string]any{"type": "boolean"}, domain.DataBoolean, domain.TypeSpec{Kind: domain.TypeBool}},
		{"any", map[string]any{"type": "any"}, domain.DataAny, domain.TypeSpec{Kind: domain.TypeAny}},
		{"legacy config without the key", map[string]any{}, domain.DataAny, domain.TypeSpec{Kind: domain.TypeAny}},
		{"nil config", nil, domain.DataAny, domain.TypeSpec{Kind: domain.TypeAny}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := module.Resolve(domain.FlowNode{Type: "data:json_parse", Data: map[string]any{"config": test.config}})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			output := resolved.Outputs[0]
			if output.DataType != test.dataType {
				t.Fatalf("output data type = %s, want %s", output.DataType, test.dataType)
			}
			if output.Type == nil || !reflect.DeepEqual(*output.Type, test.typeSpec) {
				t.Fatalf("output type spec = %#v, want %#v", output.Type, test.typeSpec)
			}
		})
	}
}

func TestResolveRejectsUnsupportedRootType(t *testing.T) {
	module := registeredModule(t)
	for _, config := range []map[string]any{
		{"type": "bytes"},
		{"type": "widget"},
	} {
		_, err := module.Resolve(domain.FlowNode{Type: "data:json_parse", Data: map[string]any{"config": config}})
		if err == nil || !strings.Contains(err.Error(), "not a supported JSON root type") {
			t.Fatalf("Resolve(%#v) error = %v, want the unsupported root type failure", config, err)
		}
	}
}

func TestEvaluateDecodesJSON(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name string
		text string
		want any
	}{
		{"object", `{"name":"Ada","tags":["a"],"nested":{"ok":true}}`, map[string]any{"name": "Ada", "tags": []any{"a"}, "nested": map[string]any{"ok": true}}},
		{"array", `[1, "two", true, null]`, []any{1.0, "two", true, nil}},
		{"number", `2`, 2.0},
		{"float number", `2.5`, 2.5},
		{"string", `"hello"`, "hello"},
		{"boolean", `true`, true},
		{"null", `null`, nil},
		{"leading and trailing whitespace", "  {\"a\": 1}  ", map[string]any{"a": 1.0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"text": test.text}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs, map[string]any{"value": test.want}) {
				t.Fatalf("outputs = %#v, want value %#v", result.Outputs, test.want)
			}
		})
	}
}

func TestEvaluateAcceptsDeclaredRootKind(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name   string
		config map[string]any
		text   string
		want   any
	}{
		{"object root", map[string]any{"type": "object"}, `{"ok":true}`, map[string]any{"ok": true}},
		{"list root", map[string]any{"type": "list"}, `[1,2]`, []any{1.0, 2.0}},
		{"text root", map[string]any{"type": "text"}, `"hello"`, "hello"},
		{"number root", map[string]any{"type": "number"}, `2.5`, 2.5},
		{"boolean root", map[string]any{"type": "boolean"}, `false`, false},
		{"any accepts a list anyway", map[string]any{"type": "any"}, `[1]`, []any{1.0}},
		{"legacy config accepts anything", map[string]any{}, `{"ok":1}`, map[string]any{"ok": 1.0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, test.config, map[string]any{"text": test.text}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs, map[string]any{"value": test.want}) {
				t.Fatalf("outputs = %#v, want value %#v", result.Outputs, test.want)
			}
		})
	}
}

func TestEvaluateRejectsRootKindMismatch(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name    string
		config  map[string]any
		text    string
		message string
	}{
		{"object root fed a list", map[string]any{"type": "object"}, `[1,2]`, "root is list, but Root type is object; set Root type to list or any"},
		{"object root fed text", map[string]any{"type": "object"}, `"nope"`, "root is text, but Root type is object"},
		{"object root fed null", map[string]any{"type": "object"}, `null`, "root is null, but Root type is object; set Root type to any"},
		{"list root fed an object", map[string]any{"type": "list"}, `{"a":1}`, "root is object, but Root type is list; set Root type to object or any"},
		{"text root fed a number", map[string]any{"type": "text"}, `2`, "root is number, but Root type is text; set Root type to number or any"},
		{"number root fed a boolean", map[string]any{"type": "number"}, `true`, "root is boolean, but Root type is number"},
		{"boolean root fed a list", map[string]any{"type": "boolean"}, `[]`, "root is list, but Root type is boolean"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, test.config, map[string]any{"text": test.text}), nil)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Execute() error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestEvaluateRejectsUnsupportedRootTypeConfig(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module, map[string]any{"type": "bytes"}, map[string]any{"text": `{}`}), nil)
	if err == nil || !strings.Contains(err.Error(), "not a supported JSON root type") {
		t.Fatalf("Execute() error = %v, want the unsupported root type failure", err)
	}
}

func TestEvaluateRequiresTextInput(t *testing.T) {
	module := registeredModule(t)
	for name, inputs := range map[string]map[string]any{
		"missing text":    {},
		"non-string text": {"text": 42.0},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, nil, inputs), nil)
			if err == nil || !strings.Contains(err.Error(), "parse JSON requires text input") {
				t.Fatalf("Execute() error = %v, want the text-input requirement", err)
			}
		})
	}
}

func TestEvaluateRejectsInvalidJSON(t *testing.T) {
	module := registeredModule(t)
	for _, text := range []string{"{oops}", "", `{"a":}`} {
		_, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"text": text}), nil)
		if err == nil || !strings.Contains(err.Error(), "parse JSON:") {
			t.Fatalf("Execute(%q) error = %v, want a parse failure", text, err)
		}
	}
}

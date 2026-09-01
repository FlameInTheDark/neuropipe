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

func invocation(module nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:json_parse", Data: map[string]any{"config": map[string]any{}}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          map[string]any{},
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
	if len(definition.Fields) != 0 {
		t.Fatalf("fields = %#v, want none", definition.Fields)
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
			result, err := module.Execute(context.Background(), invocation(module, map[string]any{"text": test.text}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs, map[string]any{"value": test.want}) {
				t.Fatalf("outputs = %#v, want value %#v", result.Outputs, test.want)
			}
		})
	}
}

func TestEvaluateRequiresTextInput(t *testing.T) {
	module := registeredModule(t)
	for name, inputs := range map[string]map[string]any{
		"missing text":    {},
		"non-string text": {"text": 42.0},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, inputs), nil)
			if err == nil || !strings.Contains(err.Error(), "parse JSON requires text input") {
				t.Fatalf("Execute() error = %v, want the text-input requirement", err)
			}
		})
	}
}

func TestEvaluateRejectsInvalidJSON(t *testing.T) {
	module := registeredModule(t)
	for _, text := range []string{"{oops}", "", `{"a":}`} {
		_, err := module.Execute(context.Background(), invocation(module, map[string]any{"text": text}), nil)
		if err == nil || !strings.Contains(err.Error(), "parse JSON:") {
			t.Fatalf("Execute(%q) error = %v, want a parse failure", text, err)
		}
	}
}

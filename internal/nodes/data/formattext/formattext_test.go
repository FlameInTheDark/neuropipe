package formattext

import (
	"context"
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
	module, ok := registry.Get("data:format_text")
	if !ok {
		t.Fatal("data:format_text was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:format_text", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:format_text" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinInput {
		t.Fatalf("value input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "text" || got.DataType != domain.DataText || got.Direction != domain.PinOutput {
		t.Fatalf("text output = %#v", got)
	}
	field := definition.Fields[0]
	if field.Name != "format" || field.Kind != "string" || !field.Required || field.Placeholder != "{{.value}}" {
		t.Fatalf("format field = %#v", field)
	}
	if got, want := definition.DefaultConfig["format"], "{{.value}}"; got != want {
		t.Fatalf("default format = %#v, want %#v", got, want)
	}
}

func TestEvaluateRendersConfiguredTemplate(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name   string
		format string
		inputs map[string]any
		want   string
	}{
		{"substitutes the value", "Hello, {{.value}}!", map[string]any{"value": "World"}, "Hello, World!"},
		{"reads nested object fields", "Name: {{.value.name}}", map[string]any{"value": map[string]any{"name": "Ada"}}, "Name: Ada"},
		{"renders whole objects", "got {{.value}}", map[string]any{"value": map[string]any{"name": "Ada"}}, "got map[name:Ada]"},
		{"renders float numbers", "n={{.value}}", map[string]any{"value": 7.5}, "n=7.5"},
		{"renders integers", "n={{.value}}", map[string]any{"value": 7}, "n=7"},
		{"renders booleans", "ok={{.value}}", map[string]any{"value": true}, "ok=true"},
		{"repeats references", "{{.value}} and {{.value}}", map[string]any{"value": "x"}, "x and x"},
		{"supports template functions", "len={{len .value}}", map[string]any{"value": "neuro"}, "len=5"},
		{"indexes into lists", "first={{index .value 0}}", map[string]any{"value": []any{"a", "b"}}, "first=a"},
		{"missing input renders no value", "[{{.value}}]", map[string]any{}, "[<no value>]"},
		{"empty format renders empty text", "", map[string]any{"value": "x"}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, map[string]any{"format": test.format}, test.inputs), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got, ok := result.Outputs["text"].(string); !ok || got != test.want {
				t.Fatalf("text = %#v, want %q", result.Outputs["text"], test.want)
			}
		})
	}
}

func TestEvaluateRejectsUnparseableTemplate(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module, map[string]any{"format": "{{."}, map[string]any{"value": "x"}), nil)
	if err == nil || !strings.Contains(err.Error(), "incorrect format template") {
		t.Fatalf("Execute() error = %v, want incorrect format template", err)
	}
}

func TestEvaluateRejectsBadFieldAccess(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module, map[string]any{"format": "{{.value.Name}}"}, map[string]any{"value": 42}), nil)
	if err == nil || !strings.Contains(err.Error(), "unable to execute template") {
		t.Fatalf("Execute() error = %v, want unable to execute template", err)
	}
}

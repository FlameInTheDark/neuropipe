package getfield

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
	module, ok := registry.Get("data:get_field")
	if !ok {
		t.Fatal("data:get_field was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:get_field", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func outputConfig(entries ...map[string]any) map[string]any {
	items := make([]any, len(entries))
	for index, entry := range entries {
		items[index] = entry
	}
	return map[string]any{"outputs": items}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:get_field" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "source" || got.DataType != domain.DataAny || got.Direction != domain.PinInput {
		t.Fatalf("source input = %#v", got)
	}
	if len(definition.Outputs) != 0 {
		t.Fatalf("static outputs = %#v, want none until the resolver expands the configured pins", definition.Outputs)
	}
	field := definition.Fields[0]
	if field.Name != "outputs" || field.Kind != "field-outputs" || !field.Required {
		t.Fatalf("outputs field = %#v", field)
	}
	defaults, ok := definition.DefaultConfig["outputs"].([]any)
	if !ok || len(defaults) != 1 {
		t.Fatalf("default outputs = %#v, want the single value/any default", definition.DefaultConfig["outputs"])
	}
}

func TestResolveExpandsConfiguredOutputs(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:get_field", Data: map[string]any{"config": outputConfig(
		map[string]any{"id": "command", "label": "Command", "path": "terminal.command", "dataType": "text"},
		map[string]any{"id": "ready", "label": "Ready", "path": "ready", "dataType": "boolean"},
	)}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := []domain.NodePort{
		{ID: "command", Label: "Command", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &domain.TypeSpec{Kind: domain.TypeString}, Color: "#e879f9", MaxConnections: 1},
		{ID: "ready", Label: "Ready", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataBoolean, Type: &domain.TypeSpec{Kind: domain.TypeBool}, Color: "#f87171", MaxConnections: 1},
	}
	if !reflect.DeepEqual(resolved.Outputs, want) {
		t.Fatalf("resolved outputs = %#v, want %#v", resolved.Outputs, want)
	}
}

func TestResolveFallsBackToDefaultOutput(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:get_field"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(resolved.Outputs) != 1 || resolved.Outputs[0].ID != "value" || resolved.Outputs[0].DataType != domain.DataAny {
		t.Fatalf("default outputs = %#v, want the value/any default pin", resolved.Outputs)
	}
}

func TestResolveRejectsInvalidOutputs(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name    string
		config  map[string]any
		message string
	}{
		{"outputs not a list", map[string]any{"outputs": "command"}, "outputs must be a list"},
		{"outputs empty", map[string]any{"outputs": []any{}}, "add at least one output"},
		{"duplicate IDs", outputConfig(
			map[string]any{"id": "command", "label": "Command", "path": "a", "dataType": "any"},
			map[string]any{"id": "command", "label": "Command", "path": "b", "dataType": "any"},
		), `duplicate ID "command"`},
		{"missing id", outputConfig(map[string]any{"id": "  ", "label": "No ID", "path": "a", "dataType": "any"}), "needs an ID"},
		{"unsupported data type", outputConfig(map[string]any{"id": "x", "label": "X", "path": "a", "dataType": "sound"}), `unsupported data type "sound"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := module.Resolve(domain.FlowNode{Type: "data:get_field", Data: map[string]any{"config": test.config}}); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Resolve() error = %v, want it to contain %q", err, test.message)
			}
		})
	}
}

func TestEvaluateExtractsConfiguredOutputs(t *testing.T) {
	module := registeredModule(t)
	config := outputConfig(
		map[string]any{"id": "command", "label": "Command", "path": "terminal.command", "dataType": "text"},
		map[string]any{"id": "ready", "label": "Ready", "path": "ready", "dataType": "boolean"},
	)
	source := map[string]any{"terminal": map[string]any{"command": "Get-Date"}, "ready": true}
	result, err := module.Execute(context.Background(), invocation(module, config, map[string]any{"source": source}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{"command": "Get-Date", "ready": true}
	if !reflect.DeepEqual(result.Outputs, want) {
		t.Fatalf("outputs = %#v, want %#v", result.Outputs, want)
	}
}

func TestEvaluateUsesDefaultOutputWithoutConfig(t *testing.T) {
	module := registeredModule(t)
	// An unconfigured node falls back to the definition's default value/any
	// output, which extracts the "value" member of the source object.
	result, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"source": map[string]any{"value": 42.0}}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(result.Outputs, map[string]any{"value": 42.0}) {
		t.Fatalf("outputs = %#v, want the default value pick", result.Outputs)
	}
}

func TestEvaluateSupportsListIndexesAndWholeSource(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name   string
		path   string
		source any
		want   map[string]any
	}{
		{"list index", "items.0", map[string]any{"items": []any{"a", "b"}}, map[string]any{"pick": "a"}},
		{"nested path into list member", "items.1.name", map[string]any{"items": []any{map[string]any{"name": "Ada"}, map[string]any{"name": "Bob"}}}, map[string]any{"pick": "Bob"}},
		{"empty path returns the source", "", map[string]any{"a": 1.0}, map[string]any{"pick": map[string]any{"a": 1.0}}},
		{"missing path yields nil", "missing.deep", map[string]any{"a": 1.0}, map[string]any{"pick": nil}},
		{"list source with numeric path", "0", []any{"a", "b"}, map[string]any{"pick": "a"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := outputConfig(map[string]any{"id": "pick", "label": "Pick", "path": test.path, "dataType": "any"})
			result, err := module.Execute(context.Background(), invocation(module, config, map[string]any{"source": test.source}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs, test.want) {
				t.Fatalf("outputs = %#v, want %#v", result.Outputs, test.want)
			}
		})
	}
}

func TestEvaluateRejectsIncompatibleDeclaredType(t *testing.T) {
	module := registeredModule(t)
	config := outputConfig(map[string]any{"id": "count", "label": "Count", "path": "count", "dataType": "number"})
	_, err := module.Execute(context.Background(), invocation(module, config, map[string]any{"source": map[string]any{"count": "many"}}), nil)
	if err == nil || !strings.Contains(err.Error(), `get field output "Count" is declared number, but "count" is incompatible`) {
		t.Fatalf("Execute() error = %v, want a declared-type mismatch", err)
	}
}

func TestEvaluateRejectsMissingValueForStrictType(t *testing.T) {
	module := registeredModule(t)
	config := outputConfig(map[string]any{"id": "ready", "label": "Ready", "path": "missing", "dataType": "boolean"})
	_, err := module.Execute(context.Background(), invocation(module, config, map[string]any{"source": map[string]any{}}), nil)
	if err == nil || !strings.Contains(err.Error(), "is declared boolean") {
		t.Fatalf("Execute() error = %v, want a declared-type mismatch for the nil pick", err)
	}
}

func TestEvaluateRejectsInvalidOutputsConfig(t *testing.T) {
	module := registeredModule(t)
	if _, err := module.Execute(context.Background(), invocation(module, map[string]any{"outputs": "command"}, map[string]any{"source": map[string]any{}}), nil); err == nil || !strings.Contains(err.Error(), "outputs must be a list") {
		t.Fatalf("Execute() error = %v, want outputs must be a list", err)
	}
	if _, err := module.Execute(context.Background(), invocation(module, map[string]any{"outputs": []any{}}, map[string]any{"source": map[string]any{}}), nil); err == nil || !strings.Contains(err.Error(), "add at least one output") {
		t.Fatalf("Execute() error = %v, want add at least one output", err)
	}
}

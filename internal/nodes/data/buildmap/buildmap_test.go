package buildmap

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
	module, ok := registry.Get("data:build_map")
	if !ok {
		t.Fatal("data:build_map was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:build_map", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func entryConfig(entries ...map[string]any) map[string]any {
	items := make([]any, len(entries))
	for index, entry := range entries {
		items[index] = entry
	}
	return map[string]any{"entries": items}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:build_map" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if len(definition.Inputs) != 0 {
		t.Fatalf("static inputs = %#v, want none until the resolver expands the configured entries", definition.Inputs)
	}
	if got := definition.Outputs[0]; got.ID != "map" || got.DataType != domain.DataObject || got.Direction != domain.PinOutput {
		t.Fatalf("map output = %#v", got)
	}
	if len(definition.Fields) != 2 {
		t.Fatalf("fields = %#v, want the value type select plus the entries editor", definition.Fields)
	}
	typeField := definition.Fields[0]
	if typeField.Name != "valueType" || typeField.Kind != "select" || len(typeField.Options) != 7 {
		t.Fatalf("valueType field = %#v", typeField)
	}
	if typeField.Options[0].Value != "any" {
		t.Fatalf("first value type option = %q, want any", typeField.Options[0].Value)
	}
	field := definition.Fields[1]
	if field.Name != "entries" || field.Kind != "map-entries" || !field.Required {
		t.Fatalf("entries field = %#v", field)
	}
	if got := definition.DefaultConfig["valueType"]; got != "any" {
		t.Fatalf("default valueType = %#v, want any", got)
	}
	defaults, ok := definition.DefaultConfig["entries"].([]any)
	if !ok || len(defaults) != 1 {
		t.Fatalf("default entries = %#v, want the single value default", definition.DefaultConfig["entries"])
	}
}

func TestResolveExpandsConfiguredEntryPins(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:build_map", Data: map[string]any{"config": map[string]any{"valueType": "text", "entries": []any{
		map[string]any{"id": "ident", "label": "Id", "key": "id"},
		map[string]any{"id": "currency", "label": "Currency", "key": "currency", "value": "EUR"},
	}}}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := []domain.NodePort{
		{ID: "entry_ident", Label: "Id", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &domain.TypeSpec{Kind: domain.TypeString}, Color: "#e879f9", MaxConnections: 1, Required: true},
		{ID: "entry_currency", Label: "Currency", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &domain.TypeSpec{Kind: domain.TypeString}, Color: "#e879f9", MaxConnections: 1, Default: "EUR"},
	}
	if !reflect.DeepEqual(resolved.Inputs, want) {
		t.Fatalf("resolved inputs = %#v, want %#v", resolved.Inputs, want)
	}
}

func TestResolveRefinesTypedOutput(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:build_map", Data: map[string]any{"config": map[string]any{"valueType": "number", "entries": []any{
		map[string]any{"id": "total", "key": "total"},
	}}}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	output := resolved.Outputs[0]
	if output.ID != "map" || output.DataType != domain.DataObject {
		t.Fatalf("map output = %#v", output)
	}
	keyType := domain.TypeSpec{Kind: domain.TypeString}
	valueType := domain.TypeSpec{Kind: domain.TypeFloat}
	want := domain.TypeSpec{Kind: domain.TypeMap, Key: &keyType, Value: &valueType}
	if output.Type == nil || !reflect.DeepEqual(*output.Type, want) {
		t.Fatalf("typed output spec = %#v, want %#v", output.Type, want)
	}
}

func TestResolveUsesKeyAsLabelFallback(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:build_map", Data: map[string]any{"config": entryConfig(
		map[string]any{"id": "one", "key": "total"},
	)}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Inputs[0].Label != "total" {
		t.Fatalf("label fallback = %q, want the key", resolved.Inputs[0].Label)
	}
}

func TestResolveFallsBackToDefaultEntry(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:build_map"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(resolved.Inputs) != 1 || resolved.Inputs[0].ID != "entry_value" || resolved.Inputs[0].Label != "Value" {
		t.Fatalf("default inputs = %#v, want the value default pin", resolved.Inputs)
	}
}

func TestResolveRejectsInvalidConfigs(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name    string
		config  map[string]any
		message string
	}{
		{"entries not a list", map[string]any{"entries": "id"}, "entries must be a list"},
		{"entries empty", map[string]any{"entries": []any{}}, "add at least one entry"},
		{"only blank keys", entryConfig(map[string]any{"id": "one", "key": " "}), "add at least one entry"},
		{"duplicate keys", entryConfig(
			map[string]any{"id": "one", "key": "id"},
			map[string]any{"id": "two", "key": "id"},
		), `duplicate key "id"`},
		{"duplicate ids", entryConfig(
			map[string]any{"id": "one", "key": "a"},
			map[string]any{"id": "one", "key": "b"},
		), `duplicate row ID "one"`},
		{"reserved prefix", entryConfig(map[string]any{"id": "entry_one", "key": "a"}), `remove the "entry_" prefix`},
		{"unsupported value type", map[string]any{"valueType": "sound", "entries": []any{map[string]any{"id": "x", "key": "a"}}}, `valueType "sound" is not a supported data type`},
		{"number constant not numeric", map[string]any{"valueType": "number", "entries": []any{map[string]any{"id": "x", "key": "a", "value": "abc"}}}, `is not a number`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := module.Resolve(domain.FlowNode{Type: "data:build_map", Data: map[string]any{"config": test.config}}); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Resolve() error = %v, want it to contain %q", err, test.message)
			}
		})
	}
}

func TestEvaluateAssemblesMixedValuesUnderAny(t *testing.T) {
	module := registeredModule(t)
	config := map[string]any{"valueType": "any", "entries": []any{
		map[string]any{"id": "ident", "label": "Id", "key": "id"},
		map[string]any{"id": "total", "label": "Total", "key": "total"},
		map[string]any{"id": "currency", "label": "Currency", "key": "currency", "value": "EUR"},
	}}
	result, err := module.Execute(context.Background(), invocation(module, config, map[string]any{
		"entry_ident": "A-101",
		"entry_total": float64(42),
	}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{"map": map[string]any{"id": "A-101", "total": float64(42), "currency": "EUR"}}
	if !reflect.DeepEqual(result.Outputs, want) {
		t.Fatalf("outputs = %#v, want %#v", result.Outputs, want)
	}
}

func TestEvaluateHomogeneousTextValues(t *testing.T) {
	module := registeredModule(t)
	config := map[string]any{"valueType": "text", "entries": []any{
		map[string]any{"id": "ident", "label": "Id", "key": "id"},
		map[string]any{"id": "currency", "label": "Currency", "key": "currency", "value": "EUR"},
	}}
	result, err := module.Execute(context.Background(), invocation(module, config, map[string]any{
		"entry_ident": "A-101",
	}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{"map": map[string]any{"id": "A-101", "currency": "EUR"}}
	if !reflect.DeepEqual(result.Outputs, want) {
		t.Fatalf("outputs = %#v, want %#v", result.Outputs, want)
	}
}

func TestEvaluateKeepsVerbatimKeys(t *testing.T) {
	module := registeredModule(t)
	config := map[string]any{"valueType": "any", "entries": []any{
		map[string]any{"id": "odd", "label": "Odd", "key": "a.b c", "value": "flat"},
	}}
	result, err := module.Execute(context.Background(), invocation(module, config, map[string]any{}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{"map": map[string]any{"a.b c": "flat"}}
	if !reflect.DeepEqual(result.Outputs, want) {
		t.Fatalf("outputs = %#v, want the verbatim flat key %#v", result.Outputs, want)
	}
}

func TestEvaluateErrorsOnEntryWithoutValue(t *testing.T) {
	module := registeredModule(t)
	config := map[string]any{"valueType": "text", "entries": []any{
		map[string]any{"id": "ident", "label": "Id", "key": "id"},
	}}
	_, err := module.Execute(context.Background(), invocation(module, config, map[string]any{}), nil)
	if err == nil || !strings.Contains(err.Error(), `entry "Id" has no value`) {
		t.Fatalf("Execute() error = %v, want the missing-entry error", err)
	}
}

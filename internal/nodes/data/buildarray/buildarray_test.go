package buildarray

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
	module, ok := registry.Get("data:build_array")
	if !ok {
		t.Fatal("data:build_array was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:build_array", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func itemConfig(entries ...map[string]any) map[string]any {
	items := make([]any, len(entries))
	for index, entry := range entries {
		items[index] = entry
	}
	return map[string]any{"items": items}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:build_array" || definition.Mode != domain.NodePure || definition.Category != "Arrays" {
		t.Fatalf("definition header = %#v", definition)
	}
	if len(definition.Inputs) != 0 {
		t.Fatalf("static inputs = %#v, want none until the resolver expands the configured items", definition.Inputs)
	}
	if got := definition.Outputs[0]; got.ID != "array" || got.DataType != domain.DataList || got.Direction != domain.PinOutput {
		t.Fatalf("array output = %#v", got)
	}
	if len(definition.Fields) != 2 {
		t.Fatalf("fields = %#v, want the element type select plus the items editor", definition.Fields)
	}
	typeField := definition.Fields[0]
	if typeField.Name != "elementType" || typeField.Kind != "select" || len(typeField.Options) != 7 {
		t.Fatalf("elementType field = %#v", typeField)
	}
	if typeField.Options[0].Value != "any" {
		t.Fatalf("first element type option = %q, want any", typeField.Options[0].Value)
	}
	field := definition.Fields[1]
	if field.Name != "items" || field.Kind != "array-items" || !field.Required {
		t.Fatalf("items field = %#v", field)
	}
	if got := definition.DefaultConfig["elementType"]; got != "any" {
		t.Fatalf("default elementType = %#v, want any", got)
	}
	defaults, ok := definition.DefaultConfig["items"].([]any)
	if !ok || len(defaults) != 1 {
		t.Fatalf("default items = %#v, want the single first default", definition.DefaultConfig["items"])
	}
}

func TestResolveExpandsConfiguredItemPins(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:build_array", Data: map[string]any{"config": map[string]any{"elementType": "text", "items": []any{
		map[string]any{"id": "first", "label": "First", "value": "Weekly digest"},
		map[string]any{"id": "total", "label": "Total"},
	}}}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := []domain.NodePort{
		{ID: "item_first", Label: "First", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &domain.TypeSpec{Kind: domain.TypeString}, Color: "#e879f9", MaxConnections: 1, Default: "Weekly digest"},
		{ID: "item_total", Label: "Total", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &domain.TypeSpec{Kind: domain.TypeString}, Color: "#e879f9", MaxConnections: 1, Required: true},
	}
	if !reflect.DeepEqual(resolved.Inputs, want) {
		t.Fatalf("resolved inputs = %#v, want %#v", resolved.Inputs, want)
	}
}

func TestResolveRefinesTypedOutput(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:build_array", Data: map[string]any{"config": map[string]any{"elementType": "text", "items": []any{
		map[string]any{"id": "first", "label": "First", "value": "Weekly digest"},
	}}}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	output := resolved.Outputs[0]
	if output.ID != "array" || output.DataType != domain.DataList {
		t.Fatalf("array output = %#v", output)
	}
	element := domain.TypeSpec{Kind: domain.TypeString}
	want := domain.TypeSpec{Kind: domain.TypeList, Element: &element}
	if output.Type == nil || !reflect.DeepEqual(*output.Type, want) {
		t.Fatalf("typed output spec = %#v, want %#v", output.Type, want)
	}
}

func TestResolveKeepsPlainListOutputForAny(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:build_array", Data: map[string]any{"config": map[string]any{"items": []any{
		map[string]any{"id": "first", "label": "First", "value": "Weekly digest"},
	}}}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	output := resolved.Outputs[0]
	if output.Type == nil || output.Type.Kind != domain.TypeList || output.Type.Element == nil || output.Type.Element.Kind != domain.TypeAny {
		t.Fatalf("any output spec = %#v, want list<any>", output.Type)
	}
	if resolved.Inputs[0].DataType != domain.DataAny {
		t.Fatalf("any element pin = %#v, want any-typed", resolved.Inputs[0])
	}
}

func TestResolveFallsBackToDefaultItem(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:build_array"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(resolved.Inputs) != 1 || resolved.Inputs[0].ID != "item_first" || resolved.Inputs[0].Label != "First" || resolved.Inputs[0].DataType != domain.DataAny {
		t.Fatalf("default inputs = %#v, want the first/any default pin", resolved.Inputs)
	}
}

func TestResolveRejectsInvalidConfigs(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name    string
		config  map[string]any
		message string
	}{
		{"items not a list", map[string]any{"items": "first"}, "items must be a list"},
		{"items empty", map[string]any{"items": []any{}}, "add at least one item"},
		{"duplicate ids", itemConfig(
			map[string]any{"id": "first", "label": "A"},
			map[string]any{"id": "first", "label": "B"},
		), `duplicate row ID "first"`},
		{"reserved prefix", itemConfig(map[string]any{"id": "item_first", "label": "A"}), `remove the "item_" prefix`},
		{"unsupported element type", map[string]any{"elementType": "sound", "items": []any{map[string]any{"id": "x", "label": "X"}}}, `elementType "sound" is not a supported data type`},
		{"number constant not numeric", map[string]any{"elementType": "number", "items": []any{map[string]any{"id": "x", "label": "X", "value": "abc"}}}, `is not a number`},
		{"boolean constant invalid", map[string]any{"elementType": "boolean", "items": []any{map[string]any{"id": "x", "label": "X", "value": "yes"}}}, `is not a boolean`},
		{"object constant invalid JSON", map[string]any{"elementType": "object", "items": []any{map[string]any{"id": "x", "label": "X", "value": "{oops"}}}, "not a JSON object"},
		{"list constant invalid JSON", map[string]any{"elementType": "list", "items": []any{map[string]any{"id": "x", "label": "X", "value": "[1,"}}}, "not a JSON list"},
		{"bytes constants unsupported", map[string]any{"elementType": "bytes", "items": []any{map[string]any{"id": "x", "label": "X", "value": "00"}}}, "constants are not supported for bytes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := module.Resolve(domain.FlowNode{Type: "data:build_array", Data: map[string]any{"config": test.config}}); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Resolve() error = %v, want it to contain %q", err, test.message)
			}
		})
	}
}

func TestResolveRejectsRowCap(t *testing.T) {
	module := registeredModule(t)
	items := make([]any, 33)
	for index := range items {
		items[index] = map[string]any{"id": string(rune('a'+index%26)) + string(rune('a'+index/26)), "label": "Row"}
	}
	_, err := module.Resolve(domain.FlowNode{Type: "data:build_array", Data: map[string]any{"config": map[string]any{"items": items}}})
	if err == nil || !strings.Contains(err.Error(), "at most 32") {
		t.Fatalf("Resolve() error = %v, want the 32-row cap", err)
	}
}

func TestEvaluateAssemblesMixedValuesUnderAny(t *testing.T) {
	module := registeredModule(t)
	config := map[string]any{"elementType": "any", "items": []any{
		map[string]any{"id": "first", "label": "First"},
		map[string]any{"id": "total", "label": "Total"},
		map[string]any{"id": "flag", "label": "Flag", "value": "false"},
	}}
	result, err := module.Execute(context.Background(), invocation(module, config, map[string]any{
		"item_first": "wired title",
		"item_total": float64(42),
	}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{"array": []any{"wired title", float64(42), "false"}}
	if !reflect.DeepEqual(result.Outputs, want) {
		t.Fatalf("outputs = %#v, want %#v", result.Outputs, want)
	}
}

func TestEvaluateAppliesTypedConstantWithoutWire(t *testing.T) {
	module := registeredModule(t)
	config := map[string]any{"elementType": "number", "items": []any{
		map[string]any{"id": "first", "label": "First", "value": "7"},
	}}
	result, err := module.Execute(context.Background(), invocation(module, config, map[string]any{}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{"array": []any{float64(7)}}
	if !reflect.DeepEqual(result.Outputs, want) {
		t.Fatalf("outputs = %#v, want %#v", result.Outputs, want)
	}
}

func TestEvaluateErrorsOnItemWithoutValue(t *testing.T) {
	module := registeredModule(t)
	config := map[string]any{"elementType": "number", "items": []any{
		map[string]any{"id": "total", "label": "Total"},
	}}
	_, err := module.Execute(context.Background(), invocation(module, config, map[string]any{}), nil)
	if err == nil || !strings.Contains(err.Error(), `item "Total" has no value`) {
		t.Fatalf("Execute() error = %v, want the missing-item error", err)
	}
}

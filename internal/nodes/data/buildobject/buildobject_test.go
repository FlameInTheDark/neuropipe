package buildobject

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
	module, ok := registry.Get("data:build_object")
	if !ok {
		t.Fatal("data:build_object was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:build_object", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func fieldConfig(entries ...map[string]any) map[string]any {
	items := make([]any, len(entries))
	for index, entry := range entries {
		items[index] = entry
	}
	return map[string]any{"fields": items}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:build_object" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if len(definition.Inputs) != 0 {
		t.Fatalf("static inputs = %#v, want none until the resolver expands the configured fields", definition.Inputs)
	}
	if got := definition.Outputs[0]; got.ID != "object" || got.DataType != domain.DataObject || got.Direction != domain.PinOutput {
		t.Fatalf("object output = %#v", got)
	}
	field := definition.Fields[0]
	if field.Name != "fields" || field.Kind != "object-fields" || !field.Required {
		t.Fatalf("fields field = %#v", field)
	}
	defaults, ok := definition.DefaultConfig["fields"].([]any)
	if !ok || len(defaults) != 1 {
		t.Fatalf("default fields = %#v, want the single value/any default", definition.DefaultConfig["fields"])
	}
}

func TestResolveExpandsConfiguredInputPins(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:build_object", Data: map[string]any{"config": fieldConfig(
		map[string]any{"id": "name", "label": "Name", "key": "name", "dataType": "text"},
		map[string]any{"id": "age", "label": "Age", "key": "profile.age", "dataType": "number"},
	)}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := []domain.NodePort{
		{ID: "name", Label: "Name", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &domain.TypeSpec{Kind: domain.TypeString}, Color: "#e879f9", MaxConnections: 1},
		{ID: "age", Label: "Age", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataNumber, Type: &domain.TypeSpec{Kind: domain.TypeFloat}, Color: "#86efac", MaxConnections: 1},
	}
	if !reflect.DeepEqual(resolved.Inputs, want) {
		t.Fatalf("resolved inputs = %#v, want %#v", resolved.Inputs, want)
	}
	if got := resolved.Outputs[0]; got.ID != "object" || got.DataType != domain.DataObject {
		t.Fatalf("object output after resolve = %#v", got)
	}
}

func TestResolveFallsBackToDefaultField(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:build_object"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(resolved.Inputs) != 1 || resolved.Inputs[0].ID != "value" || resolved.Inputs[0].DataType != domain.DataAny {
		t.Fatalf("default inputs = %#v, want the value/any default pin", resolved.Inputs)
	}
}

func TestResolveRejectsInvalidFields(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name    string
		config  map[string]any
		message string
	}{
		{"fields not a list", map[string]any{"fields": "name"}, "fields must be a list"},
		{"fields empty", map[string]any{"fields": []any{}}, "add at least one field"},
		{"duplicate ids", fieldConfig(
			map[string]any{"id": "name", "label": "Name", "key": "a", "dataType": "any"},
			map[string]any{"id": "name", "label": "Name", "key": "b", "dataType": "any"},
		), `duplicate ID "name"`},
		{"missing id", fieldConfig(map[string]any{"id": " ", "label": "No ID", "key": "a", "dataType": "any"}), "needs an ID"},
		{"missing object key", fieldConfig(map[string]any{"id": "x", "label": "X", "key": " ", "dataType": "any"}), "needs an object key"},
		{"invalid key path", fieldConfig(map[string]any{"id": "x", "label": "X", "key": "a..b", "dataType": "any"}), `invalid object key path "a..b"`},
		{"duplicate object keys", fieldConfig(
			map[string]any{"id": "one", "label": "One", "key": "same", "dataType": "any"},
			map[string]any{"id": "two", "label": "Two", "key": "same", "dataType": "any"},
		), `duplicate object key "same"`},
		{"overlapping key paths", fieldConfig(
			map[string]any{"id": "one", "label": "One", "key": "profile", "dataType": "any"},
			map[string]any{"id": "two", "label": "Two", "key": "profile.age", "dataType": "number"},
		), `object keys "profile" and "profile.age" overlap`},
		{"unsupported data type", fieldConfig(map[string]any{"id": "x", "label": "X", "key": "a", "dataType": "sound"}), `unsupported data type "sound"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := module.Resolve(domain.FlowNode{Type: "data:build_object", Data: map[string]any{"config": test.config}}); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Resolve() error = %v, want it to contain %q", err, test.message)
			}
		})
	}
}

func TestEvaluateBuildsNestedObject(t *testing.T) {
	module := registeredModule(t)
	config := fieldConfig(
		map[string]any{"id": "name", "label": "Name", "key": "name", "dataType": "text"},
		map[string]any{"id": "age", "label": "Age", "key": "profile.age", "dataType": "number"},
		map[string]any{"id": "city", "label": "City", "key": "profile.address.city", "dataType": "text"},
	)
	result, err := module.Execute(context.Background(), invocation(module, config, map[string]any{
		"name": "Ada",
		"age":  36.0,
		"city": "London",
	}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{
		"name": "Ada",
		"profile": map[string]any{
			"age": 36.0,
			"address": map[string]any{
				"city": "London",
			},
		},
	}
	if !reflect.DeepEqual(result.Outputs, map[string]any{"object": want}) {
		t.Fatalf("outputs = %#v, want %#v", result.Outputs, want)
	}
}

func TestEvaluateUsesDefaultFieldWithoutConfig(t *testing.T) {
	module := registeredModule(t)
	// An unconfigured node falls back to the definition's default value/any
	// field, so the object mirrors whatever the value pin receives.
	result, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"value": 42.0}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(result.Outputs, map[string]any{"object": map[string]any{"value": 42.0}}) {
		t.Fatalf("outputs = %#v, want the default value object", result.Outputs)
	}
}

func TestEvaluateStoresMissingInputsAsNil(t *testing.T) {
	module := registeredModule(t)
	config := fieldConfig(
		map[string]any{"id": "name", "label": "Name", "key": "name", "dataType": "any"},
		map[string]any{"id": "age", "label": "Age", "key": "age", "dataType": "any"},
	)
	result, err := module.Execute(context.Background(), invocation(module, config, map[string]any{"name": "Ada"}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{"object": map[string]any{"name": "Ada", "age": nil}}
	if !reflect.DeepEqual(result.Outputs, want) {
		t.Fatalf("outputs = %#v, want the unconnected field stored as nil", result.Outputs)
	}
}

func TestEvaluateRejectsInvalidFieldsConfig(t *testing.T) {
	module := registeredModule(t)
	if _, err := module.Execute(context.Background(), invocation(module, map[string]any{"fields": "name"}, map[string]any{}), nil); err == nil || !strings.Contains(err.Error(), "fields must be a list") {
		t.Fatalf("Execute() error = %v, want fields must be a list", err)
	}
	if _, err := module.Execute(context.Background(), invocation(module, map[string]any{"fields": []any{}}, map[string]any{}), nil); err == nil || !strings.Contains(err.Error(), "add at least one field") {
		t.Fatalf("Execute() error = %v, want add at least one field", err)
	}
	if _, err := module.Execute(context.Background(), invocation(module, fieldConfig(map[string]any{"id": "x", "label": "X", "key": "a..b", "dataType": "any"}), map[string]any{}), nil); err == nil || !strings.Contains(err.Error(), "invalid object key path") {
		t.Fatalf("Execute() error = %v, want invalid object key path", err)
	}
}

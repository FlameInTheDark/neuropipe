package breakobject

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
	module, ok := registry.Get("data:break_object")
	if !ok {
		t.Fatal("data:break_object was not registered")
	}
	return module
}

func invocation(module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:break_object", Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
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
	if definition.Type != "data:break_object" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "source" || got.DataType != domain.DataObject || got.Direction != domain.PinInput {
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
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:break_object", Data: map[string]any{"config": outputConfig(
		map[string]any{"id": "city", "label": "City", "path": "address.city", "dataType": "text"},
		map[string]any{"id": "tags", "label": "Tags", "path": "tags", "dataType": "list"},
	)}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := []domain.NodePort{
		{ID: "city", Label: "City", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &domain.TypeSpec{Kind: domain.TypeString}, Color: "#e879f9", MaxConnections: 1},
		{ID: "tags", Label: "Tags", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataList, Type: &domain.TypeSpec{Kind: domain.TypeList, Element: &domain.TypeSpec{Kind: domain.TypeAny}}, Color: "#facc15", MaxConnections: 1},
	}
	if !reflect.DeepEqual(resolved.Outputs, want) {
		t.Fatalf("resolved outputs = %#v, want %#v", resolved.Outputs, want)
	}
	// The source input survives the dynamic output expansion.
	if got := resolved.Inputs[0]; got.ID != "source" || got.DataType != domain.DataObject {
		t.Fatalf("source input after resolve = %#v", got)
	}
}

func TestResolveFallsBackToDefaultOutput(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:break_object"})
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
		{"outputs not a list", map[string]any{"outputs": "city"}, "outputs must be a list"},
		{"outputs empty", map[string]any{"outputs": []any{}}, "add at least one output"},
		{"duplicate ids", outputConfig(
			map[string]any{"id": "city", "label": "City", "path": "a", "dataType": "any"},
			map[string]any{"id": "city", "label": "City", "path": "b", "dataType": "any"},
		), `duplicate ID "city"`},
		{"missing id", outputConfig(map[string]any{"id": " ", "label": "No ID", "path": "a", "dataType": "any"}), "needs an ID"},
		{"unsupported data type", outputConfig(map[string]any{"id": "x", "label": "X", "path": "a", "dataType": "sound"}), `unsupported data type "sound"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := module.Resolve(domain.FlowNode{Type: "data:break_object", Data: map[string]any{"config": test.config}}); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Resolve() error = %v, want it to contain %q", err, test.message)
			}
		})
	}
}

func TestEvaluateExtractsConfiguredPaths(t *testing.T) {
	module := registeredModule(t)
	source := map[string]any{
		"address": map[string]any{"city": "Oslo", "zip": "0150"},
		"tags":    []any{"alpha", "beta"},
		"items":   []any{map[string]any{"name": "Ada"}},
		"count":   3.0,
	}
	tests := []struct {
		name   string
		config map[string]any
		want   map[string]any
	}{
		{
			"dotted path and typed picks",
			outputConfig(
				map[string]any{"id": "city", "label": "City", "path": "address.city", "dataType": "text"},
				map[string]any{"id": "count", "label": "Count", "path": "count", "dataType": "number"},
			),
			map[string]any{"city": "Oslo", "count": 3.0},
		},
		{
			"list index path",
			outputConfig(map[string]any{"id": "first", "label": "First", "path": "items.0.name", "dataType": "text"}),
			map[string]any{"first": "Ada"},
		},
		{
			"whole list passthrough",
			outputConfig(map[string]any{"id": "tags", "label": "Tags", "path": "tags", "dataType": "list"}),
			map[string]any{"tags": []any{"alpha", "beta"}},
		},
		{
			"missing path yields nil for any",
			outputConfig(map[string]any{"id": "pick", "label": "Pick", "path": "missing.deep", "dataType": "any"}),
			map[string]any{"pick": nil},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation(module, test.config, map[string]any{"source": source}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs, test.want) {
				t.Fatalf("outputs = %#v, want %#v", result.Outputs, test.want)
			}
		})
	}
}

func TestEvaluateUsesDefaultOutputWithoutConfig(t *testing.T) {
	module := registeredModule(t)
	// An unconfigured node falls back to the definition's default value/any
	// output, which extracts the "value" member of the source object.
	result, err := module.Execute(context.Background(), invocation(module, nil, map[string]any{"source": map[string]any{"value": map[string]any{"ok": true}}}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(result.Outputs, map[string]any{"value": map[string]any{"ok": true}}) {
		t.Fatalf("outputs = %#v, want the default value pick", result.Outputs)
	}
}

func TestEvaluateRejectsDeclaredTypeMismatch(t *testing.T) {
	module := registeredModule(t)
	// break_object shares the Get Field executor, so the mismatch error names
	// the shared vocabulary while the rest of the message stays actionable.
	config := outputConfig(map[string]any{"id": "ready", "label": "Ready", "path": "ready", "dataType": "boolean"})
	_, err := module.Execute(context.Background(), invocation(module, config, map[string]any{"source": map[string]any{"ready": "yes"}}), nil)
	if err == nil || !strings.Contains(err.Error(), `get field output "Ready" is declared boolean, but "ready" is incompatible`) {
		t.Fatalf("Execute() error = %v, want a declared-type mismatch", err)
	}
}

func TestEvaluateRejectsInvalidOutputsConfig(t *testing.T) {
	module := registeredModule(t)
	if _, err := module.Execute(context.Background(), invocation(module, map[string]any{"outputs": "city"}, map[string]any{"source": map[string]any{}}), nil); err == nil || !strings.Contains(err.Error(), "outputs must be a list") {
		t.Fatalf("Execute() error = %v, want outputs must be a list", err)
	}
	if _, err := module.Execute(context.Background(), invocation(module, map[string]any{"outputs": []any{}}, map[string]any{"source": map[string]any{}}), nil); err == nil || !strings.Contains(err.Error(), "add at least one output") {
		t.Fatalf("Execute() error = %v, want add at least one output", err)
	}
}

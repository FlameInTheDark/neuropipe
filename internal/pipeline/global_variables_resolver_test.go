package pipeline

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/getglobalvariable"
	setglobalvariableflow "github.com/FlameInTheDark/neuropipe/internal/nodes/flow/setglobalvariable"
)

func TestSetGlobalVariableResolverTypesValuePin(t *testing.T) {
	globals := globalTestGlobals(t, []domain.GlobalVariable{
		{Name: "label", DataType: domain.DataText, DefaultValue: ""},
		{Name: "visits", DataType: domain.DataNumber, DefaultValue: float64(0)},
		{Name: "events", DataType: domain.DataList, DefaultValue: []any{}},
	})
	setglobalvariableflow.SetDeclaredType(globals.VariableType)
	t.Cleanup(func() { setglobalvariableflow.SetDeclaredType(nil) })

	module := mustRegistryNode(t, catalog.New(), "flow:set_global_variable")
	tests := []struct {
		name           string
		variable       string
		operation      string
		wantInput      domain.DataType
		wantInputKind  domain.TypeKind
		wantOutput     domain.DataType
		wantOutputKind domain.TypeKind
	}{
		{name: "set text", variable: "label", operation: "set", wantInput: domain.DataText, wantInputKind: domain.TypeString, wantOutput: domain.DataText, wantOutputKind: domain.TypeString},
		{name: "set number", variable: "visits", operation: "set", wantInput: domain.DataNumber, wantInputKind: domain.TypeFloat, wantOutput: domain.DataNumber, wantOutputKind: domain.TypeFloat},
		{name: "increment", variable: "visits", operation: "increment", wantInput: domain.DataNumber, wantInputKind: domain.TypeFloat, wantOutput: domain.DataNumber, wantOutputKind: domain.TypeFloat},
		{name: "append item", variable: "events", operation: "append", wantInput: domain.DataAny, wantInputKind: domain.TypeAny, wantOutput: domain.DataList, wantOutputKind: domain.TypeList},
		{name: "unknown variable", variable: "gone", operation: "set", wantInput: domain.DataAny, wantInputKind: domain.TypeAny, wantOutput: domain.DataAny, wantOutputKind: domain.TypeAny},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := domain.FlowNode{Type: "flow:set_global_variable", Data: map[string]any{"config": map[string]any{"name": test.variable, "operation": test.operation}}}
			resolved, err := module.Resolve(node)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			var valuePin *domain.NodePort
			for index := range resolved.Inputs {
				if resolved.Inputs[index].ID == "value" {
					valuePin = &resolved.Inputs[index]
					break
				}
			}
			if valuePin == nil {
				t.Fatal("resolved definition has no value input")
			}
			if valuePin.DataType != test.wantInput || valuePin.Type == nil || valuePin.Type.Kind != test.wantInputKind {
				t.Fatalf("value input = %#v, want data type %q and type kind %q", valuePin, test.wantInput, test.wantInputKind)
			}
			resultPin := resolved.Outputs[1]
			if resultPin.ID != "result" || resultPin.DataType != test.wantOutput || resultPin.Type == nil || resultPin.Type.Kind != test.wantOutputKind {
				t.Fatalf("value output = %#v, want data type %q and type kind %q", resultPin, test.wantOutput, test.wantOutputKind)
			}
		})
	}
}

// The node picklist regression this guards: the editor adopts the backend
// resolver output wholesale. If a resolver returns static metadata without
// injecting live declaration options, the Variables dropdown renders empty
// even though declarations exist.
func TestGlobalVariableResolversInjectPicklistOptions(t *testing.T) {
	globals := globalTestGlobals(t, []domain.GlobalVariable{
		{Name: "visits", DataType: domain.DataNumber, DefaultValue: float64(0)},
	})
	getglobalvariable.SetDeclaredOptions(globals.VariableOptions)
	setglobalvariableflow.SetDeclaredOptions(globals.VariableOptions)
	t.Cleanup(func() {
		getglobalvariable.SetDeclaredOptions(nil)
		setglobalvariableflow.SetDeclaredOptions(nil)
	})

	registry := catalog.New()
	node := domain.FlowNode{ID: "read", Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": "visits"}}}
	for _, nodeType := range []string{"data:get_global_variable", "flow:set_global_variable"} {
		module, ok := registry.Node(nodeType)
		if !ok {
			t.Fatalf("registry has no module %q", nodeType)
		}
		resolved, err := module.Resolve(node)
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", nodeType, err)
		}
		var picklist *domain.ConfigField
		for index := range resolved.Fields {
			if resolved.Fields[index].Name == "name" {
				picklist = &resolved.Fields[index]
			}
		}
		if picklist == nil {
			t.Fatalf("Resolve(%q) has no name field", nodeType)
		}
		if len(picklist.Options) != 1 || picklist.Options[0].Value != "visits" {
			t.Fatalf("Resolve(%q) options = %#v, want one entry for visits", nodeType, picklist.Options)
		}
	}
	getModule := mustRegistryNode(t, registry, "data:get_global_variable")
	getResolved, err := getModule.Resolve(node)
	if err != nil {
		t.Fatalf("Resolve get node error = %v", err)
	}
	if getResolved.Outputs[0].DataType != domain.DataNumber {
		t.Fatalf("resolved output type = %q, want number", getResolved.Outputs[0].DataType)
	}

	// Unknown declaration name keeps the resolver total: output falls back to
	// Any and the executor reports the failure at runtime.
	stray := domain.FlowNode{ID: "stray", Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": "gone"}}}
	resolved, err := mustRegistryNode(t, registry, "data:get_global_variable").Resolve(stray)
	if err != nil {
		t.Fatalf("Resolve(unknown) error = %v", err)
	}
	if resolved.Outputs[0].DataType != domain.DataAny {
		t.Fatalf("unknown variable output type = %q, want any fallback", resolved.Outputs[0].DataType)
	}
}

// TestCatalogAllInjectsVariableOptions guards the palette path: registry.All()
// must carry the same options so the library panel and freshly-placed nodes
// show declarations before any resolution happens.
func TestCatalogAllInjectsVariableOptions(t *testing.T) {
	globals := globalTestGlobals(t, []domain.GlobalVariable{
		{Name: "visits", DataType: domain.DataNumber, DefaultValue: float64(0)},
	})
	registry := catalog.New()
	registry.SetVariableOptions(globals.VariableOptions)
	found := false
	for _, definition := range registry.All() {
		if definition.Type != "data:get_global_variable" {
			continue
		}
		found = true
		for _, field := range definition.Fields {
			if field.Name == "name" && len(field.Options) == 1 && field.Options[0].Value == "visits" {
				return
			}
		}
		t.Fatalf("catalog definition fields = %#v, want injected name options", definition.Fields)
	}
	if !found {
		t.Fatal("catalog.All() did not list data:get_global_variable")
	}
}

// TestGlobalVariableAppendThroughGraph covers the third operation end to end:
// a list variable accumulates across two runs of the same definition.
func TestGlobalVariableAppendThroughGraph(t *testing.T) {
	globals := globalTestGlobals(t, []domain.GlobalVariable{
		{Name: "events", DataType: domain.DataList, DefaultValue: []any{}},
	})
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
		{ID: "value", Type: "data:constant", Data: map[string]any{"config": map[string]any{"value": "ping", "type": "text"}}},
		{ID: "write", Type: "flow:set_global_variable", Data: map[string]any{"config": map[string]any{"name": "events", "operation": "append"}}},
	}, Edges: []domain.FlowEdge{
		execEdge("start-write", "start", "out", "write", "in"),
		dataEdge("value-write", "value", "value", "write", "value"),
	}}
	engine := NewEngine(catalog.New(), nil, nil, WithGlobalVariablesStore(globals))
	for run := 0; run < 2; run++ {
		if _, err := engine.Execute(context.Background(), flow, "start", Packet{}); err != nil {
			t.Fatalf("Execute() run %d error = %v", run, err)
		}
	}
	value, err := globals.Read("events")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	list, ok := value.([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("Read() = %#v, want a two-element list after two appends", value)
	}
}

func mustRegistryNode(t *testing.T, registry *catalog.Registry, nodeType string) interface {
	Resolve(domain.FlowNode) (domain.NodeDefinition, error)
} {
	t.Helper()
	module, ok := registry.Node(nodeType)
	if !ok {
		t.Fatalf("registry has no module %q", nodeType)
	}
	return module
}

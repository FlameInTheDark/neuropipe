package pipeline

import (
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// resolveNode resolves a node's dynamic contract through the registered
// module, the same path the editor, validator, and interpreter use.
func resolveNode(t *testing.T, registry *catalog.Registry, nodeType string, config map[string]any) domain.NodeDefinition {
	t.Helper()
	definition, ok := registry.Get(nodeType)
	if !ok {
		t.Fatalf("%s definition is missing", nodeType)
	}
	resolved, err := definitionForRegisteredNode(registry, definition, domain.FlowNode{ID: "node", Type: nodeType, Data: map[string]any{"config": config}})
	if err != nil {
		t.Fatalf("resolve %s() error = %v", nodeType, err)
	}
	return resolved
}

func resolveNodeErr(t *testing.T, registry *catalog.Registry, nodeType string, config map[string]any) error {
	t.Helper()
	definition, ok := registry.Get(nodeType)
	if !ok {
		t.Fatalf("%s definition is missing", nodeType)
	}
	_, err := definitionForRegisteredNode(registry, definition, domain.FlowNode{ID: "node", Type: nodeType, Data: map[string]any{"config": config}})
	return err
}

func TestResolveBuildsSwitchOutputsFromCases(t *testing.T) {
	registry := catalog.New()
	resolved := resolveNode(t, registry, "flow:switch", map[string]any{
		"switch": map[string]any{"comparator": "equals", "cases": []any{
			map[string]any{"id": "approved", "label": "Approved", "valueType": "text", "value": "approved"},
			map[string]any{"id": "rejected", "label": "Rejected", "valueType": "text", "value": "rejected"},
		}},
	})
	if got, want := resolved.Outputs[0].ID, "approved"; got != want {
		t.Fatalf("first configured output = %q, want %q", got, want)
	}
	if got, want := resolved.Outputs[1].ID, "rejected"; got != want {
		t.Fatalf("second configured output = %q, want %q", got, want)
	}
	if last := resolved.Outputs[len(resolved.Outputs)-1]; last.ID != "default" {
		t.Fatalf("default output = %q, want it last", last.ID)
	}
}

func TestResolveSwitchUsesDefaultCasesWithoutConfiguration(t *testing.T) {
	registry := catalog.New()
	resolved := resolveNode(t, registry, "flow:switch", nil)
	if len(resolved.Outputs) < 3 {
		t.Fatalf("default switch outputs = %d, want case-a, case-b, and default", len(resolved.Outputs))
	}
	if resolved.Outputs[0].ID != "case-a" || resolved.Outputs[1].ID != "case-b" {
		t.Fatalf("default switch outputs = %q, %q", resolved.Outputs[0].ID, resolved.Outputs[1].ID)
	}
}

func TestResolveRejectsInvalidSwitchCaseConfiguration(t *testing.T) {
	registry := catalog.New()
	err := resolveNodeErr(t, registry, "flow:switch", map[string]any{
		"switch": map[string]any{"comparator": "contains", "cases": []any{
			map[string]any{"id": "bad", "label": "Bad", "valueType": "number", "value": 1},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot use") {
		t.Fatalf("resolve switch() error = %v, want invalid comparator type error", err)
	}
}

func TestDefinitionForNodeRejectsDuplicateRouteOptionIDs(t *testing.T) {
	definition, ok := catalog.New().Get("llm:choice")
	if !ok {
		t.Fatal("Choice definition is missing")
	}
	_, err := definitionForNode(definition, domain.FlowNode{
		ID:   "choice",
		Type: "llm:choice",
		Data: map[string]any{"config": map[string]any{
			"options": []any{
				map[string]any{"id": "same", "label": "First"},
				map[string]any{"id": "same", "label": "Second"},
			},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("definitionForNode() error = %v, want duplicate option ID error", err)
	}
}

func TestDefinitionForNodeBuildsChoiceOptionPorts(t *testing.T) {
	definition, ok := catalog.New().Get("llm:choice")
	if !ok {
		t.Fatal("Choice definition is missing")
	}
	resolved, err := definitionForNode(definition, domain.FlowNode{
		ID:   "choice",
		Type: "llm:choice",
		Data: map[string]any{"config": map[string]any{
			"options": []any{
				map[string]any{"id": "ship", "label": "Ship it"},
				map[string]any{"id": "hold", "label": "Hold"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("definitionForNode() error = %v", err)
	}
	if got, want := resolved.Outputs[0].ID, "ship"; got != want {
		t.Fatalf("first option port = %q, want %q", got, want)
	}
	if got, want := resolved.Outputs[1].Label, "Hold"; got != want {
		t.Fatalf("second option label = %q, want %q", got, want)
	}
}

func TestDefinitionForNodeFiltersChatContextPins(t *testing.T) {
	registry := catalog.New()
	definition, ok := registry.Get("llm:agent")
	if !ok {
		t.Fatal("Agent definition is missing")
	}
	// One-message mode without status updates hides chatId and chatRunId.
	resolved, err := definitionForRegisteredNode(registry, definition, domain.FlowNode{ID: "agent", Type: "llm:agent", Data: map[string]any{"config": map[string]any{
		"chatMode": "message", "updateChatStatus": false,
	}}})
	if err != nil {
		t.Fatalf("resolve agent() error = %v", err)
	}
	for _, pin := range resolved.Inputs {
		if pin.ID == "chatId" || pin.ID == "chatRunId" {
			t.Fatalf("one-message agent exposes %q pin", pin.ID)
		}
	}
	// History mode with status updates exposes both.
	resolved, err = definitionForRegisteredNode(registry, definition, domain.FlowNode{ID: "agent", Type: "llm:agent", Data: map[string]any{"config": map[string]any{
		"chatMode": "history", "updateChatStatus": true,
	}}})
	if err != nil {
		t.Fatalf("resolve agent (history)() error = %v", err)
	}
	var hasChatID, hasRunID bool
	for _, pin := range resolved.Inputs {
		if pin.ID == "chatId" {
			hasChatID = true
		}
		if pin.ID == "chatRunId" {
			hasRunID = true
		}
	}
	if !hasChatID || !hasRunID {
		t.Fatalf("history agent pins missing chatId/chatRunId (chatId=%v chatRunId=%v)", hasChatID, hasRunID)
	}
}

func TestResolveBuildsTypedGetFieldOutputs(t *testing.T) {
	registry := catalog.New()
	resolved := resolveNode(t, registry, "data:get_field", map[string]any{
		"outputs": []any{
			map[string]any{"id": "command", "label": "Command", "path": "terminal.command", "dataType": "text"},
			map[string]any{"id": "output", "label": "Output", "path": "terminal.output", "dataType": "text"},
		},
	})
	if got, want := len(resolved.Outputs), 2; got != want {
		t.Fatalf("output count = %d, want %d", got, want)
	}
	if got, want := resolved.Outputs[1], (domain.NodePort{ID: "output", Label: "Output", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &domain.TypeSpec{Kind: domain.TypeString}, Color: "#e879f9", MaxConnections: 1}); !reflect.DeepEqual(got, want) {
		t.Fatalf("second output = %#v, want %#v", got, want)
	}
}

func TestResolveGetFieldFallsBackToDefaultOutputs(t *testing.T) {
	registry := catalog.New()
	resolved := resolveNode(t, registry, "data:get_field", nil)
	if got, want := resolved.Outputs[0].ID, "value"; got != want {
		t.Fatalf("default output ID = %q, want %q", got, want)
	}
}

func TestResolveBuildsTypeAssertContract(t *testing.T) {
	registry := catalog.New()
	resolved := resolveNode(t, registry, "data:type_assert", map[string]any{
		"typeSpec": map[string]any{"kind": "record", "fields": []any{map[string]any{"id": "name", "name": "name", "type": map[string]any{"kind": "string"}}}},
	})
	got := resolved.Outputs[0].Type
	if got == nil || got.Kind != domain.TypeRecord || len(got.Fields) != 1 || got.Fields[0].Type.Kind != domain.TypeString {
		t.Fatalf("resolved Type Assert contract = %#v", got)
	}
}

func TestResolveTypesCastOutputs(t *testing.T) {
	registry := catalog.New()
	tests := []struct {
		target   string
		dataType domain.DataType
		kind     domain.TypeKind
	}{
		{"object", domain.DataObject, domain.TypeMap},
		{"list", domain.DataList, domain.TypeList},
		{"bytes", domain.DataBytes, domain.TypeBytes},
	}
	for _, test := range tests {
		resolved := resolveNode(t, registry, "data:cast", map[string]any{"target": test.target})
		output := resolved.Outputs[0]
		if output.DataType != test.dataType {
			t.Fatalf("target %s output data type = %q, want %q", test.target, output.DataType, test.dataType)
		}
		if output.Type == nil || output.Type.Kind != test.kind {
			t.Fatalf("target %s output contract = %#v, want kind %s", test.target, output.Type, test.kind)
		}
	}
	object := resolveNode(t, registry, "data:cast", map[string]any{"target": "object"}).Outputs[0]
	if object.Type == nil || object.Type.Key == nil || object.Type.Key.Kind != domain.TypeString || object.Type.Value == nil || object.Type.Value.Kind != domain.TypeAny {
		t.Fatalf("object cast contract = %#v, want map<string, any>", object.Type)
	}
}

func TestResolveBuildsConfiguredObjectPins(t *testing.T) {
	registry := catalog.New()
	tests := []struct {
		name       string
		nodeType   string
		config     map[string]any
		inputCount int
		outputID   string
	}{
		{
			name:     "build object inputs",
			nodeType: "data:build_object",
			config: map[string]any{"fields": []any{
				map[string]any{"id": "name", "label": "Name", "key": "customer.name", "dataType": "text"},
				map[string]any{"id": "active", "label": "Active", "key": "customer.active", "dataType": "boolean"},
			}},
			inputCount: 2,
		},
		{
			name:       "build object defaults to one value field",
			nodeType:   "data:build_object",
			config:     nil,
			inputCount: 1,
		},
		{
			name:     "break object outputs",
			nodeType: "data:break_object",
			config: map[string]any{"outputs": []any{
				map[string]any{"id": "name", "label": "Name", "path": "customer.name", "dataType": "text"},
			}},
			inputCount: 1,
			outputID:   "name",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved := resolveNode(t, registry, test.nodeType, test.config)
			if got := len(resolved.Inputs); got != test.inputCount {
				t.Fatalf("input count = %d, want %d", got, test.inputCount)
			}
			if test.outputID != "" && resolved.Outputs[0].ID != test.outputID {
				t.Fatalf("first output = %q, want %q", resolved.Outputs[0].ID, test.outputID)
			}
		})
	}
}

func TestResolveBuildObjectDefaultsToConfiguredFieldPins(t *testing.T) {
	registry := catalog.New()
	resolved := resolveNode(t, registry, "data:build_object", nil)
	if got, want := resolved.Inputs[0].ID, "value"; got != want {
		t.Fatalf("default build object input = %q, want %q", got, want)
	}
}

func TestResolveRejectsOverlappingObjectKeys(t *testing.T) {
	registry := catalog.New()
	err := resolveNodeErr(t, registry, "data:build_object", map[string]any{"fields": []any{
		map[string]any{"id": "customer", "key": "customer", "dataType": "object"},
		map[string]any{"id": "name", "key": "customer.name", "dataType": "text"},
	}})
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("resolve build object() error = %v, want overlapping key error", err)
	}
}

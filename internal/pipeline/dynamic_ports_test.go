package pipeline

import (
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestDefinitionForNodeBuildsSwitchOutputsFromCases(t *testing.T) {
	definition, ok := catalog.New().Get("flow:switch")
	if !ok {
		t.Fatal("Switch definition is missing")
	}
	node := domain.FlowNode{
		ID:   "switch",
		Type: "flow:switch",
		Data: map[string]any{"config": map[string]any{
			"switch": map[string]any{"comparator": "equals", "cases": []any{
				map[string]any{"id": "approved", "label": "Approved", "valueType": "text", "value": "approved"},
				map[string]any{"id": "rejected", "label": "Rejected", "valueType": "text", "value": "rejected"},
			},
			},
		}},
	}
	resolved, err := definitionForNode(definition, node)
	if err != nil {
		t.Fatalf("definitionForNode() error = %v", err)
	}
	if got, want := resolved.Outputs[0].ID, "approved"; got != want {
		t.Fatalf("first configured output = %q, want %q", got, want)
	}
	if got, want := resolved.Outputs[1].ID, "rejected"; got != want {
		t.Fatalf("second configured output = %q, want %q", got, want)
	}
}

func TestDefinitionForNodePreservesLegacySwitchOptions(t *testing.T) {
	definition, _ := catalog.New().Get("flow:switch")
	resolved, err := definitionForNode(definition, domain.FlowNode{ID: "switch", Type: "flow:switch", Data: map[string]any{"config": map[string]any{
		"options": []any{map[string]any{"id": "approved", "label": "Approved"}},
	}}})
	if err != nil {
		t.Fatalf("definitionForNode() error = %v", err)
	}
	if got, want := resolved.Outputs[0].ID, "approved"; got != want {
		t.Fatalf("legacy output ID = %q, want %q", got, want)
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

func TestDefinitionForNodeRejectsInvalidSwitchCaseConfiguration(t *testing.T) {
	definition, _ := catalog.New().Get("flow:switch")
	_, err := definitionForNode(definition, domain.FlowNode{ID: "switch", Type: "flow:switch", Data: map[string]any{"config": map[string]any{
		"switch": map[string]any{"comparator": "contains", "cases": []any{
			map[string]any{"id": "bad", "label": "Bad", "valueType": "number", "value": 1},
		}},
	}}})
	if err == nil || !strings.Contains(err.Error(), "cannot use") {
		t.Fatalf("definitionForNode() error = %v, want invalid comparator type error", err)
	}
}

func TestDefinitionForNodeBuildsTypedGetFieldOutputs(t *testing.T) {
	definition, ok := catalog.New().Get("data:get_field")
	if !ok {
		t.Fatal("Get Field definition is missing")
	}
	resolved, err := definitionForNode(definition, domain.FlowNode{
		ID:   "get-field",
		Type: "data:get_field",
		Data: map[string]any{"config": map[string]any{
			"outputs": []any{
				map[string]any{"id": "command", "label": "Command", "path": "terminal.command", "dataType": "text"},
				map[string]any{"id": "output", "label": "Output", "path": "terminal.output", "dataType": "text"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("definitionForNode() error = %v", err)
	}
	if got, want := len(resolved.Outputs), 2; got != want {
		t.Fatalf("output count = %d, want %d", got, want)
	}
	if got, want := resolved.Outputs[1], (domain.NodePort{ID: "output", Label: "Output", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Color: "#e879f9", MaxConnections: 1}); !reflect.DeepEqual(got, want) {
		t.Fatalf("second output = %#v, want %#v", got, want)
	}
}

func TestDefinitionForNodePreservesLegacyGetFieldPath(t *testing.T) {
	definition, _ := catalog.New().Get("data:get_field")
	resolved, err := definitionForNode(definition, domain.FlowNode{
		ID:   "get-field",
		Type: "data:get_field",
		Data: map[string]any{"config": map[string]any{"path": "terminal.output"}},
	})
	if err != nil {
		t.Fatalf("definitionForNode() error = %v", err)
	}
	if got, want := resolved.Outputs[0].ID, "value"; got != want {
		t.Fatalf("legacy output ID = %q, want %q", got, want)
	}
}

func TestDefinitionForNodeBuildsConfiguredObjectPins(t *testing.T) {
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
			definition, ok := registry.Get(test.nodeType)
			if !ok {
				t.Fatalf("%s definition is missing", test.nodeType)
			}
			resolved, err := definitionForNode(definition, domain.FlowNode{ID: "node", Type: test.nodeType, Data: map[string]any{"config": test.config}})
			if err != nil {
				t.Fatalf("definitionForNode() error = %v", err)
			}
			if got := len(resolved.Inputs); got != test.inputCount {
				t.Fatalf("input count = %d, want %d", got, test.inputCount)
			}
			if test.outputID != "" && resolved.Outputs[0].ID != test.outputID {
				t.Fatalf("first output = %q, want %q", resolved.Outputs[0].ID, test.outputID)
			}
		})
	}
}

func TestDefinitionForNodePreservesLegacyBuildObjectPins(t *testing.T) {
	definition, _ := catalog.New().Get("data:build_object")
	resolved, err := definitionForNode(definition, domain.FlowNode{ID: "build", Type: "data:build_object", Data: map[string]any{"config": map[string]any{}}})
	if err != nil {
		t.Fatalf("definitionForNode() error = %v", err)
	}
	if got, want := []string{resolved.Inputs[0].ID, resolved.Inputs[1].ID}, []string{"key", "value"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy inputs = %#v, want %#v", got, want)
	}
}

func TestDefinitionForNodeRejectsOverlappingObjectKeys(t *testing.T) {
	definition, _ := catalog.New().Get("data:build_object")
	_, err := definitionForNode(definition, domain.FlowNode{ID: "build", Type: "data:build_object", Data: map[string]any{"config": map[string]any{"fields": []any{
		map[string]any{"id": "customer", "key": "customer", "dataType": "object"},
		map[string]any{"id": "name", "key": "customer.name", "dataType": "text"},
	}}}})
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("definitionForNode() error = %v, want overlapping key error", err)
	}
}

package pipeline

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestValidateRejectsLegacyGraphsAndInvalidPinKinds(t *testing.T) {
	tests := []domain.FlowDefinition{
		{Nodes: []domain.FlowNode{{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}}}},
		{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{v2Node("start", "trigger:button", map[string]any{"label": "Run"}), v2Node("constant", "data:constant", map[string]any{"value": true})}, Edges: []domain.FlowEdge{execEdge("bad", "start", "out", "constant", "value")}},
	}
	for _, flow := range tests {
		if err := Validate(flow, catalog.New()); err == nil {
			t.Fatal("Validate() error = nil, want failure")
		}
	}
}

func TestValidateAcceptsTypedDataAndExecEdges(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{v2Node("start", "trigger:button", map[string]any{"label": "Run"}), v2Node("constant", "data:constant", map[string]any{"value": true}), v2Node("branch", "flow:branch", nil)}, Edges: []domain.FlowEdge{execEdge("exec", "start", "out", "branch", "in"), dataEdge("data", "constant", "value", "branch", "condition")}}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

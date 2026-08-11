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

func TestValidateV3AcceptsWaypointEdgesAsDirectWires(t *testing.T) {
	waypoints := []domain.Position{{X: 120, Y: 240}, {X: 300, Y: 80}}
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("constant", "data:constant", map[string]any{"value": true, "type": "boolean"}),
		v2Node("branch", "flow:branch", nil),
		v2Node("notice", "action:notification", map[string]any{"title": "Ready", "message": "Done"}),
	}, Edges: []domain.FlowEdge{
		{ID: "exec", Source: "start", SourceHandle: "out", Target: "branch", TargetHandle: "in", Kind: domain.PinExec, Waypoints: waypoints},
		{ID: "data", Source: "constant", SourceHandle: "value", Target: "branch", TargetHandle: "condition", Kind: domain.PinData, Waypoints: waypoints},
		{ID: "then", Source: "branch", SourceHandle: "true", Target: "notice", TargetHandle: "in", Kind: domain.PinExec, Waypoints: []domain.Position{{X: -48, Y: 512}}},
	}}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateV3WaypointsDoNotChangeEdgeContractErrors(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("constant", "data:constant", map[string]any{"value": true}),
		v2Node("branch", "flow:branch", nil),
	}, Edges: []domain.FlowEdge{
		{ID: "exec", Source: "start", SourceHandle: "out", Target: "branch", TargetHandle: "in", Kind: domain.PinExec},
		{ID: "data", Source: "constant", SourceHandle: "value", Target: "branch", TargetHandle: "condition", Kind: domain.PinData, Waypoints: []domain.Position{{X: 1, Y: 2}}},
	}}
	if err := Validate(flow, catalog.New()); err == nil {
		t.Fatal("Validate() accepted an incompatible waypoint edge")
	}
}

func TestValidateV3RejectsLegacyRerouteNodes(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("relay", "flow:reroute", nil),
	}}
	if err := Validate(flow, catalog.New()); err == nil {
		t.Fatal("Validate() accepted a legacy reroute in a V3 graph")
	}
}

func TestPinsCompatibleRequiresExplicitV3Conversion(t *testing.T) {
	integer := domain.NodePort{Kind: domain.PinData, Type: &domain.TypeSpec{Kind: domain.TypeInt}}
	text := domain.NodePort{Kind: domain.PinData, Type: &domain.TypeSpec{Kind: domain.TypeString}}
	any := domain.NodePort{Kind: domain.PinData, Type: &domain.TypeSpec{Kind: domain.TypeAny}}
	if pinsCompatible(integer, text) || pinsCompatible(any, integer) {
		t.Fatal("V3 pin validation accepted an implicit conversion or any narrowing")
	}
	if !pinsCompatible(integer, any) {
		t.Fatal("concrete V3 output should assign to any")
	}
}

func TestValidateV3RejectsAnyNarrowingAndAllowsExplicitConstantType(t *testing.T) {
	registry := catalog.New()
	base := []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("constant", "data:constant", map[string]any{"value": "true"}),
		v2Node("branch", "flow:branch", nil),
	}
	edges := []domain.FlowEdge{execEdge("exec", "start", "out", "branch", "in"), dataEdge("data", "constant", "value", "branch", "condition")}
	if err := Validate(domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: base, Edges: edges}, registry); err == nil {
		t.Fatal("V3 accepted an any output connected to Boolean")
	}
	base[1] = v2Node("constant", "data:constant", map[string]any{"value": "true", "type": "boolean"})
	if err := Validate(domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: base, Edges: edges}, registry); err != nil {
		t.Fatalf("V3 rejected an explicitly typed Boolean constant: %v", err)
	}
}

func TestValidateV3AllowsExplicitTypeAssertNarrowing(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("constant", "data:constant", map[string]any{"value": true}),
		v2Node("assert", "data:type_assert", map[string]any{"typeSpec": map[string]any{"kind": "bool"}}),
		v2Node("branch", "flow:branch", nil),
	}, Edges: []domain.FlowEdge{
		execEdge("exec", "start", "out", "branch", "in"),
		dataEdge("source-assert", "constant", "value", "assert", "value"),
		dataEdge("assert-condition", "assert", "value", "branch", "condition"),
	}}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

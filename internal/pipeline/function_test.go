package pipeline

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

func TestBlueprintExecutesPublishedImpureFunction(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	function, err := store.CreateFunction(context.Background(), "Run once", domain.NodeImpure)
	if err != nil {
		t.Fatal(err)
	}
	function, err = store.PublishFunction(context.Background(), function)
	if err != nil {
		t.Fatal(err)
	}
	registry := catalog.New()
	definitions, err := store.PublishedFunctionDefinitions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	registry.ReplaceDynamic(definitions)
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{v2Node("start", "trigger:button", map[string]any{"label": "Run"}), v2Node("call", "function:"+function.ID, nil)}, Edges: []domain.FlowEdge{execEdge("call", "start", "out", "call", "in")}}
	result, err := NewEngine(registry, nil, nil, WithFunctionResolver(store)).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.NodeRuns) != 4 {
		t.Fatalf("node runs = %d, want event, call, inner entry, and inner return", len(result.NodeRuns))
	}
}

func TestBlueprintCachesPublishedPureFunction(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	function, err := store.CreateFunction(context.Background(), "Identity", domain.NodePure)
	if err != nil {
		t.Fatal(err)
	}
	function.Inputs = []domain.FunctionPin{{ID: "input", Name: "Input", DataType: domain.DataBoolean, Required: true}}
	function.Outputs = []domain.FunctionPin{{ID: "output", Name: "Output", DataType: domain.DataBoolean}}
	function.DraftDefinition.Edges = []domain.FlowEdge{dataEdge("identity", "inputs", "input", "outputs", "output")}
	function, err = store.PublishFunction(context.Background(), function)
	if err != nil {
		t.Fatal(err)
	}
	registry := catalog.New()
	definitions, err := store.PublishedFunctionDefinitions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	registry.ReplaceDynamic(definitions)
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{v2Node("start", "trigger:button", map[string]any{"label": "Run"}), v2Node("truth", "data:constant", map[string]any{"value": true}), v2Node("call", "function:"+function.ID, nil), v2Node("branch", "flow:branch", nil)}, Edges: []domain.FlowEdge{execEdge("start-branch", "start", "out", "branch", "in"), dataEdge("truth-call", "truth", "value", "call", "input"), dataEdge("call-branch", "call", "output", "branch", "condition")}}
	if _, err := NewEngine(registry, nil, nil, WithFunctionResolver(store)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

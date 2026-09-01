package pipeline

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestDataCastReturnsPickedObjectElement(t *testing.T) {
	element := map[string]any{"id": 7.0, "name": "neuro"}
	scaffold := baseScaffoldFor(t, []any{map[string]any{"id": 1.0}, element},
		cfgNode("index", "data:constant", map[string]any{"type": "number", "value": 1.0}),
		cfgNode("pick", "data:array_get", nil),
		cfgNode("cast", "data:cast", map[string]any{"target": "object"}),
	)
	edges := append(scaffold.edges,
		dataEdge("base-pick", "base", "value", "pick", "array"),
		dataEdge("index-pick", "index", "value", "pick", "index"),
		dataEdge("pick-cast", "pick", "value", "cast", "value"),
		storeEdge("cast", "value"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "cast")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:cast", outputs, map[string]any{"value": element})
}

func TestDataCastSerializesPickedObjectToText(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{map[string]any{"name": "neuro", "level": 2.0}},
		cfgNode("index", "data:constant", map[string]any{"type": "number", "value": 0.0}),
		cfgNode("pick", "data:array_get", nil),
		cfgNode("cast", "data:cast", map[string]any{"target": "text"}),
	)
	edges := append(scaffold.edges,
		dataEdge("base-pick", "base", "value", "pick", "array"),
		dataEdge("index-pick", "index", "value", "pick", "index"),
		dataEdge("pick-cast", "pick", "value", "cast", "value"),
		storeEdge("cast", "value"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "cast")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:cast", outputs, map[string]any{"value": `{"level":2,"name":"neuro"}`})
}

// V3 wiring rejects an untyped Pick from Array output on an object pin; the
// Cast node with the object target is the explicit bridge. This mirrors the
// SQL rows → Pick from Array → Cast → KV Hash Set workflow.
func TestValidateV3CastObjectBridgesAnyIntoObjectPins(t *testing.T) {
	registry := catalog.New()
	base := []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("query", "action:sql", map[string]any{"databaseId": "db-1", "sql": "SELECT id, email FROM users"}),
		cfgNode("pick", "data:array_get", map[string]any{"index": 0.0}),
		cfgNode("hash", "action:kv_hash_set", map[string]any{"databaseId": "kv-1", "key": "user:1", "mode": "set"}),
	}
	exec := []domain.FlowEdge{
		execEdge("start-query", "start", "out", "query", "in"),
		execEdge("query-hash", "query", "out", "hash", "in"),
		dataEdge("rows-pick", "query", "rows", "pick", "array"),
	}

	direct := append(append([]domain.FlowEdge{}, exec...),
		dataEdge("pick-hash", "pick", "value", "hash", "fields"))
	if err := Validate(domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: base, Edges: direct}, registry); err == nil {
		t.Fatal("V3 accepted an untyped Pick from Array output wired straight into an object pin")
	}

	castFlow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3,
		Nodes: append(append([]domain.FlowNode{}, base[:3]...),
			cfgNode("cast", "data:cast", map[string]any{"target": "object"}),
			base[3]),
		Edges: append(append([]domain.FlowEdge{}, exec...),
			dataEdge("pick-cast", "pick", "value", "cast", "value"),
			dataEdge("cast-hash", "cast", "value", "hash", "fields")),
	}
	if err := Validate(castFlow, registry); err != nil {
		t.Fatalf("V3 rejected the cast object bridge: %v", err)
	}
}

// The cast list output carries the list<any> contract, so a typed rows value
// stays wireable after an explicit cast.
func TestValidateV3CastListFeedsListInputs(t *testing.T) {
	registry := catalog.New()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("query", "action:sql", map[string]any{"databaseId": "db-1", "sql": "SELECT id FROM users"}),
		cfgNode("cast", "data:cast", map[string]any{"target": "list"}),
		cfgNode("pick", "data:array_get", map[string]any{"index": 0.0}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-query", "start", "out", "query", "in"),
		dataEdge("rows-cast", "query", "rows", "cast", "value"),
		dataEdge("cast-pick", "cast", "value", "pick", "array"),
	}}
	if err := Validate(flow, registry); err != nil {
		t.Fatalf("V3 rejected the cast list feeding a list input: %v", err)
	}
}

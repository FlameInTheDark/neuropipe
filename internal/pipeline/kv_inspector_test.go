package pipeline

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// kvCommandRecorder captures every command the engine dispatches so tests can
// assert on the exact Redis wire arguments produced by inspector config.
type kvCommandRecorder struct {
	requests []domain.KVCommandRequest
}

func (r *kvCommandRecorder) ExecuteCommand(_ context.Context, request domain.KVCommandRequest) (domain.KVCommandResult, error) {
	r.requests = append(r.requests, request)
	return domain.KVCommandResult{Value: "OK"}, nil
}

// TestKVSetRunsFromInspectorConfigAlone proves the KV Set node is fully
// drivable from the inspector: key, value, and TTL come from config because
// the engine copies them into the unwired pins at execution time.
func TestKVSetRunsFromInspectorConfigAlone(t *testing.T) {
	recorder := &kvCommandRecorder{}
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("set", "action:kv_set", map[string]any{
			"databaseId": "db-1",
			"key":        "user:42:name",
			"value":      "Ada Lovelace",
			"ttlSeconds": float64(30),
		}),
	}, Edges: []domain.FlowEdge{execEdge("start-set", "start", "out", "set", "in")}}

	if _, err := NewEngine(catalog.New(), nil, nil, WithKVExecutor(recorder)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(recorder.requests) != 1 || recorder.requests[0].Command != "SET" {
		t.Fatalf("requests = %#v", recorder.requests)
	}
	args := recorder.requests[0].Args
	if len(args) != 4 || args[0] != "user:42:name" || args[1] != "Ada Lovelace" || args[2] != "EX" || args[3] != "30" {
		t.Fatalf("args = %#v", args)
	}
}

// TestKVHashSetAcceptsInspectorObject regresses the list-vs-object pin bug:
// a fields map typed into the inspector used to die on the list pin's type
// validation before ever reaching Redis.
func TestKVHashSetAcceptsInspectorObject(t *testing.T) {
	recorder := &kvCommandRecorder{}
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("hset", "action:kv_hash_set", map[string]any{
			"databaseId": "db-1",
			"key":        "user:42",
			"fields":     map[string]any{"email": "ada@example.com"},
		}),
	}, Edges: []domain.FlowEdge{execEdge("start-hset", "start", "out", "hset", "in")}}

	if _, err := NewEngine(catalog.New(), nil, nil, WithKVExecutor(recorder)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(recorder.requests) != 1 || recorder.requests[0].Command != "HSET" {
		t.Fatalf("requests = %#v", recorder.requests)
	}
	args := recorder.requests[0].Args
	if len(args) != 3 || args[0] != "user:42" || args[1] != "email" || args[2] != "ada@example.com" {
		t.Fatalf("args = %#v", args)
	}
}

// TestKVListPushAcceptsInspectorList drives the visual list editor's payload
// (an array of strings) through the engine's config fallback.
func TestKVListPushAcceptsInspectorList(t *testing.T) {
	recorder := &kvCommandRecorder{}
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("push", "action:kv_list_push", map[string]any{
			"databaseId": "db-1",
			"key":        "queue:jobs",
			"values":     []any{"first", "second"},
		}),
	}, Edges: []domain.FlowEdge{execEdge("start-push", "start", "out", "push", "in")}}

	if _, err := NewEngine(catalog.New(), nil, nil, WithKVExecutor(recorder)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(recorder.requests) != 1 || recorder.requests[0].Command != "RPUSH" {
		t.Fatalf("requests = %#v", recorder.requests)
	}
	args := recorder.requests[0].Args
	if len(args) != 3 || args[0] != "queue:jobs" || args[1] != "first" || args[2] != "second" {
		t.Fatalf("args = %#v", args)
	}
}

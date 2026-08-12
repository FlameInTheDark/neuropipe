package pipeline

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/getglobalvariable"
	setglobalvariableflow "github.com/FlameInTheDark/neuropipe/internal/nodes/flow/setglobalvariable"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/variables"
)

func globalTestGlobals(t *testing.T, definitions []domain.GlobalVariable) *variables.Service {
	t.Helper()
	store, err := persistence.New(t.TempDir())
	if err != nil {
		t.Fatalf("persistence.New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	globals, err := variables.New(store)
	if err != nil {
		t.Fatalf("variables.New() error = %v", err)
	}
	for _, definition := range definitions {
		if _, err := globals.Create(context.Background(), definition); err != nil {
			t.Fatalf("Create(%q) error = %v", definition.Name, err)
		}
	}
	// Wire the resolver exactly as Desktop does so tests validate typed pins.
	getglobalvariable.SetDeclaredType(globals.VariableType)
	t.Cleanup(func() { getglobalvariable.SetDeclaredType(nil) })
	setglobalvariableflow.SetDeclaredType(globals.VariableType)
	t.Cleanup(func() { setglobalvariableflow.SetDeclaredType(nil) })
	return globals
}

func TestBlueprintGlobalVariablesFlow(t *testing.T) {
	globals := globalTestGlobals(t, []domain.GlobalVariable{
		{Name: "greeting", DataType: domain.DataText, DefaultValue: "hello"},
	})
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
		{ID: "read", Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": "greeting"}}},
		{ID: "store", Type: "flow:set_variable", Data: map[string]any{"config": map[string]any{"name": "Sink"}}},
	}, Edges: []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("read-store", "read", "value", "store", "value"),
	}}
	result, err := NewEngine(catalog.New(), nil, nil, WithGlobalVariablesStore(globals)).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() read error = %v", err)
	}
	// A read before any write surfaces the declared default through the graph,
	// and the Set Variable run captured that value on its output.
	for _, run := range result.NodeRuns {
		if run.NodeID == "store" {
			output, ok := run.Output.(map[string]any)
			if !ok {
				continue
			}
			if got := output["result"]; got != "hello" {
				t.Fatalf("stored value = %#v, want the declared default", got)
			}
			return
		}
	}
	t.Fatal("Set Variable did not run; the read's output never reached the graph")
}

func TestBlueprintGlobalWriteThenRead(t *testing.T) {
	globals := globalTestGlobals(t, []domain.GlobalVariable{
		{Name: "visits", DataType: domain.DataNumber, DefaultValue: float64(0)},
	})
	write := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
		{ID: "value", Type: "data:constant", Data: map[string]any{"config": map[string]any{"value": float64(42), "type": "number"}}},
		{ID: "write", Type: "flow:set_global_variable", Data: map[string]any{"config": map[string]any{"name": "visits", "operation": "set"}}},
	}, Edges: []domain.FlowEdge{
		execEdge("start-write", "start", "out", "write", "in"),
		dataEdge("value-write", "value", "value", "write", "value"),
	}}
	if _, err := NewEngine(catalog.New(), nil, nil, WithGlobalVariablesStore(globals)).Execute(context.Background(), write, "start", Packet{}); err != nil {
		t.Fatalf("Execute() write error = %v", err)
	}
	stored, err := globals.Read("visits")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if stored != float64(42) {
		t.Fatalf("Read() after Set = %#v, want 42", stored)
	}
}

func TestBlueprintGlobalSetTypeMismatchFailsNode(t *testing.T) {
	globals := globalTestGlobals(t, []domain.GlobalVariable{
		{Name: "counter", DataType: domain.DataNumber, DefaultValue: float64(0)},
	})
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
		{ID: "value", Type: "data:constant", Data: map[string]any{"config": map[string]any{"value": "not-a-number", "type": "text"}}},
		{ID: "write", Type: "flow:set_global_variable", Data: map[string]any{"config": map[string]any{"name": "counter", "operation": "set"}}},
	}, Edges: []domain.FlowEdge{
		execEdge("start-write", "start", "out", "write", "in"),
		dataEdge("value-write", "value", "value", "write", "value"),
	}}
	if _, err := NewEngine(catalog.New(), nil, nil, WithGlobalVariablesStore(globals)).Execute(context.Background(), flow, "start", Packet{}); err == nil {
		t.Fatal("Execute() accepted a text value for a number variable")
	}
}

func TestBlueprintGlobalGetUnknownFailsNode(t *testing.T) {
	globals := globalTestGlobals(t, nil)
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
		{ID: "read", Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": "missing"}}},
		{ID: "store", Type: "flow:set_variable", Data: map[string]any{"config": map[string]any{"name": "Sink"}}},
	}, Edges: []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("read-store", "read", "value", "store", "value"),
	}}
	if _, err := NewEngine(catalog.New(), nil, nil, WithGlobalVariablesStore(globals)).Execute(context.Background(), flow, "start", Packet{}); err == nil {
		t.Fatal("Execute() accepted an unknown global variable name")
	}
}

// TestBlueprintGlobalIncrementIsAtomicAcrossEngines mirrors concurrent pipelines
// hitting the same counter. A lost update would surface as a wrong total.
func TestBlueprintGlobalIncrementIsAtomicAcrossEngines(t *testing.T) {
	globals := globalTestGlobals(t, []domain.GlobalVariable{
		{Name: "visits", DataType: domain.DataNumber, DefaultValue: float64(0)},
	})
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
		{ID: "one", Type: "data:constant", Data: map[string]any{"config": map[string]any{"value": float64(1), "type": "number"}}},
		{ID: "inc", Type: "flow:set_global_variable", Data: map[string]any{"config": map[string]any{"name": "visits", "operation": "increment"}}},
	}, Edges: []domain.FlowEdge{
		execEdge("start-inc", "start", "out", "inc", "in"),
		dataEdge("one-inc", "one", "value", "inc", "value"),
	}}
	const runs = 32
	var wait sync.WaitGroup
	errors := make(chan error, runs)
	for index := 0; index < runs; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := NewEngine(catalog.New(), nil, nil, WithGlobalVariablesStore(globals)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
				errors <- err
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Fatalf("Execute() error = %v", err)
	}
	value, err := globals.Read("visits")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if value != float64(runs) {
		t.Fatalf("Read() = %#v, want %d after %d concurrent increments", value, runs, runs)
	}
}

// TestBlueprintGlobalNodeErrorsWhenHostUnavailable mirrors the existing
// pattern where a missing host surfaces an explicit node error rather than a
// nil-pointer crash.
func TestBlueprintGlobalNodeErrorsWhenHostUnavailable(t *testing.T) {
	read := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
		{ID: "read", Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": "whatever"}}},
		{ID: "store", Type: "flow:set_variable", Data: map[string]any{"config": map[string]any{"name": "Sink"}}},
	}, Edges: []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("read-store", "read", "value", "store", "value"),
	}}
	if _, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), read, "start", Packet{}); err == nil {
		t.Fatal("Execute() read did not fail when the global variable runtime is unavailable")
	}

	write := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
		{ID: "value", Type: "data:constant", Data: map[string]any{"config": map[string]any{"value": float64(1), "type": "number"}}},
		{ID: "write", Type: "flow:set_global_variable", Data: map[string]any{"config": map[string]any{"name": "whatever", "operation": "set"}}},
	}, Edges: []domain.FlowEdge{
		execEdge("start-write", "start", "out", "write", "in"),
		dataEdge("value-write", "value", "value", "write", "value"),
	}}
	if _, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), write, "start", Packet{}); err == nil {
		t.Fatal("Execute() write did not fail when the global variable runtime is unavailable")
	}
}

// TestVariablesListIncludesNewlyCreated mirrors the VariablesView preview list
// which follows the declaration through the in-memory summaries.
func TestVariablesListIncludesNewlyCreated(t *testing.T) {
	store, err := persistence.New(t.TempDir())
	if err != nil {
		t.Fatalf("persistence.New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	globals, err := variables.New(store)
	if err != nil {
		t.Fatalf("variables.New() error = %v", err)
	}
	for index := 0; index < 3; index++ {
		if _, err := globals.Create(context.Background(), domain.GlobalVariable{
			Name:         fmt.Sprintf("item_%d", index),
			DataType:     domain.DataText,
			DefaultValue: fmt.Sprintf("default-%d", index),
		}); err != nil {
			t.Fatalf("Create(%d) error = %v", index, err)
		}
	}
	summaries, err := globals.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(summaries) != 3 {
		t.Fatalf("List() len = %d, want 3", len(summaries))
	}
}

// TestBlueprintGlobalSetLiteralFallback covers the config-field fallback: a
// disconnected Value pin writes the inspector literal, coerced to the
// declared type. Wire priority stays pinned: a connected pin overrides it.
func TestBlueprintGlobalSetLiteralFallback(t *testing.T) {
	t.Run("literal coerced to declared type", func(t *testing.T) {
		globals := globalTestGlobals(t, []domain.GlobalVariable{
			{Name: "visits", DataType: domain.DataNumber, DefaultValue: float64(0)},
		})
		flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
			{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "write", Type: "flow:set_global_variable", Data: map[string]any{"config": map[string]any{"name": "visits", "operation": "set", "value": "42"}}},
		}, Edges: []domain.FlowEdge{
			execEdge("start-write", "start", "out", "write", "in"),
		}}
		if _, err := NewEngine(catalog.New(), nil, nil, WithGlobalVariablesStore(globals)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		value, err := globals.Read("visits")
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if value != float64(42) {
			t.Fatalf("Read() = %#v (%T), want float64(42)", value, value)
		}
	})

	t.Run("connected pin overrides literal", func(t *testing.T) {
		globals := globalTestGlobals(t, []domain.GlobalVariable{
			{Name: "visits", DataType: domain.DataNumber, DefaultValue: float64(0)},
		})
		flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
			{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "source", Type: "data:constant", Data: map[string]any{"config": map[string]any{"value": float64(7), "type": "number"}}},
			{ID: "write", Type: "flow:set_global_variable", Data: map[string]any{"config": map[string]any{"name": "visits", "operation": "set", "value": "99"}}},
		}, Edges: []domain.FlowEdge{
			execEdge("start-write", "start", "out", "write", "in"),
			dataEdge("source-write", "source", "value", "write", "value"),
		}}
		if _, err := NewEngine(catalog.New(), nil, nil, WithGlobalVariablesStore(globals)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		value, err := globals.Read("visits")
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if value != float64(7) {
			t.Fatalf("Read() = %#v, want the wired 7 to win over the literal 99", value)
		}
	})

	t.Run("invalid literal fails the node", func(t *testing.T) {
		globals := globalTestGlobals(t, []domain.GlobalVariable{
			{Name: "visits", DataType: domain.DataNumber, DefaultValue: float64(0)},
		})
		flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
			{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "write", Type: "flow:set_global_variable", Data: map[string]any{"config": map[string]any{"name": "visits", "operation": "set", "value": "not-a-number"}}},
		}, Edges: []domain.FlowEdge{
			execEdge("start-write", "start", "out", "write", "in"),
		}}
		if _, err := NewEngine(catalog.New(), nil, nil, WithGlobalVariablesStore(globals)).Execute(context.Background(), flow, "start", Packet{}); err == nil {
			t.Fatal("Execute() accepted an invalid numeric literal")
		}
	})

	t.Run("empty literal on increment means one", func(t *testing.T) {
		globals := globalTestGlobals(t, []domain.GlobalVariable{
			{Name: "counter", DataType: domain.DataNumber, DefaultValue: float64(0)},
		})
		flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
			{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "write", Type: "flow:set_global_variable", Data: map[string]any{"config": map[string]any{"name": "counter", "operation": "increment", "value": ""}}},
		}, Edges: []domain.FlowEdge{
			execEdge("start-write", "start", "out", "write", "in"),
		}}
		if _, err := NewEngine(catalog.New(), nil, nil, WithGlobalVariablesStore(globals)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		value, err := globals.Read("counter")
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if value != float64(1) {
			t.Fatalf("Read() = %#v, want increment-by-one on an empty literal", value)
		}
	})
}

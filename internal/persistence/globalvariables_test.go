package persistence_test

import (
	"context"
	"sync"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

func newStore(t *testing.T) (*persistence.Store, context.Context) {
	t.Helper()
	store, err := persistence.New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, context.Background()
}

func TestGlobalVariableCRUDLifecycle(t *testing.T) {
	store, ctx := newStore(t)
	created, err := store.CreateGlobalVariable(ctx, domain.GlobalVariable{
		Name:         "visits",
		Description:  "Counter",
		DataType:     domain.DataNumber,
		DefaultValue: float64(1),
	})
	if err != nil {
		t.Fatalf("CreateGlobalVariable() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("CreateGlobalVariable() returned an empty ID")
	}

	stored, err := store.GetGlobalVariable(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetGlobalVariable() error = %v", err)
	}
	if stored.Name != "visits" || stored.DataType != domain.DataNumber {
		t.Fatalf("GetGlobalVariable() = %#v", stored)
	}

	// Constrain which edits succeed.
	stored.Description = "Updated description"
	updated, err := store.UpdateGlobalVariableMetadata(ctx, stored)
	if err != nil {
		t.Fatalf("UpdateGlobalVariableMetadata() error = %v", err)
	}
	if updated.Description != "Updated description" {
		t.Fatalf("UpdateGlobalVariableMetadata() description = %q", updated.Description)
	}

	stored.Name = "renamed"
	if _, err := store.UpdateGlobalVariableMetadata(ctx, stored); err == nil {
		t.Fatal("UpdateGlobalVariableMetadata() accepted a rename")
	}
	stored, _ = store.GetGlobalVariable(ctx, created.ID)
	stored.DataType = domain.DataText
	if _, err := store.UpdateGlobalVariableMetadata(ctx, stored); err == nil {
		t.Fatal("UpdateGlobalVariableMetadata() accepted a type change")
	}

	if _, err := store.CreateGlobalVariable(ctx, domain.GlobalVariable{Name: "visits", DataType: domain.DataNumber}); err == nil {
		t.Fatal("CreateGlobalVariable() accepted a duplicate name")
	}

	if err := store.DeleteGlobalVariable(ctx, created.ID); err != nil {
		t.Fatalf("DeleteGlobalVariable() error = %v", err)
	}
	if _, err := store.GetGlobalVariable(ctx, created.ID); err == nil {
		t.Fatal("GetGlobalVariable() found a deleted variable")
	}
}

func TestGlobalVariableValuesRoundtrip(t *testing.T) {
	store, ctx := newStore(t)
	variable, err := store.CreateGlobalVariable(ctx, domain.GlobalVariable{
		Name:         "state",
		DataType:     domain.DataObject,
		DefaultValue: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateGlobalVariable() error = %v", err)
	}

	if err := store.FlushGlobalVariableValues(ctx, map[string]any{"state": map[string]any{"count": float64(3)}}); err != nil {
		t.Fatalf("FlushGlobalVariableValues() error = %v", err)
	}
	loaded, err := store.LoadGlobalVariableValues(ctx)
	if err != nil {
		t.Fatalf("LoadGlobalVariableValues() error = %v", err)
	}
	state, ok := loaded["state"].(map[string]any)
	if !ok || state["count"] != float64(3) {
		t.Fatalf("LoadGlobalVariableValues() = %#v", loaded)
	}

	// Overwrite in a single flushed transaction.
	if err := store.FlushGlobalVariableValues(ctx, map[string]any{"state": map[string]any{"count": float64(4)}}); err != nil {
		t.Fatalf("FlushGlobalVariableValues() second run error = %v", err)
	}
	loaded, err = store.LoadGlobalVariableValues(ctx)
	if err != nil {
		t.Fatalf("LoadGlobalVariableValues() after overwrite error = %v", err)
	}
	state, _ = loaded["state"].(map[string]any)
	if state["count"] != float64(4) {
		t.Fatalf("LoadGlobalVariableValues() after overwrite = %#v", loaded)
	}

	_ = variable // silence unused warning for the reader-friendliness guard
}

func TestDeleteGlobalVariableReferencedByPipeline(t *testing.T) {
	store, ctx := newStore(t)
	// Definition references the variable inside the new node type.
	definition := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{{ID: "read", Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": "shared"}}}, {ID: "start", Type: "trigger:button"}}}
	pipeline, err := store.CreatePipeline(ctx, "uses-variable", definition)
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	variable, err := store.CreateGlobalVariable(ctx, domain.GlobalVariable{Name: "shared", DataType: domain.DataText, DefaultValue: ""})
	if err != nil {
		t.Fatalf("CreateGlobalVariable() error = %v", err)
	}
	if err := store.DeleteGlobalVariable(ctx, variable.ID); err == nil {
		t.Fatal("DeleteGlobalVariable() accepted a referenced variable")
	}
	if err := store.DeletePipeline(ctx, pipeline.ID); err != nil {
		t.Fatalf("DeletePipeline() cleanup error = %v", err)
	}
	if err := store.DeleteGlobalVariable(ctx, variable.ID); err != nil {
		t.Fatalf("DeleteGlobalVariable() after unreferencing error = %v", err)
	}
}

func TestFlushGlobalVariableValuesWithoutDirtyWork(t *testing.T) {
	store, ctx := newStore(t)
	if err := store.FlushGlobalVariableValues(ctx, map[string]any{}); err != nil {
		t.Fatalf("FlushGlobalVariableValues() on empty map error = %v", err)
	}
}

func TestConcurrentFlushGlobalVariableValues(t *testing.T) {
	store, ctx := newStore(t)
	created, err := store.CreateGlobalVariable(ctx, domain.GlobalVariable{Name: "counter", DataType: domain.DataNumber, DefaultValue: float64(0)})
	if err != nil {
		t.Fatalf("CreateGlobalVariable() error = %v", err)
	}
	var wait sync.WaitGroup
	for index := 0; index < 32; index++ {
		wait.Add(1)
		go func(value float64) {
			defer wait.Done()
			if err := store.FlushGlobalVariableValues(ctx, map[string]any{created.Name: value}); err != nil {
				t.Errorf("FlushGlobalVariableValues() error = %v", err)
			}
		}(float64(index))
	}
	wait.Wait()
	loaded, err := store.LoadGlobalVariableValues(ctx)
	if err != nil {
		t.Fatalf("LoadGlobalVariableValues() error = %v", err)
	}
	if _, ok := loaded[created.Name]; !ok {
		t.Fatal("concurrent flush produced no persisted value")
	}
}

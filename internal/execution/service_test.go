package execution

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

func TestLimiterQueuesUntilContextIsCancelled(t *testing.T) {
	queue := newLimiter(1)
	if err := queue.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() first slot error = %v", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := queue.Acquire(cancelled); err == nil {
		t.Fatal("Acquire() while queue is full error = nil, want cancellation")
	}

	queue.Release()
	if err := queue.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	queue.Release()
}

func TestLimiterNormalizesInvalidCapacity(t *testing.T) {
	queue := newLimiter(0)
	if err := queue.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() with normalized capacity error = %v", err)
	}
	queue.Release()
}

func TestRunDraftExecutesAndPersistsConnectedNodes(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	definition := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV2,
		Nodes: []domain.FlowNode{
			{ID: "button", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "set", Type: "action:notification", Data: map[string]any{"config": map[string]any{"title": "Done", "message": "done"}}},
		},
		Edges: []domain.FlowEdge{{ID: "button-to-set", Source: "button", SourceHandle: "out", Target: "set", TargetHandle: "in", Kind: domain.PinExec}},
	}
	item, err := store.CreatePipeline(context.Background(), "Draft run", definition)
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	service := NewService(store, catalog.New(), nil, nil)

	execution, err := service.RunDraft(context.Background(), item.ID, "button", definition, pipeline.Packet{"trigger": "manual"})
	if err != nil {
		t.Fatalf("RunDraft() error = %v", err)
	}
	if execution.Status != domain.RunCompleted || len(execution.NodeRuns) != 2 {
		t.Fatalf("RunDraft() = %#v, want completed execution with two node runs", execution)
	}
	history, err := store.ListExecutions(context.Background(), item.ID, 20)
	if err != nil {
		t.Fatalf("ListExecutions() error = %v", err)
	}
	if len(history) != 1 || len(history[0].NodeRuns) != 2 {
		t.Fatalf("execution history = %#v, want persisted node results", history)
	}
}

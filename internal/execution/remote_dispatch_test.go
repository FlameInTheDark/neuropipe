package execution

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

// recordingDispatcher captures dispatched runs and can be told to fail.
type recordingDispatcher struct {
	mu           sync.Mutex
	runs         []RemoteDispatch
	targets      []string
	failDispatch bool
	cancelled    []string
}

func (d *recordingDispatcher) Dispatch(_ context.Context, executorID string, run RemoteDispatch) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failDispatch {
		return errors.New("executor is offline")
	}
	d.runs = append(d.runs, run)
	d.targets = append(d.targets, executorID)
	return nil
}

func (d *recordingDispatcher) CancelRun(_ context.Context, executorID, executionID string) error {
	d.cancelled = append(d.cancelled, executorID+"/"+executionID)
	return nil
}

func remoteButtonDefinition() domain.FlowDefinition {
	return domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			{ID: "button", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
		},
	}
}

func TestRunDraftTargetsRemoteExecutor(t *testing.T) {
	store := newTestStore(t)
	pipeline, err := store.CreatePipeline(context.Background(), "Remote draft", "exec-1", remoteButtonDefinition())
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	service := NewService(store, catalog.New(), nil, nil)
	dispatcher := &recordingDispatcher{}
	service.SetRemoteDispatcher(dispatcher)

	execution, err := service.RunDraft(context.Background(), pipeline.ID, "button", pipeline.DraftDefinition, nil)
	if err != nil {
		t.Fatalf("RunDraft() error = %v", err)
	}
	if len(dispatcher.runs) != 1 || dispatcher.targets[0] != "exec-1" {
		t.Fatalf("dispatched = %#v to %#v", dispatcher.runs, dispatcher.targets)
	}
	run := dispatcher.runs[0]
	if run.ExecutionID != execution.ID || run.TriggerNodeID != "button" {
		t.Fatalf("dispatch payload = %#v", run)
	}
	if run.EmbeddedDefinition == nil || len(run.EmbeddedDefinition.Nodes) != 1 {
		t.Fatalf("embedded definition missing from dispatch: %#v", run.EmbeddedDefinition)
	}

	stored, err := store.GetExecution(context.Background(), execution.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if stored.ExecutorID != "exec-1" || stored.Status == domain.RunCompleted {
		t.Fatalf("local record = %#v, want running remote record", stored)
	}
}

func TestApplyRemoteRunUpdateCompletesRecord(t *testing.T) {
	store := newTestStore(t)
	pipeline, err := store.CreatePipeline(context.Background(), "Remote draft", "exec-2", remoteButtonDefinition())
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	service := NewService(store, catalog.New(), nil, nil)
	dispatcher := &recordingDispatcher{}
	service.SetRemoteDispatcher(dispatcher)

	execution, err := service.RunDraft(context.Background(), pipeline.ID, "button", pipeline.DraftDefinition, nil)
	if err != nil {
		t.Fatalf("RunDraft() error = %v", err)
	}

	finished := time.Now().UTC()
	service.ApplyRemoteRunUpdate(domain.Execution{
		ID:         execution.ID,
		PipelineID: pipeline.ID,
		Status:     domain.RunCompleted,
		NodeRuns:   []domain.NodeRun{{NodeID: "button", NodeType: "trigger:button", Status: domain.RunCompleted}},
		FinishedAt: &finished,
	})

	stored, err := store.GetExecution(context.Background(), execution.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if stored.Status != domain.RunCompleted || stored.ExecutorID != "exec-2" || len(stored.NodeRuns) != 1 {
		t.Fatalf("completed record = %#v", stored)
	}

	// A duplicate event must not resurrect or double-write the run.
	before := stored.Status
	service.ApplyRemoteRunUpdate(domain.Execution{
		ID:         execution.ID,
		PipelineID: pipeline.ID,
		Status:     domain.RunFailed,
		Error:      "late failure",
	})
	after, _ := store.GetExecution(context.Background(), execution.ID)
	if after.Status != before {
		t.Fatalf("duplicate event changed status %s -> %s", before, after.Status)
	}
}

func TestRemoteDispatchOfflineFailsFast(t *testing.T) {
	store := newTestStore(t)
	pipeline, err := store.CreatePipeline(context.Background(), "Offline target", "exec-offline", remoteButtonDefinition())
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	service := NewService(store, catalog.New(), nil, nil)
	service.SetRemoteDispatcher(&recordingDispatcher{failDispatch: true})

	execution, err := service.RunDraft(context.Background(), pipeline.ID, "button", pipeline.DraftDefinition, nil)
	if err != nil {
		t.Fatalf("RunDraft() error = %v", err)
	}
	stored, err := store.GetExecution(context.Background(), execution.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if stored.Status != domain.RunFailed || stored.Error == "" {
		t.Fatalf("offline record = %#v, want explicit failure", stored)
	}
}

func newTestStore(t *testing.T) *persistence.Store {
	t.Helper()
	store, err := persistence.New(t.TempDir())
	if err != nil {
		t.Fatalf("persistence.New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestRemoteExecutorCRUD(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()

	executors, err := store.ListRemoteExecutors(ctx)
	if err != nil || len(executors) != 0 {
		t.Fatalf("initial ListRemoteExecutors() = %#v, %v", executors, err)
	}

	item := domain.RemoteExecutor{
		ID:        "exec-1",
		Name:      "Studio PC",
		Address:   "192.168.1.50:47777",
		TokenRef:  "executor-token:abc",
		UseTLS:    true,
		LLMMode:   domain.ExecutorLLMProxy,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.SaveRemoteExecutor(ctx, item); err != nil {
		t.Fatalf("SaveRemoteExecutor() error = %v", err)
	}

	got, err := store.GetRemoteExecutor(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetRemoteExecutor() error = %v", err)
	}
	if got.Name != item.Name || got.Address != item.Address || !got.UseTLS || got.TokenRef == "" {
		t.Fatalf("GetRemoteExecutor() = %#v", got)
	}

	list, err := store.ListRemoteExecutors(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListRemoteExecutors() = %#v, %v", list, err)
	}

	pipeline, err := store.CreatePipeline(ctx, "Remote job", item.ID, domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	if pipeline.ExecutorID != item.ID {
		t.Fatalf("pipeline executor = %q, want %q", pipeline.ExecutorID, item.ID)
	}
	if count, err := store.CountPipelinesForExecutor(ctx, item.ID); err != nil || count != 1 {
		t.Fatalf("CountPipelinesForExecutor() = %d, %v", count, err)
	}

	// Summaries carry the executor identity for the Remote category.
	summaries, err := store.ListPipelines(ctx)
	if err != nil || len(summaries) != 1 {
		t.Fatalf("ListPipelines() = %#v, %v", summaries, err)
	}
	if summaries[0].ExecutorID != item.ID || summaries[0].ExecutorName != item.Name {
		t.Fatalf("summary executor fields = %#v", summaries[0])
	}

	// Detaching turns its pipelines back into local ones.
	if err := store.DetachExecutorFromPipelines(ctx, item.ID); err != nil {
		t.Fatalf("DetachExecutorFromPipelines() error = %v", err)
	}
	if count, _ := store.CountPipelinesForExecutor(ctx, item.ID); count != 0 {
		t.Fatalf("count after detach = %d, want 0", count)
	}

	if err := store.DeleteRemoteExecutor(ctx, item.ID); err != nil {
		t.Fatalf("DeleteRemoteExecutor() error = %v", err)
	}
	if _, err := store.GetRemoteExecutor(ctx, item.ID); err == nil {
		t.Fatalf("GetRemoteExecutor() after delete = %v, want error", err)
	}
}

func TestPipelineExecutorIDDetectsTargeting(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()

	local, err := store.CreatePipeline(ctx, "Local", "", domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	if id := store.PipelineExecutorID(ctx, local.ID); id != "" {
		t.Fatalf("local pipeline target = %q, want empty", id)
	}
	if id := store.PipelineExecutorID(ctx, "missing"); id != "" {
		t.Fatalf("unknown pipeline target = %q, want empty", id)
	}
}

func TestAdoptRemoteExecutionIsIdempotent(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()

	pipeline, err := store.CreatePipeline(ctx, "Scheduled remotely", "", domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	now := time.Now().UTC()
	execution := domain.Execution{
		ID:         "exec-remote-1",
		PipelineID: pipeline.ID,
		Status:     domain.RunCompleted,
		StartedAt:  now,
		FinishedAt: &now,
		ExecutorID: "exec-9",
		NodeRuns: []domain.NodeRun{{
			NodeID:    "button",
			NodeType:  "trigger:cron",
			Status:    domain.RunCompleted,
			StartedAt: now,
		}},
	}
	if err := store.AdoptRemoteExecution(ctx, execution); err != nil {
		t.Fatalf("AdoptRemoteExecution() error = %v", err)
	}
	if err := store.AdoptRemoteExecution(ctx, execution); err != nil {
		t.Fatalf("second AdoptRemoteExecution() error = %v", err)
	}

	executions, err := store.ListExecutions(ctx, pipeline.ID, 10)
	if err != nil {
		t.Fatalf("ListExecutions() error = %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("execution count = %d, want exactly one adopted run", len(executions))
	}
	got := executions[0]
	if got.ExecutorID != "exec-9" || got.Status != domain.RunCompleted || len(got.NodeRuns) != 1 {
		t.Fatalf("adopted execution = %#v", got)
	}
}

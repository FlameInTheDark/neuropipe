package metrics

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

func TestOverviewRecordsExecutionQueueTimingAndPriceEstimate(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	pipeline, err := store.CreatePipeline(ctx, "Measured pipeline", "", domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	service := NewService(store, nil, nil, nil)
	service.UpdateSettings(domain.MetricsSettings{DetailRetentionDays: 30, RollupRetentionDays: 365, PriceRates: []domain.ModelPriceRate{{ProviderID: "hosted", Model: "gpt-test", InputUSDPerMillion: 2, OutputUSDPerMillion: 4}}})
	queued := time.Now().UTC().Add(-3 * time.Second)
	started := queued.Add(2 * time.Second)
	finished := started.Add(800 * time.Millisecond)
	execution := domain.Execution{
		ID: "metric-run", PipelineID: pipeline.ID, TriggerID: "draft:button", Status: domain.RunCompleted,
		StartedAt: queued, QueuedAt: &queued, RunStartedAt: &started, FinishedAt: &finished,
		NodeRuns: []domain.NodeRun{{NodeID: "terminal", NodeType: "action:terminal", Status: domain.RunCompleted, StartedAt: started, FinishedAt: finished}},
	}
	if err := service.RecordExecution(ctx, execution); err != nil {
		t.Fatalf("RecordExecution() error = %v", err)
	}
	if err := service.RecordLLM(ctx, domain.LLMUsage{ProviderID: "hosted", ProviderName: "Hosted", ProviderKind: domain.ProviderOpenAICompatible, Model: "gpt-test", TokensReported: true, PromptTokens: 500_000, CompletionTokens: 250_000, Succeeded: true, Duration: 800 * time.Millisecond, OccurredAt: finished}); err != nil {
		t.Fatalf("RecordLLM() error = %v", err)
	}

	overview, err := service.Overview(ctx, domain.MetricsFilter{From: queued.Add(-time.Hour), To: time.Now().UTC().Add(time.Hour)})
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if overview.Runs.Value != 1 || overview.SuccessRate.Value != 100 {
		t.Fatalf("run overview = %#v, want one successful run", overview)
	}
	if len(overview.QueueSeries) != 1 || overview.QueueSeries[0].Value < 1_900 || overview.QueueSeries[0].Value > 2_100 {
		t.Fatalf("queue series = %#v, want approximately two seconds", overview.QueueSeries)
	}
	if !overview.EstimatedCostUSD.Available || overview.EstimatedCostUSD.Value != 2 {
		t.Fatalf("estimated cost = %#v, want $2", overview.EstimatedCostUSD)
	}
}

func TestPurgeCompactsDetailedFactsIntoDailyRollups(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	pipeline, err := store.CreatePipeline(ctx, "Historical pipeline", "", domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	service := NewService(store, nil, nil, nil)
	service.UpdateSettings(domain.MetricsSettings{DetailRetentionDays: 1, RollupRetentionDays: 365})
	finished := time.Now().UTC().AddDate(0, 0, -3)
	started := finished.Add(-time.Second)
	execution := domain.Execution{ID: "old-metric-run", PipelineID: pipeline.ID, TriggerID: "draft:button", Status: domain.RunCompleted, StartedAt: started, RunStartedAt: &started, FinishedAt: &finished}
	if err := service.RecordExecution(ctx, execution); err != nil {
		t.Fatalf("RecordExecution() error = %v", err)
	}
	if err := service.Purge(ctx); err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	overview, err := service.Overview(ctx, domain.MetricsFilter{From: finished.Add(-time.Hour), To: finished.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if overview.Runs.Value != 1 || len(overview.RunSeries) != 1 {
		t.Fatalf("rollup overview = %#v, want one compacted run", overview)
	}
	if overview.P95DurationMS.Available {
		t.Fatalf("p95 = %#v, want unavailable when the range includes compacted data", overview.P95DurationMS)
	}
	data, err := store.ReadMetrics(ctx, domain.MetricsFilter{From: finished.Add(-time.Hour), To: finished.Add(time.Hour)})
	if err != nil {
		t.Fatalf("ReadMetrics() error = %v", err)
	}
	if len(data.Executions) != 0 || len(data.ExecutionRollups) != 1 {
		t.Fatalf("metric data after purge = %#v, want rollup only", data)
	}
}

func TestLocalAndUnpricedUsageRemainExplicit(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	service := NewService(store, nil, nil, nil)
	now := time.Now().UTC()
	ctx := context.Background()
	if err := service.RecordLLM(ctx, domain.LLMUsage{ProviderID: "local", ProviderName: "Managed llama.cpp", ProviderKind: domain.ProviderLlamaCPP, Model: "local.gguf", Succeeded: true, TokensReported: false, OccurredAt: now}); err != nil {
		t.Fatalf("RecordLLM(local) error = %v", err)
	}
	if err := service.RecordLLM(ctx, domain.LLMUsage{ProviderID: "remote", ProviderName: "Remote", ProviderKind: domain.ProviderOpenAICompatible, Model: "unpriced", Succeeded: true, TokensReported: true, PromptTokens: 1, OccurredAt: now}); err != nil {
		t.Fatalf("RecordLLM(unpriced) error = %v", err)
	}
	overview, err := service.Overview(ctx, domain.MetricsFilter{From: now.Add(-time.Hour), To: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if !overview.EstimatedCostUSD.Available || overview.LocalCalls != 1 || overview.UnpricedCalls != 1 || overview.TokensUnavailable != 1 {
		t.Fatalf("usage availability = %#v, want one unpriced remote call and one unreported local call", overview)
	}
}

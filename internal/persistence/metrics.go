package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/google/uuid"
)

// MetricsData is the numerical input consumed by the metrics service. It has
// no pipeline payloads, prompts, responses, user content, credentials, URLs,
// IP addresses, or error text.
type MetricsData struct {
	Executions       []domain.MetricExecutionEvent
	Nodes            []domain.MetricNodeEvent
	LLM              []domain.LLMUsage
	Activity         []domain.MetricActivityEvent
	Resources        []domain.MetricsResourcePoint
	ExecutionRollups []MetricExecutionRollup
	LLMRollups       []MetricLLMRollup
	ActivityRollups  []MetricActivityRollup
	ResourceRollups  []MetricResourceRollup
}

type MetricExecutionRollup struct {
	Bucket          time.Time
	PipelineID      string
	PipelineName    string
	TriggerKind     domain.TriggerKind
	Status          domain.RunStatus
	RunCount        int
	DurationSumMS   float64
	QueueWaitSumMS  float64
	NodeCount       int
	FailedNodeCount int
}

type MetricLLMRollup struct {
	Bucket              time.Time
	ProviderID          string
	ProviderName        string
	ProviderKind        domain.ProviderKind
	Model               string
	Succeeded           bool
	CallCount           int
	PromptTokens        int64
	CompletionTokens    int64
	TokensReportedCount int
	DurationSumMS       float64
	QueueWaitSumMS      float64
	EstimatedCostSumUSD float64
	PricedCount         int
}

type MetricActivityRollup struct {
	Bucket        time.Time
	Kind          string
	Outcome       string
	EventCount    int
	DurationSumMS float64
}

type MetricResourceRollup struct {
	Bucket         time.Time
	Process        string
	SampleCount    int
	CPUSum         float64
	CPUPeak        float64
	WorkingSetSum  int64
	WorkingSetPeak int64
}

// RecordMetricExecution stores a payload-free projection of a finished run.
// It is deliberately separate from execution history so metrics can outlive
// the detailed, redacted logs without retaining user data.
func (s *Store) RecordMetricExecution(ctx context.Context, execution domain.Execution) error {
	if strings.TrimSpace(execution.ID) == "" {
		return fmt.Errorf("record execution metric: missing execution ID")
	}
	occurred := time.Now().UTC()
	if execution.FinishedAt != nil {
		occurred = *execution.FinishedAt
	}
	started := execution.StartedAt
	if execution.RunStartedAt != nil {
		started = *execution.RunStartedAt
	}
	duration := occurred.Sub(started)
	// A queued execution which never reached a worker has no run duration. Its
	// queue wait remains observable independently, including failed/cancelled
	// queue outcomes.
	if duration < 0 || execution.RunStartedAt == nil {
		duration = 0
	}
	queueWait := time.Duration(0)
	if execution.QueuedAt != nil {
		queueEnd := occurred
		if execution.RunStartedAt != nil {
			queueEnd = *execution.RunStartedAt
		}
		if queueEnd.After(*execution.QueuedAt) {
			queueWait = queueEnd.Sub(*execution.QueuedAt)
		}
	}
	var pipelineName string
	if err := s.db.QueryRowContext(ctx, `SELECT name FROM pipelines WHERE id = ?`, execution.PipelineID).Scan(&pipelineName); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("load metric pipeline name: %w", err)
	}
	if pipelineName == "" {
		pipelineName = "Deleted pipeline"
	}
	triggerKind := s.metricTriggerKind(ctx, execution.TriggerID)
	failedNodes := 0
	for _, run := range execution.NodeRuns {
		if run.Status == domain.RunFailed {
			failedNodes++
		}
	}
	event := domain.MetricExecutionEvent{
		ExecutionID: execution.ID, PipelineID: execution.PipelineID, PipelineName: pipelineName, TriggerKind: triggerKind,
		Status: execution.Status, OccurredAt: occurred, DurationMS: float64(duration) / float64(time.Millisecond),
		QueueWaitMS: float64(queueWait) / float64(time.Millisecond), NodeCount: len(execution.NodeRuns), FailedNodeCount: failedNodes,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin execution metric: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO metric_execution_events (execution_id, pipeline_id, pipeline_name, trigger_kind, status, occurred_at, duration_ms, queue_wait_ms, node_count, failed_node_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.ExecutionID, event.PipelineID, event.PipelineName, event.TriggerKind, event.Status, stamp(event.OccurredAt), event.DurationMS, event.QueueWaitMS, event.NodeCount, event.FailedNodeCount); err != nil {
		return fmt.Errorf("insert execution metric: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM metric_node_events WHERE execution_id = ?`, event.ExecutionID); err != nil {
		return fmt.Errorf("clear execution node metrics: %w", err)
	}
	for _, run := range execution.NodeRuns {
		durationMS := float64(run.FinishedAt.Sub(run.StartedAt)) / float64(time.Millisecond)
		if durationMS < 0 {
			durationMS = 0
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO metric_node_events (execution_id, pipeline_id, node_type, status, occurred_at, duration_ms) VALUES (?, ?, ?, ?, ?, ?)`, event.ExecutionID, event.PipelineID, run.NodeType, run.Status, stamp(event.OccurredAt), durationMS); err != nil {
			return fmt.Errorf("insert node metric: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit execution metric: %w", err)
	}
	return nil
}

func (s *Store) metricTriggerKind(ctx context.Context, triggerID string) domain.TriggerKind {
	if strings.HasPrefix(triggerID, "draft:") {
		return "manual"
	}
	if strings.HasPrefix(triggerID, "api:") {
		return "api"
	}
	if strings.HasPrefix(triggerID, "chat:") {
		return domain.TriggerChat
	}
	var kind string
	if err := s.db.QueryRowContext(ctx, `SELECT kind FROM trigger_bindings WHERE id = ?`, triggerID).Scan(&kind); err == nil {
		return domain.TriggerKind(kind)
	}
	return "manual"
}

// RecordMetricLLM stores only provider-level numerical usage and timing.
func (s *Store) RecordMetricLLM(ctx context.Context, usage domain.LLMUsage) error {
	if usage.OccurredAt.IsZero() {
		usage.OccurredAt = time.Now().UTC()
	}
	if strings.TrimSpace(usage.ProviderID) == "" || strings.TrimSpace(usage.Model) == "" {
		return fmt.Errorf("record LLM metric: provider and model are required")
	}
	var cost any
	if usage.EstimatedCostUSD != nil {
		cost = *usage.EstimatedCostUSD
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO metric_llm_events (id, execution_id, chat_run_id, pipeline_id, node_type, origin, provider_id, provider_name, provider_kind, model, succeeded, tokens_reported, prompt_tokens, completion_tokens, queue_wait_ms, duration_ms, estimated_cost_usd, occurred_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, uuid.NewString(), usage.Context.ExecutionID, usage.Context.ChatRunID, usage.Context.PipelineID, usage.Context.NodeType, usage.Context.Origin, usage.ProviderID, usage.ProviderName, usage.ProviderKind, usage.Model, usage.Succeeded, usage.TokensReported, usage.PromptTokens, usage.CompletionTokens, float64(usage.QueueWait)/float64(time.Millisecond), float64(usage.Duration)/float64(time.Millisecond), cost, stamp(usage.OccurredAt))
	if err != nil {
		return fmt.Errorf("insert LLM metric: %w", err)
	}
	return nil
}

// RecordMetricActivity records a numerical count for app activity without its content.
func (s *Store) RecordMetricActivity(ctx context.Context, event domain.MetricActivityEvent) error {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if strings.TrimSpace(event.Kind) == "" {
		return fmt.Errorf("record activity metric: kind is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO metric_activity_events (id, kind, outcome, duration_ms, occurred_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), event.Kind, event.Outcome, event.DurationMS, stamp(event.OccurredAt))
	if err != nil {
		return fmt.Errorf("insert activity metric: %w", err)
	}
	return nil
}

// RecordMetricResource stores a safe snapshot for a Neuropipe-owned process.
func (s *Store) RecordMetricResource(ctx context.Context, point domain.MetricsResourcePoint) error {
	if point.At.IsZero() {
		point.At = time.Now().UTC()
	}
	if strings.TrimSpace(point.Process) == "" {
		return fmt.Errorf("record resource metric: process is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO metric_resource_samples (id, process_name, cpu_percent, working_set_bytes, occurred_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), point.Process, point.CPUPercent, point.WorkingSet, stamp(point.At))
	if err != nil {
		return fmt.Errorf("insert resource metric: %w", err)
	}
	return nil
}

// PurgeMetrics compacts detailed numerical facts into daily aggregates before
// deleting them, then expires aggregates outside the configured retention.
func (s *Store) PurgeMetrics(ctx context.Context, settings domain.MetricsSettings) error {
	detailDays := settings.DetailRetentionDays
	if detailDays < 1 {
		detailDays = 30
	}
	rollupDays := settings.RollupRetentionDays
	if rollupDays < detailDays {
		rollupDays = 365
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -detailDays)
	rollupCutoff := time.Now().UTC().AddDate(0, 0, -rollupDays).Format("2006-01-02")
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin metrics cleanup: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stampCutoff := stamp(cutoff)
	statements := []string{
		`INSERT INTO metric_execution_rollups (bucket, pipeline_id, pipeline_name, trigger_kind, status, run_count, duration_sum_ms, queue_wait_sum_ms, node_count, failed_node_count) SELECT substr(occurred_at, 1, 10), pipeline_id, pipeline_name, trigger_kind, status, COUNT(*), SUM(duration_ms), SUM(queue_wait_ms), SUM(node_count), SUM(failed_node_count) FROM metric_execution_events WHERE occurred_at < ? GROUP BY substr(occurred_at, 1, 10), pipeline_id, pipeline_name, trigger_kind, status ON CONFLICT(bucket, pipeline_id, trigger_kind, status) DO UPDATE SET run_count = run_count + excluded.run_count, duration_sum_ms = duration_sum_ms + excluded.duration_sum_ms, queue_wait_sum_ms = queue_wait_sum_ms + excluded.queue_wait_sum_ms, node_count = node_count + excluded.node_count, failed_node_count = failed_node_count + excluded.failed_node_count`,
		`INSERT INTO metric_llm_rollups (bucket, provider_id, provider_name, provider_kind, model, succeeded, call_count, prompt_tokens, completion_tokens, tokens_reported_count, duration_sum_ms, queue_wait_sum_ms, estimated_cost_sum_usd, priced_count) SELECT substr(occurred_at, 1, 10), provider_id, provider_name, provider_kind, model, succeeded, COUNT(*), SUM(prompt_tokens), SUM(completion_tokens), SUM(tokens_reported), SUM(duration_ms), SUM(queue_wait_ms), SUM(COALESCE(estimated_cost_usd, 0)), SUM(CASE WHEN estimated_cost_usd IS NULL THEN 0 ELSE 1 END) FROM metric_llm_events WHERE occurred_at < ? GROUP BY substr(occurred_at, 1, 10), provider_id, provider_name, provider_kind, model, succeeded ON CONFLICT(bucket, provider_id, model, succeeded) DO UPDATE SET call_count = call_count + excluded.call_count, prompt_tokens = prompt_tokens + excluded.prompt_tokens, completion_tokens = completion_tokens + excluded.completion_tokens, tokens_reported_count = tokens_reported_count + excluded.tokens_reported_count, duration_sum_ms = duration_sum_ms + excluded.duration_sum_ms, queue_wait_sum_ms = queue_wait_sum_ms + excluded.queue_wait_sum_ms, estimated_cost_sum_usd = estimated_cost_sum_usd + excluded.estimated_cost_sum_usd, priced_count = priced_count + excluded.priced_count`,
		`INSERT INTO metric_activity_rollups (bucket, kind, outcome, event_count, duration_sum_ms) SELECT substr(occurred_at, 1, 10), kind, outcome, COUNT(*), SUM(duration_ms) FROM metric_activity_events WHERE occurred_at < ? GROUP BY substr(occurred_at, 1, 10), kind, outcome ON CONFLICT(bucket, kind, outcome) DO UPDATE SET event_count = event_count + excluded.event_count, duration_sum_ms = duration_sum_ms + excluded.duration_sum_ms`,
		`INSERT INTO metric_resource_rollups (bucket, process_name, sample_count, cpu_sum, cpu_peak, working_set_sum, working_set_peak) SELECT substr(occurred_at, 1, 10), process_name, COUNT(*), SUM(cpu_percent), MAX(cpu_percent), SUM(working_set_bytes), MAX(working_set_bytes) FROM metric_resource_samples WHERE occurred_at < ? GROUP BY substr(occurred_at, 1, 10), process_name ON CONFLICT(bucket, process_name) DO UPDATE SET sample_count = sample_count + excluded.sample_count, cpu_sum = cpu_sum + excluded.cpu_sum, cpu_peak = MAX(cpu_peak, excluded.cpu_peak), working_set_sum = working_set_sum + excluded.working_set_sum, working_set_peak = MAX(working_set_peak, excluded.working_set_peak)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement, stampCutoff); err != nil {
			return fmt.Errorf("roll up metrics: %w", err)
		}
	}
	for _, table := range []string{"metric_execution_events", "metric_node_events", "metric_llm_events", "metric_activity_events", "metric_resource_samples"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE occurred_at < ?", table), stampCutoff); err != nil {
			return fmt.Errorf("purge %s: %w", table, err)
		}
	}
	for _, table := range []string{"metric_execution_rollups", "metric_llm_rollups", "metric_activity_rollups", "metric_resource_rollups"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE bucket < ?", table), rollupCutoff); err != nil {
			return fmt.Errorf("purge %s: %w", table, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit metrics cleanup: %w", err)
	}
	return nil
}

// ClearMetrics removes only numerical metrics and never touches executions,
// pipelines, reports, conversations, settings, or secrets.
func (s *Store) ClearMetrics(ctx context.Context) error {
	for _, table := range []string{"metric_execution_events", "metric_node_events", "metric_llm_events", "metric_activity_events", "metric_resource_samples", "metric_execution_rollups", "metric_llm_rollups", "metric_activity_rollups", "metric_resource_rollups"} {
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	return nil
}

// ReadMetrics loads raw facts and daily rollups for a time range. Dimension
// filtering is intentionally applied in Go so the query shape remains stable
// and SQLite can reuse its timestamp indexes.
func (s *Store) ReadMetrics(ctx context.Context, filter domain.MetricsFilter) (MetricsData, error) {
	from, to := metricRange(filter)
	data := MetricsData{}
	var err error
	if data.Executions, err = s.readMetricExecutions(ctx, from, to, filter); err != nil {
		return MetricsData{}, err
	}
	if data.Nodes, err = s.readMetricNodes(ctx, from, to, filter); err != nil {
		return MetricsData{}, err
	}
	if data.LLM, err = s.readMetricLLM(ctx, from, to, filter); err != nil {
		return MetricsData{}, err
	}
	if data.Activity, err = s.readMetricActivity(ctx, from, to); err != nil {
		return MetricsData{}, err
	}
	if data.Resources, err = s.readMetricResources(ctx, from, to); err != nil {
		return MetricsData{}, err
	}
	if data.ExecutionRollups, err = s.readMetricExecutionRollups(ctx, from, to, filter); err != nil {
		return MetricsData{}, err
	}
	if data.LLMRollups, err = s.readMetricLLMRollups(ctx, from, to, filter); err != nil {
		return MetricsData{}, err
	}
	if data.ActivityRollups, err = s.readMetricActivityRollups(ctx, from, to); err != nil {
		return MetricsData{}, err
	}
	if data.ResourceRollups, err = s.readMetricResourceRollups(ctx, from, to); err != nil {
		return MetricsData{}, err
	}
	return data, nil
}

func metricRange(filter domain.MetricsFilter) (time.Time, time.Time) {
	to := filter.To.UTC()
	if to.IsZero() {
		to = time.Now().UTC()
	}
	from := filter.From.UTC()
	if from.IsZero() || !from.Before(to) {
		from = to.AddDate(0, 0, -30)
	}
	return from, to
}

func (s *Store) readMetricExecutions(ctx context.Context, from, to time.Time, filter domain.MetricsFilter) ([]domain.MetricExecutionEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT execution_id, pipeline_id, pipeline_name, trigger_kind, status, occurred_at, duration_ms, queue_wait_ms, node_count, failed_node_count FROM metric_execution_events WHERE occurred_at >= ? AND occurred_at <= ? ORDER BY occurred_at`, stamp(from), stamp(to))
	if err != nil {
		return nil, fmt.Errorf("read execution metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]domain.MetricExecutionEvent, 0)
	for rows.Next() {
		var item domain.MetricExecutionEvent
		var occurred, trigger, status string
		if err := rows.Scan(&item.ExecutionID, &item.PipelineID, &item.PipelineName, &trigger, &status, &occurred, &item.DurationMS, &item.QueueWaitMS, &item.NodeCount, &item.FailedNodeCount); err != nil {
			return nil, fmt.Errorf("scan execution metric: %w", err)
		}
		item.TriggerKind, item.Status, item.OccurredAt = domain.TriggerKind(trigger), domain.RunStatus(status), parseTime(occurred)
		if metricExecutionMatches(item, filter) {
			result = append(result, item)
		}
	}
	return result, rows.Err()
}

func (s *Store) readMetricNodes(ctx context.Context, from, to time.Time, filter domain.MetricsFilter) ([]domain.MetricNodeEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT execution_id, pipeline_id, node_type, status, occurred_at, duration_ms FROM metric_node_events WHERE occurred_at >= ? AND occurred_at <= ? ORDER BY occurred_at`, stamp(from), stamp(to))
	if err != nil {
		return nil, fmt.Errorf("read node metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]domain.MetricNodeEvent, 0)
	for rows.Next() {
		var item domain.MetricNodeEvent
		var status, occurred string
		if err := rows.Scan(&item.ExecutionID, &item.PipelineID, &item.NodeType, &status, &occurred, &item.DurationMS); err != nil {
			return nil, fmt.Errorf("scan node metric: %w", err)
		}
		item.Status, item.OccurredAt = domain.RunStatus(status), parseTime(occurred)
		if containsString(filter.PipelineIDs, item.PipelineID) || len(filter.PipelineIDs) == 0 {
			result = append(result, item)
		}
	}
	return result, rows.Err()
}

func (s *Store) readMetricLLM(ctx context.Context, from, to time.Time, filter domain.MetricsFilter) ([]domain.LLMUsage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT execution_id, chat_run_id, pipeline_id, node_type, origin, provider_id, provider_name, provider_kind, model, succeeded, tokens_reported, prompt_tokens, completion_tokens, queue_wait_ms, duration_ms, estimated_cost_usd, occurred_at FROM metric_llm_events WHERE occurred_at >= ? AND occurred_at <= ? ORDER BY occurred_at`, stamp(from), stamp(to))
	if err != nil {
		return nil, fmt.Errorf("read LLM metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]domain.LLMUsage, 0)
	for rows.Next() {
		var item domain.LLMUsage
		var kind, occurred string
		var cost sql.NullFloat64
		var queueWaitMS, durationMS float64
		if err := rows.Scan(&item.Context.ExecutionID, &item.Context.ChatRunID, &item.Context.PipelineID, &item.Context.NodeType, &item.Context.Origin, &item.ProviderID, &item.ProviderName, &kind, &item.Model, &item.Succeeded, &item.TokensReported, &item.PromptTokens, &item.CompletionTokens, &queueWaitMS, &durationMS, &cost, &occurred); err != nil {
			return nil, fmt.Errorf("scan LLM metric: %w", err)
		}
		item.ProviderKind, item.OccurredAt = domain.ProviderKind(kind), parseTime(occurred)
		item.QueueWait, item.Duration = time.Duration(queueWaitMS*float64(time.Millisecond)), time.Duration(durationMS*float64(time.Millisecond))
		if cost.Valid {
			value := cost.Float64
			item.EstimatedCostUSD = &value
		}
		if (len(filter.ProviderIDs) == 0 || containsString(filter.ProviderIDs, item.ProviderID)) && (len(filter.Models) == 0 || containsString(filter.Models, item.Model)) && (len(filter.PipelineIDs) == 0 || containsString(filter.PipelineIDs, item.Context.PipelineID)) {
			result = append(result, item)
		}
	}
	return result, rows.Err()
}

func (s *Store) readMetricActivity(ctx context.Context, from, to time.Time) ([]domain.MetricActivityEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT kind, outcome, duration_ms, occurred_at FROM metric_activity_events WHERE occurred_at >= ? AND occurred_at <= ? ORDER BY occurred_at`, stamp(from), stamp(to))
	if err != nil {
		return nil, fmt.Errorf("read activity metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]domain.MetricActivityEvent, 0)
	for rows.Next() {
		var item domain.MetricActivityEvent
		var occurred string
		if err := rows.Scan(&item.Kind, &item.Outcome, &item.DurationMS, &occurred); err != nil {
			return nil, fmt.Errorf("scan activity metric: %w", err)
		}
		item.OccurredAt = parseTime(occurred)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) readMetricResources(ctx context.Context, from, to time.Time) ([]domain.MetricsResourcePoint, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT process_name, cpu_percent, working_set_bytes, occurred_at FROM metric_resource_samples WHERE occurred_at >= ? AND occurred_at <= ? ORDER BY occurred_at`, stamp(from), stamp(to))
	if err != nil {
		return nil, fmt.Errorf("read resource metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]domain.MetricsResourcePoint, 0)
	for rows.Next() {
		var item domain.MetricsResourcePoint
		var occurred string
		if err := rows.Scan(&item.Process, &item.CPUPercent, &item.WorkingSet, &occurred); err != nil {
			return nil, fmt.Errorf("scan resource metric: %w", err)
		}
		item.At = parseTime(occurred)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) readMetricExecutionRollups(ctx context.Context, from, to time.Time, filter domain.MetricsFilter) ([]MetricExecutionRollup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT bucket, pipeline_id, pipeline_name, trigger_kind, status, run_count, duration_sum_ms, queue_wait_sum_ms, node_count, failed_node_count FROM metric_execution_rollups WHERE bucket >= ? AND bucket <= ? ORDER BY bucket`, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("read execution rollups: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]MetricExecutionRollup, 0)
	for rows.Next() {
		var item MetricExecutionRollup
		var bucket, trigger, status string
		if err := rows.Scan(&bucket, &item.PipelineID, &item.PipelineName, &trigger, &status, &item.RunCount, &item.DurationSumMS, &item.QueueWaitSumMS, &item.NodeCount, &item.FailedNodeCount); err != nil {
			return nil, fmt.Errorf("scan execution rollup: %w", err)
		}
		item.Bucket, item.TriggerKind, item.Status = parseTime(bucket+"T00:00:00Z"), domain.TriggerKind(trigger), domain.RunStatus(status)
		if metricExecutionMatches(domain.MetricExecutionEvent{PipelineID: item.PipelineID, TriggerKind: item.TriggerKind, Status: item.Status}, filter) {
			result = append(result, item)
		}
	}
	return result, rows.Err()
}

func (s *Store) readMetricLLMRollups(ctx context.Context, from, to time.Time, filter domain.MetricsFilter) ([]MetricLLMRollup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT bucket, provider_id, provider_name, provider_kind, model, succeeded, call_count, prompt_tokens, completion_tokens, tokens_reported_count, duration_sum_ms, queue_wait_sum_ms, estimated_cost_sum_usd, priced_count FROM metric_llm_rollups WHERE bucket >= ? AND bucket <= ? ORDER BY bucket`, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("read LLM rollups: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]MetricLLMRollup, 0)
	for rows.Next() {
		var item MetricLLMRollup
		var bucket, kind string
		if err := rows.Scan(&bucket, &item.ProviderID, &item.ProviderName, &kind, &item.Model, &item.Succeeded, &item.CallCount, &item.PromptTokens, &item.CompletionTokens, &item.TokensReportedCount, &item.DurationSumMS, &item.QueueWaitSumMS, &item.EstimatedCostSumUSD, &item.PricedCount); err != nil {
			return nil, fmt.Errorf("scan LLM rollup: %w", err)
		}
		item.Bucket, item.ProviderKind = parseTime(bucket+"T00:00:00Z"), domain.ProviderKind(kind)
		if (len(filter.ProviderIDs) == 0 || containsString(filter.ProviderIDs, item.ProviderID)) && (len(filter.Models) == 0 || containsString(filter.Models, item.Model)) {
			result = append(result, item)
		}
	}
	return result, rows.Err()
}

func (s *Store) readMetricActivityRollups(ctx context.Context, from, to time.Time) ([]MetricActivityRollup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT bucket, kind, outcome, event_count, duration_sum_ms FROM metric_activity_rollups WHERE bucket >= ? AND bucket <= ? ORDER BY bucket`, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("read activity rollups: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]MetricActivityRollup, 0)
	for rows.Next() {
		var item MetricActivityRollup
		var bucket string
		if err := rows.Scan(&bucket, &item.Kind, &item.Outcome, &item.EventCount, &item.DurationSumMS); err != nil {
			return nil, fmt.Errorf("scan activity rollup: %w", err)
		}
		item.Bucket = parseTime(bucket + "T00:00:00Z")
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) readMetricResourceRollups(ctx context.Context, from, to time.Time) ([]MetricResourceRollup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT bucket, process_name, sample_count, cpu_sum, cpu_peak, working_set_sum, working_set_peak FROM metric_resource_rollups WHERE bucket >= ? AND bucket <= ? ORDER BY bucket`, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("read resource rollups: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]MetricResourceRollup, 0)
	for rows.Next() {
		var item MetricResourceRollup
		var bucket string
		if err := rows.Scan(&bucket, &item.Process, &item.SampleCount, &item.CPUSum, &item.CPUPeak, &item.WorkingSetSum, &item.WorkingSetPeak); err != nil {
			return nil, fmt.Errorf("scan resource rollup: %w", err)
		}
		item.Bucket = parseTime(bucket + "T00:00:00Z")
		result = append(result, item)
	}
	return result, rows.Err()
}

func metricExecutionMatches(item domain.MetricExecutionEvent, filter domain.MetricsFilter) bool {
	if len(filter.PipelineIDs) > 0 && !containsString(filter.PipelineIDs, item.PipelineID) {
		return false
	}
	if len(filter.TriggerKinds) > 0 && !containsTrigger(filter.TriggerKinds, item.TriggerKind) {
		return false
	}
	if len(filter.Statuses) > 0 && !containsStatus(filter.Statuses, item.Status) {
		return false
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func containsTrigger(values []domain.TriggerKind, target domain.TriggerKind) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func containsStatus(values []domain.RunStatus, target domain.RunStatus) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

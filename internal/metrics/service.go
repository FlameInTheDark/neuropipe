// Package metrics provides local-only operational observability for Neuropipe.
package metrics

import (
	"context"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

// EventSink notifies the renderer that its currently visible metric query may
// be refreshed. It never carries prompts, payloads, credentials, or logs.
type EventSink func(event string, payload any)

// ProcessSampler reads CPU and working-set values for an owned process only.
type ProcessSampler interface {
	Sample(pid int) (cpuPercent float64, workingSetBytes int64, err error)
}

// Service is the single recorder and reader for local metric facts.
type Service struct {
	store    *persistence.Store
	emit     EventSink
	sampler  ProcessSampler
	llamaPID func() int

	mu       sync.RWMutex
	settings domain.MetricsSettings
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewService builds a metrics service with an optional managed-runtime PID source.
func NewService(store *persistence.Store, emit EventSink, sampler ProcessSampler, llamaPID func() int) *Service {
	return &Service{store: store, emit: emit, sampler: sampler, llamaPID: llamaPID}
}

// Start begins the one owned resource-sampling worker. Calls are idempotent.
func (s *Service) Start(ctx context.Context, settings domain.MetricsSettings) {
	s.mu.Lock()
	if s.cancel != nil {
		s.settings = settings
		s.mu.Unlock()
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	s.settings, s.cancel = settings, cancel
	s.wg.Add(1)
	s.mu.Unlock()
	go s.sampleLoop(workerCtx)
}

// UpdateSettings changes retention/pricing immediately. The owned sampler
// observes a new bounded interval on its next tick without spawning a worker.
func (s *Service) UpdateSettings(settings domain.MetricsSettings) {
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
}

// Stop terminates the owned sampler before application storage closes.
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		s.wg.Wait()
	}
}

// RecordExecution persists a payload-free terminal execution projection.
func (s *Service) RecordExecution(ctx context.Context, execution domain.Execution) error {
	if err := s.store.RecordMetricExecution(ctx, execution); err != nil {
		return err
	}
	s.updated()
	return nil
}

// RecordLLM applies a locally configured price estimate before persisting only
// numerical provider/model usage and timings.
func (s *Service) RecordLLM(ctx context.Context, usage domain.LLMUsage) error {
	if usage.ProviderKind == domain.ProviderLlamaCPP || usage.ProviderKind == domain.ProviderOllama {
		zero := 0.0
		usage.EstimatedCostUSD = &zero
	} else if usage.TokensReported {
		if rate, ok := s.priceRate(usage.ProviderID, usage.Model); ok {
			cost := float64(usage.PromptTokens)*rate.InputUSDPerMillion/1_000_000 + float64(usage.CompletionTokens)*rate.OutputUSDPerMillion/1_000_000
			usage.EstimatedCostUSD = &cost
		}
	}
	if err := s.store.RecordMetricLLM(ctx, usage); err != nil {
		return err
	}
	s.updated()
	return nil
}

// RecordActivity writes a safe app activity counter.
func (s *Service) RecordActivity(ctx context.Context, event domain.MetricActivityEvent) error {
	if err := s.store.RecordMetricActivity(ctx, event); err != nil {
		return err
	}
	s.updated()
	return nil
}

// Overview returns a filtered dashboard plus an equal-duration comparison.
func (s *Service) Overview(ctx context.Context, filter domain.MetricsFilter) (domain.MetricsOverview, error) {
	filter = normalizeFilter(filter)
	current, err := s.store.ReadMetrics(ctx, filter)
	if err != nil {
		return domain.MetricsOverview{}, err
	}
	priorFilter := filter
	span := filter.To.Sub(filter.From)
	priorFilter.To, priorFilter.From = filter.From, filter.From.Add(-span)
	prior, err := s.store.ReadMetrics(ctx, priorFilter)
	if err != nil {
		return domain.MetricsOverview{}, err
	}
	currentSummary := summarize(current, filter)
	priorSummary := summarize(prior, priorFilter)
	overview := currentSummary.overview(filter)
	overview.Runs.PreviousValue = float64(priorSummary.runs)
	overview.SuccessRate.PreviousValue = priorSummary.successRate()
	overview.AverageDurationMS.PreviousValue = priorSummary.averageDuration()
	if overview.P95DurationMS.Available {
		overview.P95DurationMS.PreviousValue = priorSummary.p95()
	}
	overview.LLMCalls.PreviousValue = float64(priorSummary.llmCalls)
	overview.PromptTokens.PreviousValue = float64(priorSummary.promptTokens)
	overview.CompletionTokens.PreviousValue = float64(priorSummary.completionTokens)
	overview.EstimatedCostUSD.PreviousValue = priorSummary.estimatedCost
	return overview, nil
}

// Clear removes local metric facts and aggregate rollups only.
func (s *Service) Clear(ctx context.Context) error {
	if err := s.store.ClearMetrics(ctx); err != nil {
		return err
	}
	s.updated()
	return nil
}

// Purge compacts and expires local numerical facts using the active policy.
func (s *Service) Purge(ctx context.Context) error {
	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	return s.store.PurgeMetrics(ctx, settings)
}

func (s *Service) priceRate(providerID, model string) (domain.ModelPriceRate, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rate := range s.settings.PriceRates {
		if rate.ProviderID == providerID && strings.EqualFold(strings.TrimSpace(rate.Model), strings.TrimSpace(model)) {
			return rate, true
		}
	}
	return domain.ModelPriceRate{}, false
}

func (s *Service) sampleLoop(ctx context.Context) {
	defer s.wg.Done()
	interval := s.currentSampleInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.sample(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sample(ctx)
			nextInterval := s.currentSampleInterval()
			if nextInterval != interval {
				ticker.Stop()
				interval = nextInterval
				ticker = time.NewTicker(interval)
			}
		}
	}
}

func (s *Service) currentSampleInterval() time.Duration {
	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	return samplingInterval(settings)
}

func (s *Service) sample(ctx context.Context) {
	if s.sampler == nil {
		return
	}
	s.sampleProcess(ctx, "Neuropipe", os.Getpid())
	if s.llamaPID != nil {
		if pid := s.llamaPID(); pid > 0 {
			s.sampleProcess(ctx, "llama.cpp", pid)
		}
	}
}

func (s *Service) sampleProcess(ctx context.Context, name string, pid int) {
	cpu, memory, err := s.sampler.Sample(pid)
	if err != nil {
		return
	}
	if err := s.store.RecordMetricResource(ctx, domain.MetricsResourcePoint{Process: name, CPUPercent: cpu, WorkingSet: memory, At: time.Now().UTC()}); err == nil {
		s.updated()
	}
}

func (s *Service) updated() {
	if s.emit != nil {
		s.emit("metrics.updated", map[string]string{"source": "local"})
	}
}

func samplingInterval(settings domain.MetricsSettings) time.Duration {
	seconds := settings.SampleIntervalSeconds
	if seconds < 10 {
		seconds = 30
	}
	if seconds > 300 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

func normalizeFilter(filter domain.MetricsFilter) domain.MetricsFilter {
	filter.To = filter.To.UTC()
	if filter.To.IsZero() {
		filter.To = time.Now().UTC()
	}
	filter.From = filter.From.UTC()
	if filter.From.IsZero() || !filter.From.Before(filter.To) {
		filter.From = filter.To.AddDate(0, 0, -30)
	}
	return filter
}

type summary struct {
	runs, completed, failed, skipped, cancelled               int
	durationSum, queueSum                                     float64
	durations                                                 []float64
	llmCalls                                                  int
	promptTokens, completionTokens                            int64
	estimatedCost                                             float64
	pricedCalls, unpricedCalls, localCalls, tokensUnavailable int
	hasExecutionRollups                                       bool
	runBuckets                                                map[time.Time]*domain.MetricsRunPoint
	durationBuckets                                           map[time.Time]*bucketValues
	llmBuckets                                                map[time.Time]*bucketValues
	queueBuckets                                              map[time.Time]*bucketValues
	pipelineBuckets                                           map[string]*pipelineBucket
	failures                                                  map[string]float64
	slowNodes                                                 map[string]*bucketValues
	models                                                    map[string]*modelBucket
	triggers                                                  map[string]float64
	activity                                                  map[string]float64
	resources                                                 []domain.MetricsResourcePoint
}

type bucketValues struct {
	sum   float64
	count int
	extra float64
}
type pipelineBucket struct {
	id, name          string
	at                time.Time
	completed, failed int
}
type modelBucket struct {
	label    string
	calls    int
	duration float64
}

func summarize(data persistence.MetricsData, filter domain.MetricsFilter) summary {
	value := summary{
		runBuckets: make(map[time.Time]*domain.MetricsRunPoint), durationBuckets: make(map[time.Time]*bucketValues), llmBuckets: make(map[time.Time]*bucketValues), queueBuckets: make(map[time.Time]*bucketValues), pipelineBuckets: make(map[string]*pipelineBucket), failures: make(map[string]float64), slowNodes: make(map[string]*bucketValues), models: make(map[string]*modelBucket), triggers: make(map[string]float64), activity: make(map[string]float64), resources: append([]domain.MetricsResourcePoint{}, data.Resources...),
	}
	for _, item := range data.Executions {
		value.addExecution(item, bucketFor(item.OccurredAt, filter), true)
	}
	for _, item := range data.ExecutionRollups {
		value.hasExecutionRollups = true
		value.addExecutionRollup(item, bucketFor(item.Bucket, filter))
	}
	for _, item := range data.Nodes {
		if item.Status == domain.RunFailed {
			value.failures[item.NodeType]++
		}
		slow := value.slowNodes[item.NodeType]
		if slow == nil {
			slow = &bucketValues{}
			value.slowNodes[item.NodeType] = slow
		}
		slow.sum += item.DurationMS
		slow.count++
	}
	for _, item := range data.LLM {
		value.addLLM(item, bucketFor(item.OccurredAt, filter))
	}
	for _, item := range data.LLMRollups {
		value.addLLMRollup(item, bucketFor(item.Bucket, filter))
	}
	for _, item := range data.Activity {
		value.activity[item.Kind]++
	}
	for _, item := range data.ActivityRollups {
		value.activity[item.Kind] += float64(item.EventCount)
	}
	for _, item := range data.ResourceRollups {
		if item.SampleCount > 0 {
			value.resources = append(value.resources, domain.MetricsResourcePoint{At: item.Bucket, Process: item.Process, CPUPercent: item.CPUSum / float64(item.SampleCount), WorkingSet: item.WorkingSetSum / int64(item.SampleCount)})
		}
	}
	return value
}

func (s *summary) addExecution(item domain.MetricExecutionEvent, bucket time.Time, detailed bool) {
	s.runs++
	s.durationSum += item.DurationMS
	s.queueSum += item.QueueWaitMS
	if detailed && item.DurationMS > 0 {
		s.durations = append(s.durations, item.DurationMS)
	}
	s.triggers[string(item.TriggerKind)]++
	point := s.runBuckets[bucket]
	if point == nil {
		point = &domain.MetricsRunPoint{At: bucket}
		s.runBuckets[bucket] = point
	}
	switch item.Status {
	case domain.RunCompleted:
		s.completed++
		point.Completed++
	case domain.RunFailed:
		s.failed++
		point.Failed++
		s.failures[item.PipelineName] += float64(item.FailedNodeCount)
	case domain.RunSkipped:
		s.skipped++
		point.Skipped++
	case domain.RunCancelled:
		s.cancelled++
		point.Cancelled++
	}
	s.addBucket(s.durationBuckets, bucket, item.DurationMS, 0)
	s.addBucket(s.queueBuckets, bucket, item.QueueWaitMS, 0)
	key := item.PipelineID + "|" + bucket.Format(time.RFC3339Nano)
	health := s.pipelineBuckets[key]
	if health == nil {
		health = &pipelineBucket{id: item.PipelineID, name: item.PipelineName, at: bucket}
		s.pipelineBuckets[key] = health
	}
	if item.Status == domain.RunCompleted {
		health.completed++
	}
	if item.Status == domain.RunFailed {
		health.failed++
	}
}

func (s *summary) addExecutionRollup(item persistence.MetricExecutionRollup, bucket time.Time) {
	for range item.RunCount {
		s.addExecution(domain.MetricExecutionEvent{PipelineID: item.PipelineID, PipelineName: item.PipelineName, TriggerKind: item.TriggerKind, Status: item.Status, DurationMS: item.DurationSumMS / float64(max(item.RunCount, 1)), QueueWaitMS: item.QueueWaitSumMS / float64(max(item.RunCount, 1))}, bucket, false)
	}
}

func (s *summary) addLLM(item domain.LLMUsage, bucket time.Time) {
	s.llmCalls++
	s.promptTokens += item.PromptTokens
	s.completionTokens += item.CompletionTokens
	if !item.TokensReported {
		s.tokensUnavailable++
	}
	if item.ProviderKind == domain.ProviderLlamaCPP || item.ProviderKind == domain.ProviderOllama {
		s.localCalls++
	} else if item.EstimatedCostUSD == nil {
		s.unpricedCalls++
	} else {
		s.pricedCalls++
		s.estimatedCost += *item.EstimatedCostUSD
	}
	s.addBucket(s.llmBuckets, bucket, float64(item.PromptTokens), float64(item.CompletionTokens))
	label := item.ProviderName + " · " + item.Model
	model := s.models[label]
	if model == nil {
		model = &modelBucket{label: label}
		s.models[label] = model
	}
	model.calls++
	model.duration += float64(item.Duration) / float64(time.Millisecond)
}

func (s *summary) addLLMRollup(item persistence.MetricLLMRollup, bucket time.Time) {
	s.llmCalls += item.CallCount
	s.promptTokens += item.PromptTokens
	s.completionTokens += item.CompletionTokens
	s.tokensUnavailable += item.CallCount - item.TokensReportedCount
	if item.ProviderKind == domain.ProviderLlamaCPP || item.ProviderKind == domain.ProviderOllama {
		s.localCalls += item.CallCount
	} else {
		s.unpricedCalls += item.CallCount - item.PricedCount
		s.pricedCalls += item.PricedCount
		s.estimatedCost += item.EstimatedCostSumUSD
	}
	s.addBucket(s.llmBuckets, bucket, float64(item.PromptTokens), float64(item.CompletionTokens))
	label := item.ProviderName + " · " + item.Model
	model := s.models[label]
	if model == nil {
		model = &modelBucket{label: label}
		s.models[label] = model
	}
	model.calls += item.CallCount
	model.duration += item.DurationSumMS
}

func (s *summary) addBucket(buckets map[time.Time]*bucketValues, bucket time.Time, value, extra float64) {
	item := buckets[bucket]
	if item == nil {
		item = &bucketValues{}
		buckets[bucket] = item
	}
	item.sum += value
	item.extra += extra
	item.count++
}
func (s summary) successRate() float64 {
	denominator := s.completed + s.failed
	if denominator == 0 {
		return 0
	}
	return float64(s.completed) * 100 / float64(denominator)
}
func (s summary) averageDuration() float64 {
	if s.runs == 0 {
		return 0
	}
	return s.durationSum / float64(s.runs)
}
func (s summary) p95() float64 {
	if len(s.durations) == 0 {
		return 0
	}
	values := append([]float64(nil), s.durations...)
	sort.Float64s(values)
	return values[min(int(math.Ceil(float64(len(values))*0.95))-1, len(values)-1)]
}

func (s summary) overview(filter domain.MetricsFilter) domain.MetricsOverview {
	result := domain.MetricsOverview{Filter: filter, Granularity: granularity(filter), Runs: domain.MetricsKPI{Value: float64(s.runs), Available: true}, SuccessRate: domain.MetricsKPI{Value: s.successRate(), Available: s.runs > 0}, AverageDurationMS: domain.MetricsKPI{Value: s.averageDuration(), Available: s.runs > 0}, P95DurationMS: domain.MetricsKPI{Value: s.p95(), Available: len(s.durations) > 0 && !s.hasExecutionRollups}, LLMCalls: domain.MetricsKPI{Value: float64(s.llmCalls), Available: true}, PromptTokens: domain.MetricsKPI{Value: float64(s.promptTokens), Available: s.llmCalls > s.tokensUnavailable}, CompletionTokens: domain.MetricsKPI{Value: float64(s.completionTokens), Available: s.llmCalls > s.tokensUnavailable}, EstimatedCostUSD: domain.MetricsKPI{Value: s.estimatedCost, Available: s.pricedCalls > 0 || s.localCalls > 0}, TokensUnavailable: s.tokensUnavailable, UnpricedCalls: s.unpricedCalls, LocalCalls: s.localCalls, Resources: s.resources}
	result.RunSeries = sortedRunPoints(s.runBuckets)
	result.DurationSeries = sortedBucketPoints(s.durationBuckets, true)
	result.QueueSeries = sortedBucketPoints(s.queueBuckets, true)
	result.LLMSeries = sortedBucketPoints(s.llmBuckets, false)
	result.Pipelines = sortedPipelines(s.pipelineBuckets)
	result.Failures = sortedBreakdowns(s.failures)
	result.SlowNodes = sortedAverageBreakdowns(s.slowNodes)
	result.Triggers = sortedBreakdowns(s.triggers)
	result.Activity = sortedBreakdowns(s.activity)
	result.Models = sortedModels(s.models)
	return result
}

func granularity(filter domain.MetricsFilter) string {
	duration := filter.To.Sub(filter.From)
	if duration <= 7*24*time.Hour {
		return "hour"
	}
	if duration <= 90*24*time.Hour {
		return "day"
	}
	return "month"
}
func bucketFor(at time.Time, filter domain.MetricsFilter) time.Time {
	at = at.UTC()
	switch granularity(filter) {
	case "hour":
		return at.Truncate(time.Hour)
	case "month":
		return time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	}
}
func sortedRunPoints(values map[time.Time]*domain.MetricsRunPoint) []domain.MetricsRunPoint {
	result := make([]domain.MetricsRunPoint, 0, len(values))
	for _, item := range values {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].At.Before(result[j].At) })
	return result
}
func sortedBucketPoints(values map[time.Time]*bucketValues, average bool) []domain.MetricsPoint {
	result := make([]domain.MetricsPoint, 0, len(values))
	for at, item := range values {
		value := item.sum
		if average && item.count > 0 {
			value /= float64(item.count)
		}
		result = append(result, domain.MetricsPoint{At: at, Value: value, Value2: item.extra, Value3: float64(item.count)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].At.Before(result[j].At) })
	return result
}
func sortedPipelines(values map[string]*pipelineBucket) []domain.MetricsPipelineHealth {
	result := make([]domain.MetricsPipelineHealth, 0, len(values))
	for _, item := range values {
		result = append(result, domain.MetricsPipelineHealth{PipelineID: item.id, Name: item.name, At: item.at, Completed: item.completed, Failed: item.failed})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].At.Before(result[j].At)
		}
		return result[i].Name < result[j].Name
	})
	return result
}
func sortedBreakdowns(values map[string]float64) []domain.MetricsBreakdown {
	result := make([]domain.MetricsBreakdown, 0, len(values))
	for key, value := range values {
		result = append(result, domain.MetricsBreakdown{ID: key, Label: key, Value: value})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Value > result[j].Value })
	if len(result) > 8 {
		result = result[:8]
	}
	return result
}
func sortedModels(values map[string]*modelBucket) []domain.MetricsBreakdown {
	result := make([]domain.MetricsBreakdown, 0, len(values))
	for key, item := range values {
		result = append(result, domain.MetricsBreakdown{ID: key, Label: item.label, Value: float64(item.calls), Secondary: item.duration})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Value > result[j].Value })
	if len(result) > 8 {
		result = result[:8]
	}
	return result
}

func sortedAverageBreakdowns(values map[string]*bucketValues) []domain.MetricsBreakdown {
	result := make([]domain.MetricsBreakdown, 0, len(values))
	for label, item := range values {
		if item.count == 0 {
			continue
		}
		result = append(result, domain.MetricsBreakdown{ID: label, Label: label, Value: item.sum / float64(item.count), Secondary: float64(item.count)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Value > result[j].Value })
	if len(result) > 8 {
		result = result[:8]
	}
	return result
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

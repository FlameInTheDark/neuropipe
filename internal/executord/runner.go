package executord

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/llm"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	executorv1 "github.com/FlameInTheDark/neuropipe/internal/proto/executor/v1"
	"github.com/FlameInTheDark/neuropipe/internal/remoteexec"
	"github.com/FlameInTheDark/neuropipe/internal/security"
)

const (
	runDeadline      = 30 * time.Minute
	eventBuffer      = 64
	defaultQueueSize = 256
)

// bundleGate authorizes node capabilities against the allowlist captured at
// deploy time. Manual desktop-dispatched runs skip the trust check exactly
// like a local button press; autonomous schedule fires require every
// requested capability to be present.
type bundleGate struct {
	allowed    map[domain.Capability]struct{}
	unattended bool
}

func newBundleGate(capabilities []string, unattended bool) *bundleGate {
	gate := &bundleGate{allowed: make(map[domain.Capability]struct{}, len(capabilities)), unattended: unattended}
	for _, capability := range capabilities {
		gate.allowed[domain.Capability(capability)] = struct{}{}
	}
	return gate
}

func (g *bundleGate) Allow(_ context.Context, _ domain.FlowNode, capabilities []domain.Capability) error {
	if !g.unattended || len(capabilities) == 0 {
		return nil
	}
	for _, capability := range capabilities {
		if _, ok := g.allowed[capability]; !ok {
			return security.ApprovalRequiredError{Capability: capability}
		}
	}
	return nil
}

// deployedFunctions resolves custom-function nodes against the functions
// bundled with deployments.
type deployedFunctions struct{ store *store }

func (f deployedFunctions) GetPublishedFunction(_ context.Context, id string) (domain.CustomFunction, error) {
	for _, bundle := range f.store.ListBundles() {
		for _, function := range bundle.Functions {
			if function.ID == id {
				return function, nil
			}
		}
	}
	return domain.CustomFunction{}, fmt.Errorf("function %q is not deployed on this executor", id)
}

// runnerJob is one queued execution.
type runnerJob struct {
	record        RunRecord
	definition    domain.FlowDefinition
	triggerNodeID string
	input         pipeline.Packet
	unattended    bool
	capabilities  []string
}

// Runner owns executor-side execution: queue, workers, per-pipeline guards,
// engine assembly, and the lifecycle-event bus feeding StreamEvents.
type Runner struct {
	store     *store
	registry  *catalog.Registry
	globals   *executorGlobals
	tunnel    *TunnelCaller
	notifier  pipeline.NotificationSender
	runtime   *runtimeStore
	vault     SecretStore
	dataDir   string
	startedAt time.Time

	ctx     context.Context
	cancel  context.CancelFunc
	queue   chan runnerJob
	workers sync.WaitGroup

	busyMu sync.Mutex
	busy   map[string]bool

	cancelledMu sync.Mutex
	cancelled   map[string]bool

	activeMu sync.Mutex
	active   map[string]context.CancelFunc

	eventsMu    sync.Mutex
	subscribers map[chan *executorv1.ExecutionEvent]struct{}
}

// SecretStore reads provider keys from the executor vault.
type SecretStore interface {
	Get(name string) (string, error)
	Put(name, value string) error
}

// NewRunner assembles the execution core.
func NewRunner(store *store, registry *catalog.Registry, globals *executorGlobals, tunnel *TunnelCaller, notifier pipeline.NotificationSender, runtime *runtimeStore, vault SecretStore) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		store:       store,
		registry:    registry,
		globals:     globals,
		tunnel:      tunnel,
		notifier:    notifier,
		runtime:     runtime,
		vault:       vault,
		startedAt:   time.Now().UTC(),
		ctx:         ctx,
		cancel:      cancel,
		queue:       make(chan runnerJob, defaultQueueSize),
		busy:        make(map[string]bool),
		cancelled:   make(map[string]bool),
		active:      make(map[string]context.CancelFunc),
		subscribers: make(map[chan *executorv1.ExecutionEvent]struct{}),
	}
}

// Start launches the worker pool.
func (r *Runner) Start() {
	for range DefaultMaxConcurrentRuns {
		r.workers.Add(1)
		go r.worker(r.ctx)
	}
}

// Stop drains workers and closes the event bus.
func (r *Runner) Stop() {
	r.cancel()
	close(r.queue)
	r.workers.Wait()
	r.eventsMu.Lock()
	defer r.eventsMu.Unlock()
	for subscriber := range r.subscribers {
		close(subscriber)
		delete(r.subscribers, subscriber)
	}
}

// Subscribe registers an event sink; the returned channel is closed when the
// runner stops or Unsubscribe is called.
func (r *Runner) Subscribe() chan *executorv1.ExecutionEvent {
	r.eventsMu.Lock()
	defer r.eventsMu.Unlock()
	events := make(chan *executorv1.ExecutionEvent, eventBuffer)
	r.subscribers[events] = struct{}{}
	return events
}

func (r *Runner) unsubscribe(events chan *executorv1.ExecutionEvent) {
	r.eventsMu.Lock()
	defer r.eventsMu.Unlock()
	if _, ok := r.subscribers[events]; ok {
		delete(r.subscribers, events)
		close(events)
	}
}

func (r *Runner) publish(event *executorv1.ExecutionEvent) {
	r.eventsMu.Lock()
	defer r.eventsMu.Unlock()
	for subscriber := range r.subscribers {
		select {
		case subscriber <- event:
		default:
			// A slow UI stream must never stall an execution.
		}
	}
}

// Enqueue validates and queues one run, persisting its initial record.
func (r *Runner) Enqueue(record RunRecord, definition domain.FlowDefinition, triggerNodeID string, input pipeline.Packet, unattended bool, capabilities []string) error {
	return r.enqueueJob(runnerJob{
		record:        record,
		definition:    definition,
		triggerNodeID: triggerNodeID,
		input:         input,
		unattended:    unattended,
		capabilities:  capabilities,
	})
}

func (r *Runner) enqueueJob(job runnerJob) error {
	if err := r.store.SaveRun(job.record); err != nil {
		return err
	}
	select {
	case r.queue <- job:
		return nil
	default:
		record := job.record
		record.Status = domain.RunFailed
		record.Error = "the executor run queue is full"
		finished := time.Now().UTC()
		record.FinishedAt = &finished
		_ = r.store.SaveRun(record)
		r.publish(runEventFromRecord(record))
		return fmt.Errorf("the executor run queue is full")
	}
}

func (r *Runner) worker(ctx context.Context) {
	defer r.workers.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-r.queue:
			if !ok {
				return
			}
			r.runQueued(ctx, job)
		}
	}
}

func (r *Runner) runQueued(ctx context.Context, job runnerJob) {
	record := job.record
	if record.Status == domain.RunCancelled || r.isCancelled(record.ExecutionID) {
		r.complete(job, domain.RunCancelled, "Cancelled by user")
		return
	}
	runCtx, cancel := context.WithTimeout(ctx, runDeadline)
	r.registerCancel(record.ExecutionID, cancel)
	defer r.unregisterCancel(record.ExecutionID)
	defer cancel()

	if !r.acquire(record.PipelineID) {
		r.complete(job, domain.RunSkipped, "pipeline already has an active run")
		return
	}
	defer r.release(record.PipelineID)

	started := time.Now().UTC()
	record.Status = domain.RunRunning
	record.RunStartedAt = &started
	_ = r.store.SaveRun(record)
	r.publish(runEventFromRecord(record))

	result, runErr := r.executeSafely(runCtx, record, job.definition, job.triggerNodeID, job.input)
	switch {
	case r.isCancelled(record.ExecutionID):
		r.complete(job, domain.RunCancelled, "Cancelled by user")
		return
	case runCtx.Err() == context.DeadlineExceeded:
		r.complete(job, domain.RunFailed, "Pipeline exceeded the 30-minute execution deadline")
		return
	case ctx.Err() != nil && runErr != nil:
		r.complete(job, domain.RunCancelled, "Executor is shutting down")
		return
	case runErr != nil:
		r.complete(job, domain.RunFailed, runErr.Error())
		return
	default:
		record.NodeRuns = redactNodeRuns(result.NodeRuns)
		job.record.NodeRuns = record.NodeRuns
		r.complete(job, domain.RunCompleted, "")
	}
}

// executeSafely isolates node-module panics so a worker goroutine survives
// and the execution record completes with the failure instead.
func (r *Runner) executeSafely(ctx context.Context, record RunRecord, definition domain.FlowDefinition, triggerNodeID string, input pipeline.Packet) (result pipeline.RunResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("pipeline panic: %v", recovered)
		}
	}()
	return r.buildEngine(record).Execute(ctx, definition, triggerNodeID, input)
}

// buildEngine assembles the Blueprint engine with the same capability set as
// a desktop-local run, with desktop-hosted services routed over the tunnel.
func (r *Runner) buildEngine(record RunRecord) *pipeline.Engine {
	reports := proxiedReports{tunnel: r.tunnel}
	chat := proxiedChat{tunnel: r.tunnel}
	options := []pipeline.EngineOption{
		pipeline.WithReportWriter(reports, pipeline.ReportContext{PipelineID: record.PipelineID, ExecutionID: record.ExecutionID}),
		pipeline.WithFunctionResolver(deployedFunctions{store: r.store}),
		pipeline.WithNotificationSender(r.notifier),
		pipeline.WithChatWriter(chat),
		pipeline.WithJavaScriptHost(newExecutorHost(r, record)),
		pipeline.WithGlobalVariablesStore(r.globals),
		pipeline.WithSQLExecutor(proxiedSQL{tunnel: r.tunnel}),
		pipeline.WithTwitchChatSender(proxiedTwitch{tunnel: r.tunnel}),
	}
	return pipeline.NewEngine(r.registry, r.llmRunner(), nil, options...)
}

// llmRunner selects proxy or local mode from the runtime configuration.
func (r *Runner) llmRunner() pipeline.LLMRunner {
	config := r.runtime.Get()
	switch config.LLMMode {
	case domain.ExecutorLLMLocal:
		return r.buildLocalLLM(config)
	default:
		return proxiedLLM{tunnel: r.tunnel}
	}
}

// buildLocalLLM assembles a provider manager from executor-local settings.
// Provider keys resolve through the executor vault; they are configured once
// via UpdateConfig and never leave this machine.
func (r *Runner) buildLocalLLM(config RuntimeConfig) pipeline.LLMRunner {
	providers := make([]domain.ProviderConfig, 0, len(config.Providers))
	for _, provider := range config.Providers {
		if !provider.Enabled {
			continue
		}
		providers = append(providers, domain.ProviderConfig{
			ID:        provider.ID,
			Name:      provider.Name,
			Kind:      domain.ProviderKind(provider.Kind),
			BaseURL:   provider.BaseURL,
			Model:     provider.Model,
			APIKeyRef: provider.APIKeyRef,
			Enabled:   true,
		})
	}
	if len(providers) == 0 {
		return nil
	}
	return llm.NewManager(domain.Settings{
		DefaultProviderID:    config.DefaultProviderID,
		Providers:            providers,
		MaxConcurrentLLMRuns: DefaultMaxConcurrentRuns,
	}, r.vault)
}

func (r *Runner) acquire(pipelineID string) bool {
	r.busyMu.Lock()
	defer r.busyMu.Unlock()
	if r.busy[pipelineID] {
		return false
	}
	r.busy[pipelineID] = true
	return true
}

func (r *Runner) release(pipelineID string) {
	r.busyMu.Lock()
	defer r.busyMu.Unlock()
	delete(r.busy, pipelineID)
}

func (r *Runner) complete(job runnerJob, status domain.RunStatus, message string) {
	record := job.record
	record.Status = status
	record.Error = message
	record.NodeRuns = job.record.NodeRuns
	finished := time.Now().UTC()
	record.FinishedAt = &finished
	_ = r.store.SaveRun(record)
	r.publish(runEventFromRecord(record))
}

// CancelExecution cancels a queued-or-running execution. Running work is
// interrupted through its context; queued work is marked so the worker
// completes it as cancelled.
func (r *Runner) CancelExecution(executionID string) bool {
	if cancel := r.lookupCancel(executionID); cancel != nil {
		cancel()
		return true
	}
	r.cancelledMu.Lock()
	defer r.cancelledMu.Unlock()
	if _, ok := r.cancelled[executionID]; ok {
		return false
	}
	r.cancelled[executionID] = true
	return true
}

func (r *Runner) registerCancel(executionID string, cancel context.CancelFunc) {
	r.activeMu.Lock()
	defer r.activeMu.Unlock()
	r.active[executionID] = cancel
}

func (r *Runner) unregisterCancel(executionID string) {
	r.activeMu.Lock()
	defer r.activeMu.Unlock()
	delete(r.active, executionID)
}

func (r *Runner) lookupCancel(executionID string) context.CancelFunc {
	r.activeMu.Lock()
	defer r.activeMu.Unlock()
	return r.active[executionID]
}

func (r *Runner) isCancelled(executionID string) bool {
	r.cancelledMu.Lock()
	defer r.cancelledMu.Unlock()
	return r.cancelled[executionID]
}

func runEventFromRecord(record RunRecord) *executorv1.ExecutionEvent {
	return &executorv1.ExecutionEvent{Run: runSnapshotFromRecord(record)}
}

// runSnapshotFromRecord converts the stored record into the wire snapshot.
func runSnapshotFromRecord(record RunRecord) *executorv1.RunSnapshot {
	snapshot := &executorv1.RunSnapshot{
		ExecutionId:      record.ExecutionID,
		PipelineId:       record.PipelineID,
		TriggerNodeId:    record.TriggerNodeID,
		TriggerBindingId: record.TriggerBindingID,
		ChatRunId:        record.ChatRunID,
		Revision:         record.Revision,
		Status:           string(record.Status),
		Error:            record.Error,
		StartedAt:        remoteexec.TimestampOrNull(remoteexec.TimeOrNow(record.StartedAt)),
	}
	if record.RunStartedAt != nil {
		snapshot.RunStartedAt = remoteexec.TimestampOrNull(*record.RunStartedAt)
	}
	if record.FinishedAt != nil {
		snapshot.FinishedAt = remoteexec.TimestampOrNull(*record.FinishedAt)
	}
	for _, nodeRun := range record.NodeRuns {
		snapshot.NodeRuns = append(snapshot.NodeRuns, remoteexec.NodeRunToProto(nodeRun))
	}
	return snapshot
}

func redactNodeRuns(runs []domain.NodeRun) []domain.NodeRun {
	result := make([]domain.NodeRun, len(runs))
	for index, run := range runs {
		result[index] = run
		result[index].Input = security.Redact(run.Input)
		result[index].Output = security.Redact(run.Output)
	}
	return result
}

// Package execution coordinates persistence, policy, and the pure pipeline engine.
package execution

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/FlameInTheDark/neuropipe/internal/security"
)

// EventSink lets the desktop shell notify React without coupling this package to Wails.
type EventSink func(event string, payload any)

// MetricsRecorder accepts payload-free execution projections. Recording errors
// never alter the completed user-visible execution result.
type MetricsRecorder interface {
	RecordExecution(context.Context, domain.Execution) error
}

// Service serializes concurrent runs per pipeline and persists execution history.
type Service struct {
	store    *persistence.Store
	registry *catalog.Registry
	llm      pipeline.LLMRunner
	emit     EventSink
	notifier pipeline.NotificationSender
	metrics  MetricsRecorder
	twitch   nodes.TwitchChatSender
	globals  pipeline.GlobalVariablesStore

	mu            sync.Mutex
	running       map[string]struct{}
	activeCancels map[string]context.CancelFunc
	cancelled     map[string]struct{}
	limitMu       sync.RWMutex
	limiter       *limiter

	queueMu     sync.Mutex
	queueCtx    context.Context
	queueCancel context.CancelFunc
	queue       chan queuedRun
	workers     sync.WaitGroup
	started     bool
}

type queuedRun struct {
	execution     domain.Execution
	triggerNodeID string
	definition    domain.FlowDefinition
	input         pipeline.Packet
	gate          pipeline.CapabilityGate
	bindingID     string
	chatRunID     string
}

// ServiceOption extends an execution coordinator with optional local services.
type ServiceOption func(*Service)

// WithTwitchChatSender attaches the narrow outbound Twitch capability used by
// graph execution; OAuth and EventSub remain outside this coordinator.
func WithTwitchChatSender(sender nodes.TwitchChatSender) ServiceOption {
	return func(service *Service) { service.twitch = sender }
}

// WithGlobalVariablesStore attaches the workspace-scoped variable store used
// by graph execution. A nil store keeps the engine usable in headless tests.
func WithGlobalVariablesStore(store pipeline.GlobalVariablesStore) ServiceOption {
	return func(service *Service) { service.globals = store }
}

// SetTwitchChatSender completes Desktop composition before workers start.
func (s *Service) SetTwitchChatSender(sender nodes.TwitchChatSender) { s.twitch = sender }

// NewService creates an execution coordinator.
func NewService(store *persistence.Store, registry *catalog.Registry, llm pipeline.LLMRunner, emit EventSink, options ...ServiceOption) *Service {
	service := &Service{store: store, registry: registry, llm: llm, emit: emit, running: make(map[string]struct{}), activeCancels: make(map[string]context.CancelFunc), cancelled: make(map[string]struct{}), limiter: newLimiter(2), queue: make(chan queuedRun, 64)}
	for _, option := range options {
		option(service)
	}
	return service
}

// Start launches the bounded, application-owned worker pool used by HTTP and
// webhook starts. Synchronous UI and scheduler runs remain synchronous.
func (s *Service) Start(ctx context.Context) {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	if s.started {
		return
	}
	s.queueCtx, s.queueCancel = context.WithCancel(ctx)
	s.started = true
	for range 4 {
		s.workers.Add(1)
		go s.worker(s.queueCtx)
	}
}

// Stop cancels queued work and waits for owned workers to exit.
func (s *Service) Stop() {
	s.queueMu.Lock()
	if !s.started {
		s.queueMu.Unlock()
		return
	}
	cancel := s.queueCancel
	s.started = false
	s.queueCancel = nil
	s.queueMu.Unlock()
	cancel()
	s.workers.Wait()
}

// WithNotificationSender configures native desktop delivery for notification nodes.
func WithNotificationSender(sender pipeline.NotificationSender) ServiceOption {
	return func(service *Service) { service.notifier = sender }
}

// WithMetricsRecorder attaches local observability without coupling execution
// to a concrete SQLite or renderer implementation.
func WithMetricsRecorder(recorder MetricsRecorder) ServiceOption {
	return func(service *Service) { service.metrics = recorder }
}

// SetMaxConcurrentRuns changes the capacity used by future queued executions.
func (s *Service) SetMaxConcurrentRuns(limit int) {
	s.limitMu.Lock()
	defer s.limitMu.Unlock()
	s.limiter = newLimiter(limit)
}

// RunBinding executes a published trigger binding. Unattended callers require trust.
func (s *Service) RunBinding(ctx context.Context, bindingID string, input pipeline.Packet, unattended bool) (domain.Execution, error) {
	binding, err := s.store.GetTrigger(ctx, bindingID)
	if err != nil {
		return domain.Execution{}, err
	}
	if !binding.Enabled {
		return domain.Execution{}, fmt.Errorf("trigger %q is disabled", binding.Label)
	}
	if unattended && !binding.Trusted {
		return domain.Execution{}, security.ApprovalRequiredError{Capability: "pipeline-trust"}
	}
	definition, err := s.store.PublishedDefinition(ctx, binding.PipelineID, binding.Revision)
	if err != nil {
		return domain.Execution{}, err
	}
	execution, runErr := s.runDefinition(ctx, binding.PipelineID, binding.ID, binding.NodeID, definition, input, security.NewRevisionGate(s.store, binding.PipelineID, binding.Revision, unattended))
	if execution.ID == "" {
		return execution, runErr
	}
	finished := time.Now().UTC()
	if execution.FinishedAt != nil {
		finished = *execution.FinishedAt
	}
	if err := s.store.SetTriggerLastRun(ctx, binding.ID, execution.Status, finished); err != nil {
		return domain.Execution{}, err
	}
	return execution, runErr
}

// RunDraft executes the editor's saved draft from a selected trigger. It is a
// manual action, so the click itself is the approval boundary; it never creates
// or alters a trigger binding or published revision.
func (s *Service) RunDraft(ctx context.Context, pipelineID, triggerNodeID string, definition domain.FlowDefinition, input pipeline.Packet) (domain.Execution, error) {
	return s.runDefinition(ctx, pipelineID, "draft:"+triggerNodeID, triggerNodeID, definition, input, nil)
}

// QueuePublished creates an execution record immediately and lets an owned
// worker start the newest published Blueprint-v2 revision. It intentionally
// does not require unattended trust: an authenticated API call is an explicit
// run request, unlike a timer or webhook delivery.
func (s *Service) QueuePublished(ctx context.Context, pipelineID, triggerNodeID string, input pipeline.Packet) (domain.Execution, error) {
	item, err := s.store.GetPipeline(ctx, pipelineID)
	if err != nil {
		return domain.Execution{}, err
	}
	if item.Status != domain.PipelineActive || item.PublishedRevision < 1 {
		return domain.Execution{}, fmt.Errorf("pipeline %q has no published Blueprint revision", item.Name)
	}
	definition, err := s.store.PublishedDefinition(ctx, item.ID, item.PublishedRevision)
	if err != nil {
		return domain.Execution{}, err
	}
	triggerNodeID, err = s.publishedEvent(definition, triggerNodeID)
	if err != nil {
		return domain.Execution{}, err
	}
	execution, err := s.store.QueueExecution(ctx, item.ID, "api:"+triggerNodeID)
	if err != nil {
		return domain.Execution{}, err
	}
	s.queueMu.Lock()
	started, queueCtx := s.started, s.queueCtx
	s.queueMu.Unlock()
	if !started || queueCtx == nil {
		execution.Status, execution.Error = domain.RunFailed, "execution queue is not running"
		_ = s.store.CompleteExecution(ctx, execution)
		s.recordMetrics(execution)
		return domain.Execution{}, fmt.Errorf("execution queue is not running")
	}
	job := queuedRun{execution: execution, triggerNodeID: triggerNodeID, definition: definition, input: input}
	return s.enqueue(ctx, queueCtx, job)
}

// QueueBinding queues a published trigger after enforcing the unattended trust
// rule used by schedules and externally delivered webhooks.
func (s *Service) QueueBinding(ctx context.Context, bindingID string, input pipeline.Packet, unattended bool) (domain.Execution, error) {
	binding, err := s.store.GetTrigger(ctx, bindingID)
	if err != nil {
		return domain.Execution{}, err
	}
	if !binding.Enabled {
		return domain.Execution{}, fmt.Errorf("trigger %q is disabled", binding.Label)
	}
	if unattended && !binding.Trusted {
		return domain.Execution{}, security.ApprovalRequiredError{Capability: "pipeline-trust"}
	}
	definition, err := s.store.PublishedDefinition(ctx, binding.PipelineID, binding.Revision)
	if err != nil {
		return domain.Execution{}, err
	}
	execution, err := s.store.QueueExecution(ctx, binding.PipelineID, binding.ID)
	if err != nil {
		return domain.Execution{}, err
	}
	s.queueMu.Lock()
	started, queueCtx := s.started, s.queueCtx
	s.queueMu.Unlock()
	if !started || queueCtx == nil {
		execution.Status, execution.Error = domain.RunFailed, "execution queue is not running"
		_ = s.store.CompleteExecution(ctx, execution)
		s.recordMetrics(execution)
		return domain.Execution{}, fmt.Errorf("execution queue is not running")
	}
	job := queuedRun{execution: execution, triggerNodeID: binding.NodeID, definition: definition, input: input, bindingID: binding.ID, gate: security.NewRevisionGate(s.store, binding.PipelineID, binding.Revision, unattended)}
	return s.enqueue(ctx, queueCtx, job)
}

// QueueChatBinding queues a user-initiated published Chat Trigger. It carries
// a distinct chat run ID into the Blueprint engine while retaining the normal
// execution record and bounded worker lifecycle.
func (s *Service) QueueChatBinding(ctx context.Context, bindingID, chatRunID string, input pipeline.Packet) (domain.Execution, error) {
	binding, err := s.store.GetTrigger(ctx, bindingID)
	if err != nil {
		return domain.Execution{}, err
	}
	if binding.Kind != domain.TriggerChat {
		return domain.Execution{}, fmt.Errorf("trigger %q is not a chat trigger", binding.Label)
	}
	if !binding.Enabled {
		return domain.Execution{}, fmt.Errorf("chat trigger %q is disabled", binding.Label)
	}
	definition, err := s.store.PublishedDefinition(ctx, binding.PipelineID, binding.Revision)
	if err != nil {
		return domain.Execution{}, err
	}
	if _, err := s.publishedEvent(definition, binding.NodeID); err != nil {
		return domain.Execution{}, err
	}
	execution, err := s.store.QueueExecution(ctx, binding.PipelineID, "chat:"+binding.ID)
	if err != nil {
		return domain.Execution{}, err
	}
	if err := s.store.UpdateChatRun(ctx, chatRunID, domain.RunPending, "Working", execution.ID, ""); err != nil {
		return domain.Execution{}, err
	}
	s.queueMu.Lock()
	started, queueCtx := s.started, s.queueCtx
	s.queueMu.Unlock()
	if !started || queueCtx == nil {
		execution.Status, execution.Error = domain.RunFailed, "execution queue is not running"
		_ = s.store.CompleteExecution(ctx, execution)
		s.recordMetrics(execution)
		_ = s.store.UpdateChatRun(ctx, chatRunID, domain.RunFailed, "Unable to start", execution.ID, execution.Error)
		return domain.Execution{}, fmt.Errorf("execution queue is not running")
	}
	job := queuedRun{execution: execution, triggerNodeID: binding.NodeID, definition: definition, input: input, bindingID: binding.ID, chatRunID: chatRunID}
	return s.enqueue(ctx, queueCtx, job)
}

// CancelExecution stops a queued or running owned execution. The worker still
// writes the final redacted record, so cancellation is visible in history.
func (s *Service) CancelExecution(ctx context.Context, executionID string) error {
	execution, err := s.store.GetExecution(ctx, executionID)
	if err != nil {
		return err
	}
	if execution.Status == domain.RunCompleted || execution.Status == domain.RunFailed || execution.Status == domain.RunCancelled || execution.Status == domain.RunSkipped {
		return nil
	}
	s.mu.Lock()
	s.cancelled[execution.ID] = struct{}{}
	cancel := s.activeCancels[execution.ID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		return nil
	}
	execution.Status, execution.Error = domain.RunCancelled, "Cancelled by user"
	if err := s.store.CompleteExecution(ctx, execution); err != nil {
		return err
	}
	s.recordMetrics(execution)
	s.emitEvent("execution:completed", execution)
	return nil
}

func (s *Service) enqueue(ctx, queueCtx context.Context, job queuedRun) (domain.Execution, error) {
	execution := job.execution
	select {
	case s.queue <- job:
		return execution, nil
	case <-ctx.Done():
		execution.Status, execution.Error = domain.RunCancelled, ctx.Err().Error()
		_ = s.store.CompleteExecution(context.Background(), execution)
		s.recordMetrics(execution)
		return domain.Execution{}, fmt.Errorf("queue published pipeline: %w", ctx.Err())
	case <-queueCtx.Done():
		execution.Status, execution.Error = domain.RunCancelled, "Neuropipe is shutting down"
		_ = s.store.CompleteExecution(context.Background(), execution)
		s.recordMetrics(execution)
		return domain.Execution{}, fmt.Errorf("execution queue is stopping")
	}
}

func (s *Service) publishedEvent(definition domain.FlowDefinition, requested string) (string, error) {
	if requested != "" {
		for _, node := range definition.Nodes {
			if node.ID != requested {
				continue
			}
			definition, exists := s.registry.Get(node.Type)
			if !exists || definition.Mode != domain.NodeEvent {
				return "", fmt.Errorf("node %q is not a published event trigger", requested)
			}
			return requested, nil
		}
		return "", fmt.Errorf("published trigger node %q was not found", requested)
	}
	for _, node := range definition.Nodes {
		definition, exists := s.registry.Get(node.Type)
		if exists && definition.Mode == domain.NodeEvent {
			return node.ID, nil
		}
	}
	return "", fmt.Errorf("published pipeline has no event trigger")
}

func (s *Service) worker(ctx context.Context) {
	defer s.workers.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-s.queue:
			s.runQueued(ctx, job)
		}
	}
}

func (s *Service) runQueued(ctx context.Context, job queuedRun) {
	execution := job.execution
	if s.isCancelled(execution.ID) {
		s.completeCancelled(job, "Cancelled by user")
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	if !s.setActiveCancel(execution.ID, cancel) {
		cancel()
		s.completeCancelled(job, "Cancelled by user")
		return
	}
	defer s.clearActiveCancel(execution.ID)
	acquired := s.acquire(execution.PipelineID)
	if !acquired && job.chatRunID != "" {
		var err error
		acquired, err = s.waitAcquire(runCtx, execution.PipelineID)
		if err != nil {
			s.completeCancelled(job, err.Error())
			return
		}
	}
	if !acquired {
		execution.Status, execution.Error = domain.RunSkipped, "pipeline already has an active run"
		_ = s.store.CompleteExecution(context.Background(), execution)
		s.recordMetrics(execution)
		return
	}
	defer s.release(execution.PipelineID)

	s.limitMu.RLock()
	limiter := s.limiter
	s.limitMu.RUnlock()
	if err := limiter.Acquire(runCtx); err != nil {
		s.completeCancelled(job, err.Error())
		return
	}
	defer limiter.Release()
	if err := s.store.MarkExecutionRunning(runCtx, execution.ID); err != nil {
		if s.isCancelled(execution.ID) {
			s.completeCancelled(job, "Cancelled by user")
		}
		return
	}
	execution.Status = domain.RunRunning
	runStarted := time.Now().UTC()
	execution.RunStartedAt = &runStarted
	if job.chatRunID != "" {
		_ = s.store.UpdateChatRun(context.Background(), job.chatRunID, domain.RunRunning, "Working", execution.ID, "")
		s.emitEvent("chat.run.updated", map[string]string{"chatRunId": job.chatRunID})
	}
	s.emitEvent("execution:started", execution)
	reportWriter := emittingReportWriter{writer: s.store, emit: s.emit}
	chatWriter := emittingChatWriter{writer: s.store, emit: s.emit}
	engine := pipeline.NewEngine(s.registry, s.llm, job.gate,
		pipeline.WithReportWriter(reportWriter, pipeline.ReportContext{PipelineID: execution.PipelineID, ExecutionID: execution.ID}),
		pipeline.WithFunctionResolver(s.store),
		pipeline.WithNotificationSender(s.notifier),
		pipeline.WithChatWriter(chatWriter),
		pipeline.WithJavaScriptHost(newJavaScriptHost(s.store, reportWriter, chatWriter, s.notifier, execution.PipelineID, execution.ID)),
		pipeline.WithTwitchChatSender(s.twitch),
		pipeline.WithGlobalVariablesStore(s.globals),
	)
	result, runErr := engine.Execute(runCtx, job.definition, job.triggerNodeID, job.input)
	execution.NodeRuns = redactNodeRuns(result.NodeRuns)
	if runCtx.Err() != nil || s.isCancelled(execution.ID) {
		execution.Status, execution.Error = domain.RunCancelled, "Cancelled by user"
	} else if runErr != nil {
		execution.Status, execution.Error = domain.RunFailed, runErr.Error()
	} else {
		execution.Status = domain.RunCompleted
	}
	finished := time.Now().UTC()
	execution.FinishedAt = &finished
	if s.store.CompleteExecution(context.Background(), execution) == nil {
		s.recordMetrics(execution)
		if job.chatRunID != "" {
			statusText := "Completed"
			switch execution.Status {
			case domain.RunCancelled:
				statusText = "Stopped"
			case domain.RunFailed:
				statusText = "Failed"
			}
			_ = s.store.UpdateChatRun(context.Background(), job.chatRunID, execution.Status, statusText, execution.ID, execution.Error)
			s.emitEvent("chat.run.updated", map[string]string{"chatRunId": job.chatRunID})
		}
		if job.bindingID != "" {
			_ = s.store.SetTriggerLastRun(context.Background(), job.bindingID, execution.Status, finished)
		}
		s.emitEvent("execution:completed", execution)
	}
}

func (s *Service) completeCancelled(job queuedRun, reason string) {
	execution := job.execution
	execution.Status, execution.Error = domain.RunCancelled, reason
	_ = s.store.CompleteExecution(context.Background(), execution)
	s.recordMetrics(execution)
	if job.chatRunID != "" {
		_ = s.store.UpdateChatRun(context.Background(), job.chatRunID, domain.RunCancelled, "Stopped", execution.ID, reason)
		s.emitEvent("chat.run.updated", map[string]string{"chatRunId": job.chatRunID})
	}
	s.emitEvent("execution:completed", execution)
}

func (s *Service) runDefinition(ctx context.Context, pipelineID, executionTriggerID, triggerNodeID string, definition domain.FlowDefinition, input pipeline.Packet, gate pipeline.CapabilityGate) (domain.Execution, error) {
	if !s.acquire(pipelineID) {
		execution, err := s.store.StartExecution(ctx, pipelineID, executionTriggerID)
		if err != nil {
			return domain.Execution{}, err
		}
		execution.Status, execution.Error = domain.RunSkipped, "pipeline already has an active run"
		if err := s.store.CompleteExecution(ctx, execution); err != nil {
			return domain.Execution{}, err
		}
		s.recordMetrics(execution)
		return execution, nil
	}
	defer s.release(pipelineID)

	s.limitMu.RLock()
	limiter := s.limiter
	s.limitMu.RUnlock()
	if err := limiter.Acquire(ctx); err != nil {
		return domain.Execution{}, err
	}
	defer limiter.Release()

	execution, err := s.store.StartExecution(ctx, pipelineID, executionTriggerID)
	if err != nil {
		return domain.Execution{}, err
	}
	s.emitEvent("execution:started", execution)

	reportWriter := emittingReportWriter{writer: s.store, emit: s.emit}
	chatWriter := emittingChatWriter{writer: s.store, emit: s.emit}
	engine := pipeline.NewEngine(s.registry, s.llm, gate,
		pipeline.WithReportWriter(reportWriter, pipeline.ReportContext{PipelineID: pipelineID, ExecutionID: execution.ID}),
		pipeline.WithFunctionResolver(s.store),
		pipeline.WithNotificationSender(s.notifier),
		pipeline.WithChatWriter(chatWriter),
		pipeline.WithJavaScriptHost(newJavaScriptHost(s.store, reportWriter, chatWriter, s.notifier, pipelineID, execution.ID)),
		pipeline.WithTwitchChatSender(s.twitch),
		pipeline.WithGlobalVariablesStore(s.globals),
	)
	result, runErr := engine.Execute(ctx, definition, triggerNodeID, input)
	execution.NodeRuns = redactNodeRuns(result.NodeRuns)
	if runErr != nil {
		execution.Status, execution.Error = domain.RunFailed, runErr.Error()
	} else {
		execution.Status = domain.RunCompleted
	}
	finished := time.Now().UTC()
	execution.FinishedAt = &finished
	if err := s.store.CompleteExecution(ctx, execution); err != nil {
		return domain.Execution{}, err
	}
	s.recordMetrics(execution)
	s.emitEvent("execution:completed", execution)
	return execution, runErr
}

type limiter struct{ slots chan struct{} }

func newLimiter(limit int) *limiter {
	if limit < 1 {
		limit = 1
	}
	return &limiter{slots: make(chan struct{}, limit)}
}

func (l *limiter) Acquire(ctx context.Context) error {
	select {
	case l.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for pipeline execution queue: %w", ctx.Err())
	}
}

func (l *limiter) Release() { <-l.slots }

func (s *Service) acquire(pipelineID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.running[pipelineID]; exists {
		return false
	}
	s.running[pipelineID] = struct{}{}
	return true
}

func (s *Service) isCancelled(executionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.cancelled[executionID]
	return exists
}

func (s *Service) setActiveCancel(executionID string, cancel context.CancelFunc) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, stopped := s.cancelled[executionID]; stopped {
		delete(s.cancelled, executionID)
		return false
	}
	s.activeCancels[executionID] = cancel
	return true
}

func (s *Service) clearActiveCancel(executionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activeCancels, executionID)
	delete(s.cancelled, executionID)
}

// waitAcquire serializes chat turns for a shared pipeline instead of dropping
// a user message while an earlier turn is active. It is used only by the owned
// execution workers and always obeys application cancellation.
func (s *Service) waitAcquire(ctx context.Context, pipelineID string) (bool, error) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if s.acquire(pipelineID) {
			return true, nil
		}
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("wait for active chat pipeline: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (s *Service) release(pipelineID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, pipelineID)
}

func (s *Service) emitEvent(event string, payload any) {
	if s.emit != nil {
		s.emit(event, payload)
	}
}

func (s *Service) recordMetrics(execution domain.Execution) {
	if s.metrics == nil {
		return
	}
	if err := s.metrics.RecordExecution(context.Background(), execution); err != nil {
		s.emitEvent("metrics.error", err.Error())
	}
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

// emittingReportWriter keeps report creation decoupled from the renderer while
// notifying an open Reports view only after the report is safely persisted.
type emittingReportWriter struct {
	writer pipeline.ReportWriter
	emit   EventSink
}

func (w emittingReportWriter) CreateReport(ctx context.Context, report domain.Report) (domain.Report, error) {
	created, err := w.writer.CreateReport(ctx, report)
	if err == nil && w.emit != nil {
		w.emit("reports.updated", nil)
	}
	return created, err
}

// emittingChatWriter keeps Blueprint chat nodes decoupled from Wails while
// allowing their persisted replies and status changes to refresh the UI as
// soon as each node completes.
type emittingChatWriter struct {
	writer pipeline.ChatWriter
	emit   EventSink
}

func (w emittingChatWriter) AppendChatReply(ctx context.Context, chatRunID, content string) (domain.ChatMessage, error) {
	message, err := w.writer.AppendChatReply(ctx, chatRunID, content)
	if err == nil && w.emit != nil {
		w.emit("chat.run.updated", map[string]string{"chatRunId": chatRunID})
	}
	return message, err
}

func (w emittingChatWriter) UpdateChatStatus(ctx context.Context, chatRunID, status string) error {
	err := w.writer.UpdateChatStatus(ctx, chatRunID, status)
	if err == nil && w.emit != nil {
		w.emit("chat.run.updated", map[string]string{"chatRunId": chatRunID})
	}
	return err
}

func (w emittingChatWriter) ReadChatHistory(ctx context.Context, chatID string, limit int) ([]domain.ChatMessage, error) {
	return w.writer.ReadChatHistory(ctx, chatID, limit)
}

// Package chat coordinates durable local conversations and model tool calls.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/execution"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/FlameInTheDark/neuropipe/internal/security"
	"github.com/google/uuid"
)

const (
	maxAssistantToolRounds = 8
	chatPipelineWait       = 90 * time.Second
)

// systemGuidance is the tiny always-present pointer injected before model turns when authoring tools exist. Deep documentation is tool-fetched.
const systemGuidance = "You are Neuropipe automation assistant with access to the user local pipelines, reports and executions. You can also AUTHOR automations: read get_authoring_guide first, discover nodes via list_nodes/get_node_contract (always fetch a node contract before using that node type), build Blueprint v3 definitions, save them as drafts with save_pipeline_draft / save_function_draft (validation errors come back in the result - fix them), and only then offer publish_* which requires user approval. Deleting anything also requires approval."

// systemGuidance is the tiny always-present pointer injected before model turns when authoring tools exist. Deep documentation is tool-fetched.

// Assistant performs one provider-neutral, tool-capable model turn.
type Assistant interface {
	Converse(context.Context, domain.AssistantChatRequest) (domain.AssistantChatResponse, error)
}

// StreamingAssistant is optionally implemented by Assistant backends that can
// forward assistant text token by token while the model generates. When the
// configured assistant does not implement it, the service falls back to the
// blocking Converse turn and live token display is simply disabled.
type StreamingAssistant interface {
	ConverseStream(ctx context.Context, request domain.AssistantChatRequest, onDelta func(delta string)) (domain.AssistantChatResponse, error)
}

// chatTokenEvent is the payload of chat.token events forwarded to the UI while
// an assistant turn streams. ConversationID lets the renderer drop tokens
// belonging to a transcript that is not on screen.
type chatTokenEvent struct {
	ChatRunID      string `json:"chatRunId"`
	ConversationID string `json:"conversationId"`
	Delta          string `json:"delta"`
}

// tokenFlushInterval coalesces provider deltas into UI events. Local models
// can emit hundreds of tokens per second, and one webview event per token
// would flood the renderer; buffered deltas flush on this cadence instead,
// which keeps the update rate near 25 repaints per second.
const tokenFlushInterval = 40 * time.Millisecond

// EventSink keeps the service independent from Wails.
type EventSink func(event string, payload any)

// Service owns a bounded, application-lifecycle worker for model turns. The
// single worker deliberately preserves transcript order; the provider manager
// still owns the configurable LLM concurrency limiter used by all AI nodes.
type Service struct {
	store     *persistence.Store
	runs      *execution.Service
	assistant Assistant
	emit      EventSink
	authoring Authoring
	catalog   *catalog.Registry

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	queue   chan modelJob
	worker  sync.WaitGroup
	start   bool
	active  map[string]context.CancelFunc
	stopped map[string]struct{}
}

type modelJob struct {
	conversationID string
	chatRunID      string
}

// WithAuthoring enables the pipeline/function creation tools.
func WithAuthoring(authoring Authoring) Option {
	return func(s *Service) { s.authoring = authoring }
}

// WithNodeCatalog enables node discovery/contract tools.
func WithNodeCatalog(registry *catalog.Registry) Option {
	return func(s *Service) { s.catalog = registry }
}

// Option extends the chat service with optional capabilities.
type Option func(*Service)

// NewService composes the durable conversation coordinator.
func NewService(store *persistence.Store, runs *execution.Service, assistant Assistant, emit EventSink, options ...Option) *Service {
	service := &Service{store: store, runs: runs, assistant: assistant, emit: emit, queue: make(chan modelJob, 64), active: make(map[string]context.CancelFunc), stopped: make(map[string]struct{})}
	for _, option := range options {
		option(service)
	}
	return service
}

// Start begins the owned assistant worker.
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.start {
		return
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.start = true
	s.worker.Add(1)
	go s.work(s.ctx)
}

// Stop cancels queued model work and waits for its sole owned worker.
func (s *Service) Stop() {
	s.mu.Lock()
	if !s.start {
		s.mu.Unlock()
		return
	}
	cancel := s.cancel
	s.cancel = nil
	s.start = false
	s.mu.Unlock()
	cancel()
	s.worker.Wait()
}

// CreateConversation creates a new user-visible local transcript.
func (s *Service) CreateConversation(ctx context.Context, conversation domain.ChatConversation) (domain.ChatConversation, error) {
	return s.store.CreateChatConversation(ctx, conversation)
}

// Send submits one user message to the configured conversation mode.
func (s *Service) Send(ctx context.Context, conversationID, text string) (domain.ChatRun, error) {
	conversation, err := s.store.GetChatConversation(ctx, conversationID)
	if err != nil {
		return domain.ChatRun{}, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return domain.ChatRun{}, fmt.Errorf("message cannot be empty")
	}
	run, err := s.store.CreateChatRun(ctx, conversation.ID)
	if err != nil {
		return domain.ChatRun{}, err
	}
	if _, err := s.store.CreateChatMessage(ctx, domain.ChatMessage{ConversationID: conversation.ID, ChatRunID: run.ID, Role: domain.ChatRoleUser, Content: text}); err != nil {
		return domain.ChatRun{}, err
	}
	if conversation.Mode == domain.ChatModePipeline {
		return s.sendPipeline(ctx, conversation, run, text)
	}
	if s.assistant == nil {
		_ = s.store.UpdateChatRun(ctx, run.ID, domain.RunFailed, "Unavailable", "", "configure an LLM provider before chatting")
		return domain.ChatRun{}, fmt.Errorf("configure an LLM provider before chatting")
	}
	if err := s.enqueue(ctx, modelJob{conversationID: conversation.ID, chatRunID: run.ID}); err != nil {
		_ = s.store.UpdateChatRun(context.Background(), run.ID, domain.RunFailed, "Unable to queue", "", err.Error())
		return domain.ChatRun{}, err
	}
	s.emitUpdate(run.ID)
	return run, nil
}

func (s *Service) sendPipeline(ctx context.Context, conversation domain.ChatConversation, run domain.ChatRun, text string) (domain.ChatRun, error) {
	if strings.TrimSpace(conversation.TriggerBindingID) == "" {
		return domain.ChatRun{}, fmt.Errorf("pipeline conversation has no chat trigger")
	}
	if _, err := s.store.AddChatRunEvent(ctx, domain.ChatRunEvent{ChatRunID: run.ID, Kind: "pipeline", Summary: "Ran pipeline", Status: domain.RunPending}); err != nil {
		return domain.ChatRun{}, err
	}
	_, err := s.runs.QueueChatBinding(ctx, conversation.TriggerBindingID, run.ID, pipeline.Packet{"text": text, "chatId": conversation.ID, "chatRunId": run.ID})
	if err != nil {
		_ = s.store.UpdateChatRun(context.Background(), run.ID, domain.RunFailed, "Unable to start", "", err.Error())
		_, _ = s.store.AddChatRunEvent(context.Background(), domain.ChatRunEvent{ChatRunID: run.ID, Kind: "pipeline", Summary: "Pipeline failed", Detail: err.Error(), Status: domain.RunFailed})
		return domain.ChatRun{}, err
	}
	s.emitUpdate(run.ID)
	return run, nil
}

// ResolveApproval resumes a paused model turn. Denials are represented as a
// tool result, giving the model enough context to choose a safe alternative.
func (s *Service) ResolveApproval(ctx context.Context, approvalID string, approved bool) error {
	approval, err := s.store.ResolveChatApproval(ctx, approvalID, approved)
	if err != nil {
		return err
	}
	conversation, err := s.store.GetChatConversation(ctx, approval.ConversationID)
	if err != nil {
		return err
	}
	result := "The user denied this action."
	if approved {
		_ = s.rememberPipelineGrant(ctx, conversation, approval.ToolCall)
		result = s.executeTool(ctx, conversation, approval.ChatRunID, approval.ToolCall)
	}
	if _, err := s.store.CreateChatMessage(ctx, domain.ChatMessage{ConversationID: approval.ConversationID, ChatRunID: approval.ChatRunID, Role: domain.ChatRoleTool, ToolCallID: approval.ToolCall.ID, ToolName: approval.ToolCall.Name, Content: result}); err != nil {
		return err
	}
	if _, err := s.store.AddChatRunEvent(ctx, domain.ChatRunEvent{ChatRunID: approval.ChatRunID, Kind: "tool", Summary: toolSummary(approval.ToolCall), Detail: result, Status: domain.RunCompleted}); err != nil {
		return err
	}
	// Mark the run resumable BEFORE enqueueing: a fast model round could
	// otherwise complete and then be overwritten back to pending.
	if err := s.store.UpdateChatRun(ctx, approval.ChatRunID, domain.RunPending, "Working", "", ""); err != nil {
		return err
	}
	return s.enqueue(ctx, modelJob{conversationID: approval.ConversationID, chatRunID: approval.ChatRunID})
}

// Cancel stops one local chat turn and retires any approval dialog tied to it.
// Pipeline turns forward cancellation to the owned execution queue.
func (s *Service) Cancel(ctx context.Context, chatRunID string) error {
	run, err := s.store.GetChatRun(ctx, chatRunID)
	if err != nil {
		return err
	}
	if isFinished(run.Status) {
		return nil
	}
	s.mu.Lock()
	s.stopped[run.ID] = struct{}{}
	cancel := s.active[run.ID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if err := s.store.CancelChatApprovalsForRun(ctx, run.ID); err != nil {
		return err
	}
	if err := s.store.UpdateChatRun(ctx, run.ID, domain.RunCancelled, "Stopped", run.ExecutionID, "Cancelled by user"); err != nil {
		return err
	}
	if _, err := s.store.AddChatRunEvent(ctx, domain.ChatRunEvent{ChatRunID: run.ID, Kind: "cancel", Summary: "Stopped", Detail: "Cancelled by user", Status: domain.RunCancelled}); err != nil {
		return err
	}
	if run.ExecutionID != "" && s.runs != nil {
		if err := s.runs.CancelExecution(ctx, run.ExecutionID); err != nil {
			return err
		}
	}
	s.emitUpdate(run.ID)
	return nil
}

func (s *Service) enqueue(ctx context.Context, job modelJob) error {
	s.mu.Lock()
	started, workerCtx := s.start, s.ctx
	s.mu.Unlock()
	if !started || workerCtx == nil {
		return fmt.Errorf("chat service is not running")
	}
	select {
	case s.queue <- job:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("queue chat message: %w", ctx.Err())
	case <-workerCtx.Done():
		return fmt.Errorf("chat service is stopping")
	}
}

func (s *Service) work(ctx context.Context) {
	defer s.worker.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-s.queue:
			s.runModel(ctx, job)
		}
	}
}

func (s *Service) runModel(ctx context.Context, job modelJob) {
	runCtx, cancel, stopped := s.startRun(ctx, job.chatRunID)
	if stopped {
		return
	}
	defer s.finishRun(job.chatRunID, cancel)
	conversation, err := s.store.GetChatConversation(ctx, job.conversationID)
	if err != nil {
		return
	}
	if conversation.Mode != domain.ChatModeModel {
		return
	}
	if err := s.store.UpdateChatRun(runCtx, job.chatRunID, domain.RunRunning, "Working", "", ""); err != nil {
		return
	}
	messages, err := s.store.ListChatMessages(runCtx, conversation.ID, 200)
	if err != nil {
		if runCtx.Err() != nil || s.isStopped(job.chatRunID) {
			s.finishStopped(job.chatRunID)
			return
		}
		s.failRun(job.chatRunID, err)
		return
	}
	toolRounds := 0
	for _, message := range messages {
		if message.ChatRunID == job.chatRunID && message.Role == domain.ChatRoleTool {
			toolRounds++
		}
	}
	if toolRounds >= maxAssistantToolRounds {
		s.failRun(job.chatRunID, fmt.Errorf("assistant reached the %d-tool safety limit", maxAssistantToolRounds))
		return
	}
	metricContext := domain.LLMMetricContext{ChatRunID: job.chatRunID, PipelineID: conversation.PipelineID, Origin: "chat"}
	request := domain.AssistantChatRequest{Messages: messages, Tools: s.toolDefinitions(), Metrics: metricContext}
	if s.authoring != nil || s.catalog != nil {
		request.Messages = append([]domain.ChatMessage{{Role: domain.ChatRoleSystem, Content: systemGuidance}}, request.Messages...)
	}
	response, err := s.converse(runCtx, job, conversation, request)
	if err != nil && toolSupportUnavailable(err) {
		_, _ = s.store.AddChatRunEvent(runCtx, domain.ChatRunEvent{ChatRunID: job.chatRunID, Kind: "notice", Summary: "Model tools are unavailable", Detail: "This provider accepted normal chat but rejected native tool definitions.", Status: domain.RunCompleted})
		noTools := request
		noTools.Tools = nil
		response, err = s.converse(runCtx, job, conversation, noTools)
	}
	if err != nil {
		if runCtx.Err() != nil || s.isStopped(job.chatRunID) {
			s.finishStopped(job.chatRunID)
			return
		}
		s.failRun(job.chatRunID, err)
		return
	}
	if len(response.ToolCalls) == 0 {
		if runCtx.Err() != nil || s.isStopped(job.chatRunID) {
			s.finishStopped(job.chatRunID)
			return
		}
		if strings.TrimSpace(response.Content) == "" {
			response.Content = "The model completed without a response."
		}
		_, _ = s.store.CreateChatMessage(runCtx, domain.ChatMessage{ConversationID: conversation.ID, ChatRunID: job.chatRunID, Role: domain.ChatRoleAssistant, Content: response.Content})
		_ = s.store.UpdateChatRun(runCtx, job.chatRunID, domain.RunCompleted, "Completed", "", "")
		s.emitUpdate(job.chatRunID)
		return
	}
	// Some providers (Ollama among them) return tool calls without IDs, but
	// the follow-up round must echo a stable tool_call_id back to the
	// provider and the UI keys everything by it — assign one when missing.
	for index := range response.ToolCalls {
		if strings.TrimSpace(response.ToolCalls[index].ID) == "" {
			response.ToolCalls[index].ID = uuid.NewString()
		}
	}
	_, _ = s.store.CreateChatMessage(runCtx, domain.ChatMessage{ConversationID: conversation.ID, ChatRunID: job.chatRunID, Role: domain.ChatRoleAssistant, Content: response.Content, ToolCalls: response.ToolCalls})
	for _, call := range response.ToolCalls {
		requiresApproval, approvalErr := s.requiresApproval(runCtx, conversation, call)
		if approvalErr != nil {
			s.failRun(job.chatRunID, approvalErr)
			return
		}
		if requiresApproval {
			approval, err := s.store.CreateChatApproval(runCtx, domain.ChatApproval{ConversationID: conversation.ID, ChatRunID: job.chatRunID, ToolCall: call})
			if err != nil {
				if runCtx.Err() != nil || s.isStopped(job.chatRunID) {
					s.finishStopped(job.chatRunID)
					return
				}
				s.failRun(job.chatRunID, err)
				return
			}
			_ = s.store.UpdateChatRun(runCtx, job.chatRunID, domain.RunPending, "Approval required", "", "")
			_, _ = s.store.AddChatRunEvent(runCtx, domain.ChatRunEvent{ChatRunID: job.chatRunID, Kind: "approval", Summary: "Approval required: " + toolSummary(call), Detail: approval.ID, Status: domain.RunPending})
			if s.emit != nil {
				s.emit("chat.approval.requested", approval)
			}
			s.emitUpdate(job.chatRunID)
			return
		}
		_ = s.rememberPipelineGrant(runCtx, conversation, call)
		result := s.executeTool(runCtx, conversation, job.chatRunID, call)
		_, _ = s.store.CreateChatMessage(runCtx, domain.ChatMessage{ConversationID: conversation.ID, ChatRunID: job.chatRunID, Role: domain.ChatRoleTool, ToolCallID: call.ID, ToolName: call.Name, Content: result})
		_, _ = s.store.AddChatRunEvent(runCtx, domain.ChatRunEvent{ChatRunID: job.chatRunID, Kind: "tool", Summary: toolSummary(call), Detail: result, Status: domain.RunCompleted})
	}
	if runCtx.Err() != nil || s.isStopped(job.chatRunID) {
		s.finishStopped(job.chatRunID)
		return
	}
	if err := s.enqueue(ctx, job); err != nil {
		s.failRun(job.chatRunID, err)
	}
}

func (s *Service) startRun(parent context.Context, chatRunID string) (context.Context, context.CancelFunc, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, stopped := s.stopped[chatRunID]; stopped {
		delete(s.stopped, chatRunID)
		return nil, func() {}, true
	}
	ctx, cancel := context.WithCancel(parent)
	s.active[chatRunID] = cancel
	return ctx, cancel, false
}

func (s *Service) finishRun(chatRunID string, cancel context.CancelFunc) {
	cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, chatRunID)
	delete(s.stopped, chatRunID)
}

func (s *Service) isStopped(chatRunID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, stopped := s.stopped[chatRunID]
	return stopped
}

func (s *Service) finishStopped(chatRunID string) {
	_ = s.store.UpdateChatRun(context.Background(), chatRunID, domain.RunCancelled, "Stopped", "", "Cancelled by user")
	s.emitUpdate(chatRunID)
}

func isFinished(status domain.RunStatus) bool {
	return status == domain.RunCompleted || status == domain.RunFailed || status == domain.RunCancelled || status == domain.RunSkipped
}

func (s *Service) requiresApproval(ctx context.Context, conversation domain.ChatConversation, call domain.ChatToolCall) (bool, error) {
	if !stateChanging(call.Name) || conversation.ActionPolicy != domain.ChatActionAlways {
		return stateChanging(call.Name), nil
	}
	targetID, revision, scoped, err := s.pipelineGrantScope(ctx, call)
	if err != nil {
		return true, nil
	}
	if !scoped {
		return false, nil
	}
	granted, err := s.store.HasChatToolGrant(ctx, conversation.ID, call.Name, targetID, revision)
	if err != nil {
		return true, err
	}
	return !granted, nil
}

func (s *Service) rememberPipelineGrant(ctx context.Context, conversation domain.ChatConversation, call domain.ChatToolCall) error {
	if conversation.ActionPolicy != domain.ChatActionAlways {
		return nil
	}
	targetID, revision, scoped, err := s.pipelineGrantScope(ctx, call)
	if err != nil || !scoped {
		return err
	}
	return s.store.SaveChatToolGrant(ctx, conversation.ID, call.Name, targetID, revision)
}

func (s *Service) pipelineGrantScope(ctx context.Context, call domain.ChatToolCall) (string, int, bool, error) {
	switch call.Name {
	case "run_pipeline":
		pipelineID := stringArg(call.Arguments, "pipelineId")
		item, err := s.store.GetPipeline(ctx, pipelineID)
		if err != nil {
			return "", 0, false, err
		}
		if item.Status != domain.PipelineActive || item.PublishedRevision < 1 {
			return "", 0, false, fmt.Errorf("pipeline %q has no published Blueprint revision", item.Name)
		}
		return item.ID, item.PublishedRevision, true, nil
	case "send_to_chat_pipeline":
		bindingID := stringArg(call.Arguments, "bindingId")
		binding, err := s.store.GetTrigger(ctx, bindingID)
		if err != nil {
			return "", 0, false, err
		}
		if binding.Kind != domain.TriggerChat || !binding.Enabled {
			return "", 0, false, fmt.Errorf("chat trigger %q is unavailable", binding.Label)
		}
		return binding.ID, binding.Revision, true, nil
	case "publish_pipeline", "delete_pipeline":
		pipelineID := stringArg(call.Arguments, "pipelineId")
		item, err := s.store.GetPipeline(ctx, pipelineID)
		if err != nil {
			return "", 0, false, err
		}
		return item.ID, item.PublishedRevision, true, nil
	default:
		return "", 0, false, nil
	}
}

func (s *Service) failRun(chatRunID string, err error) {
	_ = s.store.UpdateChatRun(context.Background(), chatRunID, domain.RunFailed, "Failed", "", err.Error())
	_, _ = s.store.AddChatRunEvent(context.Background(), domain.ChatRunEvent{ChatRunID: chatRunID, Kind: "error", Summary: "Chat failed", Detail: err.Error(), Status: domain.RunFailed})
	s.emitUpdate(chatRunID)
}

func (s *Service) executeTool(ctx context.Context, conversation domain.ChatConversation, chatRunID string, call domain.ChatToolCall) string {
	result, err := s.toolResult(ctx, conversation, chatRunID, call)
	if err != nil {
		return "Tool failed: " + err.Error()
	}
	data, err := json.Marshal(security.Redact(result))
	if err != nil {
		return fmt.Sprintf("Tool completed: %v", result)
	}
	return string(data)
}

func (s *Service) toolResult(ctx context.Context, conversation domain.ChatConversation, chatRunID string, call domain.ChatToolCall) (any, error) {
	switch call.Name {
	case "list_pipelines":
		items, err := s.store.ListPipelines(ctx)
		if err != nil {
			return nil, err
		}
		query := strings.ToLower(strings.TrimSpace(stringArg(call.Arguments, "query")))
		filtered := make([]domain.PipelineSummary, 0, len(items))
		for _, item := range items {
			if query == "" || strings.Contains(strings.ToLower(item.Name+" "+item.Description), query) {
				filtered = append(filtered, item)
			}
		}
		return filtered, nil
	case "list_chat_pipelines":
		return s.store.ListChatPipelines(ctx)
	case "run_pipeline":
		pipelineID := stringArg(call.Arguments, "pipelineId")
		triggerID := stringArg(call.Arguments, "triggerNodeId")
		input, _ := call.Arguments["input"].(map[string]any)
		if input == nil {
			input = map[string]any{}
		}
		execution, err := s.runs.QueuePublished(ctx, pipelineID, triggerID, pipeline.Packet(input))
		if err != nil {
			return nil, err
		}
		return map[string]any{"executionId": execution.ID, "status": execution.Status}, nil
	case "send_to_chat_pipeline":
		bindingID := stringArg(call.Arguments, "bindingId")
		text := stringArg(call.Arguments, "text")
		if bindingID == "" || text == "" {
			return nil, fmt.Errorf("bindingId and text are required")
		}
		child, err := s.store.CreateChatRun(ctx, conversation.ID)
		if err != nil {
			return nil, err
		}
		if _, err := s.runs.QueueChatBinding(ctx, bindingID, child.ID, pipeline.Packet{"text": text, "chatId": conversation.ID, "chatRunId": child.ID}); err != nil {
			return nil, err
		}
		if err := s.waitForPipeline(ctx, child.ID); err != nil {
			return nil, err
		}
		events, err := s.store.ListChatRunEvents(ctx, child.ID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"chatRunId": child.ID, "events": events}, nil
	case "list_reports":
		reports, err := s.store.ListReports(ctx, 100)
		if err != nil {
			return nil, err
		}
		query, pipelineID, tag := strings.ToLower(stringArg(call.Arguments, "query")), stringArg(call.Arguments, "pipelineId"), strings.ToLower(stringArg(call.Arguments, "tag"))
		filtered := make([]domain.Report, 0, len(reports))
		for _, report := range reports {
			if query != "" && !strings.Contains(strings.ToLower(report.Title+" "+report.PipelineName), query) {
				continue
			}
			if pipelineID != "" && report.PipelineID != pipelineID {
				continue
			}
			if tag != "" {
				matched := false
				for _, item := range report.Tags {
					if strings.EqualFold(item, tag) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			filtered = append(filtered, report)
		}
		return filtered, nil
	case "read_report":
		return s.store.GetReport(ctx, stringArg(call.Arguments, "reportId"))
	case "delete_report":
		id := stringArg(call.Arguments, "reportId")
		if err := s.store.DeleteReport(ctx, id); err != nil {
			return nil, err
		}
		return map[string]any{"deleted": id}, nil
	case "get_execution":
		return s.store.GetExecution(ctx, stringArg(call.Arguments, "executionId"))
	case "list_nodes":
		if s.catalog == nil {
			return nil, fmt.Errorf("node discovery is unavailable in this conversation")
		}
		return listNodeEntries(s.catalog, stringArg(call.Arguments, "query")), nil
	case "get_node_contract":
		if s.catalog == nil {
			return nil, fmt.Errorf("node discovery is unavailable in this conversation")
		}
		return nodeContractFor(s.catalog, stringArg(call.Arguments, "nodeType"))
	case "get_authoring_guide":
		return guide(stringArg(call.Arguments, "section"))
	default:
		if call.Name == "" {
			return nil, fmt.Errorf("unknown tool")
		}
		return s.authoringToolResult(ctx, call)
	}
}

// authoringToolResult handles pipeline/function authoring tools. Validation
// problems are returned as successful results carrying a validationError so
// the model can correct its draft on the next round.
func (s *Service) authoringToolResult(ctx context.Context, call domain.ChatToolCall) (any, error) {
	if s.authoring == nil {
		return nil, fmt.Errorf("pipeline authoring is unavailable in this conversation")
	}
	switch call.Name {
	case "get_pipeline":
		item, err := s.authoring.GetPipelineFull(ctx, stringArg(call.Arguments, "pipelineId"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": item.ID, "name": item.Name, "description": item.Description, "status": item.Status, "publishedRevision": item.PublishedRevision, "draftDefinition": item.DraftDefinition}, nil
	case "save_pipeline_draft":
		definition, defErr := definitionFrom(call.Arguments["definition"])
		if defErr != nil {
			return nil, defErr
		}
		if valErr := s.authoring.ValidatePipeline(definition); valErr != nil {
			return map[string]any{"saved": false, "validationError": valErr.Error()}, nil
		}
		var saved domain.Pipeline
		var saveErr error
		if id := stringArg(call.Arguments, "pipelineId"); id != "" {
			saved, saveErr = s.authoring.SavePipelineDraft(ctx, id, stringArg(call.Arguments, "name"), stringArg(call.Arguments, "description"), definition)
		} else {
			saved, saveErr = s.authoring.CreatePipelineDraft(ctx, stringArg(call.Arguments, "name"), stringArg(call.Arguments, "description"), definition)
		}
		if saveErr != nil {
			return nil, saveErr
		}
		return map[string]any{"saved": true, "pipelineId": saved.ID, "name": saved.Name, "status": saved.Status}, nil
	case "publish_pipeline":
		published, pubErr := s.authoring.PublishPipeline(ctx, stringArg(call.Arguments, "pipelineId"))
		if pubErr != nil {
			return nil, pubErr
		}
		return map[string]any{"published": true, "pipelineId": published.ID, "revision": published.PublishedRevision}, nil
	case "delete_pipeline":
		id := stringArg(call.Arguments, "pipelineId")
		if delErr := s.authoring.DeletePipeline(ctx, id); delErr != nil {
			return nil, delErr
		}
		return map[string]any{"deleted": id}, nil
	case "list_functions":
		return s.authoring.ListFunctions(ctx)
	case "get_function":
		fn, fnErr := s.authoring.GetFunction(ctx, stringArg(call.Arguments, "functionId"))
		if fnErr != nil {
			return nil, fnErr
		}
		return fn, nil
	case "save_function_draft":
		fn, fnErr := functionFrom(call)
		if fnErr != nil {
			return nil, fnErr
		}
		if valErr := s.authoring.ValidateFunction(fn); valErr != nil {
			return map[string]any{"saved": false, "validationError": valErr.Error()}, nil
		}
		saved, saveErr := s.authoring.SaveFunctionDraft(ctx, fn)
		if saveErr != nil {
			return nil, saveErr
		}
		return map[string]any{"saved": true, "functionId": saved.ID, "name": saved.Name}, nil
	case "publish_function":
		current, curErr := s.authoring.GetFunction(ctx, stringArg(call.Arguments, "functionId"))
		if curErr != nil {
			return nil, curErr
		}
		published, pubErr := s.authoring.PublishFunction(ctx, current)
		if pubErr != nil {
			return nil, pubErr
		}
		return map[string]any{"published": true, "functionId": published.ID, "revision": published.PublishedRevision}, nil
	case "delete_function":
		id := stringArg(call.Arguments, "functionId")
		if delErr := s.authoring.DeleteFunction(ctx, id); delErr != nil {
			return nil, delErr
		}
		return map[string]any{"deleted": id}, nil
	default:
		return nil, fmt.Errorf("unknown tool %q", call.Name)
	}
}

// definitionFrom decodes and normalises a Blueprint v3 graph argument.
func definitionFrom(value any) (domain.FlowDefinition, error) {
	if value == nil {
		return domain.FlowDefinition{}, fmt.Errorf("definition is required")
	}
	var definition domain.FlowDefinition
	switch typed := value.(type) {
	case domain.FlowDefinition:
		definition = typed
	case map[string]any:
		data, err := json.Marshal(typed)
		if err != nil {
			return domain.FlowDefinition{}, fmt.Errorf("encode definition: %w", err)
		}
		if err := json.Unmarshal(data, &definition); err != nil {
			return domain.FlowDefinition{}, fmt.Errorf("decode definition: %w", err)
		}
	default:
		return domain.FlowDefinition{}, fmt.Errorf("definition must be a Blueprint v3 object")
	}
	if definition.SchemaVersion == 0 {
		definition.SchemaVersion = domain.GraphSchemaV3
	}
	if definition.SchemaVersion != domain.GraphSchemaV3 {
		return domain.FlowDefinition{}, fmt.Errorf("definition must be a Blueprint v3 object")
	}
	return definition, nil
}

// functionFrom assembles a CustomFunction draft from tool arguments.
func functionFrom(call domain.ChatToolCall) (domain.CustomFunction, error) {
	mode := domain.NodeExecutionMode(strings.TrimSpace(stringArg(call.Arguments, "mode")))
	if mode != domain.NodePure && mode != domain.NodeImpure {
		return domain.CustomFunction{}, fmt.Errorf("mode must be pure or impure")
	}
	fn := domain.CustomFunction{
		Name:        strings.TrimSpace(stringArg(call.Arguments, "name")),
		Description: strings.TrimSpace(stringArg(call.Arguments, "description")),
		Mode:        mode,
		Category:    "Functions",
		Icon:        "Braces",
	}
	if fn.Name == "" {
		return domain.CustomFunction{}, fmt.Errorf("name is required")
	}
	if id := stringArg(call.Arguments, "functionId"); id != "" {
		fn.ID = id
	}
	for _, key := range []string{"inputs", "outputs"} {
		raw, _ := call.Arguments[key].([]any)
		pins := make([]domain.FunctionPin, 0, len(raw))
		for _, item := range raw {
			entry, ok := item.(map[string]any)
			if !ok {
				return domain.CustomFunction{}, fmt.Errorf("%s pin entries must be objects", key)
			}
			pin := domain.FunctionPin{ID: strings.TrimSpace(fmt.Sprint(entry["id"])), Name: strings.TrimSpace(fmt.Sprint(entry["name"])), Description: strings.TrimSpace(fmt.Sprint(entry["description"])), Required: entry["required"] == true}
			if dataType, ok := entry["dataType"].(string); ok && dataType != "" {
				pin.DataType = domain.DataType(dataType)
			} else {
				pin.DataType = domain.DataAny
			}
			if spec, ok := entry["type"].(map[string]any); ok {
				pin.Type = typespecFromMap(spec)
			}
			if pin.ID == "" || pin.Name == "" {
				return domain.CustomFunction{}, fmt.Errorf("%s pins need id and name", key)
			}
			pins = append(pins, pin)
		}
		if key == "inputs" {
			fn.Inputs = pins
		} else {
			fn.Outputs = pins
		}
	}
	definition, defErr := definitionFrom(call.Arguments["draftDefinition"])
	if defErr != nil {
		return domain.CustomFunction{}, defErr
	}
	fn.DraftDefinition = definition
	return fn, nil
}

func typespecFromMap(spec map[string]any) *domain.TypeSpec {
	data, err := json.Marshal(spec)
	if err != nil {
		return nil
	}
	var result domain.TypeSpec
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return &result
}

func (s *Service) waitForPipeline(ctx context.Context, chatRunID string) error {
	deadline := time.NewTimer(chatPipelineWait)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		run, err := s.store.GetChatRun(ctx, chatRunID)
		if err != nil {
			return err
		}
		if run.Status == domain.RunCompleted {
			return nil
		}
		if run.Status == domain.RunFailed || run.Status == domain.RunCancelled || run.Status == domain.RunSkipped {
			if strings.TrimSpace(run.Error) == "" {
				return errors.New("chat pipeline did not complete")
			}
			return errors.New(run.Error)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("chat pipeline did not complete within %s", chatPipelineWait)
		case <-ticker.C:
		}
	}
}

func (s *Service) emitUpdate(chatRunID string) {
	if s.emit != nil {
		s.emit("chat.updated", map[string]string{"chatRunId": chatRunID})
	}
}

// converse performs one assistant turn, forwarding tokens to the UI through a
// coalescing pump when the assistant supports streaming. The pump is fully
// drained before the turn returns, so its trailing chat.token.end event always
// lands before the completion events the caller emits afterwards.
func (s *Service) converse(ctx context.Context, job modelJob, conversation domain.ChatConversation, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	streamer, ok := s.assistant.(StreamingAssistant)
	if !ok || s.emit == nil {
		return s.assistant.Converse(ctx, request)
	}
	deltas := make(chan string, 128)
	pumped := make(chan struct{})
	go s.pumpTokens(job, conversation, deltas, pumped)
	response, err := streamer.ConverseStream(ctx, request, func(delta string) { deltas <- delta })
	close(deltas)
	<-pumped
	return response, err
}

// pumpTokens coalesces streamed deltas into chat.token events and closes the
// turn with chat.token.end. It runs until the deltas channel closes.
func (s *Service) pumpTokens(job modelJob, conversation domain.ChatConversation, deltas <-chan string, done chan<- struct{}) {
	defer close(done)
	var buffer strings.Builder
	flush := func() {
		if buffer.Len() == 0 {
			return
		}
		s.emit("chat.token", chatTokenEvent{ChatRunID: job.chatRunID, ConversationID: conversation.ID, Delta: buffer.String()})
		buffer.Reset()
	}
	ticker := time.NewTicker(tokenFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case delta, ok := <-deltas:
			if !ok {
				flush()
				s.emit("chat.token.end", map[string]string{"chatRunId": job.chatRunID})
				return
			}
			buffer.WriteString(delta)
		case <-ticker.C:
			flush()
		}
	}
}

func stringArg(arguments map[string]any, key string) string {
	value, exists := arguments[key]
	if !exists || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func stateChanging(name string) bool {
	switch name {
	case "run_pipeline", "send_to_chat_pipeline", "delete_report",
		"save_pipeline_draft", "publish_pipeline", "delete_pipeline",
		"save_function_draft", "publish_function", "delete_function":
		return true
	default:
		return false
	}
}

func toolSupportUnavailable(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "tool") && (strings.Contains(message, "unsupported") || strings.Contains(message, "unknown") || strings.Contains(message, "invalid"))
}

func toolSummary(call domain.ChatToolCall) string {
	switch call.Name {
	case "run_pipeline":
		return "Ran pipeline"
	case "send_to_chat_pipeline":
		return "Ran chat pipeline"
	case "delete_report":
		return "Deleted report"
	case "read_report":
		return "Read report"
	case "list_reports":
		return "Listed reports"
	case "list_pipelines":
		return "Listed pipelines"
	case "list_chat_pipelines":
		return "Listed chat pipelines"
	case "get_execution":
		return "Inspected execution"
	case "save_pipeline_draft":
		return "Saved pipeline draft"
	case "publish_pipeline":
		return "Published pipeline"
	case "delete_pipeline":
		return "Deleted pipeline"
	case "list_nodes":
		return "Listed node types"
	case "get_node_contract":
		return "Read node contract"
	case "get_authoring_guide":
		return "Read authoring guide"
	case "get_pipeline":
		return "Read pipeline"
	case "list_functions":
		return "Listed functions"
	case "get_function":
		return "Read function"
	case "save_function_draft":
		return "Saved function draft"
	case "publish_function":
		return "Published function"
	case "delete_function":
		return "Deleted function"
	default:
		return call.Name
	}
}

func (s *Service) toolDefinitions() []domain.ChatToolDefinition {
	// object builds an object JSON Schema. `required` is omitted when no
	// field is mandatory: JSON Schema draft 2020-12 requires it to be an
	// array of strings, and a nil variadic slice marshals to JSON null,
	// which schema validators (and strict LLM providers) reject.
	object := func(properties map[string]any, required ...string) map[string]any {
		schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}
	text := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	tools := []domain.ChatToolDefinition{
		{Name: "list_pipelines", Description: "List published Neuropipe pipelines, optionally filtered by a text query.", InputSchema: object(map[string]any{"query": text("Optional pipeline name search")})},
		{Name: "list_chat_pipelines", Description: "List published pipelines that can receive a local chat message.", InputSchema: object(map[string]any{})},
		{Name: "run_pipeline", Description: "Start a published pipeline with JSON input.", InputSchema: object(map[string]any{"pipelineId": text("Pipeline ID"), "triggerNodeId": text("Optional event trigger node ID"), "input": map[string]any{"type": "object", "description": "JSON input"}}, "pipelineId")},
		{Name: "send_to_chat_pipeline", Description: "Send explicit text to a published chat pipeline binding and wait for its replies.", InputSchema: object(map[string]any{"bindingId": text("Chat trigger binding ID"), "text": text("Message to send")}, "bindingId", "text")},
		{Name: "list_reports", Description: "List and filter recent local pipeline reports.", InputSchema: object(map[string]any{"query": text("Optional title or pipeline search"), "pipelineId": text("Optional pipeline ID"), "tag": text("Optional report tag")})},
		{Name: "read_report", Description: "Read one local report by ID.", InputSchema: object(map[string]any{"reportId": text("Report ID")}, "reportId")},
		{Name: "delete_report", Description: "Permanently delete one local report by ID.", InputSchema: object(map[string]any{"reportId": text("Report ID")}, "reportId")},
		{Name: "get_execution", Description: "Inspect a pipeline execution by ID.", InputSchema: object(map[string]any{"executionId": text("Execution ID")}, "executionId")},
	}
	if s.catalog != nil {
		tools = append(tools,
			domain.ChatToolDefinition{Name: "list_nodes", Description: "List available Blueprint node types with label and category. Call get_node_contract before using any node.", InputSchema: object(map[string]any{"query": text("Optional filter over type/label/description")})},
			domain.ChatToolDefinition{Name: "get_node_contract", Description: "Full machine contract for one node type: pins (ids, kinds, data types, required), config field keys, capabilities. Required before composing that node.", InputSchema: object(map[string]any{"nodeType": text("Node type, e.g. action:http")}, "nodeType")},
		)
	}
	if s.authoring != nil {
		objectArray := func(description string) map[string]any {
			return map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "description": description}
		}
		tools = append(tools,
			domain.ChatToolDefinition{Name: "get_authoring_guide", Description: "How Neuropipe graphs are structured (v3 wire format, pins, triggers) and how custom functions work. Read before authoring.", InputSchema: object(map[string]any{"section": text(`"authoring" (default) or "functions"`)})},
			domain.ChatToolDefinition{Name: "get_pipeline", Description: "Read one pipeline's full draft definition for editing.", InputSchema: object(map[string]any{"pipelineId": text("Pipeline ID")}, "pipelineId")},
			domain.ChatToolDefinition{Name: "save_pipeline_draft", Description: "Create (empty pipelineId) or update a pipeline draft from a Blueprint v3 definition. Validates; fix reported errors and save again. Publishing is a separate call.", InputSchema: object(map[string]any{"pipelineId": text("Existing pipeline ID, empty to create"), "name": text("Pipeline name"), "definition": map[string]any{"type": "object", "description": "Blueprint v3 FlowDefinition JSON"}, "description": text("Optional description")}, "name", "definition")},
			domain.ChatToolDefinition{Name: "publish_pipeline", Description: "Publish the current draft of a pipeline so it becomes runnable.", InputSchema: object(map[string]any{"pipelineId": text("Pipeline ID")}, "pipelineId")},
			domain.ChatToolDefinition{Name: "delete_pipeline", Description: "Permanently delete a pipeline and its history.", InputSchema: object(map[string]any{"pipelineId": text("Pipeline ID")}, "pipelineId")},
			domain.ChatToolDefinition{Name: "list_functions", Description: "List custom functions available for reuse inside pipelines.", InputSchema: object(map[string]any{})},
			domain.ChatToolDefinition{Name: "get_function", Description: "Read one custom function including its draft graph.", InputSchema: object(map[string]any{"functionId": text("Function ID")}, "functionId")},
			domain.ChatToolDefinition{Name: "save_function_draft", Description: "Create (empty functionId) or update a custom function draft: boundary pins plus a Blueprint v3 graph wrapped in function boundary nodes. See get_authoring_guide section functions.", InputSchema: object(map[string]any{"functionId": text("Existing function ID, empty to create"), "name": text("Function name"), "mode": text("pure or impure"), "inputs": objectArray("Boundary input pins with id/name/dataType/required/type"), "outputs": objectArray("Boundary output pins"), "draftDefinition": map[string]any{"type": "object", "description": "Graph containing function boundary nodes"}}, "name", "mode", "draftDefinition")},
			domain.ChatToolDefinition{Name: "publish_function", Description: "Publish a function draft so pipelines can call it as function:<id>.", InputSchema: object(map[string]any{"functionId": text("Function ID")}, "functionId")},
			domain.ChatToolDefinition{Name: "delete_function", Description: "Permanently delete a custom function.", InputSchema: object(map[string]any{"functionId": text("Function ID")}, "functionId")},
		)
	}
	return tools
}

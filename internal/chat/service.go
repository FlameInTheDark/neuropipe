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

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/execution"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/FlameInTheDark/neuropipe/internal/security"
)

const (
	maxAssistantToolRounds = 8
	chatPipelineWait       = 90 * time.Second
)

// Assistant performs one provider-neutral, tool-capable model turn.
type Assistant interface {
	Converse(context.Context, domain.AssistantChatRequest) (domain.AssistantChatResponse, error)
}

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

// NewService composes the durable conversation coordinator.
func NewService(store *persistence.Store, runs *execution.Service, assistant Assistant, emit EventSink) *Service {
	return &Service{store: store, runs: runs, assistant: assistant, emit: emit, queue: make(chan modelJob, 64), active: make(map[string]context.CancelFunc), stopped: make(map[string]struct{})}
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
	if err := s.enqueue(ctx, modelJob{conversationID: approval.ConversationID, chatRunID: approval.ChatRunID}); err != nil {
		return err
	}
	return s.store.UpdateChatRun(ctx, approval.ChatRunID, domain.RunPending, "Working", "", "")
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
	response, err := s.assistant.Converse(runCtx, domain.AssistantChatRequest{Messages: messages, Tools: toolDefinitions(), Metrics: metricContext})
	if err != nil && toolSupportUnavailable(err) {
		_, _ = s.store.AddChatRunEvent(runCtx, domain.ChatRunEvent{ChatRunID: job.chatRunID, Kind: "notice", Summary: "Model tools are unavailable", Detail: "This provider accepted normal chat but rejected native tool definitions.", Status: domain.RunCompleted})
		response, err = s.assistant.Converse(runCtx, domain.AssistantChatRequest{Messages: messages, Metrics: metricContext})
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
	default:
		return nil, fmt.Errorf("unknown tool %q", call.Name)
	}
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

func stringArg(arguments map[string]any, key string) string {
	value, exists := arguments[key]
	if !exists || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func stateChanging(name string) bool {
	return name == "run_pipeline" || name == "send_to_chat_pipeline" || name == "delete_report"
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
	default:
		return call.Name
	}
}

func toolDefinitions() []domain.ChatToolDefinition {
	object := func(properties map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	text := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	return []domain.ChatToolDefinition{
		{Name: "list_pipelines", Description: "List published Neuropipe pipelines, optionally filtered by a text query.", InputSchema: object(map[string]any{"query": text("Optional pipeline name search")})},
		{Name: "list_chat_pipelines", Description: "List published pipelines that can receive a local chat message.", InputSchema: object(map[string]any{})},
		{Name: "run_pipeline", Description: "Start a published pipeline with JSON input.", InputSchema: object(map[string]any{"pipelineId": text("Pipeline ID"), "triggerNodeId": text("Optional event trigger node ID"), "input": map[string]any{"type": "object", "description": "JSON input"}}, "pipelineId")},
		{Name: "send_to_chat_pipeline", Description: "Send explicit text to a published chat pipeline binding and wait for its replies.", InputSchema: object(map[string]any{"bindingId": text("Chat trigger binding ID"), "text": text("Message to send")}, "bindingId", "text")},
		{Name: "list_reports", Description: "List and filter recent local pipeline reports.", InputSchema: object(map[string]any{"query": text("Optional title or pipeline search"), "pipelineId": text("Optional pipeline ID"), "tag": text("Optional report tag")})},
		{Name: "read_report", Description: "Read one local report by ID.", InputSchema: object(map[string]any{"reportId": text("Report ID")}, "reportId")},
		{Name: "delete_report", Description: "Permanently delete one local report by ID.", InputSchema: object(map[string]any{"reportId": text("Report ID")}, "reportId")},
		{Name: "get_execution", Description: "Inspect a pipeline execution by ID.", InputSchema: object(map[string]any{"executionId": text("Execution ID")}, "executionId")},
	}
}

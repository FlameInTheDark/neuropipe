package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

type staticAssistant struct{}

func (staticAssistant) Converse(_ context.Context, _ domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	return domain.AssistantChatResponse{Content: "Local response"}, nil
}

type approvalAssistant struct{}

func (approvalAssistant) Converse(_ context.Context, _ domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	return domain.AssistantChatResponse{ToolCalls: []domain.ChatToolCall{{ID: "tool-1", Name: "delete_report", Arguments: map[string]any{"reportId": "report-1"}}}}, nil
}

type blockingAssistant struct{ started chan struct{} }

func (assistant blockingAssistant) Converse(ctx context.Context, _ domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	close(assistant.started)
	<-ctx.Done()
	return domain.AssistantChatResponse{}, ctx.Err()
}

func TestModelConversationQueuesAndPersistsAssistantReply(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	service := NewService(store, nil, staticAssistant{}, nil)
	service.Start(context.Background())
	defer service.Stop()
	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Test"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	run, err := service.Send(context.Background(), conversation.ID, "Hello")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		loaded, getErr := store.GetChatRun(context.Background(), run.ID)
		if getErr == nil && loaded.Status == domain.RunCompleted {
			messages, listErr := store.ListChatMessages(context.Background(), conversation.ID, 10)
			if listErr != nil || len(messages) != 2 || messages[1].Content != "Local response" {
				t.Fatalf("messages = %#v, %v", messages, listErr)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("model chat run did not complete")
}

func TestStateChangingToolPausesForPersistedApproval(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	service := NewService(store, nil, approvalAssistant{}, nil)
	service.Start(context.Background())
	defer service.Stop()
	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Approval"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := service.Send(context.Background(), conversation.ID, "Delete the report"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		approvals, listErr := store.ListPendingChatApprovals(context.Background(), conversation.ID)
		if listErr == nil && len(approvals) == 1 {
			if approvals[0].ToolCall.Name != "delete_report" {
				t.Fatalf("approval = %#v", approvals[0])
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("tool call did not pause for approval")
}

func TestCancelModelConversationStopsTheActiveTurn(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	assistant := blockingAssistant{started: make(chan struct{})}
	service := NewService(store, nil, assistant, nil)
	service.Start(context.Background())
	defer service.Stop()
	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Stop"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	run, err := service.Send(context.Background(), conversation.ID, "Wait")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	select {
	case <-assistant.started:
	case <-time.After(time.Second):
		t.Fatal("assistant did not start")
	}
	if err := service.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		loaded, getErr := store.GetChatRun(context.Background(), run.ID)
		if getErr == nil && loaded.Status == domain.RunCancelled {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("cancelled chat run was not persisted")
}

func TestToolSchemasMarshalWithoutNullRequired(t *testing.T) {
	// Regression for the ai-sdk schema compiler failure:
	//   compile tool "list_pipelines" schema: ... '/required': got null, want array
	// Tool definitions without mandatory arguments must omit `required`
	// entirely instead of emitting a null array.
	service := &Service{}
	for _, tool := range service.toolDefinitions(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel}) {
		data, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("tool %q schema marshal error = %v", tool.Name, err)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("tool %q schema decode error = %v", tool.Name, err)
		}
		if raw, exists := decoded["required"]; exists && strings.TrimSpace(string(raw)) == "null" {
			t.Fatalf("tool %q schema marshals required as null: %s", tool.Name, data)
		}
		if _, exists := decoded["required"]; !exists && tool.Name == "list_pipelines" {
			t.Logf("list_pipelines schema = %s", data)
		}
	}
}

// streamingAssistant emits two deltas then completes the turn normally.
type streamingAssistant struct{}

func (streamingAssistant) Converse(_ context.Context, _ domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	return domain.AssistantChatResponse{Content: "blocking reply"}, nil
}

func (streamingAssistant) ConverseStream(_ context.Context, _ domain.AssistantChatRequest, onDelta func(string)) (domain.AssistantChatResponse, error) {
	onDelta("Hel")
	onDelta("lo")
	return domain.AssistantChatResponse{Content: "Hello"}, nil
}

// streamingCancelAssistant streams one delta then blocks until cancellation.
type streamingCancelAssistant struct{ started chan struct{} }

func (a streamingCancelAssistant) Converse(_ context.Context, _ domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	return domain.AssistantChatResponse{Content: "blocking reply"}, nil
}

func (a streamingCancelAssistant) ConverseStream(ctx context.Context, _ domain.AssistantChatRequest, onDelta func(string)) (domain.AssistantChatResponse, error) {
	close(a.started)
	onDelta("par")
	<-ctx.Done()
	return domain.AssistantChatResponse{}, ctx.Err()
}

// eventLog records emitted events from the worker and pump goroutines.
type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) sink(event string, _ any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *eventLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events...)
}

func TestStreamingAssistantForwardsTokenEvents(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	log := &eventLog{}
	service := NewService(store, nil, streamingAssistant{}, log.sink)
	service.Start(context.Background())
	defer service.Stop()
	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Stream"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	run, err := service.Send(context.Background(), conversation.ID, "Hello")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	waitForRunStatus(t, store, run.ID, domain.RunCompleted)

	events := log.snapshot()
	tokenEvents, endAt, lastUpdateAt := 0, -1, -1
	for index, event := range events {
		switch event {
		case "chat.token":
			tokenEvents++
		case "chat.token.end":
			endAt = index
		case "chat.updated":
			lastUpdateAt = index
		}
	}
	if tokenEvents == 0 {
		t.Fatalf("events = %#v, want chat.token events during the streamed turn", events)
	}
	if endAt == -1 {
		t.Fatalf("events = %#v, want a chat.token.end closing the turn", events)
	}
	if lastUpdateAt == -1 || endAt > lastUpdateAt {
		t.Fatalf("events = %#v, want chat.token.end (%d) before the completion chat.updated (%d)", events, endAt, lastUpdateAt)
	}

	// the persisted transcript must match the streamed text exactly
	messages, err := store.ListChatMessages(context.Background(), conversation.ID, 10)
	if err != nil || len(messages) != 2 || messages[1].Content != "Hello" {
		t.Fatalf("messages = %#v, %v, want the persisted assistant reply", messages, err)
	}
}

func TestCancelStreamingTurnStillClosesTokenStream(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	log := &eventLog{}
	assistant := streamingCancelAssistant{started: make(chan struct{})}
	service := NewService(store, nil, assistant, log.sink)
	service.Start(context.Background())
	defer service.Stop()
	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Stop stream"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	run, err := service.Send(context.Background(), conversation.ID, "Wait")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	select {
	case <-assistant.started:
	case <-time.After(time.Second):
		t.Fatal("streaming assistant did not start")
	}
	if err := service.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	waitForRunStatus(t, store, run.ID, domain.RunCancelled)

	// The pump drains the stream and emits its closing event from the
	// worker goroutine, which unblocks concurrently with the store write
	// observed above; poll for the closing event instead of racing it.
	var events []string
	hasTokens, hasEnd := false, false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		events = log.snapshot()
		hasTokens, hasEnd = false, false
		for _, event := range events {
			switch event {
			case "chat.token":
				hasTokens = true
			case "chat.token.end":
				hasEnd = true
			}
		}
		if hasTokens && hasEnd {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !hasTokens {
		t.Fatalf("events = %#v, want the partial chat.token emitted before cancellation", events)
	}
	if !hasEnd {
		t.Fatalf("events = %#v, want chat.token.end even after a cancelled turn", events)
	}
}

// systemMessage extracts the injected system prompt from a request.
func systemMessage(request domain.AssistantChatRequest) string {
	for _, message := range request.Messages {
		if message.Role == domain.ChatRoleSystem {
			return message.Content
		}
	}
	return ""
}

// renameSequenceAssistant models a well-behaved model: the first turn calls
// rename_conversation, the second turn answers normally. It fails loudly if
// the tool's availability or the system-prompt rule does not match the
// conversation's rename state.
type renameSequenceAssistant struct {
	mu    sync.Mutex
	round int
}

func (a *renameSequenceAssistant) Converse(_ context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.round++
	hasRenameTool := false
	for _, tool := range request.Tools {
		if tool.Name == "rename_conversation" {
			hasRenameTool = true
			break
		}
	}
	prompt := systemMessage(request)
	switch a.round {
	case 1:
		if !hasRenameTool {
			return domain.AssistantChatResponse{}, fmt.Errorf("rename_conversation tool missing on the first turn")
		}
		if !strings.Contains(prompt, "rename_conversation") {
			return domain.AssistantChatResponse{}, fmt.Errorf("system prompt misses the naming rule on the first turn")
		}
		return domain.AssistantChatResponse{ToolCalls: []domain.ChatToolCall{{ID: "rename-1", Name: "rename_conversation", Arguments: map[string]any{"title": "Weather in Munich"}}}}, nil
	default:
		if hasRenameTool {
			return domain.AssistantChatResponse{}, fmt.Errorf("rename_conversation still offered after the rename")
		}
		if strings.Contains(prompt, "rename_conversation") {
			return domain.AssistantChatResponse{}, fmt.Errorf("naming rule still present after the rename")
		}
		return domain.AssistantChatResponse{Content: "Renamed and done"}, nil
	}
}

func TestRenameConversationToolIsOneShotAndRemovesPromptRule(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	assistant := &renameSequenceAssistant{}
	service := NewService(store, nil, assistant, nil)
	service.Start(context.Background())
	defer service.Stop()
	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "New chat"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	run, err := service.Send(context.Background(), conversation.ID, "What is the weather in Munich?")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	waitForRunStatus(t, store, run.ID, domain.RunCompleted)

	loaded, err := store.GetChatConversation(context.Background(), conversation.ID)
	if err != nil {
		t.Fatalf("GetChatConversation() error = %v", err)
	}
	if loaded.Title != "Weather in Munich" {
		t.Fatalf("title = %q, want the model-provided rename", loaded.Title)
	}
	if loaded.RenamedAt == nil {
		t.Fatal("renamed_at was not stamped by the rename tool")
	}

	// A second turn must no longer offer the tool nor carry the rule; the
	// second assistant round already asserts both from inside the request.
	second, err := service.Send(context.Background(), conversation.ID, "Thanks!")
	if err != nil {
		t.Fatalf("second Send() error = %v", err)
	}
	waitForRunStatus(t, store, second.ID, domain.RunCompleted)
}

// captureAssistant records the last request it received.
type captureAssistant struct {
	mu      sync.Mutex
	request domain.AssistantChatRequest
}

func (a *captureAssistant) Converse(_ context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.request = request
	return domain.AssistantChatResponse{Content: "ok"}, nil
}

func (a *captureAssistant) lastRequest() domain.AssistantChatRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.request
}

func TestModelConversationRoutesProviderModelAndReasoning(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	assistant := &captureAssistant{}
	service := NewService(store, nil, assistant, nil)
	service.Start(context.Background())
	defer service.Stop()
	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Routed", ProviderID: "prov-1", Model: "model-a", Reasoning: string(domain.ChatReasoningLow)})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	run, err := service.Send(context.Background(), conversation.ID, "Hello")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	waitForRunStatus(t, store, run.ID, domain.RunCompleted)

	request := assistant.lastRequest()
	if request.ProviderID != "prov-1" || request.Model != "model-a" {
		t.Fatalf("request routing = %q/%q, want prov-1/model-a", request.ProviderID, request.Model)
	}
	if request.Reasoning != string(domain.ChatReasoningLow) {
		t.Fatalf("request reasoning = %q, want %q", request.Reasoning, domain.ChatReasoningLow)
	}
}

func TestInvalidReasoningLevelIsRejected(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	conversation, err := store.CreateChatConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Bad"})
	if err != nil {
		t.Fatalf("CreateChatConversation() error = %v", err)
	}
	conversation.Reasoning = "maximum"
	if _, err := store.SaveChatConversation(context.Background(), conversation); err == nil {
		t.Fatal("SaveChatConversation() accepted an unknown reasoning level")
	}
}

// questionsSequenceAssistant models a model that needs clarification: the
// first turn calls ask_user_questions, and once the user's answers arrive as
// the tool result it finishes the turn. It fails loudly when the tool, the
// system-prompt rule, or the resumed transcript is malformed.
type questionsSequenceAssistant struct {
	mu    sync.Mutex
	round int
}

func (a *questionsSequenceAssistant) Converse(_ context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.round++
	hasQuestionsTool := false
	for _, tool := range request.Tools {
		if tool.Name == "ask_user_questions" {
			hasQuestionsTool = true
			break
		}
	}
	switch a.round {
	case 1:
		if !hasQuestionsTool {
			return domain.AssistantChatResponse{}, fmt.Errorf("ask_user_questions tool missing on the first turn")
		}
		if !strings.Contains(systemMessage(request), "ask_user_questions") {
			return domain.AssistantChatResponse{}, fmt.Errorf("system prompt misses the clarification rule on the first turn")
		}
		return domain.AssistantChatResponse{ToolCalls: []domain.ChatToolCall{
			{ID: "ask-1", Name: "ask_user_questions", Arguments: map[string]any{"questions": []any{
				map[string]any{"question": "Which database engine should we use?", "options": []any{
					map[string]any{"label": "PostgreSQL", "description": "Relational, strong consistency"},
					map[string]any{"label": "MongoDB", "description": "Document store, flexible schema"},
				}},
				map[string]any{"question": "Should we migrate existing data?"},
			}}},
			{ID: "ask-2", Name: "list_pipelines", Arguments: map[string]any{}},
		}}, nil
	default:
		if !hasQuestionsTool {
			return domain.AssistantChatResponse{}, fmt.Errorf("ask_user_questions tool must stay offered after being used once")
		}
		toolResult := ""
		for _, message := range request.Messages {
			if message.Role == domain.ChatRoleTool && message.ToolName == "ask_user_questions" {
				toolResult = message.Content
			}
		}
		if !strings.Contains(toolResult, `"answer":"PostgreSQL"`) || !strings.Contains(toolResult, `"source":"rejected"`) {
			return domain.AssistantChatResponse{}, fmt.Errorf("resumed request misses the user answers: %q", toolResult)
		}
		return domain.AssistantChatResponse{Content: "Continuing with PostgreSQL"}, nil
	}
}

func waitForPendingQuestions(t *testing.T, store *persistence.Store, conversationID string, want int) []domain.ChatQuestions {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		records, err := store.ListPendingChatQuestions(context.Background(), conversationID)
		if err == nil && len(records) == want {
			return records
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("pending question forms did not reach %d in time", want)
	return nil
}

func TestAskUserQuestionsPausesTurnAndResumesWithAnswers(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	assistant := &questionsSequenceAssistant{}
	service := NewService(store, nil, assistant, nil)
	service.Start(context.Background())
	defer service.Stop()
	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Questions"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	run, err := service.Send(context.Background(), conversation.ID, "Help me pick a database")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	records := waitForPendingQuestions(t, store, conversation.ID, 1)
	record := records[0]
	if record.ChatRunID != run.ID || record.ToolCallID != "ask-1" {
		t.Fatalf("record = %#v, want run %s and tool call ask-1", record, run.ID)
	}
	if len(record.Questions) != 2 || len(record.Questions[0].Options) != 2 || record.Questions[0].Options[0].Description != "Relational, strong consistency" {
		t.Fatalf("questions = %#v", record.Questions)
	}
	paused, err := store.GetChatRun(context.Background(), run.ID)
	if err != nil || paused.Status != domain.RunPending {
		t.Fatalf("paused run = %#v, %v", paused, err)
	}

	// No tool result may exist while the form is open, but the sibling call
	// must already carry its skip result to keep the transcript valid.
	messages, err := store.ListChatMessages(context.Background(), conversation.ID, 50)
	if err != nil {
		t.Fatalf("ListChatMessages() error = %v", err)
	}
	for _, message := range messages {
		if message.Role == domain.ChatRoleTool && message.ToolCallID == "ask-1" {
			t.Fatalf("answered tool result appeared before the user replied: %q", message.Content)
		}
	}
	siblingSkipped := false
	for _, message := range messages {
		if message.Role == domain.ChatRoleTool && message.ToolCallID == "ask-2" && strings.Contains(message.Content, "Skipped") {
			siblingSkipped = true
		}
	}
	if !siblingSkipped {
		t.Fatal("sibling tool call has no skip result while the form is open")
	}

	answers := []domain.ChatQuestionAnswer{
		{Question: record.Questions[0].Question, Source: domain.ChatAnswerSourceOption, ChosenLabel: "postgresql"},
		{Question: record.Questions[1].Question, Source: domain.ChatAnswerSourceRejected},
	}
	if err := service.ResolveQuestions(context.Background(), record.ID, answers); err != nil {
		t.Fatalf("ResolveQuestions() error = %v", err)
	}
	waitForRunStatus(t, store, run.ID, domain.RunCompleted)

	resolved, err := store.GetChatQuestions(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("GetChatQuestions() error = %v", err)
	}
	if resolved.Status != domain.ChatQuestionsAnswered || len(resolved.Answers) != 2 {
		t.Fatalf("resolved record = %#v", resolved)
	}
	if resolved.Answers[0].ChosenLabel != "PostgreSQL" || resolved.Answers[0].ChosenDescription != "Relational, strong consistency" {
		t.Fatalf("option answer = %#v, want the canonical label and server-resolved description", resolved.Answers[0])
	}
	pending, err := store.ListPendingChatQuestions(context.Background(), conversation.ID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending questions after resolve = %#v, %v", pending, err)
	}
	messages, err = store.ListChatMessages(context.Background(), conversation.ID, 50)
	if err != nil {
		t.Fatalf("ListChatMessages() error = %v", err)
	}
	var finalAssistant *domain.ChatMessage
	for index := range messages {
		if messages[index].Role == domain.ChatRoleAssistant && messages[index].Content == "Continuing with PostgreSQL" {
			finalAssistant = &messages[index]
		}
	}
	if finalAssistant == nil {
		t.Fatalf("transcript missing the resumed assistant reply: %s", dumpString(messages))
	}
}

// askOnceAssistant calls ask_user_questions on the first turn only, then
// answers normally. It records the latest request for transcript assertions.
type askOnceAssistant struct {
	mu      sync.Mutex
	round   int
	request domain.AssistantChatRequest
}

func (a *askOnceAssistant) Converse(_ context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.round++
	a.request = request
	if a.round == 1 {
		return domain.AssistantChatResponse{ToolCalls: []domain.ChatToolCall{{ID: "ask-1", Name: "ask_user_questions", Arguments: map[string]any{"questions": []any{
			map[string]any{"question": "What scope?", "options": []any{
				map[string]any{"label": "Local"},
				map[string]any{"label": "Global"},
			}},
		}}}}}, nil
	}
	return domain.AssistantChatResponse{Content: "ok"}, nil
}

func (a *askOnceAssistant) lastRequest() domain.AssistantChatRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.request
}

func TestSendExpiresPendingQuestions(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	assistant := &askOnceAssistant{}
	service := NewService(store, nil, assistant, nil)
	service.Start(context.Background())
	defer service.Stop()
	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Expired"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	firstRun, err := service.Send(context.Background(), conversation.ID, "Ask me anything")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	records := waitForPendingQuestions(t, store, conversation.ID, 1)

	secondRun, err := service.Send(context.Background(), conversation.ID, "Never mind, new topic")
	if err != nil {
		t.Fatalf("second Send() error = %v", err)
	}
	waitForRunStatus(t, store, secondRun.ID, domain.RunCompleted)

	expired, err := store.GetChatQuestions(context.Background(), records[0].ID)
	if err != nil || expired.Status != domain.ChatQuestionsExpired {
		t.Fatalf("expired record = %#v, %v", expired, err)
	}
	abandonedRun, err := store.GetChatRun(context.Background(), firstRun.ID)
	if err != nil || abandonedRun.Status != domain.RunCancelled {
		t.Fatalf("abandoned run = %#v, %v", abandonedRun, err)
	}
	messages, err := store.ListChatMessages(context.Background(), conversation.ID, 50)
	if err != nil {
		t.Fatalf("ListChatMessages() error = %v", err)
	}
	keptValid := false
	for _, message := range messages {
		if message.Role == domain.ChatRoleTool && message.ToolCallID == records[0].ToolCallID && strings.Contains(message.Content, "without answering") {
			keptValid = true
		}
	}
	if !keptValid {
		t.Fatal("expired question form has no skipped tool result in the transcript")
	}
	// The next model request must see the closed form before the new message.
	request := assistant.lastRequest()
	sawToolResult, sawNewMessage := false, false
	for _, message := range request.Messages {
		if message.Role == domain.ChatRoleTool && message.ToolName == "ask_user_questions" {
			sawToolResult = true
		}
		if message.Role == domain.ChatRoleUser && message.Content == "Never mind, new topic" {
			sawNewMessage = true
		}
	}
	if !sawToolResult || !sawNewMessage {
		t.Fatalf("resumed request = %#v", request.Messages)
	}
}

func TestCancelRetiresPendingQuestions(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	assistant := &askOnceAssistant{}
	service := NewService(store, nil, assistant, nil)
	service.Start(context.Background())
	defer service.Stop()
	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Stopped"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	run, err := service.Send(context.Background(), conversation.ID, "Ask me anything")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	records := waitForPendingQuestions(t, store, conversation.ID, 1)
	if err := service.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	cancelled, err := store.GetChatQuestions(context.Background(), records[0].ID)
	if err != nil || cancelled.Status != domain.ChatQuestionsCancelled {
		t.Fatalf("cancelled record = %#v, %v", cancelled, err)
	}
}

func TestParseQuestionsNormalizesModelArguments(t *testing.T) {
	if _, err := parseQuestions(map[string]any{}); err == nil {
		t.Fatal("parseQuestions() accepted missing arguments")
	}
	if _, err := parseQuestions(map[string]any{"questions": []any{}}); err == nil {
		t.Fatal("parseQuestions() accepted an empty array")
	}
	questions, err := parseQuestions(map[string]any{"questions": []any{
		map[string]any{"question": "   "},
		map[string]any{"question": "Pick one", "options": []any{
			map[string]any{"label": "  Alpha  ", "description": " first "},
			map[string]any{"description": "no label"},
			"junk",
		}},
	}})
	if err != nil {
		t.Fatalf("parseQuestions() error = %v", err)
	}
	if len(questions) != 1 || questions[0].Question != "Pick one" {
		t.Fatalf("questions = %#v, want one normalized question", questions)
	}
	if len(questions[0].Options) != 1 || questions[0].Options[0].Label != "Alpha" || questions[0].Options[0].Description != "first" {
		t.Fatalf("options = %#v, want one trimmed option", questions[0].Options)
	}
}

func TestValidateAnswersChecksEveryStep(t *testing.T) {
	questions := []domain.ChatQuestion{{Question: "Q", Options: []domain.ChatQuestionOption{{Label: "A", Description: "da"}, {Label: "B"}}}}
	if _, err := validateAnswers(questions, nil); err == nil {
		t.Fatal("validateAnswers() accepted a mismatched answer count")
	}
	if _, err := validateAnswers(questions, []domain.ChatQuestionAnswer{{Source: domain.ChatAnswerSourceOption, ChosenLabel: "C"}}); err == nil {
		t.Fatal("validateAnswers() accepted an unknown option")
	}
	if _, err := validateAnswers(questions, []domain.ChatQuestionAnswer{{Source: domain.ChatAnswerSourceCustom, Custom: "   "}}); err == nil {
		t.Fatal("validateAnswers() accepted an empty custom answer")
	}
	if _, err := validateAnswers(questions, []domain.ChatQuestionAnswer{{Source: "psychic"}}); err == nil {
		t.Fatal("validateAnswers() accepted an unknown source")
	}
	validated, err := validateAnswers(questions, []domain.ChatQuestionAnswer{{Source: domain.ChatAnswerSourceOption, ChosenLabel: " a "}})
	if err != nil {
		t.Fatalf("validateAnswers() error = %v", err)
	}
	if validated[0].ChosenLabel != "A" || validated[0].ChosenDescription != "da" {
		t.Fatalf("validated = %#v, want the canonical option with its description", validated[0])
	}
}

// dumpString renders messages for failure output.
func dumpString(messages []domain.ChatMessage) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		parts = append(parts, fmt.Sprintf("%s/%s: %.60q", message.Role, message.ToolName, message.Content))
	}
	return strings.Join(parts, " | ")
}

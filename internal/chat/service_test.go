package chat

import (
	"context"
	"encoding/json"
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
	for _, tool := range service.toolDefinitions() {
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

	events := log.snapshot()
	hasTokens, hasEnd := false, false
	for _, event := range events {
		switch event {
		case "chat.token":
			hasTokens = true
		case "chat.token.end":
			hasEnd = true
		}
	}
	if !hasTokens {
		t.Fatalf("events = %#v, want the partial chat.token emitted before cancellation", events)
	}
	if !hasEnd {
		t.Fatalf("events = %#v, want chat.token.end even after a cancelled turn", events)
	}
}

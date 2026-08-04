package chat

import (
	"context"
	"path/filepath"
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

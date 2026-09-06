package persistence

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestListChatMessagesPagesBackwards(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()

	conversation, err := store.CreateChatConversation(ctx, domain.ChatConversation{ID: "conv-1", Mode: domain.ChatModeModel, Title: "Paged"})
	if err != nil {
		t.Fatalf("CreateChatConversation() error = %v", err)
	}

	const total = 7
	for i := 0; i < total; i += 1 {
		run, err := store.CreateChatRun(ctx, conversation.ID)
		if err != nil {
			t.Fatalf("CreateChatRun() error = %v", err)
		}
		if _, err := store.CreateChatMessage(ctx, domain.ChatMessage{ConversationID: conversation.ID, ChatRunID: run.ID, Role: domain.ChatRoleUser, Content: string(rune('a' + i))}); err != nil {
			t.Fatalf("CreateChatMessage() error = %v", err)
		}
	}

	// offsets count backwards from the newest row (g..a newest-first).
	first, err := store.ListChatMessagesPaged(ctx, conversation.ID, 0, 3)
	if err != nil {
		t.Fatalf("page 1 error = %v", err)
	}
	if first.Total != total || !first.HasMore || len(first.Messages) != 3 {
		t.Fatalf("page 1 = total %d hasMore %v len %d", first.Total, first.HasMore, len(first.Messages))
	}
	if first.Messages[0].Content != "e" || first.Messages[2].Content != "g" {
		t.Fatalf("page 1 contents = %q..%q, want the three newest e..g", first.Messages[0].Content, first.Messages[2].Content)
	}

	second, err := store.ListChatMessagesPaged(ctx, conversation.ID, 3, 3)
	if err != nil {
		t.Fatalf("page 2 error = %v", err)
	}
	if !second.HasMore || second.Total != total || second.Messages[0].Content != "b" {
		t.Fatalf("page 2 = hasMore %v total %d first %q", second.HasMore, second.Total, second.Messages[0].Content)
	}

	last, err := store.ListChatMessagesPaged(ctx, conversation.ID, 6, 3)
	if err != nil {
		t.Fatalf("page 3 error = %v", err)
	}
	if last.HasMore || last.Total != total {
		t.Fatalf("final page = hasMore %v total %d, want exhausted", last.HasMore, last.Total)
	}
	if last.Messages[0].Content != "a" || len(last.Messages) != 1 {
		t.Fatalf("final page = %#v, want only the oldest row", last.Messages)
	}

	beyond, err := store.ListChatMessagesPaged(ctx, conversation.ID, 50, 3)
	if err != nil {
		t.Fatalf("beyond-end error = %v", err)
	}
	if beyond.HasMore || len(beyond.Messages) != 0 || beyond.Total != total {
		t.Fatalf("beyond-end page = %+v, want empty with total intact", beyond)
	}
}

func TestChatMessageReasoningRoundTrip(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()

	conversation, err := store.CreateChatConversation(ctx, domain.ChatConversation{ID: "conv-r", Mode: domain.ChatModeModel, Title: "Reasoning"})
	if err != nil {
		t.Fatalf("CreateChatConversation() error = %v", err)
	}
	run, err := store.CreateChatRun(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("CreateChatRun() error = %v", err)
	}
	if _, err := store.CreateChatMessage(ctx, domain.ChatMessage{ConversationID: conversation.ID, ChatRunID: run.ID, Role: domain.ChatRoleUser, Content: "question"}); err != nil {
		t.Fatalf("CreateChatMessage() error = %v", err)
	}
	if _, err := store.CreateChatMessage(ctx, domain.ChatMessage{ConversationID: conversation.ID, ChatRunID: run.ID, Role: domain.ChatRoleAssistant, Content: "answer", Reasoning: "worked it out"}); err != nil {
		t.Fatalf("CreateChatMessage() error = %v", err)
	}

	messages, err := store.ListChatMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListChatMessages() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}
	// The user row has no reasoning; the assistant row round-trips it.
	if messages[0].Reasoning != "" {
		t.Fatalf("user reasoning = %q, want empty", messages[0].Reasoning)
	}
	if messages[1].Reasoning != "worked it out" {
		t.Fatalf("assistant reasoning = %q, want the persisted trace", messages[1].Reasoning)
	}

	page, err := store.ListChatMessagesPaged(ctx, conversation.ID, 0, 10)
	if err != nil {
		t.Fatalf("ListChatMessagesPaged() error = %v", err)
	}
	if len(page.Messages) != 2 || page.Messages[1].Reasoning != "worked it out" {
		t.Fatalf("paged messages = %#v, want the reasoning on the assistant row", page.Messages)
	}
}

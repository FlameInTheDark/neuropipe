package chathistory

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

// historyStub is the narrow ChatHistoryReader capability the node needs.
type historyStub struct {
	messages   []domain.ChatMessage
	err        error
	calledWith struct {
		chatID string
		limit  int
	}
}

func (stub *historyStub) ReadChatHistory(_ context.Context, chatID string, limit int) ([]domain.ChatMessage, error) {
	stub.calledWith.chatID = chatID
	stub.calledWith.limit = limit
	return stub.messages, stub.err
}

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	module, ok := registry.Get("data:chat_history")
	if !ok {
		t.Fatal("data:chat_history was not registered")
	}
	return module
}

func invocation(module nodes.Node, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "data:chat_history", Data: map[string]any{"config": map[string]any{}}},
		Definition:      module.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:chat_history" || definition.Mode != domain.NodePure || definition.Category != "Chat" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "chatId" || got.DataType != domain.DataText || got.Direction != domain.PinInput {
		t.Fatalf("chatId input = %#v", got)
	}
	if got := definition.Inputs[1]; got.ID != "limit" || got.DataType != domain.DataNumber || got.Direction != domain.PinInput {
		t.Fatalf("limit input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "messages" || got.DataType != domain.DataList || got.Direction != domain.PinOutput {
		t.Fatalf("messages output = %#v", got)
	}
	if !reflect.DeepEqual(definition.DefaultConfig, map[string]any{"limit": 50}) {
		t.Fatalf("default config = %#v, want limit 50", definition.DefaultConfig)
	}
}

func TestEvaluateMapsMessagesToGraphValues(t *testing.T) {
	module := registeredModule(t)
	first := time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC)
	second := time.Date(2025, 2, 3, 4, 5, 7, 0, time.UTC)
	stub := &historyStub{messages: []domain.ChatMessage{
		{ID: "m1", Role: domain.ChatRoleUser, Content: "Hello", CreatedAt: first},
		{ID: "m2", Role: domain.ChatRoleAssistant, Content: "Hi there", CreatedAt: second},
	}}
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"chatId": "chat-1", "limit": 10.0}), stub)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{"messages": []any{
		map[string]any{"id": "m1", "role": "user", "content": "Hello", "createdAt": first.Format(time.RFC3339)},
		map[string]any{"id": "m2", "role": "assistant", "content": "Hi there", "createdAt": second.Format(time.RFC3339)},
	}}
	if !reflect.DeepEqual(result.Outputs, want) {
		t.Fatalf("outputs = %#v, want %#v", result.Outputs, want)
	}
	if stub.calledWith.chatID != "chat-1" || stub.calledWith.limit != 10 {
		t.Fatalf("reader called with (%q, %d), want (chat-1, 10)", stub.calledWith.chatID, stub.calledWith.limit)
	}
}

func TestEvaluateAcceptsIntegerLimitKinds(t *testing.T) {
	module := registeredModule(t)
	for name, limit := range map[string]any{"int": 5, "int64": int64(5), "float64": 5.0} {
		t.Run(name, func(t *testing.T) {
			stub := &historyStub{}
			if _, err := module.Execute(context.Background(), invocation(module, map[string]any{"chatId": "chat-1", "limit": limit}), stub); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if stub.calledWith.limit != 5 {
				t.Fatalf("reader limit = %d, want 5", stub.calledWith.limit)
			}
		})
	}
}

func TestEvaluateReturnsEmptyListWithoutMessages(t *testing.T) {
	module := registeredModule(t)
	stub := &historyStub{}
	result, err := module.Execute(context.Background(), invocation(module, map[string]any{"chatId": "chat-1", "limit": 10.0}), stub)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	messages, ok := result.Outputs["messages"].([]any)
	if !ok || len(messages) != 0 {
		t.Fatalf("messages = %#v, want an empty list", result.Outputs["messages"])
	}
}

func TestEvaluateTrimsChatID(t *testing.T) {
	module := registeredModule(t)
	stub := &historyStub{}
	if _, err := module.Execute(context.Background(), invocation(module, map[string]any{"chatId": "  chat-1  ", "limit": 10.0}), stub); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stub.calledWith.chatID != "chat-1" {
		t.Fatalf("reader chat ID = %q, want chat-1 after trimming", stub.calledWith.chatID)
	}
}

func TestEvaluatePropagatesReaderErrors(t *testing.T) {
	module := registeredModule(t)
	stub := &historyStub{err: errors.New("boom")}
	_, err := module.Execute(context.Background(), invocation(module, map[string]any{"chatId": "chat-1", "limit": 10.0}), stub)
	if err == nil || !strings.Contains(err.Error(), "read chat history: boom") {
		t.Fatalf("Execute() error = %v, want the wrapped reader failure", err)
	}
}

func TestEvaluateRejectsInvalidInvocations(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name    string
		runtime nodes.Runtime
		inputs  map[string]any
		message string
	}{
		{"nil runtime", nil, map[string]any{"chatId": "chat-1", "limit": 10.0}, "chat history is unavailable for this execution"},
		{"runtime without capability", struct{}{}, map[string]any{"chatId": "chat-1", "limit": 10.0}, "chat history is unavailable for this execution"},
		{"missing chat ID", &historyStub{}, map[string]any{"limit": 10.0}, "read chat history requires a chat ID"},
		{"blank chat ID", &historyStub{}, map[string]any{"chatId": "   ", "limit": 10.0}, "read chat history requires a chat ID"},
		{"missing limit", &historyStub{}, map[string]any{"chatId": "chat-1"}, "read chat history requires a positive integer Limit"},
		{"zero limit", &historyStub{}, map[string]any{"chatId": "chat-1", "limit": 0.0}, "read chat history requires a positive integer Limit"},
		{"negative limit", &historyStub{}, map[string]any{"chatId": "chat-1", "limit": -3.0}, "read chat history requires a positive integer Limit"},
		{"fractional limit", &historyStub{}, map[string]any{"chatId": "chat-1", "limit": 2.5}, "read chat history requires a positive integer Limit"},
		{"text limit", &historyStub{}, map[string]any{"chatId": "chat-1", "limit": "10"}, "read chat history requires a positive integer Limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), invocation(module, test.inputs), test.runtime)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Execute() error = %v, want it to contain %q", err, test.message)
			}
		})
	}
}

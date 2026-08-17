package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

func chatHistoryFixture() []domain.ChatMessage {
	return []domain.ChatMessage{
		{ID: "m1", Role: domain.ChatRoleUser, Content: "What is Neuropipe?"},
		{ID: "m2", Role: domain.ChatRoleAssistant, Content: "A local automation workspace."},
		{ID: "m3", Role: domain.ChatRoleTool, Content: `{"stale":"tool chatter"}`, ToolCallID: "old-call"},
		{ID: "m4", Role: domain.ChatRoleUser, Content: "Summarise that in one word."},
	}
}

// A tool-connected agent in chat-history mode continues the conversation:
// system prompt, then the sanitized history turns, and no duplicated user
// message because the current user turn is already the last history entry.
func TestAgentWithToolsContinuesChatHistory(t *testing.T) {
	ctx := context.Background()
	store, err := persistence.New(t.TempDir() + "/data")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	weather, err := createPublishedToolFunction(ctx, store, "Weather", "forecast for Yekaterinburg")
	if err != nil {
		t.Fatal(err)
	}
	registry := catalog.New()
	definitions, err := store.PublishedFunctionDefinitions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	registry.ReplaceDynamic(definitions)

	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("chat", "trigger:chat", map[string]any{"label": "Chat"}),
		v2Node("agent", "llm:agent", map[string]any{"instructions": "Answer briefly.", "maxTurns": 2.0, "chatMode": "history"}),
		v2Node("weather", "function:"+weather.ID, nil),
	}, Edges: []domain.FlowEdge{
		execEdge("chat-agent", "chat", "out", "agent", "in"),
		dataEdge("chat-chatid", "chat", "chatId", "agent", "chatId"),
		{ID: "weather-tool", Source: "weather", SourceHandle: "tool", Target: "agent", TargetHandle: "tools", Kind: domain.PinTool},
	}}
	runner := &instructionsCaptureRunner{}
	chat := &recordingChatWriter{history: chatHistoryFixture()}
	if _, err := NewEngine(registry, runner, nil, WithFunctionResolver(store), WithChatWriter(chat)).Execute(ctx, flow, "chat", Packet{"text": "ignored: history carries the turn", "chatId": "conv-1", "chatRunId": "run-1"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(runner.requests) == 0 {
		t.Fatal("assistant was never called")
	}
	messages := runner.requests[0].Messages
	wantRoles := []domain.ChatMessageRole{domain.ChatRoleSystem, domain.ChatRoleUser, domain.ChatRoleAssistant, domain.ChatRoleUser}
	if len(messages) != len(wantRoles) {
		t.Fatalf("message count = %d (%#v), want %d", len(messages), messages, len(wantRoles))
	}
	for index, role := range wantRoles {
		if messages[index].Role != role {
			t.Fatalf("message %d role = %s, want %s", index, messages[index].Role, role)
		}
	}
	if messages[0].Content != "Answer briefly." {
		t.Fatalf("system = %q, want instructions", messages[0].Content)
	}
	if messages[len(messages)-1].Content != "Summarise that in one word." {
		t.Fatalf("final user turn = %q, want the latest history entry", messages[len(messages)-1].Content)
	}
	for _, message := range messages {
		if strings.Contains(message.Content, "tool chatter") || message.ToolCallID != "" {
			t.Fatalf("history tool chatter leaked into the context: %#v", message)
		}
	}
}

// An agent without connected tools becomes a plain chat agent: history feeds
// a multi-turn Converse request and the answer becomes llm.content.
func TestAgentWithoutToolsConversesOverChatHistory(t *testing.T) {
	ctx := context.Background()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("chat", "trigger:chat", map[string]any{"label": "Chat"}),
		v2Node("agent", "llm:agent", map[string]any{"instructions": "You are a local assistant.", "chatMode": "history"}),
	}, Edges: []domain.FlowEdge{
		execEdge("chat-agent", "chat", "out", "agent", "in"),
		dataEdge("chat-chatid", "chat", "chatId", "agent", "chatId"),
	}}
	runner := &instructionsCaptureRunner{}
	chat := &recordingChatWriter{history: chatHistoryFixture()}
	if _, err := NewEngine(catalog.New(), runner, nil, WithChatWriter(chat)).Execute(ctx, flow, "chat", Packet{"text": "ignored", "chatId": "conv-1", "chatRunId": "run-1"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("assistant turns = %d, want exactly 1", len(runner.requests))
	}
	messages := runner.requests[0].Messages
	if len(messages) != 4 || messages[0].Role != domain.ChatRoleSystem || messages[3].Content != "Summarise that in one word." {
		t.Fatalf("messages = %#v, want system + three dialogue turns ending on the current user turn", messages)
	}
}

// Chat-history mode answers the conversation as loaded: even when history
// ends on an assistant reply, no synthetic task user turn is appended.
func TestAgentChatHistoryDoesNotAppendTaskPrompt(t *testing.T) {
	ctx := context.Background()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("agent", "llm:agent", map[string]any{"instructions": "Follow up on the conversation.", "chatMode": "history", "chatId": "conv-1"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-agent", "start", "out", "agent", "in"),
	}}
	runner := &instructionsCaptureRunner{}
	chat := &recordingChatWriter{history: chatHistoryFixture()[:2]} // user question, assistant answer
	if _, err := NewEngine(catalog.New(), runner, nil, WithChatWriter(chat)).Execute(ctx, flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	messages := runner.requests[0].Messages
	if len(messages) != 3 || messages[len(messages)-1].Role != domain.ChatRoleAssistant {
		t.Fatalf("messages = %#v, want the loaded history without an appended task turn", messages)
	}
}

// Graphs saved with the earlier Use-chat-history toggle keep working.
func TestAgentLegacyPullChatHistoryToggle(t *testing.T) {
	ctx := context.Background()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("chat", "trigger:chat", map[string]any{"label": "Chat"}),
		v2Node("agent", "llm:agent", map[string]any{"instructions": "Legacy mode.", "pullChatHistory": true}),
	}, Edges: []domain.FlowEdge{
		execEdge("chat-agent", "chat", "out", "agent", "in"),
		dataEdge("chat-chatid", "chat", "chatId", "agent", "chatId"),
	}}
	runner := &instructionsCaptureRunner{}
	chat := &recordingChatWriter{history: chatHistoryFixture()}
	if _, err := NewEngine(catalog.New(), runner, nil, WithChatWriter(chat)).Execute(ctx, flow, "chat", Packet{"text": "ignored", "chatId": "conv-1", "chatRunId": "run-1"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	messages := runner.requests[0].Messages
	if len(messages) != 4 || messages[3].Content != "Summarise that in one word." {
		t.Fatalf("messages = %#v, want the legacy toggle to behave as chat-history mode", messages)
	}
}

func TestAgentChatHistoryRequiresChatID(t *testing.T) {
	ctx := context.Background()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("agent", "llm:agent", map[string]any{"instructions": "Anything.", "chatMode": "history"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-agent", "start", "out", "agent", "in"),
	}}
	_, err := NewEngine(catalog.New(), &instructionsCaptureRunner{}, nil, WithChatWriter(&recordingChatWriter{})).Execute(ctx, flow, "start", Packet{})
	if err == nil || !strings.Contains(err.Error(), "no Chat ID") {
		t.Fatalf("error = %v, want missing Chat ID failure", err)
	}
}

func TestAgentChatHistoryRejectsEmptyConversation(t *testing.T) {
	ctx := context.Background()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("agent", "llm:agent", map[string]any{"instructions": "Anything.", "chatMode": "history", "chatId": "conv-empty"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-agent", "start", "out", "agent", "in"),
	}}
	_, err := NewEngine(catalog.New(), &instructionsCaptureRunner{}, nil, WithChatWriter(&recordingChatWriter{})).Execute(ctx, flow, "start", Packet{})
	if err == nil || !strings.Contains(err.Error(), "no messages to answer") {
		t.Fatalf("error = %v, want empty-conversation failure", err)
	}
}

// The Chat ID pin only exists in chat-history mode, so the runtime,
// validator, and editor agree on the agent's contract.
func TestAgentChatIDPinFollowsChatMode(t *testing.T) {
	registry := catalog.New()
	definition, ok := registry.Get("llm:agent")
	if !ok {
		t.Fatal("llm:agent is not registered")
	}
	off, err := definitionForNode(definition, v2Node("agent", "llm:agent", map[string]any{"instructions": "x", "chatMode": "message"}))
	if err != nil {
		t.Fatalf("definitionForNode() error = %v", err)
	}
	for _, pin := range off.Inputs {
		if pin.ID == "chatId" {
			t.Fatal("chatId pin must be hidden outside chat-history mode")
		}
	}
	on, err := definitionForNode(definition, v2Node("agent", "llm:agent", map[string]any{"instructions": "x", "chatMode": "history"}))
	if err != nil {
		t.Fatalf("definitionForNode() error = %v", err)
	}
	found := false
	for _, pin := range on.Inputs {
		if pin.ID == "chatId" {
			found = true
			if pin.Required {
				t.Fatal("chatId pin must stay optional so existing graphs keep validating")
			}
		}
	}
	if !found {
		t.Fatal("chatId pin must exist in chat-history mode")
	}
}

// loopingToolRunner calls a tool on the first toolRounds requests, then
// answers with content so turn-budget behaviour is observable.
type loopingToolRunner struct {
	requests   int
	toolRounds int
}

func (r *loopingToolRunner) Chat(_ context.Context, _ ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, fmt.Errorf("unexpected non-tool LLM request")
}

func (r *loopingToolRunner) Converse(_ context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	r.requests++
	if r.requests <= r.toolRounds && len(request.Tools) > 0 {
		return domain.AssistantChatResponse{ToolCalls: []domain.ChatToolCall{{ID: "loop-call", Name: request.Tools[0].Name, Arguments: map[string]any{}}}}, nil
	}
	return domain.AssistantChatResponse{Content: "done"}, nil
}

func agentToolFlow(t *testing.T, agentConfig map[string]any) (domain.FlowDefinition, *persistence.Store) {
	t.Helper()
	ctx := context.Background()
	store, err := persistence.New(t.TempDir() + "/data")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	weather, err := createPublishedToolFunction(ctx, store, "Weather", "forecast")
	if err != nil {
		t.Fatal(err)
	}
	return domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("agent", "llm:agent", agentConfig),
		v2Node("weather", "function:"+weather.ID, nil),
	}, Edges: []domain.FlowEdge{
		execEdge("start-agent", "start", "out", "agent", "in"),
		{ID: "weather-tool", Source: "weather", SourceHandle: "tool", Target: "agent", TargetHandle: "tools", Kind: domain.PinTool},
	}}, store
}

func publishedRegistry(t *testing.T, store *persistence.Store) *catalog.Registry {
	t.Helper()
	registry := catalog.New()
	definitions, err := store.PublishedFunctionDefinitions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	registry.ReplaceDynamic(definitions)
	return registry
}

// Unlimited turns removes the cap entirely, even beyond the configured
// maximum of 32.
func TestAgentUnlimitedTurnsRunsPastConfiguredCap(t *testing.T) {
	flow, store := agentToolFlow(t, map[string]any{"instructions": "Keep going.", "maxTurns": 32.0, "unlimitedTurns": true})
	runner := &loopingToolRunner{toolRounds: 40}
	if _, err := NewEngine(publishedRegistry(t, store), runner, nil, WithFunctionResolver(store)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.requests != 41 {
		t.Fatalf("assistant turns = %d, want 40 tool rounds plus the final answer", runner.requests)
	}
}

func TestAgentCappedTurnsStillEnforceBudget(t *testing.T) {
	flow, store := agentToolFlow(t, map[string]any{"instructions": "Keep going.", "maxTurns": 3.0})
	runner := &loopingToolRunner{toolRounds: 40}
	_, err := NewEngine(publishedRegistry(t, store), runner, nil, WithFunctionResolver(store)).Execute(context.Background(), flow, "start", Packet{})
	if err == nil || !strings.Contains(err.Error(), "maximum number of tool turns") {
		t.Fatalf("error = %v, want turn-budget exhaustion", err)
	}
}

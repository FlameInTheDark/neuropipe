package pipeline

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// providerCaptureRunner records every chat request an AI node issues.
type providerCaptureRunner struct {
	requests []ChatRequest
}

func (r *providerCaptureRunner) Chat(_ context.Context, request ChatRequest) (ChatResponse, error) {
	r.requests = append(r.requests, request)
	// The boolean router decodes a JSON decision, so seed one by default.
	return ChatResponse{Content: `{"decision":"true"}`, JSON: map[string]any{"decision": "true"}}, nil
}

func TestLLMPromptPassesProviderAndModelSelection(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("prompt", "llm:prompt", map[string]any{"prompt": "Summarise.", "providerId": "claude", "model": "claude-sonnet-4-5"}),
	}, Edges: []domain.FlowEdge{execEdge("start-prompt", "start", "out", "prompt", "in")}}
	runner := &providerCaptureRunner{}
	if _, err := NewEngine(catalog.New(), runner, nil).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("LLM calls = %d, want 1", len(runner.requests))
	}
	request := runner.requests[0]
	if request.ProviderID != "claude" {
		t.Fatalf("provider = %q, want the node's provider selection", request.ProviderID)
	}
	if request.Model != "claude-sonnet-4-5" {
		t.Fatalf("model = %q, want the node's model selection", request.Model)
	}
}

func TestLLMBooleanPassesProviderAndModelSelection(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("route", "llm:boolean", map[string]any{"prompt": "Ready?", "providerId": "ollama-local", "model": "qwen3:8b"}),
	}, Edges: []domain.FlowEdge{execEdge("start-route", "start", "out", "route", "in")}}
	runner := &providerCaptureRunner{}
	if _, err := NewEngine(catalog.New(), runner, nil).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("LLM calls = %d, want 1", len(runner.requests))
	}
	request := runner.requests[0]
	if request.ProviderID != "ollama-local" || request.Model != "qwen3:8b" {
		t.Fatalf("provider = %q model = %q, want the node's selections", request.ProviderID, request.Model)
	}
}

// Empty selections must stay empty so the manager resolves the default
// provider and the provider's default model at execution time.
func TestLLMNodesDefaultToEmptyProviderAndModel(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("prompt", "llm:prompt", map[string]any{"prompt": "Summarise."}),
	}, Edges: []domain.FlowEdge{execEdge("start-prompt", "start", "out", "prompt", "in")}}
	runner := &providerCaptureRunner{}
	if _, err := NewEngine(catalog.New(), runner, nil).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if request := runner.requests[0]; request.ProviderID != "" || request.Model != "" {
		t.Fatalf("provider = %q model = %q, want empty defaults", request.ProviderID, request.Model)
	}
}

// Legacy graphs wired a free-text Model pin; the connected value must keep
// overriding the inspector selection at runtime.
func TestLLMPromptWiredModelPinOverridesConfig(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("model", "data:constant", map[string]any{"type": "text", "value": "wired-model"}),
		cfgNode("prompt", "llm:prompt", map[string]any{"prompt": "Summarise.", "providerId": "claude", "model": "claude-sonnet-4-5"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-prompt", "start", "out", "prompt", "in"),
		dataEdge("model-prompt", "model", "value", "prompt", "model"),
	}}
	runner := &providerCaptureRunner{}
	if _, err := NewEngine(catalog.New(), runner, nil).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if request := runner.requests[0]; request.Model != "wired-model" {
		t.Fatalf("model = %q, want the wired pin value", request.Model)
	}
	if request := runner.requests[0]; request.ProviderID != "claude" {
		t.Fatalf("provider = %q, want the configured provider", request.ProviderID)
	}
}

// assistantProviderCaptureRunner records converse requests for agent paths.
type assistantProviderCaptureRunner struct {
	requests []domain.AssistantChatRequest
}

func (r *assistantProviderCaptureRunner) Chat(_ context.Context, _ ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, context.DeadlineExceeded
}

func (r *assistantProviderCaptureRunner) Converse(_ context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	r.requests = append(r.requests, request)
	return domain.AssistantChatResponse{Content: "ok"}, nil
}

func TestAgentPassesProviderAndModelSelection(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("chat", "trigger:chat", map[string]any{"label": "Chat"}),
		cfgNode("agent", "llm:agent", map[string]any{"instructions": "Standby.", "providerId": "router", "model": "m1", "chatMode": "history", "chatId": "conv-1"}),
	}, Edges: []domain.FlowEdge{
		execEdge("chat-agent", "chat", "out", "agent", "in"),
	}}
	runner := &assistantProviderCaptureRunner{}
	chat := &recordingChatWriter{history: chatHistoryFixture()}
	if _, err := NewEngine(catalog.New(), runner, nil, WithChatWriter(chat)).Execute(context.Background(), flow, "chat", Packet{"text": "Hi", "chatId": "conv-1", "chatRunId": "r1"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("assistant calls = %d, want 1", len(runner.requests))
	}
	request := runner.requests[0]
	if request.ProviderID != "router" || request.Model != "m1" {
		t.Fatalf("provider = %q model = %q, want the node's selections", request.ProviderID, request.Model)
	}
}

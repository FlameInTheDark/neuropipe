package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

type recordingToolRunner struct {
	requests []domain.AssistantChatRequest
}

func (r *recordingToolRunner) Chat(_ context.Context, _ ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, fmt.Errorf("unexpected non-tool LLM request")
}

func (r *recordingToolRunner) Converse(_ context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	r.requests = append(r.requests, request)
	if len(r.requests) == 1 {
		if len(request.Tools) != 2 {
			return domain.AssistantChatResponse{}, fmt.Errorf("tool definitions = %d, want 2", len(request.Tools))
		}
		return domain.AssistantChatResponse{ToolCalls: []domain.ChatToolCall{{ID: "call-1", Name: request.Tools[0].Name, Arguments: map[string]any{"city": "Yekaterinburg"}}}}, nil
	}
	if len(request.Messages) == 0 {
		return domain.AssistantChatResponse{}, fmt.Errorf("tool result message is missing")
	}
	toolResult := request.Messages[len(request.Messages)-1]
	if toolResult.Role != domain.ChatRoleTool || !strings.Contains(toolResult.Content, "forecast for Yekaterinburg") {
		return domain.AssistantChatResponse{}, fmt.Errorf("tool result = %#v", toolResult)
	}
	return domain.AssistantChatResponse{Content: "The forecast is ready."}, nil
}

func TestBlueprintExecutesPublishedImpureFunction(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	function, err := store.CreateFunction(context.Background(), "Run once", domain.NodeImpure)
	if err != nil {
		t.Fatal(err)
	}
	function, err = store.PublishFunction(context.Background(), function)
	if err != nil {
		t.Fatal(err)
	}
	registry := catalog.New()
	definitions, err := store.PublishedFunctionDefinitions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	registry.ReplaceDynamic(definitions)
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{v2Node("start", "trigger:button", map[string]any{"label": "Run"}), v2Node("call", "function:"+function.ID, nil)}, Edges: []domain.FlowEdge{execEdge("call", "start", "out", "call", "in")}}
	result, err := NewEngine(registry, nil, nil, WithFunctionResolver(store)).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.NodeRuns) != 4 {
		t.Fatalf("node runs = %d, want event, call, inner entry, and inner return", len(result.NodeRuns))
	}
}

func TestBlueprintCachesPublishedPureFunction(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	function, err := store.CreateFunction(context.Background(), "Identity", domain.NodePure)
	if err != nil {
		t.Fatal(err)
	}
	function.Inputs = []domain.FunctionPin{{ID: "input", Name: "Input", DataType: domain.DataBoolean, Required: true}}
	function.Outputs = []domain.FunctionPin{{ID: "output", Name: "Output", DataType: domain.DataBoolean}}
	function.DraftDefinition.Edges = []domain.FlowEdge{dataEdge("identity", "inputs", "input", "outputs", "output")}
	function, err = store.PublishFunction(context.Background(), function)
	if err != nil {
		t.Fatal(err)
	}
	registry := catalog.New()
	definitions, err := store.PublishedFunctionDefinitions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	registry.ReplaceDynamic(definitions)
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{v2Node("start", "trigger:button", map[string]any{"label": "Run"}), v2Node("truth", "data:constant", map[string]any{"value": true}), v2Node("call", "function:"+function.ID, nil), v2Node("branch", "flow:branch", nil)}, Edges: []domain.FlowEdge{execEdge("start-branch", "start", "out", "branch", "in"), dataEdge("truth-call", "truth", "value", "call", "input"), dataEdge("call-branch", "call", "output", "branch", "condition")}}
	if _, err := NewEngine(registry, nil, nil, WithFunctionResolver(store)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestAgentRunsMultipleConnectedLLMToolFunctions(t *testing.T) {
	ctx := context.Background()
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	weather, err := createPublishedToolFunction(ctx, store, "Weather", "forecast for Yekaterinburg")
	if err != nil {
		t.Fatal(err)
	}
	calendar, err := createPublishedToolFunction(ctx, store, "Calendar", "calendar result")
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
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("agent", "llm:agent", map[string]any{"instructions": "Use a connected tool before answering.", "maxTurns": 3.0}),
		v2Node("weather", "function:"+weather.ID, nil),
		v2Node("calendar", "function:"+calendar.ID, nil),
	}, Edges: []domain.FlowEdge{
		execEdge("start-agent", "start", "out", "agent", "in"),
		{ID: "weather-tool", Source: "weather", SourceHandle: "tool", Target: "agent", TargetHandle: "tools", Kind: domain.PinTool},
		{ID: "calendar-tool", Source: "calendar", SourceHandle: "tool", Target: "agent", TargetHandle: "tools", Kind: domain.PinTool},
	}}
	runner := &recordingToolRunner{}
	result, err := NewEngine(registry, runner, nil, WithFunctionResolver(store)).Execute(ctx, flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := len(runner.requests); got != 2 {
		t.Fatalf("assistant turns = %d, want 2", got)
	}
	if len(result.NodeRuns) < 4 {
		t.Fatalf("node runs = %#v, want agent and tool-function runs", result.NodeRuns)
	}
}

type captureChatRunner struct {
	requests []ChatRequest
}

func (r *captureChatRunner) Chat(_ context.Context, request ChatRequest) (ChatResponse, error) {
	r.requests = append(r.requests, request)
	return ChatResponse{Content: "ok"}, nil
}

// Regression: an Agent without connected tools is the legacy fallback path;
// its instructions field must be honored there too.
func TestAgentWithoutToolsUsesInstructionsField(t *testing.T) {
	ctx := context.Background()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		v2Node("chat", "trigger:chat", map[string]any{"label": "Chat"}),
		v2Node("agent", "llm:agent", map[string]any{"instructions": "Standby."}),
	}, Edges: []domain.FlowEdge{
		execEdge("chat-agent", "chat", "out", "agent", "in"),
		{ID: "chat-instructions", Source: "chat", SourceHandle: "text", Target: "agent", TargetHandle: "instructions", Kind: domain.PinData},
	}}
	runner := &captureChatRunner{}
	if _, err := NewEngine(catalog.New(), runner, nil).Execute(ctx, flow, "chat", Packet{"text": "Wired instructions no tools.", "chatId": "c1", "chatRunId": "r1"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("LLM calls = %d, want 1", len(runner.requests))
	}
	if !strings.Contains(runner.requests[0].Prompt, "Wired instructions no tools.") {
		t.Fatalf("prompt = %q, want wired instructions", runner.requests[0].Prompt)
	}
}

type instructionsCaptureRunner struct {
	requests []domain.AssistantChatRequest
}

func (r *instructionsCaptureRunner) Chat(_ context.Context, _ ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, fmt.Errorf("unexpected non-tool LLM request")
}

func (r *instructionsCaptureRunner) Converse(_ context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	r.requests = append(r.requests, request)
	return domain.AssistantChatResponse{Content: "ok"}, nil
}

// A wired instructions pin must override the inspector text: the model has to
// receive the connected value as the agent's system prompt.
func TestAgentUsesWiredInstructionsPin(t *testing.T) {
	ctx := context.Background()
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
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
		v2Node("agent", "llm:agent", map[string]any{"instructions": "Inspector fallback must be overridden.", "maxTurns": 2.0}),
		v2Node("weather", "function:"+weather.ID, nil),
	}, Edges: []domain.FlowEdge{
		execEdge("chat-agent", "chat", "out", "agent", "in"),
		{ID: "chat-instructions", Source: "chat", SourceHandle: "text", Target: "agent", TargetHandle: "instructions", Kind: domain.PinData},
		{ID: "weather-tool", Source: "weather", SourceHandle: "tool", Target: "agent", TargetHandle: "tools", Kind: domain.PinTool},
	}}
	runner := &instructionsCaptureRunner{}
	if _, err := NewEngine(registry, runner, nil, WithFunctionResolver(store)).Execute(ctx, flow, "chat", Packet{"text": "Wired instructions reach the model.", "chatId": "c1", "chatRunId": "r1"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(runner.requests) == 0 {
		t.Fatal("assistant was never called")
	}
	messages := runner.requests[0].Messages
	var system string
	for _, message := range messages {
		if message.Role == domain.ChatRoleSystem {
			system = message.Content
		}
	}
	if system != "Wired instructions reach the model." {
		t.Fatalf("agent system prompt = %q, want wired instructions value", system)
	}
}

func createPublishedToolFunction(ctx context.Context, store *persistence.Store, name, value string) (domain.CustomFunction, error) {
	function, err := store.CreateFunctionWithRequest(ctx, domain.CreateFunctionRequest{Name: name, Description: name + " tool", Kind: domain.FunctionTool, Mode: domain.NodeImpure})
	if err != nil {
		return domain.CustomFunction{}, err
	}
	function.Inputs = []domain.FunctionPin{{ID: "city", Name: "City", Description: "The city and country to look up.", DataType: domain.DataText, Required: true}}
	function.Outputs = []domain.FunctionPin{{ID: "answer", Name: "Answer", Description: "The requested tool result.", DataType: domain.DataText, Required: true}}
	function.DraftDefinition.Nodes = append(function.DraftDefinition.Nodes, v2Node("answer", "data:constant", map[string]any{"type": "text", "value": value}))
	function.DraftDefinition.Edges = append(function.DraftDefinition.Edges, dataEdge("answer-return", "answer", "value", "return", "answer"))
	return store.PublishFunction(ctx, function)
}

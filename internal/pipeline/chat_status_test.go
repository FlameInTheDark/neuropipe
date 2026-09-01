package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// A tool-connected agent with status updates enabled reports thinking before
// every model turn and the tool name for every connected-tool call.
func TestAgentReportsToolProgressToChatRun(t *testing.T) {
	flow, store := agentToolFlow(t, map[string]any{"instructions": "Use the tool.", "maxTurns": 3.0, "updateChatStatus": true})
	// Replace the fixture's button trigger with the chat trigger that owns
	// the run being reported on.
	flow.Nodes[0] = cfgNode("chat", "trigger:chat", map[string]any{"label": "Chat"})
	flow.Edges[0] = execEdge("chat-agent", "chat", "out", "agent", "in")
	flow.Edges = append(flow.Edges, dataEdge("chat-runid", "chat", "chatRunId", "agent", "chatRunId"))
	runner := &loopingToolRunner{toolRounds: 1}
	chat := &recordingChatWriter{}
	if _, err := NewEngine(publishedRegistry(t, store), runner, nil, WithFunctionResolver(store), WithChatWriter(chat)).Execute(context.Background(), flow, "chat", Packet{"text": "status please", "chatId": "conv-1", "chatRunId": "run-42"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(chat.statuses) != 3 {
		t.Fatalf("statuses = %#v, want thinking, tool, thinking", chat.statuses)
	}
	if !strings.HasPrefix(chat.statuses[0], "run-42:Thinking") {
		t.Fatalf("first status = %q, want Thinking on the wired run", chat.statuses[0])
	}
	if !strings.Contains(chat.statuses[1], "Running Weather") {
		t.Fatalf("tool status = %q, want Running Weather", chat.statuses[1])
	}
	if !strings.HasPrefix(chat.statuses[2], "run-42:Thinking") {
		t.Fatalf("final status = %q, want Thinking before the answering turn", chat.statuses[2])
	}
}

// Plain LLM nodes report a single thinking status and stay silent with the
// toggle off.
func TestLLMPromptReportsThinkingStatus(t *testing.T) {
	ctx := context.Background()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("chat", "trigger:chat", map[string]any{"label": "Chat"}),
		cfgNode("prompt", "llm:prompt", map[string]any{"prompt": "Hello.", "updateChatStatus": true}),
	}, Edges: []domain.FlowEdge{
		execEdge("chat-prompt", "chat", "out", "prompt", "in"),
		dataEdge("chat-runid", "chat", "chatRunId", "prompt", "chatRunId"),
	}}
	runner := &captureChatRunner{}
	chat := &recordingChatWriter{}
	if _, err := NewEngine(catalog.New(), runner, nil, WithChatWriter(chat)).Execute(ctx, flow, "chat", Packet{"text": "hi", "chatId": "c", "chatRunId": "run-7"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(chat.statuses) != 1 || chat.statuses[0] != "run-7:Thinking" {
		t.Fatalf("statuses = %#v, want one Thinking on run-7", chat.statuses)
	}
}

func TestLLMNodesStaySilentWithoutStatusToggle(t *testing.T) {
	ctx := context.Background()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("prompt", "llm:prompt", map[string]any{"prompt": "Hello."}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-prompt", "start", "out", "prompt", "in"),
	}}
	runner := &captureChatRunner{}
	chat := &recordingChatWriter{}
	if _, err := NewEngine(catalog.New(), runner, nil, WithChatWriter(chat)).Execute(ctx, flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(chat.statuses) != 0 {
		t.Fatalf("statuses = %#v, want none while the toggle is off", chat.statuses)
	}
}

func TestLLMStatusToggleRequiresChatRunID(t *testing.T) {
	ctx := context.Background()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("prompt", "llm:prompt", map[string]any{"prompt": "Hello.", "updateChatStatus": true}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-prompt", "start", "out", "prompt", "in"),
	}}
	_, err := NewEngine(catalog.New(), &captureChatRunner{}, nil, WithChatWriter(&recordingChatWriter{})).Execute(ctx, flow, "start", Packet{})
	if err == nil || !strings.Contains(err.Error(), "no Chat Run ID") {
		t.Fatalf("error = %v, want missing Chat Run ID failure", err)
	}
}

// The Chat Run ID pin exists on every AI node only while its toggle is on.
func TestLLMChatRunIDPinFollowsToggle(t *testing.T) {
	registry := catalog.New()
	for _, nodeType := range []string{"llm:prompt", "llm:extract", "llm:boolean", "llm:choice", "llm:summarize", "llm:agent", "llm:coding_agent"} {
		definition, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s definition is missing", nodeType)
		}
		off, err := definitionForNode(definition, cfgNode("node", nodeType, map[string]any{"updateChatStatus": false}))
		if err != nil {
			t.Fatalf("%s definitionForNode() error = %v", nodeType, err)
		}
		for _, pin := range off.Inputs {
			if pin.ID == "chatRunId" {
				t.Fatalf("%s chatRunId pin must be hidden while status updates are off", nodeType)
			}
		}
		on, err := definitionForNode(definition, cfgNode("node", nodeType, map[string]any{"updateChatStatus": true}))
		if err != nil {
			t.Fatalf("%s definitionForNode() error = %v", nodeType, err)
		}
		found := false
		for _, pin := range on.Inputs {
			if pin.ID == "chatRunId" {
				found = true
				if pin.Required || pin.DataType != domain.DataText {
					t.Fatalf("%s chatRunId pin = %#v, want optional Text input", nodeType, pin)
				}
			}
		}
		if !found {
			t.Fatalf("%s chatRunId pin must exist while status updates are on", nodeType)
		}
	}
}

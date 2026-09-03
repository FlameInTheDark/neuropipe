package llm

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestChatCompletionsBaseURL(t *testing.T) {
	tests := []struct {
		baseURL string
		want    string
	}{
		// OpenAI itself is configured without the version segment.
		{"https://api.openai.com", "https://api.openai.com/v1"},
		// Compatible providers are conventionally configured with the versioned base.
		{"https://openrouter.ai/api/v1", "https://openrouter.ai/api/v1"},
		{"https://api.groq.com/openai/v1", "https://api.groq.com/openai/v1"},
		{"http://127.0.0.1:1234/v1", "http://127.0.0.1:1234/v1"},
		{"http://127.0.0.1:1234", "http://127.0.0.1:1234/v1"},
		// Trailing slashes and whitespace are normalized.
		{" https://openrouter.ai/api/v1/ ", "https://openrouter.ai/api/v1"},
	}
	for _, test := range tests {
		provider := domain.ProviderConfig{BaseURL: test.baseURL}
		if got := chatCompletionsBaseURL(provider); got != test.want {
			t.Fatalf("chatCompletionsBaseURL(%q) = %q, want %q", test.baseURL, got, test.want)
		}
	}
}

func TestAnthropicBaseURL(t *testing.T) {
	tests := []struct {
		baseURL string
		want    string
	}{
		// The official SDK appends /v1/messages itself.
		{"https://api.anthropic.com", "https://api.anthropic.com"},
		{"https://api.anthropic.com/v1", "https://api.anthropic.com"},
		{"https://proxy.example.com/claude/", "https://proxy.example.com/claude"},
		// An empty base falls back to the public API.
		{"", "https://api.anthropic.com"},
	}
	for _, test := range tests {
		provider := domain.ProviderConfig{BaseURL: test.baseURL}
		if got := anthropicBaseURL(provider); got != test.want {
			t.Fatalf("anthropicBaseURL(%q) = %q, want %q", test.baseURL, got, test.want)
		}
	}
}

func TestModelMessagesConvertsTranscript(t *testing.T) {
	messages, err := modelMessages([]domain.ChatMessage{
		{Role: domain.ChatRoleSystem, Content: "sys"},
		{Role: domain.ChatRoleUser, Content: "question"},
		{Role: domain.ChatRoleAssistant, Content: "answer"},
		{Role: domain.ChatRoleAssistant, Content: "", ToolCalls: []domain.ChatToolCall{{ID: "t1", Name: "weather", Arguments: map[string]any{"city": "Ekb"}}}},
		{Role: domain.ChatRoleTool, ToolCallID: "t1", ToolName: "weather", Content: "sunny"},
	})
	if err != nil {
		t.Fatalf("modelMessages() error = %v", err)
	}
	if len(messages) != 5 {
		t.Fatalf("messages = %d, want 5", len(messages))
	}
	if messages[0].Role != "system" || len(messages[0].Content) != 1 || messages[0].Content[0].Text != "sys" {
		t.Fatalf("system message = %#v", messages[0])
	}
	if messages[1].Role != "user" || messages[1].Content[0].Text != "question" {
		t.Fatalf("user message = %#v", messages[1])
	}
	if messages[3].Role != "assistant" || len(messages[3].Content) != 1 {
		t.Fatalf("tool-call assistant message = %#v", messages[3])
	}
	call := messages[3].Content[0]
	if call.Type != "tool-call" || call.ToolCallID != "t1" || call.ToolName != "weather" || string(call.Input) != `{"city":"Ekb"}` {
		t.Fatalf("tool call part = %#v", call)
	}
	result := messages[4]
	if result.Role != "tool" || len(result.Content) != 1 {
		t.Fatalf("tool message = %#v", result)
	}
	part := result.Content[0]
	if part.Type != "tool-result" || part.ToolCallID != "t1" || part.Output == nil || part.Output.Text != "sunny" {
		t.Fatalf("tool result part = %#v", part)
	}
}

func TestModelMessagesKeepsEmptyTranscriptValid(t *testing.T) {
	messages, err := modelMessages(nil)
	if err != nil {
		t.Fatalf("modelMessages() error = %v", err)
	}
	if len(messages) != 1 || messages[0].Role != "user" {
		t.Fatalf("messages = %#v, want one empty user turn", messages)
	}
}

func TestAIToolSetCompilesObjectSchemas(t *testing.T) {
	// Regression: tool definitions without required fields (list_pipelines
	// & friends) previously marshaled `"required": null`, which the ai-sdk's
	// draft 2020-12 metaschema validation rejects with exactly:
	//   '/required': got null, want array
	tools := []domain.ChatToolDefinition{
		{Name: "list_pipelines", Description: "List pipelines", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "Optional search"}}, "additionalProperties": false}},
		{Name: "run_pipeline", Description: "Run one", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"pipelineId": map[string]any{"type": "string"}}, "required": []string{"pipelineId"}, "additionalProperties": false}},
		{Name: "bare", Description: "No schema at all", InputSchema: nil},
	}
	toolSet, err := aiToolSet(tools)
	if err != nil {
		t.Fatalf("aiToolSet() error = %v, want the required-less schema to compile", err)
	}
	if len(toolSet) != 3 {
		t.Fatalf("tool set = %#v, want three tools", toolSet)
	}
	data, _ := json.Marshal(toolSet["list_pipelines"].InputSchema)
	if strings.Contains(string(data), `"required"`) {
		t.Fatalf("list_pipelines schema = %s, want required omitted entirely", data)
	}
	data, _ = json.Marshal(toolSet["bare"].InputSchema)
	if string(data) != defaultToolSchema {
		t.Fatalf("bare schema = %s, want the default object schema", data)
	}
}

func TestAIToolSetRejectsNullRequired(t *testing.T) {
	// Pins the failure mode the user hit: a null required value fails
	// compilation with the compile-tool error instead of reaching a provider.
	tools := []domain.ChatToolDefinition{
		{Name: "broken", Description: "Null required", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "required": nil, "additionalProperties": false}},
	}
	_, err := aiToolSet(tools)
	if err == nil || !strings.Contains(err.Error(), `compile tool "broken" schema`) {
		t.Fatalf("aiToolSet() error = %v, want the compile tool failure", err)
	}
}

func TestSamplingOptionsDefaultsPerKind(t *testing.T) {
	manager := NewManager(domain.Settings{}, nil)

	// OpenAI-compatible: 0.2 temperature default, unset parameters omitted.
	options := manager.samplingOptions(domain.ProviderConfig{Kind: domain.ProviderOpenAICompatible, Model: "m"}, "m")
	if len(options) != 1 {
		t.Fatalf("options = %#v, want only temperature for the bare OpenAI-compatible provider", options)
	}

	// Ollama: nothing is sent unless configured.
	options = manager.samplingOptions(domain.ProviderConfig{Kind: domain.ProviderOllama, Model: "m"}, "m")
	if len(options) != 0 {
		t.Fatalf("options = %#v, want no sampling options for the bare Ollama provider", options)
	}

	// Anthropic: the 4096 completion budget is always present.
	temperature := 0.9
	options = manager.samplingOptions(domain.ProviderConfig{Kind: domain.ProviderAnthropic, Model: "m", Parameters: &domain.GenerationParameters{Temperature: &temperature}}, "m")
	if len(options) != 2 {
		t.Fatalf("options = %#v, want max tokens plus temperature", options)
	}
}

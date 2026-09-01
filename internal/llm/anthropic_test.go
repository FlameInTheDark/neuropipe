package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

type secretStub map[string]string

func (s secretStub) Get(name string) (string, error) {
	value, ok := s[name]
	if !ok {
		return "", errSecretNotFound(name)
	}
	return value, nil
}

type secretError struct{ name string }

func (e secretError) Error() string { return "secret " + e.name + " not found" }

func errSecretNotFound(name string) error { return secretError{name: name} }

func anthropicProvider(serverURL string) domain.ProviderConfig {
	return domain.ProviderConfig{
		ID:        "claude",
		Name:      "Claude",
		Kind:      domain.ProviderAnthropic,
		BaseURL:   serverURL,
		Model:     "claude-sonnet-4-5",
		APIKeyRef: "anthropic-key",
		Enabled:   true,
	}
}

func TestAnthropicChatSendsMessagesAPIContract(t *testing.T) {
	var capturedPath string
	var capturedHeaders http.Header
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		capturedPath = request.URL.Path
		capturedHeaders = request.Header.Clone()
		if err := json.NewDecoder(request.Body).Decode(&capturedBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		_, _ = writer.Write([]byte(`{"content":[{"type":"text","text":"{\"decision\":\"true\"}"}],"usage":{"input_tokens":12,"output_tokens":34}}`))
	}))
	defer server.Close()

	manager := NewManager(domain.Settings{DefaultProviderID: "claude", Providers: []domain.ProviderConfig{anthropicProvider(server.URL)}}, secretStub{"anthropic-key": "sk-test"})
	response, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "route", ToolName: "route", ToolChoices: []string{"true", "false"}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if capturedPath != "/v1/messages" {
		t.Fatalf("path = %s, want /v1/messages", capturedPath)
	}
	if got := capturedHeaders.Get("x-api-key"); got != "sk-test" {
		t.Fatalf("x-api-key = %q, want the resolved secret", got)
	}
	if got := capturedHeaders.Get("anthropic-version"); got != anthropicVersion {
		t.Fatalf("anthropic-version = %q, want %q", got, anthropicVersion)
	}
	if got := capturedHeaders.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want no bearer header for Anthropic", got)
	}
	if capturedBody["model"] != "claude-sonnet-4-5" {
		t.Fatalf("model = %v, want the provider default", capturedBody["model"])
	}
	if capturedBody["max_tokens"] != float64(anthropicMaxTokens) {
		t.Fatalf("max_tokens = %v, want %d", capturedBody["max_tokens"], anthropicMaxTokens)
	}
	messages, ok := capturedBody["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %#v, want one user turn", capturedBody["messages"])
	}
	turn, _ := messages[0].(map[string]any)
	if turn["role"] != "user" {
		t.Fatalf("role = %v, want user", turn["role"])
	}
	if !strings.Contains(turn["content"].(string), `"decision"`) {
		t.Fatalf("content = %v, want the structured-output contract", turn["content"])
	}
	if response.JSON["decision"] != "true" {
		t.Fatalf("decision = %v, want true", response.JSON["decision"])
	}
	if !response.Usage.TokensReported || response.Usage.PromptTokens != 12 || response.Usage.CompletionTokens != 34 {
		t.Fatalf("usage = %#v, want input 12 / output 34", response.Usage)
	}
	if response.Usage.ProviderKind != domain.ProviderAnthropic || response.Usage.ProviderID != "claude" {
		t.Fatalf("usage provider = %s/%s, want anthropic/claude", response.Usage.ProviderKind, response.Usage.ProviderID)
	}
}

func TestAnthropicChatSurfacesErrorMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	}))
	defer server.Close()

	manager := NewManager(domain.Settings{DefaultProviderID: "claude", Providers: []domain.ProviderConfig{anthropicProvider(server.URL)}}, secretStub{"anthropic-key": "sk-test"})
	_, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hello"})
	if err == nil || !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Fatalf("Chat() error = %v, want the provider message", err)
	}
}

func TestAnthropicConverseMapsToolCalls(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&capturedBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		_, _ = writer.Write([]byte(`{"content":[{"type":"text","text":"Checking."},{"type":"tool_use","id":"toolu_1","name":"weather","input":{"city":"Yekaterinburg"}}],"usage":{"input_tokens":7,"output_tokens":9}}`))
	}))
	defer server.Close()

	manager := NewManager(domain.Settings{DefaultProviderID: "claude", Providers: []domain.ProviderConfig{anthropicProvider(server.URL)}}, secretStub{"anthropic-key": "sk-test"})
	response, err := manager.Converse(context.Background(), domain.AssistantChatRequest{
		Messages: []domain.ChatMessage{
			{Role: domain.ChatRoleSystem, Content: "You are precise."},
			{Role: domain.ChatRoleUser, Content: "Weather in Yekaterinburg?"},
		},
		Tools: []domain.ChatToolDefinition{{Name: "weather", Description: "Get the forecast", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}}}},
	})
	if err != nil {
		t.Fatalf("Converse() error = %v", err)
	}
	if response.Content != "Checking." {
		t.Fatalf("content = %q, want the text block", response.Content)
	}
	if len(response.ToolCalls) != 1 {
		t.Fatalf("tool calls = %#v, want one", response.ToolCalls)
	}
	call := response.ToolCalls[0]
	if call.ID != "toolu_1" || call.Name != "weather" || call.Arguments["city"] != "Yekaterinburg" {
		t.Fatalf("tool call = %#v", call)
	}
	if capturedBody["system"] != "You are precise." {
		t.Fatalf("system = %v, want the extracted system prompt", capturedBody["system"])
	}
	tools, _ := capturedBody["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %#v, want one tool", capturedBody["tools"])
	}
	tool, _ := tools[0].(map[string]any)
	if tool["name"] != "weather" {
		t.Fatalf("tool = %#v, want the weather definition", tool)
	}
	if _, exists := tool["input_schema"]; !exists {
		t.Fatalf("tool = %#v, want input_schema", tool)
	}
}

func TestAnthropicMessagesConvertTranscript(t *testing.T) {
	system, messages := anthropicMessages([]domain.ChatMessage{
		{Role: domain.ChatRoleSystem, Content: "sys"},
		{Role: domain.ChatRoleUser, Content: "question"},
		{Role: domain.ChatRoleAssistant, Content: "", ToolCalls: []domain.ChatToolCall{{ID: "t1", Name: "weather", Arguments: map[string]any{"city": "Ekb"}}}},
		{Role: domain.ChatRoleTool, ToolCallID: "t1", Content: "sunny"},
	})
	if system != "sys" {
		t.Fatalf("system = %q", system)
	}
	if len(messages) != 3 {
		t.Fatalf("messages = %#v, want user, assistant tool_use, user tool_result", messages)
	}
	if messages[0]["role"] != "user" || messages[0]["content"] != "question" {
		t.Fatalf("first message = %#v", messages[0])
	}
	assistantBlocks, _ := messages[1]["content"].([]map[string]any)
	if len(assistantBlocks) != 1 || assistantBlocks[0]["type"] != "tool_use" || assistantBlocks[0]["id"] != "t1" {
		t.Fatalf("assistant blocks = %#v, want a tool_use block", assistantBlocks)
	}
	resultBlocks, _ := messages[2]["content"].([]map[string]any)
	if len(resultBlocks) != 1 || resultBlocks[0]["type"] != "tool_result" || resultBlocks[0]["tool_use_id"] != "t1" || resultBlocks[0]["content"] != "sunny" {
		t.Fatalf("tool result blocks = %#v", resultBlocks)
	}
}

func TestAnthropicMessagesMergesConsecutiveToolResults(t *testing.T) {
	_, messages := anthropicMessages([]domain.ChatMessage{
		{Role: domain.ChatRoleUser, Content: "both"},
		{Role: domain.ChatRoleAssistant, ToolCalls: []domain.ChatToolCall{{ID: "t1", Name: "a"}, {ID: "t2", Name: "b"}}},
		{Role: domain.ChatRoleTool, ToolCallID: "t1", Content: "one"},
		{Role: domain.ChatRoleTool, ToolCallID: "t2", Content: "two"},
	})
	if len(messages) != 3 {
		t.Fatalf("messages = %#v, want the two tool results merged into one user turn", messages)
	}
	blocks, _ := messages[2]["content"].([]map[string]any)
	if messages[2]["role"] != "user" || len(blocks) != 2 {
		t.Fatalf("tool result turn = %#v, want one user turn with two blocks", messages[2])
	}
}

func TestAnthropicListModelsUsesDisplayName(t *testing.T) {
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		capturedHeaders = request.Header.Clone()
		if request.Method != http.MethodGet || request.URL.Path != "/v1/models" {
			t.Errorf("request = %s %s, want GET /v1/models", request.Method, request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{"data":[{"id":"claude-sonnet-4-5","display_name":"Claude Sonnet 4.5"},{"id":"claude-opus-4-1"}]}`))
	}))
	defer server.Close()

	manager := NewManager(domain.Settings{DefaultProviderID: "claude", Providers: []domain.ProviderConfig{anthropicProvider(server.URL)}}, secretStub{"anthropic-key": "sk-test"})
	models, err := manager.ListModels(context.Background(), "claude")
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %#v, want two", models)
	}
	if models[0].ID != "claude-sonnet-4-5" || models[0].Name != "Claude Sonnet 4.5" {
		t.Fatalf("first model = %#v, want display name", models[0])
	}
	if models[1].Name != "claude-opus-4-1" {
		t.Fatalf("second model = %#v, want id fallback for missing display name", models[1])
	}
	if capturedHeaders.Get("x-api-key") != "sk-test" || capturedHeaders.Get("anthropic-version") != anthropicVersion {
		t.Fatalf("headers = %#v, want anthropic authentication", capturedHeaders)
	}
}

func TestChatResolvesProviderByRequestID(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		capturedPath = request.URL.Path
		if strings.HasSuffix(request.URL.Path, "/models") {
			_, _ = writer.Write([]byte(`{"data":[{"id":"m"}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	manager := NewManager(domain.Settings{DefaultProviderID: "default", Providers: []domain.ProviderConfig{
		{ID: "default", Name: "Default", Kind: domain.ProviderOllama, BaseURL: server.URL, Model: "d", Enabled: true},
		{ID: "router", Name: "Router", Kind: domain.ProviderOpenAICompatible, BaseURL: server.URL, Model: "r", Enabled: true},
	}}, nil)
	if _, err := manager.Chat(context.Background(), pipeline.ChatRequest{ProviderID: "router", Prompt: "hi"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if capturedPath != "/v1/chat/completions" {
		t.Fatalf("path = %s, want the router's OpenAI-compatible endpoint", capturedPath)
	}
	// Without an explicit selection the default provider answers.
	if _, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hi"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if capturedPath != "/api/generate" {
		t.Fatalf("path = %s, want the default Ollama endpoint", capturedPath)
	}
}

func TestChatRejectsUnknownProviderSelection(t *testing.T) {
	manager := NewManager(domain.Settings{DefaultProviderID: "default", Providers: []domain.ProviderConfig{
		{ID: "default", Name: "Default", Kind: domain.ProviderOllama, BaseURL: "http://127.0.0.1:11434", Model: "d", Enabled: true},
	}}, nil)
	_, err := manager.Chat(context.Background(), pipeline.ChatRequest{ProviderID: "missing", Prompt: "hi"})
	if err == nil || !strings.Contains(err.Error(), `"missing"`) {
		t.Fatalf("Chat() error = %v, want unknown provider failure", err)
	}
}

func TestChatRejectsDisabledProviderSelection(t *testing.T) {
	manager := NewManager(domain.Settings{DefaultProviderID: "default", Providers: []domain.ProviderConfig{
		{ID: "default", Name: "Default", Kind: domain.ProviderOllama, BaseURL: "http://127.0.0.1:11434", Model: "d", Enabled: true},
		{ID: "paused", Name: "Paused", Kind: domain.ProviderOllama, BaseURL: "http://127.0.0.1:11434", Model: "p", Enabled: false},
	}}, nil)
	_, err := manager.Chat(context.Background(), pipeline.ChatRequest{ProviderID: "paused", Prompt: "hi"})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("Chat() error = %v, want disabled provider failure", err)
	}
}

func TestModelPrecedenceRequestThenProviderDefault(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewDecoder(request.Body).Decode(&capturedBody)
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	manager := NewManager(domain.Settings{DefaultProviderID: "router", Providers: []domain.ProviderConfig{
		{ID: "router", Name: "Router", Kind: domain.ProviderOpenAICompatible, BaseURL: server.URL, Model: "default-model", Enabled: true},
	}}, nil)
	if _, err := manager.Chat(context.Background(), pipeline.ChatRequest{Model: "requested-model", Prompt: "hi"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if capturedBody["model"] != "requested-model" {
		t.Fatalf("model = %v, want the request model", capturedBody["model"])
	}
	if _, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hi"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if capturedBody["model"] != "default-model" {
		t.Fatalf("model = %v, want the provider default", capturedBody["model"])
	}
}

func TestModelMissingFailsWithActionableError(t *testing.T) {
	manager := NewManager(domain.Settings{DefaultProviderID: "bare", Providers: []domain.ProviderConfig{
		{ID: "bare", Name: "Bare", Kind: domain.ProviderOllama, BaseURL: "http://127.0.0.1:11434", Enabled: true},
	}}, nil)
	_, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hi"})
	if err == nil || !strings.Contains(err.Error(), "select a default model for Bare") {
		t.Fatalf("Chat() error = %v, want default-model guidance", err)
	}
}

func TestModelOptionsAlwaysIncludesDefault(t *testing.T) {
	provider := domain.ProviderConfig{Model: "default-model", Models: []domain.ModelConfig{
		{ID: "default-model", Name: "Dup"},
		{ID: "other", Name: "Other"},
		{ID: "  "},
	}}
	options := provider.ModelOptions()
	if len(options) != 2 {
		t.Fatalf("options = %#v, want default + other", options)
	}
	if options[0].ID != "default-model" || options[0].Name != "" {
		t.Fatalf("first option = %#v, want the bare default entry", options[0])
	}
	if options[1].ID != "other" || options[1].Name != "Other" {
		t.Fatalf("second option = %#v", options[1])
	}
}

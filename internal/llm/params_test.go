package llm

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

func floatValue(t *testing.T, payload map[string]any, key string) float64 {
	t.Helper()
	value, ok := payload[key].(float64)
	if !ok {
		t.Fatalf("payload[%q] = %#v, want a number", key, payload[key])
	}
	return value
}

func optionsValue(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	value, ok := payload["options"].(map[string]any)
	if !ok {
		t.Fatalf("payload[options] = %#v, want an Ollama options object", payload["options"])
	}
	return value
}

func TestChatOpenAICompatibleAppliesGenerationParameters(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(openAIStreamHandler(&captured, openAITextChunks("ok", 0, 0)...))
	defer server.Close()

	temperature, topP, topK, maxTokens := 1.3, 0.8, 64, 200
	provider := domain.ProviderConfig{
		ID:      "compat",
		Name:    "Compatible",
		Kind:    domain.ProviderOpenAICompatible,
		BaseURL: server.URL,
		Model:   "m",
		Enabled: true,
		Parameters: &domain.GenerationParameters{
			Temperature: &temperature,
			TopP:        &topP,
		},
		Models: []domain.ModelConfig{{
			ID: "m",
			Parameters: &domain.GenerationParameters{
				TopK:      &topK,
				MaxTokens: &maxTokens,
			},
		}},
	}
	manager := NewManager(domain.Settings{DefaultProviderID: "compat", Providers: []domain.ProviderConfig{provider}}, secretStub{})

	if _, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hi"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if got := floatValue(t, captured, "temperature"); got != 1.3 {
		t.Fatalf("temperature = %v, want the configured 1.3 to replace the 0.2 default", got)
	}
	if got := floatValue(t, captured, "top_p"); got != 0.8 {
		t.Fatalf("top_p = %v, want 0.8", got)
	}
	if got := floatValue(t, captured, "max_tokens"); got != 200 {
		t.Fatalf("max_tokens = %v, want the model-level override 200", got)
	}
	// The ai-sdk openai-compatible provider drops topK with a warning instead
	// of sending an argument strict OpenAI endpoints reject.
	if _, exists := captured["top_k"]; exists {
		t.Fatalf("payload[top_k] = %#v, want top_k dropped for the OpenAI wire", captured["top_k"])
	}
}

func TestChatOpenAICompatibleOmitsUnsetParameters(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(openAIStreamHandler(&captured, openAITextChunks("ok", 0, 0)...))
	defer server.Close()

	provider := domain.ProviderConfig{
		ID:      "compat",
		Name:    "Compatible",
		Kind:    domain.ProviderOpenAICompatible,
		BaseURL: server.URL,
		Model:   "m",
		Enabled: true,
	}
	manager := NewManager(domain.Settings{DefaultProviderID: "compat", Providers: []domain.ProviderConfig{provider}}, secretStub{})
	if _, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hi"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if got := floatValue(t, captured, "temperature"); got != 0.2 {
		t.Fatalf("temperature = %v, want the built-in 0.2 low-variance default", got)
	}
	for _, key := range []string{"top_p", "top_k", "max_tokens"} {
		if _, exists := captured[key]; exists {
			t.Fatalf("payload[%q] = %#v, want unset parameters omitted from the wire", key, captured[key])
		}
	}
}

func TestChatOpenAICompatibleJSONModeUsesJSONObject(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(openAIStreamHandler(&captured, openAITextChunks("{}", 0, 0)...))
	defer server.Close()

	provider := domain.ProviderConfig{
		ID:      "compat",
		Name:    "Compatible",
		Kind:    domain.ProviderOpenAICompatible,
		BaseURL: server.URL,
		Model:   "m",
		Enabled: true,
	}
	manager := NewManager(domain.Settings{DefaultProviderID: "compat", Providers: []domain.ProviderConfig{provider}}, secretStub{})
	if _, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hi", Schema: map[string]any{"type": "object"}}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	responseFormat, _ := captured["response_format"].(map[string]any)
	if responseFormat["type"] != "json_object" {
		t.Fatalf("response_format = %#v, want json_object for structured output", captured["response_format"])
	}
}

func TestChatOllamaAppliesGenerationParameters(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(ollamaChatHandler(&captured, `{"message":{"content":"ok"}}`))
	defer server.Close()

	temperature, topK, maxTokens, contextSize := 0.55, 30, 128, 16384
	provider := domain.ProviderConfig{
		ID:      "ollama",
		Name:    "Ollama",
		Kind:    domain.ProviderOllama,
		BaseURL: server.URL,
		Model:   "m",
		Enabled: true,
		Parameters: &domain.GenerationParameters{
			Temperature: &temperature,
			TopK:        &topK,
			MaxTokens:   &maxTokens,
			ContextSize: &contextSize,
		},
	}
	manager := NewManager(domain.Settings{DefaultProviderID: "ollama", Providers: []domain.ProviderConfig{provider}}, secretStub{})
	if _, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hi"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	options := optionsValue(t, captured)
	if got := floatValue(t, options, "temperature"); got != 0.55 {
		t.Fatalf("options.temperature = %v, want 0.55", got)
	}
	if got := floatValue(t, options, "top_k"); got != 30 {
		t.Fatalf("options.top_k = %v, want 30", got)
	}
	if got := floatValue(t, options, "num_predict"); got != 128 {
		t.Fatalf("options.num_predict = %v, want 128", got)
	}
	if got := floatValue(t, options, "num_ctx"); got != 16384 {
		t.Fatalf("options.num_ctx = %v, want 16384", got)
	}
}

func TestChatOllamaOmitsOptionsWhenUnset(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(ollamaChatHandler(&captured, `{"message":{"content":"ok"}}`))
	defer server.Close()

	provider := domain.ProviderConfig{
		ID: "ollama", Name: "Ollama", Kind: domain.ProviderOllama, BaseURL: server.URL, Model: "m", Enabled: true,
	}
	manager := NewManager(domain.Settings{DefaultProviderID: "ollama", Providers: []domain.ProviderConfig{provider}}, secretStub{})
	if _, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hi"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if _, exists := captured["options"]; exists {
		t.Fatalf("options = %#v, want no options object when nothing is configured", captured["options"])
	}
}

func TestConverseOpenAICompatibleAppliesGenerationParameters(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(openAIStreamHandler(&captured, openAITextChunks("ok", 0, 0)...))
	defer server.Close()

	maxTokens := 256
	provider := domain.ProviderConfig{
		ID: "compat", Name: "Compatible", Kind: domain.ProviderOpenAICompatible, BaseURL: server.URL, Model: "m", Enabled: true,
		Models: []domain.ModelConfig{{ID: "m", Parameters: &domain.GenerationParameters{MaxTokens: &maxTokens}}},
	}
	manager := NewManager(domain.Settings{DefaultProviderID: "compat", Providers: []domain.ProviderConfig{provider}}, secretStub{})
	if _, err := manager.Converse(context.Background(), domain.AssistantChatRequest{Messages: []domain.ChatMessage{{Role: domain.ChatRoleUser, Content: "hi"}}}); err != nil {
		t.Fatalf("Converse() error = %v", err)
	}
	if got := floatValue(t, captured, "max_tokens"); got != 256 {
		t.Fatalf("max_tokens = %v, want the model-level override 256", got)
	}
}

func TestAnthropicAppliesGenerationParameters(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(anthropicStreamHandler(&captured, anthropicTextEvents("ok", 0, 0)...))
	defer server.Close()

	temperature, topP, topK, maxTokens := 0.9, 0.7, 20, 1024
	provider := anthropicProvider(server.URL)
	provider.Parameters = &domain.GenerationParameters{
		Temperature: &temperature,
		TopP:        &topP,
		TopK:        &topK,
		MaxTokens:   &maxTokens,
	}
	manager := NewManager(domain.Settings{DefaultProviderID: "claude", Providers: []domain.ProviderConfig{provider}}, secretStub{"anthropic-key": "sk-test"})
	if _, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hi"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if got := floatValue(t, captured, "max_tokens"); got != 1024 {
		t.Fatalf("max_tokens = %v, want the configured 1024 to replace the 4096 budget", got)
	}
	if got := floatValue(t, captured, "temperature"); got != 0.9 {
		t.Fatalf("temperature = %v, want 0.9", got)
	}
	if got := floatValue(t, captured, "top_k"); got != 20 {
		t.Fatalf("top_k = %v, want 20", got)
	}
	// The ai-sdk Anthropic provider intentionally drops top_p whenever
	// temperature is set: Anthropic treats them as alternatives, so
	// temperature wins.
	if _, exists := captured["top_p"]; exists {
		t.Fatalf("top_p = %v, want dropped in favor of temperature", captured["top_p"])
	}
}

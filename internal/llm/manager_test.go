package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

func TestOllamaBooleanResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/generate" {
			t.Fatalf("path = %s", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{"response":"{\"decision\":\"true\"}"}`))
	}))
	defer server.Close()
	manager := NewManager(domain.Settings{DefaultProviderID: "local", Providers: []domain.ProviderConfig{{ID: "local", Name: "Ollama", Kind: domain.ProviderOllama, BaseURL: server.URL, Model: "qwen", Enabled: true}}}, nil)
	response, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "route", ToolName: "route", ToolChoices: []string{"true", "false"}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if response.JSON["decision"] != "true" {
		t.Fatalf("decision = %v, want true", response.JSON["decision"])
	}
}

func TestManagedLlamaRequiresStartedRuntime(t *testing.T) {
	manager := NewManager(domain.Settings{DefaultProviderID: "llama-managed", Providers: []domain.ProviderConfig{{ID: "llama-managed", Name: "Managed llama.cpp", Kind: domain.ProviderLlamaCPP, Model: "model.gguf", Enabled: true}}}, nil)
	_, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hello"})
	if err == nil || !strings.Contains(err.Error(), "start managed llama.cpp") {
		t.Fatalf("Chat() error = %v, want start instruction", err)
	}
}

func TestManagedLlamaRoutesThroughRouter(t *testing.T) {
	var servedModel string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", request.URL.Path)
		}
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(request.Body).Decode(&payload)
		servedModel = payload.Model
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{\"decision\":\"true\"}"}}]}`))
	}))
	defer server.Close()

	var routedModel string
	manager := NewManager(
		domain.Settings{DefaultProviderID: "llama-managed", Providers: []domain.ProviderConfig{
			// A stale persisted endpoint must never be used once a router is
			// installed: the loopback port changes between sessions.
			{ID: "llama-managed", Name: "Managed llama.cpp", Kind: domain.ProviderLlamaCPP, BaseURL: "http://127.0.0.1:1", Model: "default.gguf", Enabled: true},
		}},
		nil,
		WithLlamaRouter(func(ctx context.Context, model string) (string, string, error) {
			routedModel = model
			return server.URL, "Canonical.gguf", nil
		}),
	)
	response, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "route", ToolName: "route", ToolChoices: []string{"true", "false"}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if response.JSON["decision"] != "true" {
		t.Fatalf("decision = %v, want true", response.JSON["decision"])
	}
	if routedModel != "default.gguf" {
		t.Fatalf("router model = %q, want the provider default model", routedModel)
	}
	if servedModel != "Canonical.gguf" {
		t.Fatalf("request model = %q, want the canonical served alias", servedModel)
	}
}

func TestManagedLlamaRouterErrorSurfaces(t *testing.T) {
	manager := NewManager(
		domain.Settings{DefaultProviderID: "llama-managed", Providers: []domain.ProviderConfig{{ID: "llama-managed", Kind: domain.ProviderLlamaCPP, Model: "model.gguf", Enabled: true}}},
		nil,
		WithLlamaRouter(func(context.Context, string) (string, string, error) {
			return "", "", errors.New("model \"model.gguf\" is not installed; download it in Settings, Models")
		}),
	)
	_, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "hello"})
	if err == nil || !strings.Contains(err.Error(), "is not installed") {
		t.Fatalf("Chat() error = %v, want the routing error surfaced", err)
	}
	_, err = manager.Converse(context.Background(), domain.AssistantChatRequest{Messages: []domain.ChatMessage{{Role: "user", Content: "hello"}}})
	if err == nil || !strings.Contains(err.Error(), "is not installed") {
		t.Fatalf("Converse() error = %v, want the routing error surfaced", err)
	}
}

func TestManagedLlamaConverseRoutesThroughRouter(t *testing.T) {
	var servedModel string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", request.URL.Path)
		}
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(request.Body).Decode(&payload)
		servedModel = payload.Model
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"Hello there."}}]}`))
	}))
	defer server.Close()

	manager := NewManager(
		domain.Settings{DefaultProviderID: "llama-managed", Providers: []domain.ProviderConfig{{ID: "llama-managed", Kind: domain.ProviderLlamaCPP, Model: "default.gguf", Enabled: true}}},
		nil,
		WithLlamaRouter(func(ctx context.Context, model string) (string, string, error) {
			return server.URL, "Default.gguf", nil
		}),
	)
	response, err := manager.Converse(context.Background(), domain.AssistantChatRequest{Messages: []domain.ChatMessage{{Role: "user", Content: "hello"}}})
	if err != nil {
		t.Fatalf("Converse() error = %v", err)
	}
	if response.Content != "Hello there." {
		t.Fatalf("content = %q, want the assistant reply", response.Content)
	}
	if servedModel != "Default.gguf" {
		t.Fatalf("request model = %q, want the canonical served alias", servedModel)
	}
}

func TestStructuredPromptIncludesChoiceGuidance(t *testing.T) {
	prompt := structuredPrompt(pipeline.ChatRequest{
		Prompt:      "Route this request.",
		ToolName:    "choose",
		ToolChoices: []string{"approve", "needs-review"},
		ToolChoiceDescriptions: map[string]string{
			"approve":      "Display name: Approve. All policy requirements are met.",
			"needs-review": "Display name: Needs review. A human should make the final decision.",
		},
	})
	if !strings.Contains(prompt, "approve — Display name: Approve. All policy requirements are met.") {
		t.Fatalf("structured prompt does not include choice guidance: %q", prompt)
	}
	if !strings.Contains(prompt, "needs-review — Display name: Needs review. A human should make the final decision.") {
		t.Fatalf("structured prompt does not include all choice guidance: %q", prompt)
	}
}

func TestEndpointNeverDuplicatesVersionSegment(t *testing.T) {
	tests := []struct {
		baseURL string
		path    string
		want    string
	}{
		// OpenAI itself is configured without the version segment.
		{"https://api.openai.com", "/v1/chat/completions", "https://api.openai.com/v1/chat/completions"},
		// Compatible providers are configured with the versioned base.
		{"https://openrouter.ai/api/v1/", "/v1/chat/completions", "https://openrouter.ai/api/v1/chat/completions"},
		{"https://api.groq.com/openai/v1", "/v1/models", "https://api.groq.com/openai/v1/models"},
		{"http://127.0.0.1:1234/v1", "/v1/chat/completions", "http://127.0.0.1:1234/v1/chat/completions"},
		// Unversioned routes and bases stay untouched.
		{"http://127.0.0.1:11434/", "/api/generate", "http://127.0.0.1:11434/api/generate"},
		{" https://api.openai.com/ ", "/v1/chat/completions", "https://api.openai.com/v1/chat/completions"},
	}
	for _, test := range tests {
		if got := endpoint(test.baseURL, test.path); got != test.want {
			t.Fatalf("endpoint(%q, %q) = %q, want %q", test.baseURL, test.path, got, test.want)
		}
	}
}

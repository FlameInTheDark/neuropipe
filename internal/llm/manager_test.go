package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

// openAIStreamHandler serves an OpenAI-compatible SSE stream after recording
// the JSON request body. Each chunk is written as one `data:` event and the
// stream terminates with [DONE], matching what the ai-sdk openai-compatible
// provider parses.
func openAIStreamHandler(destination *map[string]any, chunks ...string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if destination != nil {
			_ = json.NewDecoder(request.Body).Decode(destination)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(writer, "data: %s\n\n", chunk)
		}
		_, _ = fmt.Fprint(writer, "data: [DONE]\n\n")
	}
}

// openAITextChunks is the minimal chunk sequence carrying one assistant text
// answer, plus usage when prompt/completion are positive.
func openAITextChunks(content string, prompt, completion int64) []string {
	chunks := []string{
		`{"id":"chatcmpl-1","model":"m","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		fmt.Sprintf(`{"id":"chatcmpl-1","model":"m","choices":[{"index":0,"delta":{"content":%s},"finish_reason":null}]}`, mustJSONString(content)),
		`{"id":"chatcmpl-1","model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
	}
	if prompt > 0 || completion > 0 {
		chunks = append(chunks, fmt.Sprintf(`{"id":"chatcmpl-1","model":"m","choices":[],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d}}`, prompt, completion, prompt+completion))
	}
	return chunks
}

// ollamaChatHandler serves one native /api/chat response after recording the
// JSON request body.
func ollamaChatHandler(destination *map[string]any, response string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if destination != nil {
			_ = json.NewDecoder(request.Body).Decode(destination)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(response))
	}
}

// ollamaStreamChatHandler serves a streaming /api/chat NDJSON body, one
// chunk per line, after recording the JSON request body.
func ollamaStreamChatHandler(destination *map[string]any, lines ...string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if destination != nil {
			_ = json.NewDecoder(request.Body).Decode(destination)
		}
		writer.Header().Set("Content-Type", "application/x-ndjson")
		flusher, _ := writer.(http.Flusher)
		for _, line := range lines {
			_, _ = fmt.Fprintln(writer, line)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

func mustJSONString(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestOllamaBooleanResponse(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		capturedPath = request.URL.Path
		ollamaStreamChatHandler(nil,
			`{"message":{"content":"{\"decision\":\"true\"}"},"done":true,"done_reason":"stop","prompt_eval_count":2,"eval_count":4}`,
		)(writer, request)
	}))
	defer server.Close()
	manager := NewManager(domain.Settings{DefaultProviderID: "local", Providers: []domain.ProviderConfig{{ID: "local", Name: "Ollama", Kind: domain.ProviderOllama, BaseURL: server.URL, Model: "qwen", Enabled: true}}}, nil)
	response, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "route", ToolName: "route", ToolChoices: []string{"true", "false"}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if capturedPath != "/api/chat" {
		t.Fatalf("path = %s, want the native /api/chat endpoint", capturedPath)
	}
	if response.JSON["decision"] != "true" {
		t.Fatalf("decision = %v, want true", response.JSON["decision"])
	}
	if !response.Usage.TokensReported || response.Usage.PromptTokens != 2 || response.Usage.CompletionTokens != 4 {
		t.Fatalf("usage = %#v, want input 2 / output 4", response.Usage)
	}
}

func TestOllamaChatSendsNativeContract(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(ollamaStreamChatHandler(&captured,
		`{"message":{"content":"ok"},"done":true,"done_reason":"stop"}`,
	))
	defer server.Close()
	manager := NewManager(domain.Settings{DefaultProviderID: "local", Providers: []domain.ProviderConfig{{ID: "local", Name: "Ollama", Kind: domain.ProviderOllama, BaseURL: server.URL, Model: "qwen", Enabled: true}}}, nil)
	if _, err := manager.Chat(context.Background(), pipeline.ChatRequest{Prompt: "route", ToolName: "route", ToolChoices: []string{"true", "false"}}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if captured["model"] != "qwen" {
		t.Fatalf("model = %v, want qwen", captured["model"])
	}
	// every model call rides the streaming NDJSON wire now, even blocking
	// turns, because the ai-sdk orchestration always invokes DoStream
	if captured["stream"] != true {
		t.Fatalf("stream = %v, want the streaming native request", captured["stream"])
	}
	if captured["format"] != "json" {
		t.Fatalf("format = %v, want json mode for structured output", captured["format"])
	}
	messages, _ := captured["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("messages = %#v, want one user turn", captured["messages"])
	}
	turn, _ := messages[0].(map[string]any)
	if turn["role"] != "user" || !strings.Contains(fmt.Sprint(turn["content"]), `"decision"`) {
		t.Fatalf("turn = %#v, want the structured prompt", turn)
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
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s, want /v1/chat/completions", request.URL.Path)
		}
		_ = json.NewDecoder(request.Body).Decode(&captured)
		servedModel = fmt.Sprint(captured["model"])
		openAIStreamHandler(nil, openAITextChunks(`{"decision":"true"}`, 0, 0)...)(writer, request)
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
	if captured["stream"] != true {
		t.Fatalf("stream = %v, want the streaming wire the ai-sdk provider sends", captured["stream"])
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
			t.Errorf("path = %s, want /v1/chat/completions", request.URL.Path)
		}
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(request.Body).Decode(&payload)
		servedModel = payload.Model
		openAIStreamHandler(nil, openAITextChunks("Hello there.", 0, 0)...)(writer, request)
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

func TestOpenAICompatibleStreamCarriesUsage(t *testing.T) {
	server := httptest.NewServer(openAIStreamHandler(nil, openAITextChunks("hi", 11, 13)...))
	defer server.Close()
	manager := NewManager(domain.Settings{DefaultProviderID: "compat", Providers: []domain.ProviderConfig{
		{ID: "compat", Name: "Compatible", Kind: domain.ProviderOpenAICompatible, BaseURL: server.URL, Model: "m", Enabled: true},
	}}, nil)
	response, err := manager.Converse(context.Background(), domain.AssistantChatRequest{Messages: []domain.ChatMessage{{Role: "user", Content: "hello"}}})
	if err != nil {
		t.Fatalf("Converse() error = %v", err)
	}
	if response.Content != "hi" {
		t.Fatalf("content = %q, want hi", response.Content)
	}
	if !response.Usage.TokensReported || response.Usage.PromptTokens != 11 || response.Usage.CompletionTokens != 13 {
		t.Fatalf("usage = %#v, want input 11 / output 13 from the stream usage chunk", response.Usage)
	}
}

func TestConverseOpenAICompatibleMapsToolCalls(t *testing.T) {
	var captured map[string]any
	chunks := []string{
		`{"id":"c","model":"m","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"weather","arguments":""}}]},"finish_reason":null}]}`,
		`{"id":"c","model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Ekb\"}"}}]},"finish_reason":null}]}`,
		`{"id":"c","model":"m","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
	}
	server := httptest.NewServer(openAIStreamHandler(&captured, chunks...))
	defer server.Close()
	manager := NewManager(domain.Settings{DefaultProviderID: "compat", Providers: []domain.ProviderConfig{
		{ID: "compat", Name: "Compatible", Kind: domain.ProviderOpenAICompatible, BaseURL: server.URL, Model: "m", Enabled: true},
	}}, nil)
	response, err := manager.Converse(context.Background(), domain.AssistantChatRequest{
		Messages: []domain.ChatMessage{{Role: "user", Content: "weather?"}},
		Tools:    []domain.ChatToolDefinition{{Name: "weather", Description: "Get the forecast", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}, "required": []string{"city"}, "additionalProperties": false}}},
	})
	if err != nil {
		t.Fatalf("Converse() error = %v", err)
	}
	if len(response.ToolCalls) != 1 {
		t.Fatalf("tool calls = %#v, want one", response.ToolCalls)
	}
	call := response.ToolCalls[0]
	if call.ID != "call_1" || call.Name != "weather" || call.Arguments["city"] != "Ekb" {
		t.Fatalf("tool call = %#v", call)
	}
	tools, _ := captured["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %#v, want the weather tool on the wire", captured["tools"])
	}
	tool, _ := tools[0].(map[string]any)
	function, _ := tool["function"].(map[string]any)
	if function["name"] != "weather" {
		t.Fatalf("wire tool = %#v, want the weather function", tool)
	}
	parameters, _ := function["parameters"].(map[string]any)
	if parameters["type"] != "object" {
		t.Fatalf("wire parameters = %#v, want the compiled object schema", parameters)
	}
}

// openAIDeltaChunks streams one assistant answer split across two content
// deltas so the incremental forwarding path is exercised end to end.
func openAIDeltaChunks(prompt, completion int64) []string {
	chunks := []string{
		`{"id":"c","model":"m","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`{"id":"c","model":"m","choices":[{"index":0,"delta":{"content":"Hel"},"finish_reason":null}]}`,
		`{"id":"c","model":"m","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":null}]}`,
		`{"id":"c","model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
	}
	if prompt > 0 || completion > 0 {
		chunks = append(chunks, fmt.Sprintf(`{"id":"c","model":"m","choices":[],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d}}`, prompt, completion, prompt+completion))
	}
	return chunks
}

func TestConverseStreamForwardsDeltasAndUsage(t *testing.T) {
	server := httptest.NewServer(openAIStreamHandler(nil, openAIDeltaChunks(11, 13)...))
	defer server.Close()
	manager := NewManager(domain.Settings{DefaultProviderID: "compat", Providers: []domain.ProviderConfig{
		{ID: "compat", Name: "Compatible", Kind: domain.ProviderOpenAICompatible, BaseURL: server.URL, Model: "m", Enabled: true},
	}}, nil)
	var deltas []string
	response, err := manager.ConverseStream(context.Background(), domain.AssistantChatRequest{Messages: []domain.ChatMessage{{Role: "user", Content: "hello"}}}, func(delta string) { deltas = append(deltas, delta) })
	if err != nil {
		t.Fatalf("ConverseStream() error = %v", err)
	}
	if strings.Join(deltas, "") != "Hello" || len(deltas) != 2 {
		t.Fatalf("deltas = %#v, want Hel and lo forwarded separately in order", deltas)
	}
	if response.Content != "Hello" {
		t.Fatalf("content = %q, want Hello", response.Content)
	}
	if !response.Usage.TokensReported || response.Usage.PromptTokens != 11 || response.Usage.CompletionTokens != 13 {
		t.Fatalf("usage = %#v, want input 11 / output 13 from the stream usage chunk", response.Usage)
	}
}

func TestConverseStreamReturnsUnresolvedToolCalls(t *testing.T) {
	// Tools are registered without executors, so the stream must end after
	// one model step and hand the unresolved call back instead of waiting for
	// tool results that never arrive.
	chunks := []string{
		`{"id":"c","model":"m","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"weather","arguments":""}}]},"finish_reason":null}]}`,
		`{"id":"c","model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Ekb\"}"}}]},"finish_reason":null}]}`,
		`{"id":"c","model":"m","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
	}
	server := httptest.NewServer(openAIStreamHandler(nil, chunks...))
	defer server.Close()
	manager := NewManager(domain.Settings{DefaultProviderID: "compat", Providers: []domain.ProviderConfig{
		{ID: "compat", Name: "Compatible", Kind: domain.ProviderOpenAICompatible, BaseURL: server.URL, Model: "m", Enabled: true},
	}}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	response, err := manager.ConverseStream(ctx, domain.AssistantChatRequest{
		Messages: []domain.ChatMessage{{Role: "user", Content: "weather?"}},
		Tools:    []domain.ChatToolDefinition{{Name: "weather", Description: "Get the forecast", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}, "required": []string{"city"}, "additionalProperties": false}}},
	}, func(string) {})
	if err != nil {
		t.Fatalf("ConverseStream() error = %v, want the unresolved tool call returned, not a hang", err)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].ID != "call_1" || response.ToolCalls[0].Arguments["city"] != "Ekb" {
		t.Fatalf("tool calls = %#v, want the mapped weather call", response.ToolCalls)
	}
}

func TestConverseStreamOllamaForwardsDeltas(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(ollamaStreamChatHandler(&captured,
		`{"message":{"content":"Hel"},"done":false}`,
		`{"message":{"content":"lo"},"done":false}`,
		`{"message":{"content":""},"done":true,"done_reason":"stop","prompt_eval_count":3,"eval_count":5}`,
	))
	defer server.Close()
	manager := NewManager(domain.Settings{DefaultProviderID: "local", Providers: []domain.ProviderConfig{{ID: "local", Name: "Ollama", Kind: domain.ProviderOllama, BaseURL: server.URL, Model: "qwen", Enabled: true}}}, nil)
	var deltas []string
	response, err := manager.ConverseStream(context.Background(), domain.AssistantChatRequest{Messages: []domain.ChatMessage{{Role: "user", Content: "hello"}}}, func(delta string) { deltas = append(deltas, delta) })
	if err != nil {
		t.Fatalf("ConverseStream() error = %v", err)
	}
	if captured["stream"] != true {
		t.Fatalf("stream = %v, want the streaming native request", captured["stream"])
	}
	if strings.Join(deltas, "") != "Hello" || len(deltas) != 2 {
		t.Fatalf("deltas = %#v, want Hel and lo forwarded separately in order", deltas)
	}
	if response.Content != "Hello" {
		t.Fatalf("content = %q, want Hello", response.Content)
	}
	if !response.Usage.TokensReported || response.Usage.PromptTokens != 3 || response.Usage.CompletionTokens != 5 {
		t.Fatalf("usage = %#v, want input 3 / output 5 from the terminal line", response.Usage)
	}
}

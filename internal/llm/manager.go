// Package llm implements provider-neutral local and OpenAI-compatible model access.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

// SecretResolver provides backend-only access to a configured API key reference.
type SecretResolver interface {
	Get(name string) (string, error)
}

// UsageRecorder receives numerical LLM usage only. It never receives a prompt,
// provider response, tool arguments, credential, or pipeline packet.
type UsageRecorder interface {
	RecordLLM(context.Context, domain.LLMUsage) error
}

// ManagerOption adds optional infrastructure without widening provider calls.
type ManagerOption func(*Manager)

// WithUsageRecorder enables local LLM observability.
func WithUsageRecorder(recorder UsageRecorder) ManagerOption {
	return func(manager *Manager) { manager.usage = recorder }
}

// ModelInfo is the provider-neutral model picker item.
type ModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Manager resolves the configured provider for pipeline AI nodes.
type Manager struct {
	mu       sync.RWMutex
	settings domain.Settings
	secrets  SecretResolver
	http     *http.Client
	limiter  *limiter
	usage    UsageRecorder
}

// NewManager creates a provider manager from persisted settings.
func NewManager(settings domain.Settings, secrets SecretResolver, options ...ManagerOption) *Manager {
	manager := &Manager{settings: settings, secrets: secrets, http: &http.Client{Timeout: 90 * time.Second}, limiter: newLimiter(settings.MaxConcurrentLLMRuns)}
	for _, option := range options {
		option(manager)
	}
	return manager
}

// Configure replaces active provider settings after a successful settings save.
func (m *Manager) Configure(settings domain.Settings) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings = settings
	m.limiter = newLimiter(settings.MaxConcurrentLLMRuns)
}

// Providers returns safe provider settings without resolving secret values.
func (m *Manager) Providers() []domain.ProviderConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.ProviderConfig(nil), m.settings.Providers...)
}

// Chat implements pipeline.LLMRunner using the configured default provider.
func (m *Manager) Chat(ctx context.Context, request pipeline.ChatRequest) (response pipeline.ChatResponse, err error) {
	m.mu.RLock()
	limiter := m.limiter
	m.mu.RUnlock()
	queued := time.Now()
	if err = limiter.Acquire(ctx); err != nil {
		return pipeline.ChatResponse{}, err
	}
	defer limiter.Release()
	queueWait := time.Since(queued)
	provider, err := m.activeProvider()
	if err != nil {
		return pipeline.ChatResponse{}, err
	}
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = strings.TrimSpace(provider.Model)
	}
	if model == "" {
		return pipeline.ChatResponse{}, fmt.Errorf("select a default model for %s in Settings", provider.Name)
	}
	if provider.Kind == domain.ProviderLlamaCPP && strings.TrimSpace(provider.BaseURL) == "" {
		return pipeline.ChatResponse{}, fmt.Errorf("start managed llama.cpp in Settings before running AI nodes")
	}
	started := time.Now()
	switch provider.Kind {
	case domain.ProviderOllama:
		response, err = m.chatOllama(ctx, provider, model, request)
	case domain.ProviderLlamaCPP, domain.ProviderOpenAICompatible:
		response, err = m.chatOpenAICompatible(ctx, provider, model, request)
	default:
		err = fmt.Errorf("unsupported provider kind %q", provider.Kind)
	}
	response.Usage = m.completeUsage(provider, model, request.Metrics, response.Usage, queueWait, time.Since(started), err)
	m.recordUsage(response.Usage)
	return response, err
}

// Converse performs one provider-neutral assistant turn using a persisted
// transcript and optional native function tools. It intentionally shares the
// same bounded LLM limiter as pipeline AI nodes.
func (m *Manager) Converse(ctx context.Context, request domain.AssistantChatRequest) (response domain.AssistantChatResponse, err error) {
	m.mu.RLock()
	limiter := m.limiter
	m.mu.RUnlock()
	queued := time.Now()
	if err = limiter.Acquire(ctx); err != nil {
		return domain.AssistantChatResponse{}, err
	}
	defer limiter.Release()
	queueWait := time.Since(queued)
	provider, err := m.activeProvider()
	if err != nil {
		return domain.AssistantChatResponse{}, err
	}
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = strings.TrimSpace(provider.Model)
	}
	if model == "" {
		return domain.AssistantChatResponse{}, fmt.Errorf("select a default model for %s in Settings", provider.Name)
	}
	if provider.Kind == domain.ProviderLlamaCPP && strings.TrimSpace(provider.BaseURL) == "" {
		return domain.AssistantChatResponse{}, fmt.Errorf("start managed llama.cpp in Settings before chatting")
	}
	started := time.Now()
	switch provider.Kind {
	case domain.ProviderOllama:
		response, err = m.converseOllama(ctx, provider, model, request)
	case domain.ProviderLlamaCPP, domain.ProviderOpenAICompatible:
		response, err = m.converseOpenAICompatible(ctx, provider, model, request)
	default:
		err = fmt.Errorf("unsupported provider kind %q", provider.Kind)
	}
	response.Usage = m.completeUsage(provider, model, request.Metrics, response.Usage, queueWait, time.Since(started), err)
	m.recordUsage(response.Usage)
	return response, err
}

type limiter struct{ slots chan struct{} }

func newLimiter(limit int) *limiter {
	if limit < 1 {
		limit = 1
	}
	return &limiter{slots: make(chan struct{}, limit)}
}

func (l *limiter) Acquire(ctx context.Context) error {
	select {
	case l.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for LLM queue: %w", ctx.Err())
	}
}

func (l *limiter) Release() { <-l.slots }

// ListModels loads available models for the selected adapter when its API supports discovery.
func (m *Manager) ListModels(ctx context.Context, providerID string) ([]ModelInfo, error) {
	provider, err := m.provider(providerID)
	if err != nil {
		return nil, err
	}
	switch provider.Kind {
	case domain.ProviderOllama:
		return m.listOllama(ctx, provider)
	case domain.ProviderLlamaCPP, domain.ProviderOpenAICompatible:
		return m.listOpenAICompatible(ctx, provider)
	default:
		return nil, fmt.Errorf("unsupported provider kind %q", provider.Kind)
	}
}

func (m *Manager) activeProvider() (domain.ProviderConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, provider := range m.settings.Providers {
		if provider.ID == m.settings.DefaultProviderID && provider.Enabled {
			return provider, nil
		}
	}
	return domain.ProviderConfig{}, fmt.Errorf("choose an enabled default LLM provider in Settings")
}

func (m *Manager) provider(id string) (domain.ProviderConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, provider := range m.settings.Providers {
		if provider.ID == id {
			return provider, nil
		}
	}
	return domain.ProviderConfig{}, fmt.Errorf("provider %q not found", id)
}

func (m *Manager) chatOllama(ctx context.Context, provider domain.ProviderConfig, model string, request pipeline.ChatRequest) (pipeline.ChatResponse, error) {
	prompt := structuredPrompt(request)
	payload := map[string]any{"model": model, "prompt": prompt, "stream": false}
	if request.Schema != nil || len(request.ToolChoices) > 0 {
		payload["format"] = "json"
	}
	var response struct {
		Response        string `json:"response"`
		Error           string `json:"error"`
		PromptEvalCount int64  `json:"prompt_eval_count"`
		EvalCount       int64  `json:"eval_count"`
	}
	if err := m.postJSON(ctx, provider, "/api/generate", payload, &response); err != nil {
		return pipeline.ChatResponse{}, err
	}
	if strings.TrimSpace(response.Error) != "" {
		return pipeline.ChatResponse{}, fmt.Errorf("ollama: %s", response.Error)
	}
	result := parseResponse(response.Response)
	result.Usage = usageFromCounts(response.PromptEvalCount, response.EvalCount)
	return result, nil
}

func (m *Manager) chatOpenAICompatible(ctx context.Context, provider domain.ProviderConfig, model string, request pipeline.ChatRequest) (pipeline.ChatResponse, error) {
	prompt := structuredPrompt(request)
	payload := map[string]any{
		"model":       model,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"temperature": 0.2,
	}
	if request.Schema != nil || len(request.ToolChoices) > 0 {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := m.postJSON(ctx, provider, "/v1/chat/completions", payload, &response); err != nil {
		return pipeline.ChatResponse{}, err
	}
	if response.Error.Message != "" {
		return pipeline.ChatResponse{}, fmt.Errorf("%s: %s", provider.Name, response.Error.Message)
	}
	if len(response.Choices) == 0 {
		return pipeline.ChatResponse{}, fmt.Errorf("%s returned no choices", provider.Name)
	}
	result := parseResponse(response.Choices[0].Message.Content)
	result.Usage = usageFromCounts(response.Usage.PromptTokens, response.Usage.CompletionTokens)
	return result, nil
}

func (m *Manager) converseOpenAICompatible(ctx context.Context, provider domain.ProviderConfig, model string, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	payload := map[string]any{"model": model, "messages": assistantMessages(request.Messages), "temperature": 0.2}
	if len(request.Tools) > 0 {
		payload["tools"] = openAITools(request.Tools)
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := m.postJSON(ctx, provider, "/v1/chat/completions", payload, &response); err != nil {
		return domain.AssistantChatResponse{}, err
	}
	if response.Error.Message != "" {
		return domain.AssistantChatResponse{}, fmt.Errorf("%s: %s", provider.Name, response.Error.Message)
	}
	if len(response.Choices) == 0 {
		return domain.AssistantChatResponse{}, fmt.Errorf("%s returned no choices", provider.Name)
	}
	result, err := decodeAssistantResponse(response.Choices[0].Message.Content, response.Choices[0].Message.ToolCalls)
	if err != nil {
		return domain.AssistantChatResponse{}, err
	}
	result.Usage = usageFromCounts(response.Usage.PromptTokens, response.Usage.CompletionTokens)
	return result, nil
}

func (m *Manager) converseOllama(ctx context.Context, provider domain.ProviderConfig, model string, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	payload := map[string]any{"model": model, "messages": assistantMessages(request.Messages), "stream": false}
	if len(request.Tools) > 0 {
		payload["tools"] = openAITools(request.Tools)
	}
	var response struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		Error           string `json:"error"`
		PromptEvalCount int64  `json:"prompt_eval_count"`
		EvalCount       int64  `json:"eval_count"`
	}
	if err := m.postJSON(ctx, provider, "/api/chat", payload, &response); err != nil {
		return domain.AssistantChatResponse{}, err
	}
	if strings.TrimSpace(response.Error) != "" {
		return domain.AssistantChatResponse{}, fmt.Errorf("ollama: %s", response.Error)
	}
	calls := make([]domain.ChatToolCall, 0, len(response.Message.ToolCalls))
	for index, call := range response.Message.ToolCalls {
		id := strings.TrimSpace(call.ID)
		if id == "" {
			id = fmt.Sprintf("ollama-tool-%d", index+1)
		}
		calls = append(calls, domain.ChatToolCall{ID: id, Name: call.Function.Name, Arguments: call.Function.Arguments})
	}
	return domain.AssistantChatResponse{Content: strings.TrimSpace(response.Message.Content), ToolCalls: calls, Usage: usageFromCounts(response.PromptEvalCount, response.EvalCount)}, nil
}

func usageFromCounts(prompt, completion int64) domain.LLMUsage {
	return domain.LLMUsage{PromptTokens: prompt, CompletionTokens: completion, TokensReported: prompt > 0 || completion > 0}
}

func (m *Manager) completeUsage(provider domain.ProviderConfig, model string, context domain.LLMMetricContext, usage domain.LLMUsage, queueWait, duration time.Duration, err error) domain.LLMUsage {
	usage.ProviderID = provider.ID
	usage.ProviderName = provider.Name
	usage.ProviderKind = provider.Kind
	usage.Model = model
	usage.QueueWait = queueWait
	usage.Duration = duration
	usage.Succeeded = err == nil
	usage.Context = context
	usage.OccurredAt = time.Now().UTC()
	return usage
}

func (m *Manager) recordUsage(usage domain.LLMUsage) {
	if m.usage == nil {
		return
	}
	_ = m.usage.RecordLLM(context.Background(), usage)
}

func decodeAssistantResponse(content string, calls []struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}) (domain.AssistantChatResponse, error) {
	result := domain.AssistantChatResponse{Content: strings.TrimSpace(content), ToolCalls: make([]domain.ChatToolCall, 0, len(calls))}
	for index, call := range calls {
		arguments := make(map[string]any)
		if strings.TrimSpace(call.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &arguments); err != nil {
				return domain.AssistantChatResponse{}, fmt.Errorf("decode tool call %q arguments: %w", call.Function.Name, err)
			}
		}
		id := strings.TrimSpace(call.ID)
		if id == "" {
			id = fmt.Sprintf("tool-%d", index+1)
		}
		result.ToolCalls = append(result.ToolCalls, domain.ChatToolCall{ID: id, Name: call.Function.Name, Arguments: arguments})
	}
	return result, nil
}

func assistantMessages(messages []domain.ChatMessage) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		item := map[string]any{"role": string(message.Role), "content": message.Content}
		if message.Role == domain.ChatRoleTool && message.ToolCallID != "" {
			item["tool_call_id"] = message.ToolCallID
		}
		if message.Role == domain.ChatRoleAssistant && len(message.ToolCalls) > 0 {
			calls := make([]map[string]any, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				arguments, _ := json.Marshal(call.Arguments)
				calls = append(calls, map[string]any{"id": call.ID, "type": "function", "function": map[string]any{"name": call.Name, "arguments": string(arguments)}})
			}
			item["tool_calls"] = calls
		}
		result = append(result, item)
	}
	return result
}

func openAITools(tools []domain.ChatToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		result = append(result, map[string]any{"type": "function", "function": map[string]any{"name": tool.Name, "description": tool.Description, "parameters": tool.InputSchema}})
	}
	return result
}

func (m *Manager) listOllama(ctx context.Context, provider domain.ProviderConfig) ([]ModelInfo, error) {
	var response struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := m.getJSON(ctx, provider, "/api/tags", &response); err != nil {
		return nil, err
	}
	result := make([]ModelInfo, 0, len(response.Models))
	for _, model := range response.Models {
		result = append(result, ModelInfo{ID: model.Name, Name: defaultText(model.Model, model.Name)})
	}
	return result, nil
}

func (m *Manager) listOpenAICompatible(ctx context.Context, provider domain.ProviderConfig) ([]ModelInfo, error) {
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := m.getJSON(ctx, provider, "/v1/models", &response); err != nil {
		return nil, err
	}
	result := make([]ModelInfo, 0, len(response.Data))
	for _, model := range response.Data {
		result = append(result, ModelInfo{ID: model.ID, Name: model.ID})
	}
	return result, nil
}

func (m *Manager) getJSON(ctx context.Context, provider domain.ProviderConfig, path string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint(provider.BaseURL, path), nil)
	if err != nil {
		return err
	}
	if err := m.authorize(request, provider); err != nil {
		return err
	}
	response, err := m.http.Do(request)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", provider.Name, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		return fmt.Errorf("%s returned %s: %s", provider.Name, response.Status, strings.TrimSpace(string(data)))
	}
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode %s response: %w", provider.Name, err)
	}
	return nil
}

func (m *Manager) postJSON(ctx context.Context, provider domain.ProviderConfig, path string, payload, destination any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode provider request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(provider.BaseURL, path), bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if err := m.authorize(request, provider); err != nil {
		return err
	}
	response, err := m.http.Do(request)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", provider.Name, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		return fmt.Errorf("%s returned %s: %s", provider.Name, response.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode %s response: %w", provider.Name, err)
	}
	return nil
}

func (m *Manager) authorize(request *http.Request, provider domain.ProviderConfig) error {
	if provider.APIKeyRef == "" {
		return nil
	}
	if m.secrets == nil {
		return fmt.Errorf("provider %s requires secret %q", provider.Name, provider.APIKeyRef)
	}
	value, err := m.secrets.Get(provider.APIKeyRef)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+value)
	return nil
}

func structuredPrompt(request pipeline.ChatRequest) string {
	prompt := request.Prompt
	if len(request.ToolChoices) > 0 {
		field := "choice"
		if request.ToolName == "route" {
			field = "decision"
		}
		if len(request.ToolChoiceDescriptions) == 0 {
			prompt += "\n\nReturn only JSON in the shape {\"" + field + "\": \"...\"}. The allowed values are: " + strings.Join(request.ToolChoices, ", ") + "."
		} else {
			choices := make([]string, 0, len(request.ToolChoices))
			for _, choice := range request.ToolChoices {
				if description := strings.TrimSpace(request.ToolChoiceDescriptions[choice]); description != "" {
					choices = append(choices, choice+" — "+description)
				} else {
					choices = append(choices, choice)
				}
			}
			prompt += "\n\nReturn only JSON in the shape {\"" + field + "\": \"...\"}. Choose one allowed value:\n- " + strings.Join(choices, "\n- ")
		}
	}
	if request.Schema != nil {
		data, _ := json.Marshal(request.Schema)
		prompt += "\n\nReturn only JSON matching this schema: " + string(data)
	}
	return prompt
}

func parseResponse(content string) pipeline.ChatResponse {
	response := pipeline.ChatResponse{Content: strings.TrimSpace(content)}
	_ = json.Unmarshal([]byte(response.Content), &response.JSON)
	return response
}

func endpoint(baseURL, path string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + path
}

func defaultText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

var _ pipeline.LLMRunner = (*Manager)(nil)

// Package llm implements provider-neutral local and OpenAI-compatible model
// access on top of the grafana/ai-sdk orchestration. The Manager owns
// concurrency, usage accounting, secret resolution, and managed llama.cpp
// routing; provider wire formats live in the ai-sdk's LanguageModel
// implementations and the local Ollama adapter.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	aiprovider "github.com/grafana/ai-sdk/provider"

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

// LlamaRouter resolves a live endpoint, plus the canonical model name being
// served, for Neuropipe's managed llama.cpp runtime. Routers may start or
// switch the local llama-server process, so a call can take as long as
// loading a model file.
type LlamaRouter func(ctx context.Context, model string) (endpoint string, canonical string, err error)

// WithUsageRecorder enables local LLM observability.
func WithUsageRecorder(recorder UsageRecorder) ManagerOption {
	return func(manager *Manager) { manager.usage = recorder }
}

// WithLlamaRouter routes managed llama.cpp provider requests through the
// app-owned runtime instead of a persisted loopback URL. Without a router the
// provider fails fast with an actionable message when its endpoint is empty.
func WithLlamaRouter(router LlamaRouter) ManagerOption {
	return func(manager *Manager) { manager.llamaRouter = router }
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
	// llamaRouter, when installed, owns the managed llama.cpp endpoint and
	// serves requests through the runtime started by the application.
	llamaRouter LlamaRouter
}

// NewManager creates a provider manager from persisted settings.
func NewManager(settings domain.Settings, secrets SecretResolver, options ...ManagerOption) *Manager {
	manager := &Manager{settings: settings, secrets: secrets, http: &http.Client{}, limiter: newLimiter(settings.MaxConcurrentLLMRuns)}
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

// Chat implements pipeline.LLMRunner using the request's provider, falling
// back to the configured default provider.
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
	provider, err := m.resolveProvider(request.ProviderID)
	if err != nil {
		return pipeline.ChatResponse{}, err
	}
	model, err := m.resolveModel(provider, request.Model)
	if err != nil {
		return pipeline.ChatResponse{}, err
	}
	if provider.Kind == domain.ProviderLlamaCPP {
		endpoint, canonical, err := m.routeLlama(ctx, provider, model)
		if err != nil {
			return pipeline.ChatResponse{}, err
		}
		provider.BaseURL, model = endpoint, canonical
	}
	started := time.Now()
	result, err := m.generate(ctx, provider, model, generationCall{
		messages: []aiprovider.Message{aiprovider.UserText(structuredPrompt(request))},
		jsonMode: request.Schema != nil || len(request.ToolChoices) > 0,
	})
	if err != nil {
		failed := m.completeUsage(provider, model, request.Metrics, domain.LLMUsage{}, queueWait, time.Since(started), err)
		m.recordUsage(failed)
		return pipeline.ChatResponse{}, err
	}
	response = parseResponse(result.Text)
	response.Usage = m.completeUsage(provider, model, request.Metrics, usageFromResult(result.TotalUsage), queueWait, time.Since(started), err)
	m.recordUsage(response.Usage)
	return response, nil
}

// assistantTurn holds one assistant model call's resolved routing and
// accounting anchors, shared by the blocking and streaming Converse paths.
type assistantTurn struct {
	provider  domain.ProviderConfig
	model     string
	queueWait time.Duration
	started   time.Time
	release   func()
}

// beginAssistantTurn acquires the shared LLM limiter slot and resolves the
// provider, model, and llama.cpp route for one assistant turn. The caller
// owns releasing the slot through turn.release.
func (m *Manager) beginAssistantTurn(ctx context.Context, request domain.AssistantChatRequest) (*assistantTurn, error) {
	m.mu.RLock()
	limiter := m.limiter
	m.mu.RUnlock()
	queued := time.Now()
	if err := limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	fail := func(err error) (*assistantTurn, error) {
		limiter.Release()
		return nil, err
	}
	provider, err := m.resolveProvider(request.ProviderID)
	if err != nil {
		return fail(err)
	}
	model, err := m.resolveModel(provider, request.Model)
	if err != nil {
		return fail(err)
	}
	if provider.Kind == domain.ProviderLlamaCPP {
		endpoint, canonical, err := m.routeLlama(ctx, provider, model)
		if err != nil {
			return fail(err)
		}
		provider.BaseURL, model = endpoint, canonical
	}
	return &assistantTurn{provider: provider, model: model, queueWait: time.Since(queued), started: time.Now(), release: limiter.Release}, nil
}

// Converse performs one provider-neutral assistant turn using a persisted
// transcript and optional native function tools. It intentionally shares the
// same bounded LLM limiter as pipeline AI nodes. Tools are sent without an
// executor, so the turn returns unresolved tool calls to the caller after one
// model step; the multi-round loop, approval flow, and persistence stay owned
// by the chat service and pipeline engine.
func (m *Manager) Converse(ctx context.Context, request domain.AssistantChatRequest) (response domain.AssistantChatResponse, err error) {
	turn, err := m.beginAssistantTurn(ctx, request)
	if err != nil {
		return domain.AssistantChatResponse{}, err
	}
	defer turn.release()
	messages, err := modelMessages(request.Messages)
	if err != nil {
		return domain.AssistantChatResponse{}, err
	}
	result, err := m.generate(ctx, turn.provider, turn.model, generationCall{messages: messages, tools: request.Tools, reasoning: request.Reasoning})
	if err != nil {
		failed := m.completeUsage(turn.provider, turn.model, request.Metrics, domain.LLMUsage{}, turn.queueWait, time.Since(turn.started), err)
		m.recordUsage(failed)
		return domain.AssistantChatResponse{}, err
	}
	response, err = assistantResponse(result)
	if err != nil {
		return domain.AssistantChatResponse{}, err
	}
	response.Usage = m.completeUsage(turn.provider, turn.model, request.Metrics, response.Usage, turn.queueWait, time.Since(turn.started), nil)
	m.recordUsage(response.Usage)
	return response, nil
}

// ConverseStream performs the same assistant turn as Converse while forwarding
// assistant text to onDelta token by token as the provider emits it. Tool
// calls, usage accounting, and limiter behavior are identical to Converse;
// only the model transport switches from a blocking request to the provider's
// streaming wire.
func (m *Manager) ConverseStream(ctx context.Context, request domain.AssistantChatRequest, onDelta func(delta string)) (response domain.AssistantChatResponse, err error) {
	turn, err := m.beginAssistantTurn(ctx, request)
	if err != nil {
		return domain.AssistantChatResponse{}, err
	}
	defer turn.release()
	messages, err := modelMessages(request.Messages)
	if err != nil {
		return domain.AssistantChatResponse{}, err
	}
	result, err := m.streamGenerate(ctx, turn.provider, turn.model, generationCall{messages: messages, tools: request.Tools, reasoning: request.Reasoning}, onDelta)
	if err != nil {
		failed := m.completeUsage(turn.provider, turn.model, request.Metrics, domain.LLMUsage{}, turn.queueWait, time.Since(turn.started), err)
		m.recordUsage(failed)
		return domain.AssistantChatResponse{}, err
	}
	response, err = assistantResponse(result)
	if err != nil {
		return domain.AssistantChatResponse{}, err
	}
	response.Usage = m.completeUsage(turn.provider, turn.model, request.Metrics, response.Usage, turn.queueWait, time.Since(turn.started), nil)
	m.recordUsage(response.Usage)
	return response, nil
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
	case domain.ProviderAnthropic:
		return m.listAnthropic(ctx, provider)
	default:
		return nil, fmt.Errorf("unsupported provider kind %q", provider.Kind)
	}
}

// resolveProvider returns the provider for one call: an explicit ID selects
// that provider, an empty ID falls back to the configured default. Both paths
// require the provider to exist and be enabled so a misrouted node fails fast
// with an actionable message.
func (m *Manager) resolveProvider(providerID string) (domain.ProviderConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id := strings.TrimSpace(providerID)
	if id == "" {
		id = strings.TrimSpace(m.settings.DefaultProviderID)
	}
	for _, provider := range m.settings.Providers {
		if provider.ID == id {
			if !provider.Enabled {
				return domain.ProviderConfig{}, fmt.Errorf("provider %s is disabled in Settings", provider.Name)
			}
			return provider, nil
		}
	}
	if strings.TrimSpace(providerID) == "" {
		return domain.ProviderConfig{}, fmt.Errorf("choose an enabled default LLM provider in Settings")
	}
	return domain.ProviderConfig{}, fmt.Errorf("LLM provider %q is not configured; pick a configured provider on the node", providerID)
}

// routeLlama resolves the managed llama.cpp endpoint for one request. The
// installed router may start the local llama-server process or switch it to
// the requested model; without a router the provider must already carry a
// live persisted endpoint.
func (m *Manager) routeLlama(ctx context.Context, provider domain.ProviderConfig, model string) (string, string, error) {
	if m.llamaRouter == nil {
		if strings.TrimSpace(provider.BaseURL) == "" {
			return "", "", fmt.Errorf("start managed llama.cpp in Settings before running AI nodes")
		}
		return provider.BaseURL, model, nil
	}
	return m.llamaRouter(ctx, model)
}

// resolveModel applies the model precedence: an explicit node model first,
// then the provider's configured default model.
func (m *Manager) resolveModel(provider domain.ProviderConfig, requested string) (string, error) {
	model := strings.TrimSpace(requested)
	if model == "" {
		model = strings.TrimSpace(provider.Model)
	}
	if model == "" {
		return "", fmt.Errorf("select a default model for %s in Settings", provider.Name)
	}
	return model, nil
}

// activeProvider returns the configured default provider. Retained for the
// settings surface that reports the active selection.
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

// authorize attaches provider-kind-appropriate credentials. OpenAI-compatible
// endpoints use bearer auth; Anthropic uses its x-api-key header pair.
func (m *Manager) authorize(request *http.Request, provider domain.ProviderConfig) error {
	if provider.Kind == domain.ProviderAnthropic {
		return anthropicAuthorize(request, provider, m.secrets)
	}
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

// endpoint joins a provider base URL with an API path. OpenAI-compatible
// providers are conventionally configured with a versioned base such as
// https://openrouter.ai/api/v1, so a path's leading "/v1" is never appended
// twice.
func endpoint(baseURL, path string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(path, "/v1/") {
		path = strings.TrimPrefix(path, "/v1")
	}
	return base + path
}

func defaultText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

var _ pipeline.LLMRunner = (*Manager)(nil)

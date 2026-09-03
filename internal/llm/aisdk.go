package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	aisdk "github.com/grafana/ai-sdk"
	aiprovider "github.com/grafana/ai-sdk/provider"
	"github.com/grafana/ai-sdk/providers/anthropic"
	"github.com/grafana/ai-sdk/providers/openai-compatible"
	"github.com/grafana/ai-sdk/schema"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// This file is the bridge between Neuropipe's provider-neutral chat types and
// the grafana/ai-sdk orchestration. The Manager keeps owning concurrency,
// usage accounting, secret resolution, and managed llama.cpp routing; the
// ai-sdk owns request building, provider wire formats, and response decoding
// through its LanguageModel implementations.

// defaultToolSchema stands in for a tool definition without a schema so the
// JSON Schema validator never sees a null document.
const defaultToolSchema = `{"type":"object","properties":{}}`

// generateTimeout caps one model call. The parent context may carry a
// 30-minute execution budget; capping every LLM call at 15 minutes keeps a
// single stuck inference from consuming it, matching the previous behavior.
const generateTimeout = 15 * time.Minute

// generationCall describes one ai-sdk generate invocation in domain terms.
type generationCall struct {
	messages []aiprovider.Message
	tools    []domain.ChatToolDefinition
	// jsonMode requests machine-readable output. It maps to response_format
	// json_object on OpenAI-compatible servers, format "json" on Ollama, and
	// stays prompt-based on Anthropic, exactly like the previous adapters.
	jsonMode bool
	// reasoning carries an optional effort level ("low", "medium", …).
	// Empty keeps the provider default: the option is never sent so servers
	// that reject unknown fields stay unaffected.
	reasoning string
}

// generate performs one provider-neutral model turn through the ai-sdk.
func (m *Manager) generate(ctx context.Context, provider domain.ProviderConfig, model string, call generationCall) (*aisdk.GenerateTextResult, error) {
	languageModel, err := m.languageModel(provider, model)
	if err != nil {
		return nil, err
	}
	options, err := m.requestOptions(provider, model, call)
	if err != nil {
		return nil, err
	}
	requestContext, cancel := context.WithTimeout(ctx, generateTimeout)
	defer cancel()
	result, err := aisdk.GenerateText(requestContext, languageModel, asGenerateOptions(options)...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", provider.Name, err)
	}
	return result, nil
}

// asGenerateOptions adapts the shared options to GenerateText's signature;
// every shared Option also implements GenerateOption, so the element-wise
// assignment is always valid.
func asGenerateOptions(options []aisdk.Option) []aisdk.GenerateOption {
	adapted := make([]aisdk.GenerateOption, len(options))
	for index, option := range options {
		adapted[index] = option
	}
	return adapted
}

// asStreamOptions adapts the shared options to StreamText's signature.
func asStreamOptions(options []aisdk.Option) []aisdk.StreamOption {
	adapted := make([]aisdk.StreamOption, len(options))
	for index, option := range options {
		adapted[index] = option
	}
	return adapted
}

// streamGenerate performs one model turn through the ai-sdk streaming path,
// forwarding every text delta to onDelta as it arrives. The returned result
// carries the same fields GenerateText produces, so response mapping, usage
// accounting, and unresolved tool-call handling stay identical between both
// paths. onDelta may be nil when only the result is wanted.
func (m *Manager) streamGenerate(ctx context.Context, provider domain.ProviderConfig, model string, call generationCall, onDelta func(delta string)) (*aisdk.GenerateTextResult, error) {
	languageModel, err := m.languageModel(provider, model)
	if err != nil {
		return nil, err
	}
	options, err := m.requestOptions(provider, model, call)
	if err != nil {
		return nil, err
	}
	requestContext, cancel := context.WithTimeout(ctx, generateTimeout)
	defer cancel()
	result := aisdk.StreamText(requestContext, languageModel, asStreamOptions(options)...)
	// Drain the stream synchronously like GenerateText does, forwarding text
	// deltas on the way. The orchestration loop blocks once its part buffer
	// fills, so the stream must always be consumed to completion.
	for part := range result.FullStream() {
		if delta, ok := part.(aisdk.StreamTextDelta); ok && onDelta != nil && delta.Text != "" {
			onDelta(delta.Text)
		}
	}
	result.Wait()
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", provider.Name, err)
	}
	// A parent cancellation aborts the stream without necessarily recording an
	// error part; surface it so callers never mistake a truncated turn for a
	// complete one.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", provider.Name, err)
	}
	return &aisdk.GenerateTextResult{
		Text:       result.Text(),
		ToolCalls:  result.ToolCalls(),
		TotalUsage: result.TotalUsage(),
	}, nil
}

// requestOptions assembles the model-agnostic ai-sdk options shared by the
// GenerateText and StreamText paths: no automatic retries, the transcript,
// per-provider sampling, response format, and tool definitions. Every option
// constructor used here returns the shared aisdk.Option, which applies to
// both invocations.
func (m *Manager) requestOptions(provider domain.ProviderConfig, model string, call generationCall) ([]aisdk.Option, error) {
	options := []aisdk.Option{
		aisdk.WithMaxRetries(0),
		aisdk.WithModelMessages(call.messages...),
	}
	options = append(options, m.samplingOptions(provider, model)...)
	if call.jsonMode {
		switch provider.Kind {
		case domain.ProviderOllama, domain.ProviderLlamaCPP, domain.ProviderOpenAICompatible:
			options = append(options, aisdk.WithResponseFormat(aiprovider.ResponseFormat{Type: aiprovider.ResponseFormatJSON}))
		}
	}
	if len(call.tools) > 0 {
		toolSet, err := aiToolSet(call.tools)
		if err != nil {
			return nil, err
		}
		options = append(options, aisdk.WithTools(toolSet))
	}
	if effort := strings.TrimSpace(call.reasoning); effort != "" {
		options = append(options, aisdk.WithReasoning(aiprovider.ReasoningEffort(effort)))
	}
	return options, nil
}

// languageModel builds the ai-sdk LanguageModel for one resolved provider.
//
// Ollama keeps its native /api/chat adapter so the options object (num_ctx and
// friends) keeps applying; llama.cpp and every OpenAI-compatible service go
// through the ai-sdk's openai-compatible provider; Anthropic goes through the
// ai-sdk's Anthropic provider built on the official SDK.
func (m *Manager) languageModel(provider domain.ProviderConfig, model string) (aiprovider.LanguageModel, error) {
	switch provider.Kind {
	case domain.ProviderOllama:
		return newOllamaModel(provider.BaseURL, model, provider.Name, m.http, contextSizeFor(provider.EffectiveParameters(model))), nil
	case domain.ProviderLlamaCPP, domain.ProviderOpenAICompatible:
		options := []openaicompatible.Option{
			openaicompatible.WithBaseURL(chatCompletionsBaseURL(provider)),
			openaicompatible.WithProviderName(provider.Name),
			openaicompatible.WithHTTPClient(m.http),
			// Streaming usage requires stream_options.include_usage; the
			// servers Neuropipe targets tolerate the field, and without it
			// OpenAI-shaped servers report no token counts at all.
			openaicompatible.WithIncludeUsage(true),
		}
		if key, err := m.resolveAPIKey(provider); err != nil {
			return nil, err
		} else if key != "" {
			options = append(options, openaicompatible.WithAPIKey(key))
		}
		return openaicompatible.New(model, options...), nil
	case domain.ProviderAnthropic:
		key, err := m.resolveAPIKey(provider)
		if err != nil {
			return nil, err
		}
		options := []anthropic.Option{
			// Keep the previous no-retry behavior; the official SDK would
			// otherwise retry twice with backoff.
			anthropic.WithRequestOptions(anthropicoption.WithMaxRetries(0)),
		}
		if base := anthropicBaseURL(provider); base != "" {
			options = append(options, anthropic.WithRequestOptions(anthropicoption.WithBaseURL(base)))
		}
		return anthropic.New(key, model, options...), nil
	default:
		return nil, fmt.Errorf("unsupported provider kind %q", provider.Kind)
	}
}

// resolveAPIKey resolves a provider's secret reference. Providers without a
// key reference stay unauthenticated.
func (m *Manager) resolveAPIKey(provider domain.ProviderConfig) (string, error) {
	if provider.APIKeyRef == "" {
		return "", nil
	}
	if m.secrets == nil {
		return "", fmt.Errorf("provider %s requires secret %q", provider.Name, provider.APIKeyRef)
	}
	return m.secrets.Get(provider.APIKeyRef)
}

// samplingOptions maps the provider's effective generation parameters onto
// ai-sdk options, preserving each previous adapter's defaults:
//
//   - OpenAI-compatible and llama.cpp always sent the low-variance 0.2
//     temperature default and omitted every unset parameter;
//   - Ollama and Anthropic only sent explicitly configured values;
//   - Anthropic requires a completion budget, defaulting to 4096.
func (m *Manager) samplingOptions(provider domain.ProviderConfig, model string) []aisdk.Option {
	params := provider.EffectiveParameters(model)
	var options []aisdk.Option
	switch provider.Kind {
	case domain.ProviderLlamaCPP, domain.ProviderOpenAICompatible:
		temperature := 0.2
		if params.Temperature != nil {
			temperature = *params.Temperature
		}
		options = append(options, aisdk.WithTemperature(temperature))
		if params.TopP != nil {
			options = append(options, aisdk.WithTopP(*params.TopP))
		}
		if params.MaxTokens != nil {
			options = append(options, aisdk.WithMaxOutputTokens(*params.MaxTokens))
		}
	case domain.ProviderOllama:
		if params.Temperature != nil {
			options = append(options, aisdk.WithTemperature(*params.Temperature))
		}
		if params.TopP != nil {
			options = append(options, aisdk.WithTopP(*params.TopP))
		}
		if params.TopK != nil {
			options = append(options, aisdk.WithTopK(*params.TopK))
		}
		if params.MaxTokens != nil {
			options = append(options, aisdk.WithMaxOutputTokens(*params.MaxTokens))
		}
	case domain.ProviderAnthropic:
		options = append(options, aisdk.WithMaxOutputTokens(anthropicCompletionBudget(params)))
		if params.Temperature != nil {
			options = append(options, aisdk.WithTemperature(*params.Temperature))
		}
		if params.TopP != nil {
			options = append(options, aisdk.WithTopP(*params.TopP))
		}
		if params.TopK != nil {
			options = append(options, aisdk.WithTopK(*params.TopK))
		}
	}
	return options
}

// chatCompletionsBaseURL normalizes an OpenAI-compatible base URL so the
// provider's /chat/completions suffix lands correctly. Neuropipe providers are
// conventionally configured either bare (https://api.openai.com) or versioned
// (https://openrouter.ai/api/v1); both must behave identically.
func chatCompletionsBaseURL(provider domain.ProviderConfig) string {
	base := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	return base
}

// anthropicBaseURL normalizes an Anthropic base URL. The official SDK appends
// /v1/messages itself, so a persisted trailing /v1 must be stripped, and an
// empty base falls back to the public API.
func anthropicBaseURL(provider domain.ProviderConfig) string {
	base := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	base = strings.TrimSuffix(base, "/v1")
	if base == "" {
		return "https://api.anthropic.com"
	}
	return base
}

// contextSizeFor resolves the Ollama num_ctx setting, 0 keeping Ollama's own
// default.
func contextSizeFor(params domain.GenerationParameters) int {
	if params.ContextSize != nil && *params.ContextSize > 0 {
		return *params.ContextSize
	}
	return 0
}

// modelMessages converts the durable transcript into provider messages:
// assistant tool calls become tool-call parts and tool results become
// tool-result parts keyed by their call ID.
func modelMessages(messages []domain.ChatMessage) ([]aiprovider.Message, error) {
	result := make([]aiprovider.Message, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case domain.ChatRoleSystem:
			result = append(result, aiprovider.NewSystemMessage(message.Content))
		case domain.ChatRoleUser:
			result = append(result, aiprovider.UserText(message.Content))
		case domain.ChatRoleAssistant:
			parts := []aiprovider.ContentPart{}
			if strings.TrimSpace(message.Content) != "" {
				parts = append(parts, aiprovider.TextPart(message.Content))
			}
			for _, call := range message.ToolCalls {
				arguments, err := json.Marshal(call.Arguments)
				if err != nil {
					return nil, fmt.Errorf("encode tool call %q arguments: %w", call.Name, err)
				}
				parts = append(parts, aiprovider.ContentPart{
					Type:       aiprovider.ContentPartTypeToolCall,
					ToolCallID: call.ID,
					ToolName:   call.Name,
					Input:      arguments,
				})
			}
			if len(parts) == 0 {
				parts = append(parts, aiprovider.TextPart(""))
			}
			result = append(result, aiprovider.NewAssistantMessage(parts...))
		case domain.ChatRoleTool:
			result = append(result, aiprovider.NewToolMessage(aiprovider.ContentPart{
				Type:       aiprovider.ContentPartTypeToolResult,
				ToolCallID: message.ToolCallID,
				ToolName:   message.ToolName,
				Output:     &aiprovider.ToolResultOutput{Type: aiprovider.ToolOutputText, Text: message.Content},
			}))
		default:
			return nil, fmt.Errorf("unsupported chat message role %q", message.Role)
		}
	}
	if len(result) == 0 {
		result = append(result, aiprovider.UserText(""))
	}
	return result, nil
}

// aiToolSet converts native tool definitions into an ai-sdk ToolSet. Tools are
// registered without an Execute function: the caller keeps executing tool
// calls itself (approval flow, persistence, and event emission stay owned by
// the chat service and pipeline engine), so each generate turn returns
// unresolved tool calls to the caller after one model step.
//
// Every input schema is validated through the ai-sdk's JSON Schema compiler,
// which is where a schema violating the draft 2020-12 metaschema — for
// example a null `required` — fails loudly instead of reaching the provider.
func aiToolSet(tools []domain.ChatToolDefinition) (aisdk.ToolSet, error) {
	toolSet := make(aisdk.ToolSet, len(tools))
	for _, tool := range tools {
		data, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("encode tool %q schema: %w", tool.Name, err)
		}
		if len(data) == 0 || string(data) == "null" {
			data = []byte(defaultToolSchema)
		}
		inputSchema, err := schema.SchemaFromJSON(data)
		if err != nil {
			return nil, fmt.Errorf("compile tool %q schema: %w", tool.Name, err)
		}
		toolSet[tool.Name] = aisdk.Tool{Description: tool.Description, InputSchema: inputSchema}
	}
	return toolSet, nil
}

// assistantResponse maps one generate result back onto the assistant turn.
func assistantResponse(result *aisdk.GenerateTextResult) (domain.AssistantChatResponse, error) {
	response := domain.AssistantChatResponse{
		Content:   strings.TrimSpace(result.Text),
		ToolCalls: make([]domain.ChatToolCall, 0, len(result.ToolCalls)),
		Usage:     usageFromResult(result.TotalUsage),
	}
	for _, call := range result.ToolCalls {
		arguments := map[string]any{}
		if input := strings.TrimSpace(string(call.Input)); input != "" {
			if err := json.Unmarshal([]byte(input), &arguments); err != nil {
				return domain.AssistantChatResponse{}, fmt.Errorf("decode tool call %q arguments: %w", call.ToolName, err)
			}
		}
		id := strings.TrimSpace(call.ToolCallID)
		if id == "" {
			id = fmt.Sprintf("tool-%d", len(response.ToolCalls)+1)
		}
		response.ToolCalls = append(response.ToolCalls, domain.ChatToolCall{ID: id, Name: call.ToolName, Arguments: arguments})
	}
	return response, nil
}

// usageFromResult converts ai-sdk usage into the local usage record.
func usageFromResult(usage aiprovider.Usage) domain.LLMUsage {
	var prompt, completion int64
	if usage.InputTokens.Total != nil {
		prompt = int64(*usage.InputTokens.Total)
	}
	if usage.OutputTokens.Total != nil {
		completion = int64(*usage.OutputTokens.Total)
	}
	return usageFromCounts(prompt, completion)
}

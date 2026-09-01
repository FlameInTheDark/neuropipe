package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

// anthropicVersion is the pinned Messages API version sent with every request.
const anthropicVersion = "2023-06-01"

// anthropicMaxTokens is the required completion budget for the Messages API.
// Anthropic requires an explicit cap; 4096 keeps single-answer nodes and agent
// turns generous without unbounded spend. A configured max-tokens override,
// provider-level or per-model, replaces it.
const anthropicMaxTokens = 4096

// anthropicCompletionBudget resolves the required max_tokens value from the
// effective generation parameters.
func anthropicCompletionBudget(params domain.GenerationParameters) int {
	if params.MaxTokens != nil && *params.MaxTokens > 0 {
		return *params.MaxTokens
	}
	return anthropicMaxTokens
}

// applyAnthropicParameters writes configured sampling parameters into a
// Messages API payload. Only explicitly configured fields are sent.
func applyAnthropicParameters(payload map[string]any, params domain.GenerationParameters) {
	if params.Temperature != nil {
		payload["temperature"] = *params.Temperature
	}
	if params.TopP != nil {
		payload["top_p"] = *params.TopP
	}
	if params.TopK != nil {
		payload["top_k"] = *params.TopK
	}
}

// chatAnthropic runs one single-shot Messages API turn for pipeline AI nodes.
// Anthropic has no response_format JSON mode, so structured output relies on
// the same prompt-engineering contract the Ollama adapter uses.
func (m *Manager) chatAnthropic(ctx context.Context, provider domain.ProviderConfig, model string, request pipeline.ChatRequest) (pipeline.ChatResponse, error) {
	payload := map[string]any{
		"model":      model,
		"max_tokens": anthropicCompletionBudget(provider.EffectiveParameters(model)),
		"messages":   []map[string]any{{"role": "user", "content": structuredPrompt(request)}},
	}
	applyAnthropicParameters(payload, provider.EffectiveParameters(model))
	var response anthropicResponse
	if err := m.postJSON(ctx, provider, "/v1/messages", payload, &response); err != nil {
		return pipeline.ChatResponse{}, err
	}
	if message := response.Error.Message; message != "" {
		return pipeline.ChatResponse{}, fmt.Errorf("%s: %s", provider.Name, message)
	}
	result := parseResponse(response.textContent())
	result.Usage = usageFromCounts(response.Usage.InputTokens, response.Usage.OutputTokens)
	return result, nil
}

// converseAnthropic performs one tool-capable assistant turn against the
// Messages API, mapping the provider-neutral transcript to Anthropic content
// blocks and back.
func (m *Manager) converseAnthropic(ctx context.Context, provider domain.ProviderConfig, model string, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	system, messages := anthropicMessages(request.Messages)
	payload := map[string]any{
		"model":      model,
		"max_tokens": anthropicCompletionBudget(provider.EffectiveParameters(model)),
		"messages":   messages,
	}
	applyAnthropicParameters(payload, provider.EffectiveParameters(model))
	if system != "" {
		payload["system"] = system
	}
	if len(request.Tools) > 0 {
		payload["tools"] = anthropicTools(request.Tools)
	}
	var response anthropicResponse
	if err := m.postJSON(ctx, provider, "/v1/messages", payload, &response); err != nil {
		return domain.AssistantChatResponse{}, err
	}
	if message := response.Error.Message; message != "" {
		return domain.AssistantChatResponse{}, fmt.Errorf("%s: %s", provider.Name, message)
	}
	result := domain.AssistantChatResponse{Content: strings.TrimSpace(response.textContent()), ToolCalls: make([]domain.ChatToolCall, 0)}
	for _, block := range response.Content {
		if block.Type != "tool_use" {
			continue
		}
		arguments := map[string]any{}
		if len(block.Input) > 0 {
			arguments = block.Input
		}
		id := strings.TrimSpace(block.ID)
		if id == "" {
			id = fmt.Sprintf("anthropic-tool-%d", len(result.ToolCalls)+1)
		}
		result.ToolCalls = append(result.ToolCalls, domain.ChatToolCall{ID: id, Name: block.Name, Arguments: arguments})
	}
	result.Usage = usageFromCounts(response.Usage.InputTokens, response.Usage.OutputTokens)
	return result, nil
}

// listAnthropic discovers models through the public /v1/models listing.
func (m *Manager) listAnthropic(ctx context.Context, provider domain.ProviderConfig) ([]ModelInfo, error) {
	var response struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
		Error anthropicError `json:"error"`
	}
	if err := m.getJSON(ctx, provider, "/v1/models", &response); err != nil {
		return nil, err
	}
	if response.Error.Message != "" {
		return nil, fmt.Errorf("%s: %s", provider.Name, response.Error.Message)
	}
	result := make([]ModelInfo, 0, len(response.Data))
	for _, model := range response.Data {
		result = append(result, ModelInfo{ID: model.ID, Name: defaultText(model.DisplayName, model.ID)})
	}
	return result, nil
}

// anthropicAuthorize applies Anthropic's header authentication. It is
// selected by provider kind from authorize.
func anthropicAuthorize(request *http.Request, provider domain.ProviderConfig, secrets SecretResolver) error {
	if provider.APIKeyRef == "" {
		request.Header.Set("anthropic-version", anthropicVersion)
		return nil
	}
	if secrets == nil {
		return fmt.Errorf("provider %s requires secret %q", provider.Name, provider.APIKeyRef)
	}
	value, err := secrets.Get(provider.APIKeyRef)
	if err != nil {
		return err
	}
	request.Header.Set("x-api-key", value)
	request.Header.Set("anthropic-version", anthropicVersion)
	return nil
}

// anthropicMessages converts the provider-neutral transcript into Anthropic's
// message shape. A system role becomes the top-level system prompt; tool
// results become user tool_result blocks; assistant tool calls become
// tool_use blocks.
func anthropicMessages(messages []domain.ChatMessage) (string, []map[string]any) {
	system := strings.Builder{}
	result := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case domain.ChatRoleSystem:
			if system.Len() > 0 {
				system.WriteString("\n\n")
			}
			system.WriteString(message.Content)
		case domain.ChatRoleTool:
			block := map[string]any{"type": "tool_result", "tool_use_id": message.ToolCallID, "content": message.Content}
			// Anthropic expects consecutive tool results in one user turn;
			// merge only into a previous user message built from tool results.
			if previous := len(result) - 1; previous >= 0 && result[previous]["role"] == "user" {
				if blocks, ok := result[previous]["content"].([]map[string]any); ok {
					result[previous]["content"] = append(blocks, block)
					continue
				}
			}
			result = append(result, map[string]any{"role": "user", "content": []map[string]any{block}})
		case domain.ChatRoleAssistant:
			content := []map[string]any{}
			if strings.TrimSpace(message.Content) != "" {
				content = append(content, map[string]any{"type": "text", "text": message.Content})
			}
			for _, call := range message.ToolCalls {
				content = append(content, map[string]any{"type": "tool_use", "id": call.ID, "name": call.Name, "input": call.Arguments})
			}
			if len(content) == 0 {
				content = append(content, map[string]any{"type": "text", "text": ""})
			}
			result = append(result, map[string]any{"role": "assistant", "content": content})
		default:
			result = append(result, map[string]any{"role": "user", "content": message.Content})
		}
	}
	if len(result) == 0 {
		result = append(result, map[string]any{"role": "user", "content": ""})
	}
	return system.String(), result
}

// anthropicTools maps native function tools to Anthropic's tool schema.
func anthropicTools(tools []domain.ChatToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		schema := tool.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		result = append(result, map[string]any{"name": tool.Name, "description": tool.Description, "input_schema": schema})
	}
	return result
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type anthropicContentBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Error   anthropicError          `json:"error"`
	Usage   struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

// textContent joins every text block into one string, ignoring tool_use blocks.
func (r anthropicResponse) textContent() string {
	parts := make([]string, 0, len(r.Content))
	for _, block := range r.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

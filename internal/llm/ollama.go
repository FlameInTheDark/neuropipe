package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	aiprovider "github.com/grafana/ai-sdk/provider"
)

// ollamaModel implements aiprovider.LanguageModel over Ollama's native
// /api/chat endpoint. Ollama also exposes an OpenAI-compatible /v1 surface,
// but the native API accepts the `options` object (num_ctx and friends) that
// the OpenAI wire format cannot carry, so provider-level generation settings
// would silently stop applying. DoGenerate performs one blocking request;
// DoStream requests the streaming NDJSON wire so token deltas reach the
// ai-sdk orchestration (and chat UI) as the model produces them.
type ollamaModel struct {
	modelID     string
	provider    string
	baseURL     string
	httpClient  *http.Client
	contextSize int // num_ctx; 0 keeps Ollama's own default
}

// newOllamaModel builds the native Ollama adapter. contextSize comes from the
// provider's effective generation parameters and maps to Ollama's num_ctx.
func newOllamaModel(baseURL, modelID, providerName string, client *http.Client, contextSize int) *ollamaModel {
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(providerName) == "" {
		providerName = "Ollama"
	}
	return &ollamaModel{modelID: modelID, provider: providerName, baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), httpClient: client, contextSize: contextSize}
}

var _ aiprovider.LanguageModel = (*ollamaModel)(nil)

// SpecificationVersion implements aiprovider.LanguageModel.
func (m *ollamaModel) SpecificationVersion() string { return "v4" }

// Provider implements aiprovider.LanguageModel.
func (m *ollamaModel) Provider() string { return m.provider }

// ModelID implements aiprovider.LanguageModel.
func (m *ollamaModel) ModelID() string { return m.modelID }

// SupportedURLs implements aiprovider.LanguageModel; Ollama has no URL fast path.
func (m *ollamaModel) SupportedURLs() map[string][]*regexp.Regexp { return nil }

// ollamaChatMessage mirrors Ollama's chat wire shape: plain string content,
// tool_call_id on tool replies, and OpenAI-style function tool_calls entries
// whose arguments are a JSON string.
type ollamaChatMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolCalls  []ollamaCallOut `json:"tool_calls,omitempty"`
}

type ollamaCallOut struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function ollamaCallFn `json:"function"`
}

type ollamaCallFn struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ollamaChatResponse is the native /api/chat reply: complete message, optional
// tool calls, and token counts on the terminal line.
type ollamaChatResponse struct {
	Message struct {
		Content   string         `json:"content"`
		ToolCalls []ollamaCallIn `json:"tool_calls"`
	} `json:"message"`
	Error           string `json:"error"`
	DoneReason      string `json:"done_reason"`
	PromptEvalCount int64  `json:"prompt_eval_count"`
	EvalCount       int64  `json:"eval_count"`
}

// ollamaStreamChunk is one NDJSON line of a streaming /api/chat response.
// Every line carries done=false while generating; the terminal line sets
// done=true along with done_reason and the token counts.
type ollamaStreamChunk struct {
	Message struct {
		Content   string         `json:"content"`
		ToolCalls []ollamaCallIn `json:"tool_calls"`
	} `json:"message"`
	Error           string `json:"error"`
	Done            bool   `json:"done"`
	DoneReason      string `json:"done_reason"`
	PromptEvalCount int64  `json:"prompt_eval_count"`
	EvalCount       int64  `json:"eval_count"`
}

type ollamaCallIn struct {
	ID       string `json:"id"`
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

// ollamaRequest assembles the native request body from provider call options.
func (m *ollamaModel) ollamaRequest(opts aiprovider.CallOptions) (map[string]any, error) {
	messages, err := ollamaMessages(opts.Prompt)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"model": m.modelID, "messages": messages, "stream": false}
	options := map[string]any{}
	if m.contextSize > 0 {
		options["num_ctx"] = m.contextSize
	}
	if opts.Temperature != nil {
		options["temperature"] = *opts.Temperature
	}
	if opts.TopK != nil {
		options["top_k"] = *opts.TopK
	}
	if opts.TopP != nil {
		options["top_p"] = *opts.TopP
	}
	if opts.MaxOutputTokens != nil {
		options["num_predict"] = *opts.MaxOutputTokens
	}
	if len(options) > 0 {
		body["options"] = options
	}
	if tools := ollamaTools(opts.Tools); len(tools) > 0 {
		body["tools"] = tools
	}
	if opts.ResponseFormat != nil && opts.ResponseFormat.Type == aiprovider.ResponseFormatJSON {
		body["format"] = "json"
	}
	return body, nil
}

// ollamaMessages converts provider messages to the native wire shape. Each
// tool-result part becomes its own tool message because Ollama keys a tool
// reply by tool_call_id.
func ollamaMessages(prompt []aiprovider.Message) ([]ollamaChatMessage, error) {
	result := make([]ollamaChatMessage, 0, len(prompt))
	for _, message := range prompt {
		switch message.Role {
		case aiprovider.RoleSystem:
			result = append(result, ollamaChatMessage{Role: "system", Content: contentText(message.Content)})
		case aiprovider.RoleUser:
			result = append(result, ollamaChatMessage{Role: "user", Content: contentText(message.Content)})
		case aiprovider.RoleAssistant:
			item := ollamaChatMessage{Role: "assistant", Content: contentText(message.Content)}
			for _, part := range message.Content {
				if part.Type != aiprovider.ContentPartTypeToolCall {
					continue
				}
				input := strings.TrimSpace(string(part.Input))
				if input == "" || !json.Valid([]byte(input)) {
					return nil, fmt.Errorf("ollama: tool call %q input is not valid JSON", part.ToolCallID)
				}
				item.ToolCalls = append(item.ToolCalls, ollamaCallOut{
					ID:       part.ToolCallID,
					Type:     "function",
					Function: ollamaCallFn{Name: part.ToolName, Arguments: input},
				})
			}
			result = append(result, item)
		case aiprovider.RoleTool:
			for _, part := range message.Content {
				if part.Type != aiprovider.ContentPartTypeToolResult {
					return nil, fmt.Errorf("ollama: tool messages do not support content part %q", part.Type)
				}
				content, err := ollamaToolResult(part.Output)
				if err != nil {
					return nil, err
				}
				result = append(result, ollamaChatMessage{Role: "tool", Content: content, ToolCallID: part.ToolCallID})
			}
		default:
			return nil, fmt.Errorf("ollama: unsupported message role %q", message.Role)
		}
	}
	if len(result) == 0 {
		result = append(result, ollamaChatMessage{Role: "user"})
	}
	return result, nil
}

// contentText flattens the text parts of one message.
func contentText(parts []aiprovider.ContentPart) string {
	var builder strings.Builder
	for _, part := range parts {
		if part.Type == aiprovider.ContentPartTypeText {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

// ollamaToolResult renders one tool-result output as the string content of a
// native tool message.
func ollamaToolResult(output *aiprovider.ToolResultOutput) (string, error) {
	if output == nil {
		return "null", nil
	}
	switch output.Type {
	case aiprovider.ToolOutputText, aiprovider.ToolOutputErrorText:
		return output.Text, nil
	case aiprovider.ToolOutputJSON, aiprovider.ToolOutputErrorJSON:
		if len(output.JSON) == 0 {
			return "null", nil
		}
		return string(output.JSON), nil
	case aiprovider.ToolOutputExecutionDenied:
		if output.Reason != "" {
			return output.Reason, nil
		}
		return "Tool call execution denied.", nil
	default:
		data, err := json.Marshal(output)
		if err != nil {
			return "", fmt.Errorf("ollama: encoding tool result: %w", err)
		}
		return string(data), nil
	}
}

// ollamaTools maps provider function tools to the OpenAI-style tool entries
// the native chat API accepts.
func ollamaTools(tools []aiprovider.Tool) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if tool.Type == aiprovider.ToolTypeProvider {
			continue
		}
		parameters := tool.InputSchema
		if len(parameters) == 0 || string(parameters) == "null" {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		result = append(result, map[string]any{
			"type":     "function",
			"function": map[string]any{"name": tool.Name, "description": tool.Description, "parameters": parameters},
		})
	}
	return result
}

// chat performs one blocking /api/chat request and decodes the native reply.
func (m *ollamaModel) chat(ctx context.Context, opts aiprovider.CallOptions) (*ollamaChatResponse, error) {
	body, err := m.ollamaRequest(opts)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode provider request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := m.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", m.provider, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		return nil, fmt.Errorf("%s returned %s: %s", m.provider, response.Status, strings.TrimSpace(string(detail)))
	}
	var decoded ollamaChatResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", m.provider, err)
	}
	if strings.TrimSpace(decoded.Error) != "" {
		return nil, fmt.Errorf("%s: %s", m.provider, decoded.Error)
	}
	return &decoded, nil
}

// ollamaFinish maps Ollama's done_reason to the unified finish reason.
func ollamaFinish(reason string, toolCalls int) aiprovider.FinishReason {
	if toolCalls > 0 {
		return aiprovider.FinishReason{Unified: aiprovider.FinishReasonToolCalls, Raw: reason}
	}
	switch reason {
	case "", "stop":
		return aiprovider.FinishReason{Unified: aiprovider.FinishReasonStop, Raw: reason}
	case "length":
		return aiprovider.FinishReason{Unified: aiprovider.FinishReasonLength, Raw: reason}
	default:
		return aiprovider.FinishReason{Unified: aiprovider.FinishReasonOther, Raw: reason}
	}
}

// ollamaUsage converts the native token counts.
func ollamaUsage(prompt, completion int64) aiprovider.Usage {
	usage := aiprovider.Usage{}
	if prompt > 0 {
		total := int(prompt)
		usage.InputTokens.Total = &total
	}
	if completion > 0 {
		total := int(completion)
		usage.OutputTokens.Total = &total
	}
	return usage
}

// ollamaCalls converts native tool calls into provider content parts,
// synthesizing stable IDs when Ollama omits them, exactly like the previous
// manager implementation.
func ollamaCalls(response *ollamaChatResponse) ([]aiprovider.GenerateContentPart, error) {
	parts := make([]aiprovider.GenerateContentPart, 0, len(response.Message.ToolCalls))
	for index, call := range response.Message.ToolCalls {
		id := strings.TrimSpace(call.ID)
		if id == "" {
			id = fmt.Sprintf("ollama-tool-%d", index+1)
		}
		arguments := call.Function.Arguments
		if arguments == nil {
			arguments = map[string]any{}
		}
		input, err := json.Marshal(arguments)
		if err != nil {
			return nil, fmt.Errorf("encode tool call %q arguments: %w", call.Function.Name, err)
		}
		parts = append(parts, aiprovider.GenerateContentPart{
			Type:       aiprovider.ContentToolCall,
			ToolCallID: id,
			ToolName:   call.Function.Name,
			Input:      input,
		})
	}
	return parts, nil
}

// DoGenerate implements aiprovider.LanguageModel with one blocking request.
func (m *ollamaModel) DoGenerate(ctx context.Context, opts aiprovider.CallOptions) (*aiprovider.GenerateResult, error) {
	response, err := m.chat(ctx, opts)
	if err != nil {
		return nil, err
	}
	toolParts, err := ollamaCalls(response)
	if err != nil {
		return nil, err
	}
	content := make([]aiprovider.GenerateContentPart, 0, len(toolParts)+1)
	if strings.TrimSpace(response.Message.Content) != "" {
		content = append(content, aiprovider.GenerateContentPart{Type: aiprovider.ContentText, Text: response.Message.Content})
	}
	content = append(content, toolParts...)
	return &aiprovider.GenerateResult{
		Content:      content,
		FinishReason: ollamaFinish(response.DoneReason, len(toolParts)),
		Usage:        ollamaUsage(response.PromptEvalCount, response.EvalCount),
	}, nil
}

// DoStream implements aiprovider.LanguageModel over Ollama's streaming
// NDJSON /api/chat wire. Each response line is one JSON chunk; text deltas
// are forwarded the moment they arrive, while tool calls, the finish reason,
// and token counts only exist on the terminal line and are emitted after it.
// Ollama never splits one tool call across lines, so buffering calls until the
// terminal line is lossless.
func (m *ollamaModel) DoStream(ctx context.Context, opts aiprovider.CallOptions) (*aiprovider.StreamResult, error) {
	body, err := m.ollamaRequest(opts)
	if err != nil {
		return nil, err
	}
	body["stream"] = true
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode provider request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := m.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", m.provider, err)
	}
	if response.StatusCode/100 != 2 {
		defer func() { _ = response.Body.Close() }()
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		return nil, fmt.Errorf("%s returned %s: %s", m.provider, response.Status, strings.TrimSpace(string(detail)))
	}

	stream := make(chan aiprovider.StreamPart, 32)
	go func() {
		defer func() { _ = response.Body.Close() }()
		defer close(stream)
		send := func(part aiprovider.StreamPart) bool {
			select {
			case <-ctx.Done():
				return false
			case stream <- part:
				return true
			}
		}
		if !send(aiprovider.StreamPart{Type: aiprovider.PartStreamStart}) {
			return
		}
		if !send(aiprovider.StreamPart{Type: aiprovider.PartResponseMeta, ModelID: m.modelID, Provider: m.provider}) {
			return
		}

		var (
			textOpen    bool
			toolParts   []aiprovider.GenerateContentPart
			finish      aiprovider.FinishReason
			usage       aiprovider.Usage
			streamError error
		)
		fail := func(message string) {
			streamError = fmt.Errorf("%s: %s", m.provider, message)
		}
		reader := bufio.NewReader(response.Body)
		for streamError == nil {
			line, readErr := reader.ReadString('\n')
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				var chunk ollamaStreamChunk
				if err := json.Unmarshal([]byte(trimmed), &chunk); err != nil {
					fail(fmt.Sprintf("decode stream line: %v", err))
					break
				}
				if strings.TrimSpace(chunk.Error) != "" {
					fail(chunk.Error)
					break
				}
				if chunk.Message.Content != "" {
					if !textOpen {
						if !send(aiprovider.StreamPart{Type: aiprovider.PartTextStart, ID: "txt-0"}) {
							return
						}
						textOpen = true
					}
					if !send(aiprovider.StreamPart{Type: aiprovider.PartTextDelta, ID: "txt-0", Delta: chunk.Message.Content}) {
						return
					}
				}
				if len(chunk.Message.ToolCalls) > 0 {
					var accumulated ollamaChatResponse
					accumulated.Message.ToolCalls = chunk.Message.ToolCalls
					parts, callErr := ollamaCalls(&accumulated)
					if callErr != nil {
						fail(callErr.Error())
						break
					}
					toolParts = append(toolParts, parts...)
				}
				if chunk.Done {
					finish = ollamaFinish(chunk.DoneReason, len(toolParts))
					usage = ollamaUsage(chunk.PromptEvalCount, chunk.EvalCount)
					break
				}
			}
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) {
					fail(fmt.Sprintf("read stream: %v", readErr))
				}
				// A stream that ends without a done line is truncated; the ai-sdk
				// turns the missing finish chunk into an explicit error, so simply
				// stop emitting here.
				break
			}
		}
		if streamError != nil {
			// StatusCode 0 keeps the error non-retryable, matching the
			// blocking path where mid-body failures surface as plain errors.
			_ = send(aiprovider.StreamPart{Type: aiprovider.PartError, APICallError: aiprovider.NewAPICallError(aiprovider.APICallErrorOptions{Message: streamError.Error(), URL: request.URL.String()})})
			return
		}
		if textOpen {
			if !send(aiprovider.StreamPart{Type: aiprovider.PartTextEnd, ID: "txt-0"}) {
				return
			}
		}
		for _, part := range toolParts {
			if part.Type != aiprovider.ContentToolCall {
				continue
			}
			if !send(aiprovider.StreamPart{Type: aiprovider.PartToolInputStart, ID: part.ToolCallID, ToolCallID: part.ToolCallID, ToolName: part.ToolName}) {
				return
			}
			if !send(aiprovider.StreamPart{Type: aiprovider.PartToolInputDelta, ID: part.ToolCallID, ToolCallID: part.ToolCallID, ToolName: part.ToolName, Delta: string(part.Input)}) {
				return
			}
			if !send(aiprovider.StreamPart{Type: aiprovider.PartToolInputEnd, ID: part.ToolCallID, ToolCallID: part.ToolCallID, ToolName: part.ToolName}) {
				return
			}
			if !send(aiprovider.StreamPart{Type: aiprovider.PartToolCall, ToolCallID: part.ToolCallID, ToolName: part.ToolName, Input: string(part.Input)}) {
				return
			}
		}
		_ = send(aiprovider.StreamPart{Type: aiprovider.PartFinish, FinishReason: &finish, Usage: &usage})
	}()
	return &aiprovider.StreamResult{Stream: stream}, nil
}

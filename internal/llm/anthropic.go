package llm

import (
	"context"
	"fmt"
	"net/http"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
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

// anthropicError is the error envelope of the /v1/models listing.
type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
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

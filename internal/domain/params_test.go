package domain

import (
	"reflect"
	"testing"
)

func floatPointer(value float64) *float64 { return &value }
func intPointer(value int) *int           { return &value }

func TestEffectiveParameters(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderConfig
		model    string
		want     GenerationParameters
	}{
		{
			name:     "no configuration anywhere resolves to all unset",
			provider: ProviderConfig{ID: "p", Kind: ProviderOpenAICompatible},
			model:    "m",
			want:     GenerationParameters{},
		},
		{
			name: "provider values apply when the model has none",
			provider: ProviderConfig{
				ID:         "p",
				Kind:       ProviderOpenAICompatible,
				Parameters: &GenerationParameters{Temperature: floatPointer(0.7), MaxTokens: intPointer(512)},
			},
			model: "m",
			want:  GenerationParameters{Temperature: floatPointer(0.7), MaxTokens: intPointer(512)},
		},
		{
			name: "model overrides win field by field",
			provider: ProviderConfig{
				ID:         "p",
				Kind:       ProviderOpenAICompatible,
				Parameters: &GenerationParameters{Temperature: floatPointer(0.7), TopP: floatPointer(0.9), MaxTokens: intPointer(512)},
				Models: []ModelConfig{{
					ID:         "big-context",
					Parameters: &GenerationParameters{TopP: floatPointer(0.5), ContextSize: intPointer(32768)},
				}},
			},
			model: "big-context",
			want:  GenerationParameters{Temperature: floatPointer(0.7), TopP: floatPointer(0.5), MaxTokens: intPointer(512), ContextSize: intPointer(32768)},
		},
		{
			name: "model matching is case-insensitive and ignores surrounding spaces",
			provider: ProviderConfig{
				ID:     "p",
				Kind:   ProviderOpenAICompatible,
				Models: []ModelConfig{{ID: "Qwen2.5-7B", Parameters: &GenerationParameters{TopK: intPointer(40)}}},
			},
			model: " qwen2.5-7b ",
			want:  GenerationParameters{TopK: intPointer(40)},
		},
		{
			name: "unrelated models do not contribute overrides",
			provider: ProviderConfig{
				ID:     "p",
				Kind:   ProviderOpenAICompatible,
				Models: []ModelConfig{{ID: "other", Parameters: &GenerationParameters{TopK: intPointer(40)}}},
			},
			model: "m",
			want:  GenerationParameters{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.provider.EffectiveParameters(test.model)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("EffectiveParameters(%q) = %#v, want %#v", test.model, got, test.want)
			}
		})
	}
}

func TestEffectiveParametersEmptyModel(t *testing.T) {
	provider := ProviderConfig{
		ID:         "p",
		Kind:       ProviderOllama,
		Parameters: &GenerationParameters{Temperature: floatPointer(1.1)},
	}
	if got := provider.EffectiveParameters(""); !reflect.DeepEqual(got, GenerationParameters{Temperature: floatPointer(1.1)}) {
		t.Fatalf("EffectiveParameters(\"\") = %#v, want provider values alone", got)
	}
}

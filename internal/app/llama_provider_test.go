package app

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestActivateManagedLlamaProvider(t *testing.T) {
	tests := []struct {
		name      string
		settings  domain.Settings
		wantModel string
		wantURL   string
	}{
		{
			name: "replaces Ollama with the managed provider",
			settings: domain.Settings{Providers: []domain.ProviderConfig{{
				ID: "ollama-local", Name: "Local Ollama", Kind: domain.ProviderOllama, BaseURL: "http://127.0.0.1:11434", Enabled: true,
			}}},
			wantModel: "qwen.gguf",
			wantURL:   "http://127.0.0.1:49999",
		},
		{
			name: "replaces an external provider configuration",
			settings: domain.Settings{Providers: []domain.ProviderConfig{
				{ID: "custom-llama", Name: "External llama.cpp", Kind: domain.ProviderLlamaCPP, BaseURL: "http://127.0.0.1:8080", Model: "external.gguf", Enabled: true},
				{ID: managedLlamaProviderID, Name: "Managed llama.cpp", Kind: domain.ProviderLlamaCPP, BaseURL: "http://127.0.0.1:40000", Model: "old.gguf"},
			}},
			wantModel: "qwen.gguf",
			wantURL:   "http://127.0.0.1:49999",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := test.settings
			activateManagedLlamaProvider(&settings, "qwen.gguf", "http://127.0.0.1:49999")

			if settings.DefaultProviderID != managedLlamaProviderID {
				t.Fatalf("default provider = %q, want %q", settings.DefaultProviderID, managedLlamaProviderID)
			}
			if len(settings.Providers) != 1 {
				t.Fatalf("provider count = %d, want one", len(settings.Providers))
			}
			managed := settings.Providers[0]
			if managed.ID != managedLlamaProviderID || managed.Model != test.wantModel || managed.BaseURL != test.wantURL || !managed.Enabled {
				t.Fatalf("managed provider = %#v, want enabled model %q at %q", managed, test.wantModel, test.wantURL)
			}
		})
	}
}

func TestNormalizeConfiguredProvider(t *testing.T) {
	tests := []struct {
		name       string
		settings   domain.Settings
		wantID     string
		wantKind   domain.ProviderKind
		wantURL    string
		wantSecret string
	}{
		{
			name:       "keeps the selected OpenAI compatible configuration",
			settings:   domain.Settings{DefaultProviderID: "custom", Providers: []domain.ProviderConfig{{ID: "custom", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://example.test/v1", APIKeyRef: "api-key"}, {ID: "old", Kind: domain.ProviderOllama}}},
			wantID:     "openai-compatible",
			wantKind:   domain.ProviderOpenAICompatible,
			wantURL:    "https://example.test/v1",
			wantSecret: "api-key",
		},
		{
			name:     "uses a safe Ollama default when no provider exists",
			settings: domain.Settings{},
			wantID:   "ollama-local",
			wantKind: domain.ProviderOllama,
			wantURL:  "http://127.0.0.1:11434",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := test.settings
			if err := normalizeConfiguredProvider(&settings); err != nil {
				t.Fatalf("normalizeConfiguredProvider() error = %v", err)
			}
			if len(settings.Providers) != 1 {
				t.Fatalf("provider count = %d, want one", len(settings.Providers))
			}
			provider := settings.Providers[0]
			if provider.ID != test.wantID || provider.Kind != test.wantKind || provider.BaseURL != test.wantURL || provider.APIKeyRef != test.wantSecret {
				t.Fatalf("provider = %#v, want id=%q kind=%q url=%q secret=%q", provider, test.wantID, test.wantKind, test.wantURL, test.wantSecret)
			}
			if settings.DefaultProviderID != provider.ID {
				t.Fatalf("default provider = %q, want %q", settings.DefaultProviderID, provider.ID)
			}
		})
	}
}

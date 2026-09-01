package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	localruntime "github.com/FlameInTheDark/neuropipe/internal/runtime"
)

func TestActivateManagedLlamaProvider(t *testing.T) {
	tests := []struct {
		name         string
		settings     domain.Settings
		wantModel    string
		wantURL      string
		wantKept     []string
		wantProvider int
	}{
		{
			name: "replaces an existing managed provider and keeps the others",
			settings: domain.Settings{Providers: []domain.ProviderConfig{
				{ID: "openrouter", Name: "OpenRouter", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Model: "m", Enabled: true},
				{ID: managedLlamaProviderID, Name: "Managed llama.cpp", Kind: domain.ProviderLlamaCPP, BaseURL: "http://127.0.0.1:40000", Model: "old.gguf", Models: []domain.ModelConfig{{ID: "old.gguf"}, {ID: "backup.gguf"}}},
			}},
			wantModel:    "qwen.gguf",
			wantURL:      "http://127.0.0.1:49999",
			wantKept:     []string{"openrouter", managedLlamaProviderID},
			wantProvider: 2,
		},
		{
			name:      "inserts the managed provider next to existing ones",
			settings:  domain.Settings{Providers: []domain.ProviderConfig{{ID: "ollama-local", Name: "Local Ollama", Kind: domain.ProviderOllama, BaseURL: "http://127.0.0.1:11434", Enabled: true}}},
			wantModel: "qwen.gguf",
			wantURL:   "http://127.0.0.1:49999",
			wantKept:  []string{"ollama-local", managedLlamaProviderID},
		},
		{
			name:      "seeds the managed provider into an empty list",
			settings:  domain.Settings{},
			wantModel: "qwen.gguf",
			wantURL:   "http://127.0.0.1:49999",
			wantKept:  []string{managedLlamaProviderID},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := test.settings
			activateManagedLlamaProvider(&settings, "qwen.gguf", "http://127.0.0.1:49999")

			if settings.DefaultProviderID != managedLlamaProviderID {
				t.Fatalf("default provider = %q, want %q", settings.DefaultProviderID, managedLlamaProviderID)
			}
			if want := test.wantProvider; want != 0 && len(settings.Providers) != want {
				t.Fatalf("provider count = %d, want %d", len(settings.Providers), want)
			}
			for _, id := range test.wantKept {
				found := false
				for _, provider := range settings.Providers {
					if provider.ID == id {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("provider %q was dropped by activation", id)
				}
			}
			var managed *domain.ProviderConfig
			for index, provider := range settings.Providers {
				if provider.ID == managedLlamaProviderID {
					managed = &settings.Providers[index]
					break
				}
			}
			if managed == nil {
				t.Fatalf("managed provider missing after activation: %#v", settings.Providers)
			}
			if managed.Model != test.wantModel || managed.BaseURL != test.wantURL || !managed.Enabled {
				t.Fatalf("managed provider = %#v, want enabled model %q at %q", managed, test.wantModel, test.wantURL)
			}
		})
	}
}

func TestActivateManagedLlamaProviderKeepsDiscoveredModels(t *testing.T) {
	settings := domain.Settings{Providers: []domain.ProviderConfig{
		{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, Model: "old.gguf", Models: []domain.ModelConfig{{ID: "old.gguf"}, {ID: "backup.gguf"}}},
	}}
	activateManagedLlamaProvider(&settings, "new.gguf", "http://127.0.0.1:49999")
	if len(settings.Providers) != 1 || len(settings.Providers[0].Models) != 2 {
		t.Fatalf("managed model list = %#v, want the two discovered models kept", settings.Providers)
	}
}

func TestBindManagedLlamaProviderKeepsDefault(t *testing.T) {
	settings := domain.Settings{DefaultProviderID: "router", Providers: []domain.ProviderConfig{
		{ID: "router", Name: "OpenRouter", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Enabled: true},
		{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, BaseURL: "http://127.0.0.1:40000", Model: "old.gguf", Models: []domain.ModelConfig{{ID: "old.gguf"}}},
	}}

	bindManagedLlamaProvider(&settings, "new.gguf", "http://127.0.0.1:49999")

	if settings.DefaultProviderID != "router" {
		t.Fatalf("default provider = %q, want the explicit user choice kept", settings.DefaultProviderID)
	}
	if len(settings.Providers) != 2 {
		t.Fatalf("provider count = %d, want 2", len(settings.Providers))
	}
	managed := settings.Providers[1]
	if managed.Model != "new.gguf" || managed.BaseURL != "http://127.0.0.1:49999" || !managed.Enabled {
		t.Fatalf("managed provider = %#v, want the fresh binding", managed)
	}
	if len(managed.Models) != 1 || managed.Models[0].ID != "old.gguf" {
		t.Fatalf("managed model list = %#v, want it preserved across bindings", managed.Models)
	}
}

func TestBindManagedLlamaProviderInsertsWhenMissing(t *testing.T) {
	settings := domain.Settings{DefaultProviderID: "ollama-local", Providers: []domain.ProviderConfig{
		{ID: "ollama-local", Kind: domain.ProviderOllama, BaseURL: "http://127.0.0.1:11434", Enabled: true},
	}}

	bindManagedLlamaProvider(&settings, "qwen.gguf", "http://127.0.0.1:49999")

	if settings.DefaultProviderID != "ollama-local" {
		t.Fatalf("default provider = %q, want it unchanged", settings.DefaultProviderID)
	}
	if len(settings.Providers) != 2 || settings.Providers[1].ID != managedLlamaProviderID {
		t.Fatalf("providers = %#v, want the managed entry appended", settings.Providers)
	}
}

func TestSyncManagedLlamaModels(t *testing.T) {
	tests := []struct {
		name              string
		settings          domain.Settings
		files             []domain.LocalModel
		wantModels        []string
		wantProviderDelta int
	}{
		{
			name: "rebuilds the list from installed files",
			settings: domain.Settings{Providers: []domain.ProviderConfig{
				{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, Model: "a.gguf", Models: []domain.ModelConfig{{ID: "stale.gguf"}}},
			}},
			// InstalledFiles returns a name-sorted list; sync keeps that order.
			files:      []domain.LocalModel{{Name: "a.gguf"}, {Name: "b.gguf"}},
			wantModels: []string{"a.gguf", "b.gguf"},
		},
		{
			name: "keeps an uninstalled default model selectable",
			settings: domain.Settings{Providers: []domain.ProviderConfig{
				{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, Model: "gone.gguf"},
			}},
			files:      []domain.LocalModel{{Name: "a.gguf"}},
			wantModels: []string{"a.gguf", "gone.gguf"},
		},
		{
			name: "materializes the managed provider when models exist",
			settings: domain.Settings{Providers: []domain.ProviderConfig{
				{ID: "router", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Models: []domain.ModelConfig{{ID: "m"}}},
			}},
			files:             []domain.LocalModel{{Name: "a.gguf"}},
			wantModels:        []string{"m", "a.gguf"},
			wantProviderDelta: 1,
		},
		{
			name: "stays hidden after an explicit removal",
			settings: domain.Settings{ManagedLlamaRemoved: true, Providers: []domain.ProviderConfig{
				{ID: "router", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Models: []domain.ModelConfig{{ID: "m"}}},
			}},
			files:      []domain.LocalModel{{Name: "a.gguf"}},
			wantModels: []string{"m"},
		},
		{
			name: "nothing to serve without installed models",
			settings: domain.Settings{Providers: []domain.ProviderConfig{
				{ID: "router", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Models: []domain.ModelConfig{{ID: "m"}}},
			}},
			files:      nil,
			wantModels: []string{"m"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shared := append([]domain.ProviderConfig(nil), test.settings.Providers...)
			test.settings.Providers = shared
			syncManagedLlamaModels(&test.settings, test.files)

			/* wantModels is the flat model list across every provider, in
			 * order: only the managed provider's list changes, but the
			 * assertion style keeps multi-provider cases honest. */
			got := make([]string, 0)
			for _, provider := range test.settings.Providers {
				for _, model := range provider.Models {
					got = append(got, model.ID)
				}
			}
			if strings.Join(got, ",") != strings.Join(test.wantModels, ",") {
				t.Fatalf("models = %v, want %v", got, test.wantModels)
			}
			wantProviders := len(shared) + test.wantProviderDelta
			if len(test.settings.Providers) != wantProviders {
				t.Fatalf("provider count = %d, want %d", len(test.settings.Providers), wantProviders)
			}
		})
	}
}

func TestSyncManagedLlamaModelsDoesNotMutateSharedProviders(t *testing.T) {
	shared := []domain.ProviderConfig{
		{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, Model: "a.gguf", Models: []domain.ModelConfig{{ID: "stale.gguf"}}},
	}
	settings := domain.Settings{Providers: shared}

	syncManagedLlamaModels(&settings, []domain.LocalModel{{Name: "fresh.gguf"}})

	if len(shared[0].Models) != 1 || shared[0].Models[0].ID != "stale.gguf" {
		t.Fatalf("shared provider slice was mutated: %#v", shared)
	}
	if len(settings.Providers[0].Models) != 2 {
		t.Fatalf("synced models = %#v, want the fresh file plus the kept default", settings.Providers[0].Models)
	}
}

func TestResolveManagedLlamaModel(t *testing.T) {
	files := []domain.LocalModel{{Name: "Qwen2.5-7B.gguf", Path: "/models/qwen/Qwen2.5-7B.gguf"}, {Name: "mistral.gguf", Path: "/models/mistral.gguf"}}

	if file, ok := resolveManagedLlamaModel(files, "mistral.gguf"); !ok || file.Path != "/models/mistral.gguf" {
		t.Fatalf("resolveManagedLlamaModel(exact) = %#v, %v", file, ok)
	}
	if file, ok := resolveManagedLlamaModel(files, "qwen2.5-7b.gguf"); !ok || file.Path != "/models/qwen/Qwen2.5-7B.gguf" {
		t.Fatalf("resolveManagedLlamaModel(case-insensitive) = %#v, %v", file, ok)
	}
	if _, ok := resolveManagedLlamaModel(files, "llama.gguf"); ok {
		t.Fatalf("resolveManagedLlamaModel(missing) = ok, want not found")
	}
}

func TestListProviderModelsManagedListsInstalledFiles(t *testing.T) {
	modelsDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(modelsDirectory, "Qwen2.5-7B.gguf"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write model file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelsDirectory, "mistral.gguf"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write model file: %v", err)
	}

	desktop := &Desktop{
		models: localruntime.NewModelCatalog(modelsDirectory),
		settings: domain.Settings{Providers: []domain.ProviderConfig{
			{ID: "router", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Enabled: true},
			{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, Enabled: true},
		}},
	}

	models, err := desktop.ListProviderModels(managedLlamaProviderID)
	if err != nil {
		t.Fatalf("ListProviderModels() error = %v", err)
	}
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	if strings.Join(ids, ",") != "Qwen2.5-7B.gguf,mistral.gguf" {
		t.Fatalf("managed model discovery = %v, want the installed GGUF files", ids)
	}
}

func TestNormalizeConfiguredProviders(t *testing.T) {
	tests := []struct {
		name          string
		settings      domain.Settings
		wantIDs       []string
		wantDefault   string
		wantURLs      map[string]string
		wantErrorText string
	}{
		{
			name: "keeps every configured provider",
			settings: domain.Settings{DefaultProviderID: "router", Providers: []domain.ProviderConfig{
				{ID: "router", Name: "OpenRouter", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Model: "m", Enabled: true},
				{ID: "ollama-local", Kind: domain.ProviderOllama, BaseURL: "http://127.0.0.1:11434", Enabled: true},
				{ID: "claude", Name: "Claude", Kind: domain.ProviderAnthropic, APIKeyRef: "anthropic-key", Enabled: false},
			}},
			wantIDs:     []string{"router", "ollama-local", "claude"},
			wantDefault: "router",
			wantURLs:    map[string]string{"router": "https://openrouter.ai/api/v1", "ollama-local": "http://127.0.0.1:11434", "claude": "https://api.anthropic.com"},
		},
		{
			name:        "seeds a safe Ollama default when no provider exists",
			settings:    domain.Settings{},
			wantIDs:     []string{"ollama-local"},
			wantDefault: "ollama-local",
			wantURLs:    map[string]string{"ollama-local": "http://127.0.0.1:11434"},
		},
		{
			name: "restores a default that no longer exists to the first enabled provider",
			settings: domain.Settings{DefaultProviderID: "removed", Providers: []domain.ProviderConfig{
				{ID: "a", Name: "Disabled", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://a.test/v1", Enabled: false},
				{ID: "b", Name: "Live", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://b.test/v1", Enabled: true},
			}},
			wantIDs:     []string{"a", "b"},
			wantDefault: "b",
		},
		{
			name: "keeps the default pointing at a configured but disabled provider",
			settings: domain.Settings{DefaultProviderID: "paused", Providers: []domain.ProviderConfig{
				{ID: "paused", Name: "Paused", Kind: domain.ProviderOllama, Enabled: false},
				{ID: "live", Name: "Live", Kind: domain.ProviderOllama, Enabled: true},
			}},
			wantIDs:     []string{"paused", "live"},
			wantDefault: "paused",
		},
		{
			name: "deduplicates reused provider ids",
			settings: domain.Settings{Providers: []domain.ProviderConfig{
				{ID: "same", Name: "First", Kind: domain.ProviderOllama, Enabled: true},
				{ID: "same", Name: "Second", Kind: domain.ProviderOllama, Enabled: true},
			}},
			wantIDs:     []string{"same", "same-2"},
			wantDefault: "same",
		},
		{
			name: "normalises a legacy single-provider configuration",
			settings: domain.Settings{DefaultProviderID: "custom", Providers: []domain.ProviderConfig{
				{ID: "custom", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://example.test/v1", APIKeyRef: "api-key", Enabled: true},
			}},
			wantIDs:     []string{"custom"},
			wantDefault: "custom",
			wantURLs:    map[string]string{"custom": "https://example.test/v1"},
		},
		{
			name: "collapses a duplicated managed llama.cpp entry",
			settings: domain.Settings{Providers: []domain.ProviderConfig{
				{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP},
				{ID: "extra-llama", Kind: domain.ProviderLlamaCPP, BaseURL: "http://127.0.0.1:8080"},
			}},
			wantErrorText: "only one managed llama.cpp provider",
		},
		{
			name: "rejects an unsupported provider kind",
			settings: domain.Settings{Providers: []domain.ProviderConfig{
				{ID: "mystery", Kind: domain.ProviderKind("mystery")},
			}},
			wantErrorText: `unsupported LLM provider kind "mystery"`,
		},
		{
			name: "rejects an OpenAI-compatible provider without a base URL",
			settings: domain.Settings{Providers: []domain.ProviderConfig{
				{ID: "no-url", Name: "No URL", Kind: domain.ProviderOpenAICompatible},
			}},
			wantErrorText: "requires a base URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := test.settings
			err := normalizeConfiguredProviders(&settings)
			if test.wantErrorText != "" {
				if err == nil {
					t.Fatalf("normalizeConfiguredProviders() error = nil, want %q", test.wantErrorText)
				}
				if !strings.Contains(err.Error(), test.wantErrorText) {
					t.Fatalf("normalizeConfiguredProviders() error = %v, want %q", err, test.wantErrorText)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeConfiguredProviders() error = %v", err)
			}
			if len(settings.Providers) != len(test.wantIDs) {
				t.Fatalf("provider ids = %#v, want %#v", providerIDs(settings.Providers), test.wantIDs)
			}
			for index, id := range test.wantIDs {
				if settings.Providers[index].ID != id {
					t.Fatalf("provider %d id = %q, want %q", index, settings.Providers[index].ID, id)
				}
			}
			if settings.DefaultProviderID != test.wantDefault {
				t.Fatalf("default provider = %q, want %q", settings.DefaultProviderID, test.wantDefault)
			}
			for id, url := range test.wantURLs {
				for _, provider := range settings.Providers {
					if provider.ID == id && provider.BaseURL != url {
						t.Fatalf("provider %q base URL = %q, want %q", id, provider.BaseURL, url)
					}
				}
			}
		})
	}
}

func TestNormalizeConfiguredProvidersModelList(t *testing.T) {
	settings := domain.Settings{Providers: []domain.ProviderConfig{
		{ID: "claude", Kind: domain.ProviderAnthropic, BaseURL: "https://api.anthropic.com", Enabled: true,
			Models: []domain.ModelConfig{
				{ID: "  ", Name: "blank"},
				{ID: "claude-sonnet-4-5", Name: "Sonnet"},
				{ID: "claude-sonnet-4-5", Name: "Duplicate"},
				{ID: "claude-opus-4-1", Name: "  Opus  "},
			}},
	}}
	if err := normalizeConfiguredProviders(&settings); err != nil {
		t.Fatalf("normalizeConfiguredProviders() error = %v", err)
	}
	models := settings.Providers[0].Models
	if len(models) != 2 {
		t.Fatalf("model list = %#v, want two entries", models)
	}
	if models[0].ID != "claude-sonnet-4-5" || models[0].Name != "Sonnet" {
		t.Fatalf("first model = %#v", models[0])
	}
	if models[1].ID != "claude-opus-4-1" || models[1].Name != "Opus" {
		t.Fatalf("second model = %#v", models[1])
	}
}

func providerIDs(providers []domain.ProviderConfig) []string {
	ids := make([]string, 0, len(providers))
	for _, provider := range providers {
		ids = append(ids, provider.ID)
	}
	return ids
}

func TestSyncManagedLlamaModelsPreservesModelParameters(t *testing.T) {
	contextSize, maxTokens := 32768, 512
	settings := domain.Settings{Providers: []domain.ProviderConfig{
		{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, Model: "big.gguf", Parameters: &domain.GenerationParameters{Temperature: func() *float64 { v := 0.4; return &v }()},
			Models: []domain.ModelConfig{
				{ID: "big.gguf", Parameters: &domain.GenerationParameters{ContextSize: &contextSize}},
				{ID: "gone.gguf", Parameters: &domain.GenerationParameters{MaxTokens: &maxTokens}},
			}},
	}}

	syncManagedLlamaModels(&settings, []domain.LocalModel{{Name: "big.gguf"}, {Name: "small.gguf"}})

	provider := settings.Providers[0]
	if provider.Parameters == nil || provider.Parameters.Temperature == nil || *provider.Parameters.Temperature != 0.4 {
		t.Fatalf("provider-level parameters were dropped by the sync: %#v", provider.Parameters)
	}
	var big, gone, small *domain.ModelConfig
	for index := range provider.Models {
		switch provider.Models[index].ID {
		case "big.gguf":
			big = &provider.Models[index]
		case "gone.gguf":
			gone = &provider.Models[index]
		case "small.gguf":
			small = &provider.Models[index]
		}
	}
	if big == nil || big.Parameters == nil || big.Parameters.ContextSize == nil || *big.Parameters.ContextSize != 32768 {
		t.Fatalf("installed model override lost: %#v", big)
	}
	if gone == nil || gone.Parameters == nil || gone.Parameters.MaxTokens == nil || *gone.Parameters.MaxTokens != 512 {
		t.Fatalf("uninstalled default override lost: %#v", gone)
	}
	if small == nil || small.Parameters != nil {
		t.Fatalf("fresh model should carry no overrides: %#v", small)
	}
}

func TestBindManagedLlamaProviderKeepsParametersAndClearsRemoval(t *testing.T) {
	temperature := 0.6
	settings := domain.Settings{
		ManagedLlamaRemoved: true,
		DefaultProviderID:   "router",
		Providers: []domain.ProviderConfig{
			{ID: "router", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Enabled: true},
			{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, Model: "old.gguf", Enabled: true,
				Parameters: &domain.GenerationParameters{Temperature: &temperature},
				Models:     []domain.ModelConfig{{ID: "old.gguf"}}},
		},
	}

	bindManagedLlamaProvider(&settings, "new.gguf", "http://127.0.0.1:40000")

	if settings.DefaultProviderID != "router" {
		t.Fatalf("default provider = %q, want the bind to leave it alone", settings.DefaultProviderID)
	}
	if settings.ManagedLlamaRemoved {
		t.Fatal("bind should clear the removal marker")
	}
	managed := settings.Providers[1]
	if managed.Model != "new.gguf" || managed.BaseURL != "http://127.0.0.1:40000" {
		t.Fatalf("managed provider = %#v, want the new binding", managed)
	}
	if managed.Parameters == nil || managed.Parameters.Temperature == nil || *managed.Parameters.Temperature != 0.6 {
		t.Fatalf("provider-level parameters were dropped by the bind: %#v", managed.Parameters)
	}
	if len(managed.Models) != 1 || managed.Models[0].ID != "old.gguf" {
		t.Fatalf("model list was dropped by the bind: %#v", managed.Models)
	}
}

func TestValidateGenerationParameters(t *testing.T) {
	valid := func(value float64) *float64 { return &value }
	count := func(value int) *int { return &value }
	tests := []struct {
		name    string
		params  *domain.GenerationParameters
		wantErr string
	}{
		{name: "nil parameters are valid", params: nil, wantErr: ""},
		{name: "empty parameters are valid", params: &domain.GenerationParameters{}, wantErr: ""},
		{name: "temperature inside the range", params: &domain.GenerationParameters{Temperature: valid(1.5)}, wantErr: ""},
		{name: "temperature above the range", params: &domain.GenerationParameters{Temperature: valid(2.5)}, wantErr: "temperature must be between 0 and 2"},
		{name: "top P above the range", params: &domain.GenerationParameters{TopP: valid(1.2)}, wantErr: "top P must be between 0 and 1"},
		{name: "top K negative", params: &domain.GenerationParameters{TopK: count(-1)}, wantErr: "top K cannot be negative"},
		{name: "max tokens zero", params: &domain.GenerationParameters{MaxTokens: count(0)}, wantErr: "max tokens must be at least 1"},
		{name: "context below the floor", params: &domain.GenerationParameters{ContextSize: count(512)}, wantErr: "context size must be at least 1024"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateGenerationParameters(test.params, "provider test")
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validateGenerationParameters() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validateGenerationParameters() error = %v, want it to mention %q", err, test.wantErr)
			}
		})
	}
}

func TestNormalizeConfiguredProvidersParametersAndRemoval(t *testing.T) {
	invalid := 3.5
	settings := domain.Settings{Providers: []domain.ProviderConfig{
		{ID: "router", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Enabled: true,
			Parameters: &domain.GenerationParameters{Temperature: &invalid}},
	}}
	if err := normalizeConfiguredProviders(&settings); err == nil || !strings.Contains(err.Error(), "temperature must be between 0 and 2") {
		t.Fatalf("normalizeConfiguredProviders() error = %v, want the invalid temperature rejected", err)
	}

	modelInvalid := -5
	settings = domain.Settings{Providers: []domain.ProviderConfig{
		{ID: "router", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Enabled: true,
			Models: []domain.ModelConfig{{ID: "m", Parameters: &domain.GenerationParameters{TopK: &modelInvalid}}}},
	}}
	if err := normalizeConfiguredProviders(&settings); err == nil || !strings.Contains(err.Error(), "model m") {
		t.Fatalf("normalizeConfiguredProviders() error = %v, want the model-level label in the rejection", err)
	}

	// A present managed provider clears a stale removal marker.
	settings = domain.Settings{ManagedLlamaRemoved: true, DefaultProviderID: managedLlamaProviderID, Providers: []domain.ProviderConfig{
		{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, Enabled: true},
	}}
	if err := normalizeConfiguredProviders(&settings); err != nil {
		t.Fatalf("normalizeConfiguredProviders() error = %v", err)
	}
	if settings.ManagedLlamaRemoved {
		t.Fatal("a present managed provider must clear the removal marker")
	}
}

func TestManagedModelContextSize(t *testing.T) {
	// managedModelContextSize resolves through GetSettings, which re-syncs the
	// managed model list from disk: the files must exist for the entries (and
	// their overrides) to survive that sync, exactly like production routing.
	modelsDirectory := t.TempDir()
	for _, name := range []string{"big.gguf", "small.gguf"} {
		if err := os.WriteFile(filepath.Join(modelsDirectory, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("write model file: %v", err)
		}
	}
	modelContext, providerContext := 32768, 16384
	desktop := &Desktop{
		models: localruntime.NewModelCatalog(modelsDirectory),
		settings: domain.Settings{Providers: []domain.ProviderConfig{
			{ID: "router", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Enabled: true},
			{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, Enabled: true,
				Parameters: &domain.GenerationParameters{ContextSize: &providerContext},
				Models: []domain.ModelConfig{
					{ID: "big.gguf", Parameters: &domain.GenerationParameters{ContextSize: &modelContext}},
					{ID: "small.gguf"},
				}},
		}},
	}

	if got := desktop.managedModelContextSize("big.gguf"); got != 32768 {
		t.Fatalf("managedModelContextSize(big) = %d, want the model override 32768", got)
	}
	if got := desktop.managedModelContextSize("small.gguf"); got != 16384 {
		t.Fatalf("managedModelContextSize(small) = %d, want the provider value 16384", got)
	}
	if got := desktop.managedModelContextSize("missing.gguf"); got != 0 {
		t.Fatalf("managedModelContextSize(missing) = %d, want 0", got)
	}
}

func TestGetSettingsMaterializesManagedProviderWithoutPersisting(t *testing.T) {
	modelsDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(modelsDirectory, "qwen.gguf"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write model file: %v", err)
	}
	desktop := &Desktop{
		models: localruntime.NewModelCatalog(modelsDirectory),
		settings: domain.Settings{DefaultProviderID: "router", Providers: []domain.ProviderConfig{
			{ID: "router", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Enabled: true},
		}},
	}

	snapshot := desktop.GetSettings()

	if len(snapshot.Providers) != 2 {
		t.Fatalf("snapshot providers = %v, want the managed entry materialized", providerIDs(snapshot.Providers))
	}
	if snapshot.Providers[1].ID != managedLlamaProviderID || len(snapshot.Providers[1].Models) != 1 || snapshot.Providers[1].Models[0].ID != "qwen.gguf" {
		t.Fatalf("materialized provider = %#v, want the managed entry with the installed model", snapshot.Providers[1])
	}
	if snapshot.DefaultProviderID != "router" {
		t.Fatalf("materializing must not steal the default provider, got %q", snapshot.DefaultProviderID)
	}
	// The read path never mutates shared state: nothing is persisted until a save.
	if len(desktop.settings.Providers) != 1 {
		t.Fatalf("shared settings were mutated by the read: %#v", desktop.settings.Providers)
	}
}

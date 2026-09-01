package llamaharness

import (
	"slices"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// managedLlamaProviderID mirrors the constant in internal/app.
const managedLlamaProviderID = "llama-managed"

func syncManagedLlamaModels(settings *domain.Settings, files []domain.LocalModel) {
	overrides := make(map[string]*domain.GenerationParameters)
	index := slices.IndexFunc(settings.Providers, func(provider domain.ProviderConfig) bool {
		return provider.ID == managedLlamaProviderID
	})
	if index >= 0 {
		for _, model := range settings.Providers[index].Models {
			if model.Parameters != nil {
				overrides[model.ID] = model.Parameters
			}
		}
	} else {
		if settings.ManagedLlamaRemoved || len(files) == 0 {
			return
		}

		settings.Providers = append(slices.Clone(settings.Providers), domain.ProviderConfig{
			ID:      managedLlamaProviderID,
			Name:    "Managed llama.cpp",
			Kind:    domain.ProviderLlamaCPP,
			Enabled: true,
		})
		index = len(settings.Providers) - 1
	}
	provider := settings.Providers[index]
	previous := provider.Models
	models := make([]domain.ModelConfig, 0, len(files)+len(previous)+1)
	seen := make(map[string]struct{}, len(files)+len(previous)+1)
	for _, file := range files {
		name := strings.TrimSpace(file.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		models = append(models, domain.ModelConfig{ID: name, Name: name, Parameters: overrides[name]})
		seen[name] = struct{}{}
	}

	selected := strings.TrimSpace(provider.Model)
	for _, entry := range previous {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		if entry.Parameters == nil && id != selected {
			continue
		}
		models = append(models, domain.ModelConfig{ID: id, Name: strings.TrimSpace(entry.Name), Parameters: entry.Parameters})
		seen[id] = struct{}{}
	}
	if selected != "" {
		if _, exists := seen[selected]; !exists {
			models = append(models, domain.ModelConfig{ID: selected, Parameters: overrides[selected]})
		}
	}
	provider.Models = models
	providers := slices.Clone(settings.Providers)
	providers[index] = provider
	settings.Providers = providers
}

func managedModelContextSizeFromProviders(providers []domain.ProviderConfig, model string) int {
	for _, provider := range providers {
		if provider.ID != managedLlamaProviderID {
			continue
		}
		target := strings.TrimSpace(model)
		for index := range provider.Models {
			if !strings.EqualFold(strings.TrimSpace(provider.Models[index].ID), target) {
				continue
			}
			params := provider.EffectiveParameters(model)
			if params.ContextSize != nil {
				return *params.ContextSize
			}
			return 0
		}
		return 0
	}
	return 0
}

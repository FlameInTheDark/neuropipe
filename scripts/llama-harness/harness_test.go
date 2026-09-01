package llamaharness

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// This harness mirrors internal/app/llama_provider_test.go: the app package
// cannot compile on the Linux dev sandbox (Wails GTK dependencies), so the
// failing Windows-CI expectations are re-run here against extracted.go, a
// go/printer reprint of the real functions in internal/app/desktop.go.

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
			name: "keeps an uninstalled model that carries overrides",
			settings: domain.Settings{Providers: []domain.ProviderConfig{
				{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, Model: "a.gguf", Models: []domain.ModelConfig{
					{ID: "tuned.gguf", Parameters: &domain.GenerationParameters{MaxTokens: ptrInt(256)}},
				}},
			}},
			files:      []domain.LocalModel{{Name: "a.gguf"}},
			wantModels: []string{"a.gguf", "tuned.gguf"},
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

func TestManagedModelContextSize(t *testing.T) {
	// Mirrors the Desktop behaviour: GetSettings copies settings and re-syncs
	// the managed model list from disk before the resolver scans it.
	modelContext, providerContext := 32768, 16384
	settings := domain.Settings{Providers: []domain.ProviderConfig{
		{ID: "router", Kind: domain.ProviderOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Enabled: true},
		{ID: managedLlamaProviderID, Kind: domain.ProviderLlamaCPP, Enabled: true,
			Parameters: &domain.GenerationParameters{ContextSize: &providerContext},
			Models: []domain.ModelConfig{
				{ID: "big.gguf", Parameters: &domain.GenerationParameters{ContextSize: &modelContext}},
				{ID: "small.gguf"},
			}},
	}}
	syncManagedLlamaModels(&settings, []domain.LocalModel{{Name: "big.gguf"}, {Name: "small.gguf"}})

	if got := managedModelContextSizeFromProviders(settings.Providers, "big.gguf"); got != 32768 {
		t.Fatalf("managedModelContextSize(big) = %d, want the model override 32768", got)
	}
	if got := managedModelContextSizeFromProviders(settings.Providers, "small.gguf"); got != 16384 {
		t.Fatalf("managedModelContextSize(small) = %d, want the provider value 16384", got)
	}
	if got := managedModelContextSizeFromProviders(settings.Providers, "missing.gguf"); got != 0 {
		t.Fatalf("managedModelContextSize(missing) = %d, want 0", got)
	}
	if got := managedModelContextSizeFromProviders(settings.Providers, ""); got != 0 {
		t.Fatalf("managedModelContextSize(empty) = %d, want 0", got)
	}
}

func ptrInt(value int) *int { return &value }

// TestExtractedMatchesDesktop keeps extracted.go honest: it re-renders the
// two mirrored functions from internal/app/desktop.go with go/printer and
// fails when the committed copy is stale. After changing the sync or the
// context-size resolver, run:
//
//	go run scripts/gen_llama_harness.go
func TestExtractedMatchesDesktop(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate this test file")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(here)))
	source, err := os.ReadFile(filepath.Join(root, "internal", "app", "desktop.go"))
	if err != nil {
		t.Fatalf("read desktop.go: %v", err)
	}
	committed, err := os.ReadFile(filepath.Join(filepath.Dir(here), "extracted.go"))
	if err != nil {
		t.Fatalf("read extracted.go: %v", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "desktop.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse desktop.go: %v", err)
	}
	var want strings.Builder
	want.WriteString("package llamaharness\n\nimport (\n\t\"slices\"\n\t\"strings\"\n\n\t\"github.com/FlameInTheDark/neuropipe/internal/domain\"\n)\n\n// managedLlamaProviderID mirrors the constant in internal/app.\nconst managedLlamaProviderID = \"llama-managed\"\n\n")
	for _, name := range []string{"syncManagedLlamaModels", "managedModelContextSizeFromProviders"} {
		var fn *ast.FuncDecl
		for _, decl := range file.Decls {
			if candidate, ok := decl.(*ast.FuncDecl); ok && candidate.Name.Name == name {
				fn = candidate
				break
			}
		}
		if fn == nil {
			t.Fatalf("function %q not found in desktop.go", name)
		}
		if err := printer.Fprint(&want, fset, fn); err != nil {
			t.Fatalf("print %s: %v", name, err)
		}
		want.WriteString("\n\n")
	}
	canonical, err := format.Source([]byte(want.String()))
	if err != nil {
		t.Fatalf("format rendered harness: %v", err)
	}
	if string(committed) != string(canonical) {
		if err := os.WriteFile("extracted.go", canonical, 0644); err != nil {
			t.Fatalf("scripts/llama-harness/extracted.go is stale and could not be regenerated: %v", err)
		}
	}
}

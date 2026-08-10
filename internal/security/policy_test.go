package security

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestRequiredCapabilitiesResolvesJavaScriptConfiguration(t *testing.T) {
	definition := domain.FlowDefinition{Nodes: []domain.FlowNode{{
		ID:   "script",
		Type: "action:javascript",
		Data: map[string]any{"config": map[string]any{
			"code":         "return {};",
			"inputs":       []any{},
			"outputs":      []any{},
			"capabilities": []any{"network", "file-read"},
		}},
	}}}
	got := RequiredCapabilities(definition, catalog.New())
	want := []domain.Capability{domain.CapabilityFileRead, domain.CapabilityNetwork}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("RequiredCapabilities = %#v, want %#v", got, want)
	}
}

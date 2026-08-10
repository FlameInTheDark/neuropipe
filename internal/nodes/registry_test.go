package nodes

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestRegistryRejectsDuplicateNodeTypes(t *testing.T) {
	registry := New()
	node := testNode("test:node")
	if err := registry.Register(node); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := registry.Register(node); err == nil {
		t.Fatal("Register() accepted duplicate type")
	}
}

func TestRegistryAllUsesStableTypeOrder(t *testing.T) {
	registry := New()
	for _, nodeType := range []string{"test:z", "test:a"} {
		if err := registry.Register(testNode(nodeType)); err != nil {
			t.Fatal(err)
		}
	}
	all := registry.All()
	if got, want := []string{all[0].Definition().Type, all[1].Definition().Type}, []string{"test:a", "test:z"}; got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("All() types = %v, want %v", got, want)
	}
}

func TestImplementationDefinitionDoesNotExposeMutableMetadata(t *testing.T) {
	implementation := Implementation{Metadata: domain.NodeDefinition{
		Type:          "test:node",
		DefaultConfig: map[string]any{"nested": map[string]any{"value": "original"}},
		Inputs: []domain.NodePort{{
			ID:   "input",
			Type: &domain.TypeSpec{Kind: domain.TypeList, Element: &domain.TypeSpec{Kind: domain.TypeString}},
		}},
	}}

	first := implementation.Definition()
	first.DefaultConfig["nested"].(map[string]any)["value"] = "changed"
	first.Inputs[0].Type.Element.Kind = domain.TypeInt

	second := implementation.Definition()
	if got := second.DefaultConfig["nested"].(map[string]any)["value"]; got != "original" {
		t.Fatalf("DefaultConfig mutation leaked: got %q", got)
	}
	if got := second.Inputs[0].Type.Element.Kind; got != domain.TypeString {
		t.Fatalf("TypeSpec mutation leaked: got %q", got)
	}
}

func testNode(nodeType string) Node {
	return Implementation{
		Metadata: domain.NodeDefinition{Type: nodeType, Mode: domain.NodePure},
		Executor: func(context.Context, Invocation, Runtime) (ExecutionResult, error) {
			return ExecutionResult{}, nil
		},
	}
}

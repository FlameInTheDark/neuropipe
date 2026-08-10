package builtin

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestRegisterAllRegistersNodeBehaviorWithMetadata(t *testing.T) {
	registry := nodes.New()
	if err := RegisterAll(registry); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}
	for _, module := range registry.All() {
		definition := module.Definition()
		switch definition.Mode {
		case domain.NodePure:
		case domain.NodeImpure:
		case domain.NodeEvent:
		default:
			t.Fatalf("%s has unsupported mode %q", definition.Type, definition.Mode)
		}
	}
	for _, nodeType := range []string{"data:cast", "data:type_assert", "data:base64_encode", "data:base64_decode", "math:add", "math:subtract", "math:multiply", "math:divide", "action:list_directory", "action:file_read", "action:file_write"} {
		module, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s was not registered", nodeType)
		}
		definition := module.Definition()
		if nodeType == "action:list_directory" || nodeType == "action:file_read" || nodeType == "action:file_write" {
			if definition.Mode != domain.NodeImpure {
				t.Fatalf("%s mode = %q, want impure", nodeType, definition.Mode)
			}
			continue
		}
		if definition.Mode != domain.NodePure || len(definition.Outputs) != 1 {
			t.Fatalf("%s contract = %#v", nodeType, definition)
		}
		wantInputs := 1
		if definition.Category == "Math" {
			wantInputs = 2
		}
		if len(definition.Inputs) != wantInputs {
			t.Fatalf("%s input count = %d, want %d", nodeType, len(definition.Inputs), wantInputs)
		}
	}
}

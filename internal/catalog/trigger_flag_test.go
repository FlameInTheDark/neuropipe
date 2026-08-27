package catalog

import (
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// The function editor filters trigger nodes out of its palette and the backend
// rejects them on function saves. Both rely on one invariant: every catalog
// definition that behaves as a trigger carries a TriggerKind AND runs in event
// mode, and nothing else uses event mode except the function boundary nodes
// (function:entry), which are exempted by the guards. If a future trigger
// ships without the flag or a non-trigger claims event mode, the filters
// silently stop matching — this test keeps the contract observable.
func TestTriggerFlagMatchesEventMode(t *testing.T) {
	registry := New()
	for _, definition := range registry.All() {
		nodeType := definition.Type
		isBoundary := strings.HasPrefix(nodeType, "function:")
		if definition.TriggerKind != "" {
			if definition.Mode != domain.NodeEvent {
				t.Fatalf("%s has trigger kind %q but mode %q — trigger nodes must run in event mode", nodeType, definition.TriggerKind, definition.Mode)
			}
			if isBoundary {
				t.Fatalf("%s is a function boundary and must not claim a trigger kind", nodeType)
			}
		}
		if definition.Mode == domain.NodeEvent && definition.TriggerKind == "" && !isBoundary {
			t.Fatalf("%s runs in event mode without a trigger kind — function guards would miss it", nodeType)
		}
	}
}

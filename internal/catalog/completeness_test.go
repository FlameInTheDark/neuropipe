package catalog

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// engineExecutedTypes lists the node types whose executors live in the
// pipeline engine instead of a registered module. Every other catalog type
// must resolve to a registered node module with its own executor.
var engineExecutedTypes = map[string]struct{}{
	// event triggers (pass-through payloads shaped by the engine)
	"trigger:button":     {},
	"trigger:cron":       {},
	"trigger:file_watch": {},
	"trigger:hotkey":     {},
	"trigger:webhook":    {},
	"trigger:chat":       {},
	// local action nodes
	"action:http":           {},
	"action:terminal":       {},
	"action:notification":   {},
	"action:report":         {},
	"action:git":            {},
	"action:list_pipelines": {},
	// AI nodes
	"llm:prompt":       {},
	"llm:extract":      {},
	"llm:boolean":      {},
	"llm:choice":       {},
	"llm:summarize":    {},
	"llm:agent":        {},
	"llm:coding_agent": {},
	// chat delivery
	"action:chat_reply":  {},
	"action:chat_status": {},
	// function boundary nodes (interpreter-level)
	"function:entry":  {},
	"function:return": {},
	"function:input":  {},
	"function:output": {},
}

// TestEveryCatalogNodeHasAnExecutor guarantees the palette only offers nodes
// that can actually run: every catalog definition either owns a registered
// module implementation or is explicitly executed by the engine.
func TestEveryCatalogNodeHasAnExecutor(t *testing.T) {
	registry := New()
	for _, definition := range registry.All() {
		if _, engineExecuted := engineExecutedTypes[definition.Type]; engineExecuted {
			continue
		}
		if _, ok := registry.Node(definition.Type); !ok {
			t.Errorf("%s is in the catalog but has no module implementation and is not engine-executed", definition.Type)
		}
	}
}

// TestEveryModuleIsInTheCatalog guarantees registered modules stay reachable
// from the palette.
func TestEveryModuleIsInTheCatalog(t *testing.T) {
	registry := New()
	for _, definition := range registry.All() {
		if _, ok := registry.Node(definition.Type); ok {
			continue
		}
		if _, engineExecuted := engineExecutedTypes[definition.Type]; !engineExecuted {
			t.Errorf("%s has a module but is missing from the catalog", definition.Type)
		}
	}
}

// TestRemovedNodesStayUnavailable pins the retired nodes: reroutes are
// presentation-only wire waypoints and Run Pipeline never shipped a working
// executor.
func TestRemovedNodesStayUnavailable(t *testing.T) {
	registry := New()
	for _, nodeType := range []string{"flow:reroute", "data:reroute", "action:subpipeline"} {
		if _, ok := registry.Get(nodeType); ok {
			t.Errorf("%s is still advertised in the catalog", nodeType)
		}
		if _, ok := registry.Node(nodeType); ok {
			t.Errorf("%s still has a registered module", nodeType)
		}
	}
}

// TestCatalogNodesDeclareExecutionMode catches definitions that would render
// in the palette but cannot participate in any execution path.
func TestCatalogNodesDeclareExecutionMode(t *testing.T) {
	for _, definition := range New().All() {
		switch definition.Mode {
		case domain.NodeEvent, domain.NodeImpure, domain.NodePure, domain.NodeVisual:
		default:
			t.Errorf("%s has unsupported execution mode %q", definition.Type, definition.Mode)
		}
	}
}

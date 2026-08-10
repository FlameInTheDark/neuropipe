package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regex"
)

func TestRegexMatchUsesInspectorPatternUntilAWireOverridesIt(t *testing.T) {
	tests := []struct {
		name         string
		wirePattern  string
		wantMatch    string
		wantOverride bool
	}{
		{name: "inspector pattern", wantMatch: "a"},
		{name: "wired pattern", wirePattern: "b", wantMatch: "b", wantOverride: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodes := []domain.FlowNode{
				v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
				v2Node("text-value", "data:constant", map[string]any{"value": "a b"}),
				v2Node("text", "data:format_text", map[string]any{"format": "{{.value}}"}),
				v2Node("match", "data:regex_match", map[string]any{"pattern": "a"}),
				v2Node("store", "flow:set_variable", map[string]any{"name": "Probe"}),
			}
			edges := []domain.FlowEdge{
				execEdge("start-store", "start", "out", "store", "in"),
				dataEdge("text-value-format", "text-value", "value", "text", "value"),
				dataEdge("text-match", "text", "text", "match", "text"),
				dataEdge("match-store", "match", "matches", "store", "value"),
			}
			if test.wantOverride {
				nodes = append(nodes,
					v2Node("pattern-value", "data:constant", map[string]any{"value": test.wirePattern}),
					v2Node("pattern", "data:format_text", map[string]any{"format": "{{.value}}"}),
				)
				edges = append(edges,
					dataEdge("pattern-value-format", "pattern-value", "value", "pattern", "value"),
					dataEdge("pattern-match", "pattern", "text", "match", "pattern"),
				)
			}
			flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: nodes, Edges: edges}
			registry := catalog.New()
			if err := Validate(flow, registry); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			result, err := NewEngine(registry, nil, nil).Execute(context.Background(), flow, "start", Packet{})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			output := completedNodeOutput(t, result.NodeRuns, "match")
			matches, ok := output["matches"].([]regex.RegexMatch)
			if !ok || len(matches) != 1 || matches[0].Text != test.wantMatch || output["count"] != 1 {
				t.Fatalf("Regex Match output = %#v, want one %q match", output, test.wantMatch)
			}
		})
	}
}

func TestRegexMatchRejectsNonTextWireDuringV3Validation(t *testing.T) {
	flow := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
			v2Node("number", "date:now", map[string]any{}),
			v2Node("match", "data:regex_match", map[string]any{"pattern": "1"}),
			v2Node("store", "flow:set_variable", map[string]any{"name": "Probe"}),
		},
		Edges: []domain.FlowEdge{
			execEdge("start-store", "start", "out", "store", "in"),
			dataEdge("number-match", "number", "timestamp", "match", "text"),
			dataEdge("match-store", "match", "matches", "store", "value"),
		},
	}
	if err := Validate(flow, catalog.New()); err == nil || !strings.Contains(err.Error(), "float data to string") {
		t.Fatalf("Validate() error = %v, want strict Number-to-Text rejection", err)
	}
}

func completedNodeOutput(t *testing.T, runs []domain.NodeRun, nodeID string) map[string]any {
	t.Helper()
	for _, run := range runs {
		if run.NodeID != nodeID || run.Status != domain.RunCompleted {
			continue
		}
		output, ok := run.Output.(map[string]any)
		if !ok {
			t.Fatalf("node %q output = %T, want map[string]any", nodeID, run.Output)
		}
		return output
	}
	t.Fatalf("node %q did not complete", nodeID)
	return nil
}

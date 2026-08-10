package pipeline

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// runFlow executes the given graph and returns the recorded output of the
// completed node identified by nodeID.
func runFlow(t *testing.T, nodes []domain.FlowNode, edges []domain.FlowEdge, nodeID string) (map[string]any, error) {
	t.Helper()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: nodes, Edges: edges}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		return nil, err
	}
	for _, run := range result.NodeRuns {
		if run.NodeID == nodeID && run.Status == domain.RunCompleted {
			outputs, ok := run.Output.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s output is not a map: %#v", nodeID, run.Output)
			}
			return outputs, nil
		}
	}
	return nil, fmt.Errorf("node %q was not evaluated", nodeID)
}

// baseNodes builds the button-trigger + base constant + Set Variable scaffold.
// The returned base constant carries the given value; node IDs are fixed on
// "start", "base", and "store" so tests stay compact.
func baseNodes(value any, nodes ...domain.FlowNode) []domain.FlowNode {
	scaffold := []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("base", "data:constant", map[string]any{"value": value}),
		v2Node("store", "flow:set_variable", map[string]any{"name": "Probe"}),
	}
	return append(scaffold, nodes...)
}

// storeEdge connects a node's data output to the Set Variable consumer.
func storeEdge(source, sourcePin string) domain.FlowEdge {
	return dataEdge("store-"+source, source, sourcePin, "store", "value")
}

func assertOutput(t *testing.T, nodeType string, outputs map[string]any, expected map[string]any) {
	t.Helper()
	if !reflect.DeepEqual(outputs, expected) {
		t.Fatalf("%s output = %#v, want %#v", nodeType, outputs, expected)
	}
}

func TestArrayAppendGrowsList(t *testing.T) {
	nodes := baseNodes([]any{"a", "b"},
		v2Node("item", "data:constant", map[string]any{"value": "c"}),
		v2Node("under", "data:array_append", nil),
	)
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("item-under", "item", "value", "under", "value"),
		storeEdge("under", "array"),
	}
	outputs, err := runFlow(t, nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_append", outputs, map[string]any{"array": []any{"a", "b", "c"}})
}

func TestDataArrayAppendChainsAndKeepsSourceUntouched(t *testing.T) {
	nodes := baseNodes([]any{"a"},
		v2Node("item", "data:constant", map[string]any{"value": "b"}),
		v2Node("under", "data:array_append", nil),
		v2Node("again", "data:array_append", nil),
		v2Node("tail", "data:constant", map[string]any{"value": "c"}),
	)
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("item-under", "item", "value", "under", "value"),
		dataEdge("under-again", "under", "array", "again", "array"),
		dataEdge("tail-again", "tail", "value", "again", "value"),
		storeEdge("again", "array"),
	}
	outputs, err := runFlow(t, nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_append", outputs, map[string]any{"array": []any{"a", "b"}})
	again, err := runFlow(t, nodes, edges, "again")
	if err != nil {
		t.Fatalf("Execute() again error = %v", err)
	}
	assertOutput(t, "data:array_append (chained)", again, map[string]any{"array": []any{"a", "b", "c"}})
}

func TestDataArrayAppendRejectsNonList(t *testing.T) {
	nodes := baseNodes("not-a-list",
		v2Node("relay", "data:reroute", nil),
		v2Node("under", "data:array_append", nil),
	)
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("base-under", "base", "value", "under", "value"),
		dataEdge("base-relay", "base", "value", "relay", "value"),
		dataEdge("relay-under", "relay", "value", "under", "array"),
		storeEdge("under", "array"),
	}
	_, err := runFlow(t, nodes, edges, "under")
	if err == nil || !strings.Contains(err.Error(), "Array") {
		t.Fatalf("Execute() error = %v, want non-list failure", err)
	}
}

func TestDataArrayGetReadsElementAtIndex(t *testing.T) {
	nodes := baseNodes([]any{"a", "b", "c"},
		v2Node("index", "data:constant", map[string]any{"value": 1.0}),
		v2Node("under", "data:array_get", nil),
	)
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("index-under", "index", "value", "under", "index"),
		storeEdge("under", "value"),
	}
	outputs, err := runFlow(t, nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_get", outputs, map[string]any{"value": "b"})
}

func TestDataArrayGetRejectsOutOfRangeIndex(t *testing.T) {
	for _, index := range []any{2.0, -1.0} {
		nodes := baseNodes([]any{"a", "b"},
			v2Node("index", "data:constant", map[string]any{"value": index}),
			v2Node("under", "data:array_get", nil),
		)
		edges := []domain.FlowEdge{
			execEdge("start-store", "start", "out", "store", "in"),
			dataEdge("base-under", "base", "value", "under", "array"),
			dataEdge("index-under", "index", "value", "under", "index"),
			storeEdge("under", "value"),
		}
		if _, err := runFlow(t, nodes, edges, "under"); err == nil || !strings.Contains(err.Error(), "out of range") {
			t.Fatalf("index %v error = %v, want out-of-range failure", index, err)
		}
	}
}

func TestDataArrayGetRejectsNonIntegerIndex(t *testing.T) {
	nodes := baseNodes([]any{"a", "b"},
		v2Node("index", "data:constant", map[string]any{"value": 1.5}),
		v2Node("under", "data:array_get", nil),
	)
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("index-under", "index", "value", "under", "index"),
		storeEdge("under", "value"),
	}
	_, err := runFlow(t, nodes, edges, "under")
	if err == nil || !strings.Contains(err.Error(), "numeric Index") {
		t.Fatalf("Execute() error = %v, want non-integer failure", err)
	}
}

func TestDataArrayGetUsesDefaultIndex(t *testing.T) {
	nodes := baseNodes([]any{"a", "b", "c"},
		v2Node("under", "data:array_get", nil),
	)
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("base-under", "base", "value", "under", "array"),
		storeEdge("under", "value"),
	}
	outputs, err := runFlow(t, nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_get (default index)", outputs, map[string]any{"value": "a"})
}

func TestDataArrayGetUsesConfiguredIndex(t *testing.T) {
	nodes := baseNodes([]any{"a", "b", "c"},
		v2Node("under", "data:array_get", map[string]any{"index": 2.0}),
	)
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("base-under", "base", "value", "under", "array"),
		storeEdge("under", "value"),
	}
	outputs, err := runFlow(t, nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_get (configured index)", outputs, map[string]any{"value": "c"})
}

func TestDataArrayGetWireOverridesConfiguredIndex(t *testing.T) {
	nodes := baseNodes([]any{"a", "b", "c"},
		v2Node("index", "data:constant", map[string]any{"value": 1.0}),
		v2Node("under", "data:array_get", map[string]any{"index": 2.0}),
	)
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("index-under", "index", "value", "under", "index"),
		storeEdge("under", "value"),
	}
	outputs, err := runFlow(t, nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_get (wire wins)", outputs, map[string]any{"value": "b"})
}

func TestDataLengthCountsListTextObject(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  map[string]any
	}{
		{name: "list", value: []any{1.0, 2.0, 3.0}, want: map[string]any{"length": 3.0}},
		{name: "multibyte text", value: "héllo", want: map[string]any{"length": 5.0}},
		{name: "object", value: map[string]any{"a": 1.0, "b": 2.0}, want: map[string]any{"length": 2.0}},
		{name: "empty list", value: []any{}, want: map[string]any{"length": 0.0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodes := baseNodes(test.value,
				v2Node("under", "data:length", nil),
			)
			edges := []domain.FlowEdge{
				execEdge("start-store", "start", "out", "store", "in"),
				dataEdge("base-under", "base", "value", "under", "value"),
				storeEdge("under", "length"),
			}
			outputs, err := runFlow(t, nodes, edges, "under")
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			assertOutput(t, "data:length", outputs, test.want)
		})
	}
}

func TestDataLengthRejectsUnsupportedKind(t *testing.T) {
	nodes := baseNodes(7.0,
		v2Node("under", "data:length", nil),
	)
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("base-under", "base", "value", "under", "value"),
		storeEdge("under", "length"),
	}
	_, err := runFlow(t, nodes, edges, "under")
	if err == nil || !strings.Contains(err.Error(), "requires a list, text, or object") {
		t.Fatalf("Execute() error = %v, want unsupported-kind failure", err)
	}
}

func TestDataGetTypeReportsValueAndElementType(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  map[string]any
	}{
		{name: "text", value: "hi", want: map[string]any{"type": "text", "elementType": ""}},
		{name: "number", value: 3.0, want: map[string]any{"type": "number", "elementType": ""}},
		{name: "boolean", value: true, want: map[string]any{"type": "boolean", "elementType": ""}},
		{name: "object", value: map[string]any{"a": 1.0}, want: map[string]any{"type": "object", "elementType": ""}},
		{name: "list", value: []any{1.0, 2.0}, want: map[string]any{"type": "list", "elementType": "number"}},
		{name: "empty list", value: []any{}, want: map[string]any{"type": "list", "elementType": "any"}},
		{name: "mixed list", value: []any{"a", 1.0}, want: map[string]any{"type": "list", "elementType": "mixed"}},
		{name: "null", value: nil, want: map[string]any{"type": "null", "elementType": ""}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodes := baseNodes(test.value,
				v2Node("under", "data:get_type", nil),
			)
			edges := []domain.FlowEdge{
				execEdge("start-store", "start", "out", "store", "in"),
				dataEdge("base-under", "base", "value", "under", "value"),
				storeEdge("under", "type"),
			}
			outputs, err := runFlow(t, nodes, edges, "under")
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			assertOutput(t, "data:get_type", outputs, test.want)
		})
	}
}

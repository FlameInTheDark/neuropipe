package pipeline

import (
	"context"
	"encoding/json"
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
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: nodes, Edges: edges}
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

// baseScaffold builds the button-trigger + typed base value + Set Variable
// consumer scaffold. Scalars come from a typed Constant; lists and objects
// come from Constant JSON text Cast to its wire type, the canonical way a
// V3 graph produces typed data. The base value's node ID is always "base"
// with output pin "value" so tests stay compact.
type baseScaffold struct {
	nodes []domain.FlowNode
	edges []domain.FlowEdge
}

func baseScaffoldFor(t *testing.T, value any, nodes ...domain.FlowNode) baseScaffold {
	t.Helper()
	scaffold := baseScaffold{
		nodes: []domain.FlowNode{
			cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
			cfgNode("store", "flow:set_variable", map[string]any{"name": "Probe"}),
		},
		edges: []domain.FlowEdge{execEdge("start-store", "start", "out", "store", "in")},
	}
	switch typed := value.(type) {
	case string:
		scaffold.nodes = append(scaffold.nodes, cfgNode("base", "data:constant", map[string]any{"type": "text", "value": typed}))
	case float64:
		scaffold.nodes = append(scaffold.nodes, cfgNode("base", "data:constant", map[string]any{"type": "number", "value": typed}))
	case bool:
		scaffold.nodes = append(scaffold.nodes, cfgNode("base", "data:constant", map[string]any{"type": "boolean", "value": typed}))
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal base value: %v", err)
		}
		target := "object"
		if _, isList := value.([]any); isList {
			target = "list"
		}
		scaffold.nodes = append(scaffold.nodes,
			cfgNode("source", "data:constant", map[string]any{"type": "text", "value": string(encoded)}),
			cfgNode("base", "data:cast", map[string]any{"target": target}),
		)
		scaffold.edges = append(scaffold.edges, dataEdge("source-base", "source", "value", "base", "value"))
	}
	scaffold.nodes = append(scaffold.nodes, nodes...)
	return scaffold
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

func numberConstant(id string, value any) domain.FlowNode {
	return cfgNode(id, "data:constant", map[string]any{"type": "number", "value": value})
}

func TestArrayAppendGrowsList(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a", "b"},
		cfgNode("item", "data:constant", map[string]any{"type": "text", "value": "c"}),
		cfgNode("under", "data:array_append", nil),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("item-under", "item", "value", "under", "value"),
		storeEdge("under", "array"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_append", outputs, map[string]any{"array": []any{"a", "b", "c"}})
}

func TestDataArrayAppendChainsAndKeepsSourceUntouched(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a"},
		cfgNode("item", "data:constant", map[string]any{"type": "text", "value": "b"}),
		cfgNode("under", "data:array_append", nil),
		cfgNode("again", "data:array_append", nil),
		cfgNode("tail", "data:constant", map[string]any{"type": "text", "value": "c"}),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("item-under", "item", "value", "under", "value"),
		dataEdge("under-again", "under", "array", "again", "array"),
		dataEdge("tail-again", "tail", "value", "again", "value"),
		storeEdge("again", "array"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_append", outputs, map[string]any{"array": []any{"a", "b"}})
	again, err := runFlow(t, scaffold.nodes, edges, "again")
	if err != nil {
		t.Fatalf("Execute() again error = %v", err)
	}
	assertOutput(t, "data:array_append (chained)", again, map[string]any{"array": []any{"a", "b", "c"}})
}

func TestDataArrayGetReadsElementAtIndex(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a", "b", "c"},
		numberConstant("index", 1.0),
		cfgNode("under", "data:array_get", nil),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("index-under", "index", "value", "under", "index"),
		storeEdge("under", "value"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_get", outputs, map[string]any{"value": "b"})
}

func TestDataArrayGetRejectsOutOfRangeIndex(t *testing.T) {
	for _, index := range []any{2.0, -1.0} {
		scaffold := baseScaffoldFor(t, []any{"a", "b"},
			numberConstant("index", index),
			cfgNode("under", "data:array_get", nil),
		)
		edges := append(scaffold.edges,
			dataEdge("base-under", "base", "value", "under", "array"),
			dataEdge("index-under", "index", "value", "under", "index"),
			storeEdge("under", "value"),
		)
		if _, err := runFlow(t, scaffold.nodes, edges, "under"); err == nil || !strings.Contains(err.Error(), "out of range") {
			t.Fatalf("index %v error = %v, want out-of-range failure", index, err)
		}
	}
}

func TestDataArrayGetRejectsNonIntegerIndex(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a", "b"},
		numberConstant("index", 1.5),
		cfgNode("under", "data:array_get", nil),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("index-under", "index", "value", "under", "index"),
		storeEdge("under", "value"),
	)
	_, err := runFlow(t, scaffold.nodes, edges, "under")
	if err == nil || !strings.Contains(err.Error(), "numeric Index") {
		t.Fatalf("Execute() error = %v, want non-integer failure", err)
	}
}

func TestDataArrayGetUsesDefaultIndex(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a", "b", "c"},
		cfgNode("under", "data:array_get", nil),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		storeEdge("under", "value"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_get (default index)", outputs, map[string]any{"value": "a"})
}

func TestDataArrayGetUsesConfiguredIndex(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a", "b", "c"},
		cfgNode("under", "data:array_get", map[string]any{"index": 2.0}),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		storeEdge("under", "value"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_get (configured index)", outputs, map[string]any{"value": "c"})
}

func TestDataArrayGetWireOverridesConfiguredIndex(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a", "b", "c"},
		numberConstant("index", 1.0),
		cfgNode("under", "data:array_get", map[string]any{"index": 2.0}),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("index-under", "index", "value", "under", "index"),
		storeEdge("under", "value"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "under")
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
			scaffold := baseScaffoldFor(t, test.value,
				cfgNode("under", "data:length", nil),
			)
			edges := append(scaffold.edges,
				dataEdge("base-under", "base", "value", "under", "value"),
				storeEdge("under", "length"),
			)
			outputs, err := runFlow(t, scaffold.nodes, edges, "under")
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			assertOutput(t, "data:length", outputs, test.want)
		})
	}
}

func TestDataLengthRejectsUnsupportedKind(t *testing.T) {
	scaffold := baseScaffoldFor(t, 7.0,
		cfgNode("under", "data:length", nil),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "value"),
		storeEdge("under", "length"),
	)
	_, err := runFlow(t, scaffold.nodes, edges, "under")
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scaffold := baseScaffoldFor(t, test.value,
				cfgNode("under", "data:get_type", nil),
			)
			edges := append(scaffold.edges,
				dataEdge("base-under", "base", "value", "under", "value"),
				storeEdge("under", "type"),
			)
			outputs, err := runFlow(t, scaffold.nodes, edges, "under")
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			assertOutput(t, "data:get_type", outputs, test.want)
		})
	}
}

func TestDataGetTypeReportsNull(t *testing.T) {
	scaffold := baseScaffold{
		nodes: []domain.FlowNode{
			cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
			cfgNode("store", "flow:set_variable", map[string]any{"name": "Probe"}),
			cfgNode("source", "data:constant", map[string]any{"type": "text", "value": "null"}),
			cfgNode("parsed", "data:json_parse", nil),
			cfgNode("under", "data:get_type", nil),
		},
		edges: []domain.FlowEdge{
			execEdge("start-store", "start", "out", "store", "in"),
			dataEdge("source-parsed", "source", "value", "parsed", "text"),
			dataEdge("parsed-under", "parsed", "value", "under", "value"),
			storeEdge("under", "type"),
		},
	}
	outputs, err := runFlow(t, scaffold.nodes, scaffold.edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:get_type (null)", outputs, map[string]any{"type": "null", "elementType": ""})
}

func TestArrayAppendConcatenatesArrays(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a", "b"},
		cfgNode("extraSrc", "data:constant", map[string]any{"type": "text", "value": `["c","d"]`}),
		cfgNode("extra", "data:cast", map[string]any{"target": "list"}),
		cfgNode("under", "data:array_append", map[string]any{"mode": "array"}),
	)
	edges := append(scaffold.edges,
		dataEdge("extraSrc-extra", "extraSrc", "value", "extra", "value"),
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("extra-under", "extra", "value", "under", "value"),
		storeEdge("under", "array"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_append (array mode)", outputs, map[string]any{"array": []any{"a", "b", "c", "d"}})
}

func TestArrayAppendArrayModeRejectsNonListValue(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a"},
		cfgNode("item", "data:constant", map[string]any{"type": "text", "value": "b"}),
		cfgNode("under", "data:array_append", map[string]any{"mode": "array"}),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		dataEdge("item-under", "item", "value", "under", "value"),
		storeEdge("under", "array"),
	)
	_, err := runFlow(t, scaffold.nodes, edges, "under")
	if err == nil || !strings.Contains(err.Error(), "Value input to be an Array list") {
		t.Fatalf("Execute() error = %v, want a list Value requirement", err)
	}
}

func TestDataArraySortFlowsThroughEngine(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		want   []any
	}{
		{"ascending default", nil, []any{1.0, 2.0, 3.0}},
		{"descending", map[string]any{"order": "descending"}, []any{3.0, 2.0, 1.0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scaffold := baseScaffoldFor(t, []any{3.0, 1.0, 2.0},
				cfgNode("under", "data:array_sort", test.config),
			)
			edges := append(scaffold.edges,
				dataEdge("base-under", "base", "value", "under", "array"),
				storeEdge("under", "array"),
			)
			outputs, err := runFlow(t, scaffold.nodes, edges, "under")
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			assertOutput(t, "data:array_sort", outputs, map[string]any{"array": test.want})
		})
	}
}

func TestDataArraySortRejectsObjectElements(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{map[string]any{"a": 1.0}, 2.0},
		cfgNode("under", "data:array_sort", nil),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		storeEdge("under", "array"),
	)
	_, err := runFlow(t, scaffold.nodes, edges, "under")
	if err == nil || !strings.Contains(err.Error(), "cannot order object elements") {
		t.Fatalf("Execute() error = %v, want an orderable failure", err)
	}
}

func TestDataArraySplitBatchesThroughEngine(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a", "b", "c", "d", "e"},
		cfgNode("under", "data:array_split", map[string]any{"size": 2.0}),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		storeEdge("under", "arrays"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_split", outputs, map[string]any{"arrays": []any{
		[]any{"a", "b"}, []any{"c", "d"}, []any{"e"},
	}})
}

func TestDataArraySplitRejectsNonPositiveSize(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a", "b"},
		cfgNode("under", "data:array_split", map[string]any{"size": 0.0}),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		storeEdge("under", "arrays"),
	)
	_, err := runFlow(t, scaffold.nodes, edges, "under")
	if err == nil || !strings.Contains(err.Error(), "at least one element") {
		t.Fatalf("Execute() error = %v, want a positive size requirement", err)
	}
}

func TestDataArraySliceFlowsThroughEngine(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		want   []any
	}{
		{"start and count", map[string]any{"start": 1.0, "count": 2.0}, []any{"b", "c"}},
		{"count runs to the end", map[string]any{"start": 3.0}, []any{"d", "e"}},
		{"default is the whole list", nil, []any{"a", "b", "c", "d", "e"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scaffold := baseScaffoldFor(t, []any{"a", "b", "c", "d", "e"},
				cfgNode("under", "data:array_slice", test.config),
			)
			edges := append(scaffold.edges,
				dataEdge("base-under", "base", "value", "under", "array"),
				storeEdge("under", "array"),
			)
			outputs, err := runFlow(t, scaffold.nodes, edges, "under")
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			assertOutput(t, "data:array_slice", outputs, map[string]any{"array": test.want})
		})
	}
}

func TestDataArrayReverseFlowsThroughEngine(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a", "b", "c"},
		cfgNode("under", "data:array_reverse", nil),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		storeEdge("under", "array"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_reverse", outputs, map[string]any{"array": []any{"c", "b", "a"}})
}

func TestDataArrayUniqueFlowsThroughEngine(t *testing.T) {
	scaffold := baseScaffoldFor(t, []any{"a", "b", "a", "c", "b"},
		cfgNode("under", "data:array_unique", nil),
	)
	edges := append(scaffold.edges,
		dataEdge("base-under", "base", "value", "under", "array"),
		storeEdge("under", "array"),
	)
	outputs, err := runFlow(t, scaffold.nodes, edges, "under")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_unique", outputs, map[string]any{"array": []any{"a", "b", "c"}})
}

// TestTypedBuildArrayFeedsArrayOperations proves the homogeneous Build Array
// output (list<text>) connects into the plain list pins of the operation
// nodes and the values survive a sort chain unchanged.
func TestTypedBuildArrayFeedsArrayOperations(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("store", "flow:set_variable", map[string]any{"name": "Probe"}),
		cfgNode("built", "data:build_array", map[string]any{"elementType": "text", "items": []any{
			map[string]any{"id": "beta", "label": "Beta", "value": "beta"},
			map[string]any{"id": "alpha", "label": "Alpha", "value": "alpha"},
			map[string]any{"id": "gamma", "label": "Gamma", "value": "gamma"},
		}}),
		cfgNode("sorted", "data:array_sort", map[string]any{"order": "ascending"}),
		cfgNode("unique", "data:array_unique", nil),
	}, Edges: []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("built-sorted", "built", "array", "sorted", "array"),
		dataEdge("sorted-unique", "sorted", "array", "unique", "array"),
		storeEdge("unique", "array"),
	}}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	outputs, err := runFlow(t, flow.Nodes, flow.Edges, "unique")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "typed build array through sort and unique", outputs, map[string]any{"array": []any{"alpha", "beta", "gamma"}})
}

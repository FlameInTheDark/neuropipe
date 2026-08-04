package pipeline

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestEvaluateMath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		nodeType string
		a, b     any
		want     float64
		wantErr  string
	}{
		{name: "add", nodeType: "math:add", a: 2, b: 3, want: 5},
		{name: "subtract", nodeType: "math:subtract", a: 2, b: 3, want: -1},
		{name: "multiply", nodeType: "math:multiply", a: 2.5, b: 4, want: 10},
		{name: "divide", nodeType: "math:divide", a: 9, b: 2, want: 4.5},
		{name: "zero divisor", nodeType: "math:divide", a: 9, b: 0, wantErr: "non-zero B"},
		{name: "invalid input", nodeType: "math:add", a: "nope", b: 1, wantErr: "A must be a finite number"},
		{name: "overflow", nodeType: "math:multiply", a: math.MaxFloat64, b: 2, wantErr: "non-finite result"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputs, err := evaluateMath(test.nodeType, map[string]any{"a": test.a, "b": test.b})
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("evaluateMath() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("evaluateMath() error = %v", err)
			}
			if got, ok := outputs["result"].(float64); !ok || got != test.want {
				t.Fatalf("result = %#v, want %v", outputs["result"], test.want)
			}
		})
	}
}

func TestMathNodesUseOnlyNumberDataPins(t *testing.T) {
	t.Parallel()
	registry := catalog.New()
	for _, nodeType := range []string{"math:add", "math:subtract", "math:multiply", "math:divide"} {
		definition, exists := registry.Get(nodeType)
		if !exists {
			t.Fatalf("missing %s", nodeType)
		}
		if definition.Mode != domain.NodePure || len(definition.Inputs) != 2 || len(definition.Outputs) != 1 {
			t.Fatalf("%s definition = %#v", nodeType, definition)
		}
		for _, pin := range append(definition.Inputs, definition.Outputs...) {
			if pin.Kind != domain.PinData || pin.DataType != domain.DataNumber {
				t.Fatalf("%s pin %#v is not numeric data", nodeType, pin)
			}
		}
		if got := definition.DefaultConfig; got["a"] != 0.0 || got["b"] != 0.0 {
			t.Fatalf("%s defaults = %#v, want numeric A and B defaults", nodeType, got)
		}
	}
}

func TestMathNodeUsesManualValuesWhenPinsAreUnconnected(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("first", "data:constant", map[string]any{"value": 0}),
		v2Node("sum", "math:add", map[string]any{"a": 2.5, "b": 3.5}),
		v2Node("loop", "flow:for_loop", nil),
	}, Edges: []domain.FlowEdge{
		execEdge("start-loop", "start", "out", "loop", "in"),
		dataEdge("first-loop", "first", "value", "loop", "first"),
		dataEdge("sum-loop", "sum", "result", "loop", "last"),
	}}

	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, run := range result.NodeRuns {
		if run.NodeID != "sum" {
			continue
		}
		if got, ok := run.Output.(map[string]any)["result"].(float64); !ok || got != 6 {
			t.Fatalf("manual math result = %#v, want 6", run.Output)
		}
		return
	}
	t.Fatal("manual math node was not evaluated")
}

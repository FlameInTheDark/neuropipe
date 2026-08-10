package text_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	casechange "github.com/FlameInTheDark/neuropipe/internal/nodes/text/case"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/contains"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/endswith"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/indexof"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/join"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/replace"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/split"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/startswith"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/substring"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/trim"
)

func invoke(t *testing.T, node nodes.Node, values map[string]any) map[string]any {
	t.Helper()
	result, err := node.Execute(context.Background(), nodes.Invocation{Inputs: values}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return result.Outputs
}

func TestTextNodeModulesHaveStrictContractsAndExpectedValues(t *testing.T) {
	tests := []struct {
		name         string
		node         nodes.Node
		values, want map[string]any
	}{
		{"split preserves empties", split.New(), map[string]any{"text": "a,,b", "separator": ","}, map[string]any{"parts": []string{"a", "", "b"}, "count": 3}},
		{"join exact list", join.New(), map[string]any{"parts": []any{"a", "b"}, "separator": "/"}, map[string]any{"text": "a/b"}},
		{"contains", contains.New(), map[string]any{"text": "Neuropipe", "value": "uro"}, map[string]any{"contains": true}},
		{"replace count", replace.New(), map[string]any{"text": "a-a-a", "find": "a", "replacement": "x", "mode": "count", "count": 2}, map[string]any{"text": "x-x-a", "replacements": 2, "changed": true}},
		{"trim unicode", trim.New(), map[string]any{"text": "\u00a0 hi \t"}, map[string]any{"text": "hi"}},
		{"case", casechange.New(), map[string]any{"text": "hello world", "mode": "title"}, map[string]any{"text": "Hello World"}},
		{"starts with", startswith.New(), map[string]any{"text": "Neuropipe", "prefix": "Neuro"}, map[string]any{"matches": true}},
		{"ends with", endswith.New(), map[string]any{"text": "Neuropipe", "suffix": "pipe"}, map[string]any{"matches": true}},
		{"unicode index", indexof.New(), map[string]any{"text": "héllo", "value": "l"}, map[string]any{"index": 2, "found": true}},
		{"unicode substring", substring.New(), map[string]any{"text": "héllo", "start": 1, "length": 2}, map[string]any{"text": "él"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := invoke(t, test.node, test.values); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("outputs = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestTextNodesRejectInvalidRangesAndImplicitValues(t *testing.T) {
	if _, err := substring.New().Execute(context.Background(), nodes.Invocation{Inputs: map[string]any{"text": "hi", "start": 1, "length": 2}}, nil); err == nil {
		t.Fatal("substring accepted an invalid range")
	}
	if _, err := replace.New().Execute(context.Background(), nodes.Invocation{Inputs: map[string]any{"text": "a", "find": "", "replacement": "x", "mode": "all", "count": 1}}, nil); err == nil {
		t.Fatal("replace accepted empty find value")
	}
	if _, err := join.New().Execute(context.Background(), nodes.Invocation{Inputs: map[string]any{"parts": []any{"a", 1}, "separator": ","}}, nil); err == nil {
		t.Fatal("join accepted a non-text list element")
	}
}

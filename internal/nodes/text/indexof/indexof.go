// Package indexof registers the strict Unicode-aware Text Index Of node.
package indexof

import (
	"context"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	textnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/text"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                        { return Node{Metadata: definition(), Executor: nodes.Outputs(Evaluate)} }
func Register(r nodes.Registrar) error { return r.Register(New()) }
func Evaluate(ctx context.Context, i nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	text, e := textnodes.String(i.Inputs, "text")
	if e != nil {
		return nil, e
	}
	value, e := textnodes.String(i.Inputs, "value")
	if e != nil {
		return nil, e
	}
	at := strings.Index(text, value)
	if at < 0 {
		return map[string]any{"index": -1, "found": false}, ctx.Err()
	}
	return map[string]any{"index": textnodes.RuneOffset(text, at), "found": true}, ctx.Err()
}
func definition() domain.NodeDefinition {
	return textnodes.Definition(
		"text:index_of",
		"Index Of",
		"Find a value using a Unicode code-point offset.",
		"list-search",
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinInput, true),
			textnodes.TextPin("value", "Value", domain.PinInput, true),
		},
		[]domain.NodePort{
			textnodes.IntPin("index", "Index", domain.PinOutput, false),
			textnodes.BoolPin("found", "Found", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{
				Name:     "value",
				Label:    "Value",
				Kind:     "string",
				Required: true,
			},
		},
		map[string]any{
			"value": "",
		},
	)
}

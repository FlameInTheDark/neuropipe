// Package contains registers the strict Text Contains node.
package contains

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
	return map[string]any{"contains": strings.Contains(text, value)}, ctx.Err()
}
func definition() domain.NodeDefinition {
	return textnodes.Definition(
		"text:contains",
		"Contains",
		"Report whether text contains an exact text value.",
		"search",
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinInput, true),
			textnodes.TextPin("value", "Value", domain.PinInput, true),
		},
		[]domain.NodePort{
			textnodes.BoolPin("contains", "Contains", domain.PinOutput, false),
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

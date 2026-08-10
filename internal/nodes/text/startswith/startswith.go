// Package startswith registers the strict Text Starts With node.
package startswith

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
	prefix, e := textnodes.String(i.Inputs, "prefix")
	if e != nil {
		return nil, e
	}
	return map[string]any{"matches": strings.HasPrefix(text, prefix)}, ctx.Err()
}
func definition() domain.NodeDefinition {
	return textnodes.Definition(
		"text:starts_with",
		"Starts With",
		"Report whether text starts with an exact prefix.",
		"move-right",
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinInput, true),
			textnodes.TextPin("prefix", "Prefix", domain.PinInput, true),
		},
		[]domain.NodePort{
			textnodes.BoolPin("matches", "Matches", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{
				Name:     "prefix",
				Label:    "Prefix",
				Kind:     "string",
				Required: true,
			},
		},
		map[string]any{
			"prefix": "",
		},
	)
}

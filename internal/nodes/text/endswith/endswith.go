// Package endswith registers the strict Text Ends With node.
package endswith

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
	suffix, e := textnodes.String(i.Inputs, "suffix")
	if e != nil {
		return nil, e
	}
	return map[string]any{"matches": strings.HasSuffix(text, suffix)}, ctx.Err()
}
func definition() domain.NodeDefinition {
	return textnodes.Definition(
		"text:ends_with",
		"Ends With",
		"Report whether text ends with an exact suffix.",
		"move-left",
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinInput, true),
			textnodes.TextPin("suffix", "Suffix", domain.PinInput, true),
		},
		[]domain.NodePort{
			textnodes.BoolPin("matches", "Matches", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{
				Name:     "suffix",
				Label:    "Suffix",
				Kind:     "string",
				Required: true,
			},
		},
		map[string]any{
			"suffix": "",
		},
	)
}

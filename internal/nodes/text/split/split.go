// Package split registers the strict Text Split node.
package split

import (
	"context"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	textnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/text"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Executor: nodes.Outputs(Evaluate)} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }
func Evaluate(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, err := textnodes.String(invocation.Inputs, "text")
	if err != nil {
		return nil, err
	}
	separator, err := textnodes.String(invocation.Inputs, "separator")
	if err != nil {
		return nil, err
	}
	parts := strings.Split(value, separator)
	return map[string]any{"parts": parts, "count": len(parts)}, nil
}
func definition() domain.NodeDefinition {
	return textnodes.Definition(
		"text:split",
		"Split",
		"Split text at an exact separator and preserve empty segments.",
		"split",
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinInput, true),
			textnodes.TextPin("separator", "Separator", domain.PinInput, true),
		},
		[]domain.NodePort{
			textnodes.TextListPin("parts", "Parts", domain.PinOutput, false),
			textnodes.IntPin("count", "Count", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{
				Name:        "separator",
				Label:       "Separator",
				Kind:        "string",
				Placeholder: ",",
				Required:    true,
			},
		},
		map[string]any{
			"separator": ",",
		},
	)
}

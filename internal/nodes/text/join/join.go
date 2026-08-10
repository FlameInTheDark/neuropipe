// Package join registers the strict Text Join node.
package join

import (
	"context"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	textnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/text"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Executor: nodes.Outputs(Evaluate)} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }
func Evaluate(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	values, err := textnodes.Strings(invocation.Inputs, "parts")
	if err != nil {
		return nil, err
	}
	separator, err := textnodes.String(invocation.Inputs, "separator")
	if err != nil {
		return nil, err
	}
	result := ""
	for index, value := range values {
		if index > 0 {
			result += separator
		}
		result += value
	}
	return map[string]any{"text": result}, ctx.Err()
}
func definition() domain.NodeDefinition {
	return textnodes.Definition(
		"text:join",
		"Join",
		"Join a list of text values with an exact separator.",
		"list-end",
		[]domain.NodePort{
			textnodes.TextListPin("parts", "Parts", domain.PinInput, true),
			textnodes.TextPin("separator", "Separator", domain.PinInput, true),
		},
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinOutput, false),
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

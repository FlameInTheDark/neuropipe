// Package change registers the strict Text Change Case node.
package change

import (
	"context"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	textnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/text"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                        { return Node{Metadata: definition(), Executor: nodes.Outputs(Evaluate)} }
func Register(r nodes.Registrar) error { return r.Register(New()) }
func Evaluate(ctx context.Context, i nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	value, e := textnodes.String(i.Inputs, "text")
	if e != nil {
		return nil, e
	}
	mode, e := textnodes.String(i.Inputs, "mode")
	if e != nil {
		return nil, e
	}
	switch mode {
	case "lower":
		value = strings.ToLower(value)
	case "upper":
		value = strings.ToUpper(value)
	case "title":
		value = cases.Title(language.Und).String(value)
	default:
		return nil, fmt.Errorf("case mode %q is invalid", mode)
	}
	return map[string]any{"text": value}, ctx.Err()
}
func definition() domain.NodeDefinition {
	return textnodes.Definition(
		"text:change_case",
		"Change Case",
		"Change text case without converting its type.",
		"case-sensitive",
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinInput, true),
			textnodes.TextPin("mode", "Mode", domain.PinInput, true),
		},
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{
				Name:  "mode",
				Label: "Case",
				Kind:  "select",
				Options: []domain.Option{
					{Value: "lower", Label: "Lower"},
					{Value: "upper", Label: "Upper"},
					{Value: "title", Label: "Title"},
				},
				Required: true,
			},
		},
		map[string]any{
			"mode": "lower",
		},
	)
}

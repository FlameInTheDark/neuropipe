// Package trim registers the strict Text Trim node.
package trim

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
	value, e := textnodes.String(i.Inputs, "text")
	if e != nil {
		return nil, e
	}
	return map[string]any{"text": strings.TrimSpace(value)}, ctx.Err()
}
func definition() domain.NodeDefinition {
	return textnodes.Definition(
		"text:trim",
		"Trim",
		"Remove leading and trailing Unicode whitespace.",
		"text-select",
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinInput, true),
		},
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinOutput, false),
		},
		nil,
		map[string]any{},
	)
}

// Package regexsplit registers the RE2 split node.
package regexsplit

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regex"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// New creates the Regex Split module implementation.
func New() Node {
	return Node{Metadata: definition(), Executor: nodes.Outputs(evaluate)}
}

// Register contributes the complete Regex Split module to the node registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func evaluate(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("regex split cancelled: %w", err)
	}
	text, err := regex.Text(invocation.Inputs, "text")
	if err != nil {
		return nil, err
	}
	pattern, err := regex.Text(invocation.Inputs, "pattern")
	if err != nil {
		return nil, err
	}
	expression, err := regex.Compile(pattern)
	if err != nil {
		return nil, err
	}
	splits := len(expression.FindAllStringIndex(text, -1))
	parts := expression.Split(text, -1)
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("regex split cancelled: %w", err)
	}
	return map[string]any{"parts": parts, "splits": splits, "matched": splits > 0}, nil
}

func definition() domain.NodeDefinition {
	return datanodes.Node(
		"data:regex_split",
		"Data",
		"Regex Split",
		"Split text at every Go RE2 match while preserving empty leading and trailing parts.",
		"regex",
		[]domain.NodePort{
			regex.TextPin("text", "Text", domain.PinInput, true),
			regex.TextPin("pattern", "Pattern", domain.PinInput, true),
		},
		[]domain.NodePort{
			regex.StringListPin("parts", "Parts"),
			regex.IntPin("splits", "Splits"),
			regex.BoolPin("matched", "Matched"),
		},
		[]domain.ConfigField{{Name: "pattern", Label: "Pattern", Kind: "string", Placeholder: `\s+`, Required: true}},
		map[string]any{"pattern": `\s+`},
	)
}

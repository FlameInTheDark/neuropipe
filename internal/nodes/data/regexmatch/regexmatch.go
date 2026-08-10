// Package regexmatch registers the structured RE2 match extraction node.
package regexmatch

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

// New creates the Regex Match module implementation.
func New() Node {
	return Node{Metadata: definition(), Executor: nodes.Outputs(evaluate)}
}

// Register contributes the complete Regex Match module to the node registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func evaluate(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("regex match cancelled: %w", err)
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
	matches := regex.Matches(expression, text)
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("regex match cancelled: %w", err)
	}
	return map[string]any{"matched": len(matches) > 0, "count": len(matches), "matches": matches}, nil
}

func definition() domain.NodeDefinition {
	return datanodes.Node(
		"data:regex_match",
		"Data",
		"Regex Match",
		"Test text against a Go RE2 pattern and return structured matches and captures.",
		"regex",
		[]domain.NodePort{
			regex.TextPin("text", "Text", domain.PinInput, true),
			regex.TextPin("pattern", "Pattern", domain.PinInput, true),
		},
		[]domain.NodePort{
			regex.BoolPin("matched", "Matched"),
			regex.IntPin("count", "Count"),
			regex.MatchListPin("matches", "Matches"),
		},
		[]domain.ConfigField{{Name: "pattern", Label: "Pattern", Kind: "string", Placeholder: `(?P<word>\w+)`, Required: true}},
		map[string]any{"pattern": `(?P<word>\w+)`},
	)
}

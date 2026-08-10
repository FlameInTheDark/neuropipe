// Package regexreplace registers the RE2 replace-all node.
package regexreplace

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

// New creates the Regex Replace module implementation.
func New() Node {
	return Node{Metadata: definition(), Executor: nodes.Outputs(evaluate)}
}

// Register contributes the complete Regex Replace module to the node registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func evaluate(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("regex replace cancelled: %w", err)
	}
	text, err := regex.Text(invocation.Inputs, "text")
	if err != nil {
		return nil, err
	}
	pattern, err := regex.Text(invocation.Inputs, "pattern")
	if err != nil {
		return nil, err
	}
	replacement, err := regex.Text(invocation.Inputs, "replacement")
	if err != nil {
		return nil, err
	}
	expression, err := regex.Compile(pattern)
	if err != nil {
		return nil, err
	}
	replacements := len(expression.FindAllStringIndex(text, -1))
	result := expression.ReplaceAllString(text, replacement)
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("regex replace cancelled: %w", err)
	}
	return map[string]any{"text": result, "replacements": replacements, "changed": result != text}, nil
}

func definition() domain.NodeDefinition {
	return datanodes.Node(
		"data:regex_replace",
		"Data",
		"Regex Replace",
		"Replace every Go RE2 match using explicit Go capture expansion syntax.",
		"regex",
		[]domain.NodePort{
			regex.TextPin("text", "Text", domain.PinInput, true),
			regex.TextPin("pattern", "Pattern", domain.PinInput, true),
			regex.TextPin("replacement", "Replacement", domain.PinInput, true),
		},
		[]domain.NodePort{
			regex.TextPin("text", "Text", domain.PinOutput, false),
			regex.IntPin("replacements", "Replacements"),
			regex.BoolPin("changed", "Changed"),
		},
		[]domain.ConfigField{
			{Name: "pattern", Label: "Pattern", Kind: "string", Placeholder: `\s+`, Required: true},
			{Name: "replacement", Label: "Replacement", Kind: "string", Placeholder: " ", Required: true},
		},
		map[string]any{"pattern": `\s+`, "replacement": " "},
	)
}

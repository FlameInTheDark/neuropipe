// Package replace registers the strict Text Replace node.
package replace

import (
	"context"
	"fmt"
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
	find, e := textnodes.String(i.Inputs, "find")
	if e != nil {
		return nil, e
	}
	if find == "" {
		return nil, fmt.Errorf("find must not be empty")
	}
	replacement, e := textnodes.String(i.Inputs, "replacement")
	if e != nil {
		return nil, e
	}
	mode, e := textnodes.String(i.Inputs, "mode")
	if e != nil {
		return nil, e
	}
	limit := 0
	switch mode {
	case "first":
		limit = 1
	case "all":
		limit = -1
	case "count":
		limit, e = textnodes.Int(i.Inputs, "count")
		if e != nil {
			return nil, e
		}
		if limit < 1 {
			return nil, fmt.Errorf("count must be positive")
		}
	default:
		return nil, fmt.Errorf("replace mode %q is invalid", mode)
	}
	matches := strings.Count(text, find)
	if limit >= 0 && matches > limit {
		matches = limit
	}
	result := strings.Replace(text, find, replacement, limit)
	return map[string]any{"text": result, "replacements": matches, "changed": result != text}, ctx.Err()
}
func definition() domain.NodeDefinition {
	return textnodes.Definition(
		"text:replace",
		"Replace",
		"Replace the first, an exact positive count, or all exact text matches.",
		"replace",
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinInput, true),
			textnodes.TextPin("find", "Find", domain.PinInput, true),
			textnodes.TextPin("replacement", "Replacement", domain.PinInput, true),
			textnodes.TextPin("mode", "Mode", domain.PinInput, true),
			textnodes.IntPin("count", "Count", domain.PinInput, false),
		},
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinOutput, false),
			textnodes.IntPin("replacements", "Replacements", domain.PinOutput, false),
			textnodes.BoolPin("changed", "Changed", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{
				Name:     "find",
				Label:    "Find",
				Kind:     "string",
				Required: true,
			},
			{
				Name:     "replacement",
				Label:    "Replacement",
				Kind:     "string",
				Required: true,
			},
			{
				Name:  "mode",
				Label: "Mode",
				Kind:  "select",
				Options: []domain.Option{
					{Value: "first", Label: "First"},
					{Value: "count", Label: "Count"},
					{Value: "all", Label: "All"},
				},
				Required: true,
			},
			{
				Name:        "count",
				Label:       "Count",
				Kind:        "number",
				VisibleWhen: "mode",
			},
		},
		map[string]any{
			"find":        "",
			"replacement": "",
			"mode":        "all",
			"count":       1,
		},
	)
}

// Package substring registers the strict Unicode-aware Text Substring node.
package substring

import (
	"context"
	"fmt"

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
	start, e := textnodes.Int(i.Inputs, "start")
	if e != nil {
		return nil, e
	}
	length, e := textnodes.Int(i.Inputs, "length")
	if e != nil {
		return nil, e
	}
	runes := textnodes.Runes(text)
	if start < 0 || length < 0 || start > len(runes) || length > len(runes)-start {
		return nil, fmt.Errorf("substring range start=%d length=%d is outside text length %d", start, length, len(runes))
	}
	return map[string]any{"text": string(runes[start : start+length])}, ctx.Err()
}
func definition() domain.NodeDefinition {
	return textnodes.Definition(
		"text:substring",
		"Substring",
		"Read a Unicode code-point range; invalid ranges fail explicitly.",
		"text-cursor-input",
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinInput, true),
			textnodes.IntPin("start", "Start", domain.PinInput, true),
			textnodes.IntPin("length", "Length", domain.PinInput, true),
		},
		[]domain.NodePort{
			textnodes.TextPin("text", "Text", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "start", Label: "Start", Kind: "number", Required: true},
			{Name: "length", Label: "Length", Kind: "number", Required: true},
		},
		map[string]any{"start": 0, "length": 1})
}

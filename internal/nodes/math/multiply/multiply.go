// Package multiply registers the pure Multiply node.
package multiply

import (
	"context"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	mathnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/math"
)

// Node owns Multiply's metadata, static pin contract, and arithmetic operation.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node {
	return Node{
		Metadata: mathnodes.Definition("math:multiply", "Multiply", "Multiply two numeric values.", "x"),
		Executor: execute,
	}
}

func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func execute(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	a, err := mathnodes.FiniteInput(invocation.Inputs, "a")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	b, err := mathnodes.FiniteInput(invocation.Inputs, "b")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	result := a * b
	if err := mathnodes.EnsureFinite("math:multiply", result); err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": result}}, nil
}

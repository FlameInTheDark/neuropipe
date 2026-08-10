// Package divide registers the pure Divide node.
package divide

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	mathnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/math"
)

// Node owns Divide's metadata, static pin contract, and arithmetic operation.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node {
	return Node{
		Metadata: mathnodes.Definition("math:divide", "Divide", "Divide A by B.", "divide"),
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
	if b == 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("divide requires a non-zero B input")
	}
	result := a / b
	if err := mathnodes.EnsureFinite("math:divide", result); err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": result}}, nil
}

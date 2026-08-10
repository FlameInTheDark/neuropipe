package flipflop

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	flow "github.com/FlameInTheDark/neuropipe/internal/nodes/flow"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	outputs := []domain.NodePort{flow.Exec("a", "A", domain.PinOutput), flow.Exec("b", "B", domain.PinOutput)}
	return registrar.Register(Node{Metadata: flow.Node("flow:flip_flop", "Flow", "FlipFlop", "Alternate between A and B on every pulse.", "repeat-1", []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput)}, outputs, nil, map[string]any{}), Executor: Execute})
}

// Execute alternates through a focused per-node state store.
func Execute(_ context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	store, ok := runtime.(nodes.FlipFlopStore)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("flip flop runtime is unavailable")
	}
	if store.NextFlipFlop(invocation.Node.ID) {
		return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"output": "a"}}, Ports: []string{"a"}}, nil
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"output": "b"}}, Ports: []string{"b"}}, nil
}

package returnnode

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
	return registrar.Register(Node{Metadata: flow.Node("flow:return", "Flow", "Return", "Finish the current impure function or pipeline.", "corner-down-left", []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput)}, nil, nil, map[string]any{}), Executor: Execute})
}

// Execute ends the current graph invocation through the narrow ReturnSignaler.
func Execute(_ context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	result, ok := runtime.(nodes.ReturnSignaler)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("return runtime is unavailable")
	}
	values := make(map[string]any, len(invocation.Inputs))
	for key, value := range invocation.Inputs {
		values[key] = value
	}
	result.Return(values)
	return nodes.ExecutionResult{Outputs: map[string]any{"result": values}}, nil
}

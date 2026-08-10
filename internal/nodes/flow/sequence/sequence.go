package sequence

import (
	"context"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	flow "github.com/FlameInTheDark/neuropipe/internal/nodes/flow"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	outputs := []domain.NodePort{flow.Exec("then_0", "Then 0", domain.PinOutput), flow.Exec("then_1", "Then 1", domain.PinOutput)}
	return registrar.Register(Node{Metadata: flow.Node("flow:sequence", "Flow", "Sequence", "Execute each output in order.", "list-ordered", []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput)}, outputs, nil, map[string]any{}), Executor: Execute})
}

// Execute emits Sequence's ports in their declared order.
func Execute(_ context.Context, _ nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{}}, Ports: []string{"then_0", "then_1"}}, nil
}

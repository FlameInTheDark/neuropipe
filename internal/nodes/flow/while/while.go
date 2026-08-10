package while

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
	inputs := []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput), flow.Data("condition", "Condition", domain.PinInput, domain.DataBoolean)}
	outputs := []domain.NodePort{flow.Exec("loop", "Loop Body", domain.PinOutput), flow.Exec("completed", "Completed", domain.PinOutput)}
	return registrar.Register(Node{Metadata: flow.Node("flow:while", "Flow", "While", "Evaluate Condition before each bounded body iteration.", "rotate-cw", inputs, outputs, nil, map[string]any{}), Executor: Execute})
}

// Execute provides the condition contract. The host re-resolves its data pin
// on every iteration so loop traversal remains cancellation-safe.
func Execute(_ context.Context, _ nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	return nodes.ExecutionResult{Loop: &nodes.LoopPlan{ReportedCount: -1, Continue: func(inputs map[string]any) (bool, error) {
		condition, ok := inputs["condition"].(bool)
		if !ok {
			return false, fmt.Errorf("while expects Condition to be Boolean")
		}
		return condition, nil
	}}}, nil
}

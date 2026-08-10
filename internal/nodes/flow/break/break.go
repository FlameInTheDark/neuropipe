package breaknode

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
	return registrar.Register(Node{Metadata: flow.Node("flow:break", "Flow", "Break", "Stop the innermost active loop.", "circle-stop", []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput)}, nil, nil, map[string]any{}), Executor: Execute})
}

// Execute requests that the host's active loop ends after its current body.
func Execute(_ context.Context, _ nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	loop, ok := runtime.(nodes.LoopController)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("break runtime is unavailable")
	}
	if !loop.InLoop() {
		return nodes.ExecutionResult{}, fmt.Errorf("break can only run inside a loop body")
	}
	loop.RequestBreak()
	return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"break": true}}}, nil
}

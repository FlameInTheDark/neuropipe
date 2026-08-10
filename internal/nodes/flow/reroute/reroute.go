package reroute

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
	return registrar.Register(Node{Metadata: flow.Node("flow:reroute", "Flow", "Reroute", "Reorganise an execution wire without changing control flow.", "waypoints", []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput)}, []domain.NodePort{flow.Exec("out", "Then", domain.PinOutput)}, nil, map[string]any{}), Executor: Execute})
}

// Execute preserves the execution path without carrying extra state.
func Execute(_ context.Context, _ nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{}}, Ports: []string{"out"}}, nil
}

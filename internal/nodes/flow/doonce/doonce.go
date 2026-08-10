package doonce

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
	inputs := []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput), flow.Exec("reset", "Reset", domain.PinInput)}
	return registrar.Register(Node{Metadata: flow.Node("flow:do_once", "Flow", "Do Once", "Pass execution only the first time until reset.", "badge-check", inputs, []domain.NodePort{flow.Exec("out", "Then", domain.PinOutput)}, nil, map[string]any{}), Executor: Execute})
}

// Execute owns Do Once's reset and first-pulse semantics through OnceStore.
func Execute(_ context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	store, ok := runtime.(nodes.OnceStore)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("do once runtime is unavailable")
	}
	if invocation.ExecInput == "reset" {
		store.ResetOnce(invocation.Node.ID)
		return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"reset": true}}}, nil
	}
	if !store.ClaimOnce(invocation.Node.ID) {
		return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"alreadyDone": true}}}, nil
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"first": true}}, Ports: []string{"out"}}, nil
}

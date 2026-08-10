package branch

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
	outputs := []domain.NodePort{flow.Exec("true", "True", domain.PinOutput), flow.Exec("false", "False", domain.PinOutput)}
	return registrar.Register(Node{Metadata: flow.Node("flow:branch", "Flow", "Branch", "Route execution from a Boolean data value.", "git-branch", inputs, outputs, nil, map[string]any{}), Executor: Execute})
}

// Execute selects exactly one branch from a canonical Boolean input.
func Execute(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	condition, ok := invocation.Inputs["condition"].(bool)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("branch expects Condition to be Boolean")
	}
	if condition {
		return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"condition": true}}, Ports: []string{"true"}}, nil
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"condition": false}}, Ports: []string{"false"}}, nil
}

package reroute

import (
	"context"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	pins := []domain.NodePort{datanodes.Pin("value", "Value", domain.PinInput, domain.DataAny)}
	outputs := []domain.NodePort{datanodes.Pin("value", "Value", domain.PinOutput, domain.DataAny)}
	return registrar.Register(Node{Metadata: datanodes.Node("data:reroute", "Data", "Reroute", "Reorganise a data wire without changing its value.", "waypoints", pins, outputs, nil, map[string]any{}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate returns the original value without changing its type or shape.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	return map[string]any{"value": invocation.Inputs["value"]}, nil
}

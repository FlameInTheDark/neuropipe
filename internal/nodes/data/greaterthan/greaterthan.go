package greaterthan

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{datanodes.Pin("left", "Left", domain.PinInput, domain.DataNumber), datanodes.Pin("right", "Right", domain.PinInput, domain.DataNumber)}
	return registrar.Register(Node{Metadata: datanodes.Node("data:greater_than", "Data", "Greater Than", "Compare two numeric values.", "chevron-right", inputs, []domain.NodePort{datanodes.Pin("value", "True", domain.PinOutput, domain.DataBoolean)}, nil, map[string]any{}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate compares finite numeric input values with no string parsing.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	left, leftOK := number(invocation.Inputs["left"])
	right, rightOK := number(invocation.Inputs["right"])
	if !leftOK || !rightOK {
		return nil, fmt.Errorf("greater than requires numeric inputs")
	}
	return map[string]any{"value": left > right}, nil
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

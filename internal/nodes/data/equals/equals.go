package equals

import (
	"context"
	"reflect"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{datanodes.Pin("left", "Left", domain.PinInput, domain.DataAny), datanodes.Pin("right", "Right", domain.PinInput, domain.DataAny)}
	return registrar.Register(Node{Metadata: datanodes.Node("data:equals", "Data", "Equals", "Compare two values.", "equal", inputs, []domain.NodePort{datanodes.Pin("value", "Equal", domain.PinOutput, domain.DataBoolean)}, nil, map[string]any{}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate compares values without coercing distinct Go values through text.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	return map[string]any{"value": reflect.DeepEqual(invocation.Inputs["left"], invocation.Inputs["right"])}, nil
}

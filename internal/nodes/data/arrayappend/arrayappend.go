package arrayappend

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
	inputs := []domain.NodePort{datanodes.Pin("array", "Array", domain.PinInput, domain.DataList), datanodes.Pin("value", "Value", domain.PinInput, domain.DataAny)}
	return registrar.Register(Node{Metadata: datanodes.Node("data:array_append", "Data", "Append to Array", "Append a value to a list and return the new list.", "list-plus", inputs, []domain.NodePort{datanodes.Pin("array", "Array", domain.PinOutput, domain.DataList)}, nil, map[string]any{}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate appends without mutating the input list cached by the graph host.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	items, ok := invocation.Inputs["array"].([]any)
	if !ok {
		return nil, fmt.Errorf("append to array requires an Array list")
	}
	result := make([]any, 0, len(items)+1)
	result = append(result, items...)
	result = append(result, invocation.Inputs["value"])
	return map[string]any{"array": result}, nil
}

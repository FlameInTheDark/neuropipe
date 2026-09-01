// Package arrayreverse registers the strict Reverse Array node.
package arrayreverse

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

// Register contributes the Reverse Array node: it returns a copy of the list
// with the element order flipped. The input list is never mutated, so other
// consumers of the same array pin keep the original order.
func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{datanodes.Pin("array", "Array", domain.PinInput, domain.DataList)}
	return registrar.Register(Node{Metadata: datanodes.Node("data:array_reverse", "Arrays", "Reverse Array",
		"Reverse the order of a list's elements.", "arrow-right-left", inputs,
		[]domain.NodePort{datanodes.Pin("array", "Array", domain.PinOutput, domain.DataList)},
		nil, map[string]any{}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate builds the reversed copy in one pass.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	items, ok := invocation.Inputs["array"].([]any)
	if !ok {
		return nil, fmt.Errorf("reverse array requires an Array list")
	}
	result := make([]any, len(items))
	for index := range items {
		result[len(items)-1-index] = items[index]
	}
	return map[string]any{"array": result}, nil
}

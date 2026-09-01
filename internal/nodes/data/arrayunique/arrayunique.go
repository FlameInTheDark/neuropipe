// Package arrayunique registers the strict Unique Array node.
package arrayunique

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// Register contributes the Unique Array node: it removes duplicate values,
// keeping each value's first occurrence and the original order of the
// survivors. Values of any kind can participate — two elements count as
// duplicates when they serialize to the same JSON.
func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{datanodes.Pin("array", "Array", domain.PinInput, domain.DataList)}
	return registrar.Register(Node{Metadata: datanodes.Node("data:array_unique", "Arrays", "Unique Array",
		"Remove duplicate values from a list, keeping each value's first occurrence.",
		"fingerprint", inputs,
		[]domain.NodePort{datanodes.Pin("array", "Array", domain.PinOutput, domain.DataList)},
		nil, map[string]any{}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate walks the list once, hashing each element's JSON encoding as its
// identity. Numbers that are equal but carried in different number kinds
// (1 and 1.0) hash the same; text and the number 1 stay distinct.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	items, ok := invocation.Inputs["array"].([]any)
	if !ok {
		return nil, fmt.Errorf("unique array requires an Array list")
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]any, 0, len(items))
	for _, item := range items {
		encoded, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("unique array cannot compare an element: %w", err)
		}
		key := string(encoded)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return map[string]any{"array": result}, nil
}

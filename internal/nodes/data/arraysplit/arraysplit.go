// Package arraysplit registers the strict Split Array node.
package arraysplit

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

// defaultSize is the batch size used when neither a wire nor the inspector
// supplies one, so the node stays usable straight from the palette.
const defaultSize = 10.0

// Register contributes the Split Array node: it cuts a list into consecutive
// batches of a fixed size. The final batch may be shorter; an empty list
// produces no batches.
func Register(registrar nodes.Registrar) error {
	size := datanodes.Pin("size", "Size", domain.PinInput, domain.DataNumber)
	size.Default = defaultSize
	inputs := []domain.NodePort{
		datanodes.Pin("array", "Array", domain.PinInput, domain.DataList),
		size,
	}
	return registrar.Register(Node{Metadata: datanodes.Node("data:array_split", "Arrays", "Split Array",
		"Split a list into batches of a fixed size.", "split", inputs,
		[]domain.NodePort{datanodes.Pin("arrays", "Arrays", domain.PinOutput, domain.DataList)},
		[]domain.ConfigField{datanodes.Field("size", "Size", "number", "10", false)},
		map[string]any{"size": defaultSize}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate cuts consecutive batches. A wired Size pin wins over the inspector
// value, which in turn wins over the default of ten.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	items, ok := invocation.Inputs["array"].([]any)
	if !ok {
		return nil, fmt.Errorf("split array requires an Array list")
	}
	size, err := wholeNumber(invocation, "size", defaultSize)
	if err != nil {
		return nil, err
	}
	if size <= 0 {
		return nil, fmt.Errorf("split array requires a Size of at least one element")
	}
	batches := make([]any, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		batch := make([]any, end-start)
		copy(batch, items[start:end])
		batches = append(batches, batch)
	}
	return map[string]any{"arrays": batches}, nil
}

// wholeNumber resolves one numeric setting from the wired pin, then the
// inspector config, then the fallback — rejecting fractions and non-numbers.
func wholeNumber(invocation nodes.Invocation, key string, fallback float64) (int, error) {
	value, exists := invocation.Inputs[key]
	if !exists || value == nil {
		configured, ok := invocation.Config[key]
		if ok && configured != nil {
			value = configured
		} else {
			value = fallback
		}
	}
	number, ok := numeric(value)
	if !ok {
		return 0, fmt.Errorf("split array requires a numeric %s", label(key))
	}
	if number != float64(int(number)) {
		return 0, fmt.Errorf("split array requires a whole-number %s, got %v", label(key), number)
	}
	return int(number), nil
}

// label names a setting in error messages.
func label(key string) string {
	if key == "size" {
		return "Size"
	}
	return key
}

// numeric normalizes every number kind the engine can deliver.
func numeric(value any) (float64, bool) {
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

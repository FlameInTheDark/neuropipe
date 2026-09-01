// Package arrayslice registers the strict Slice Array node.
package arrayslice

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

// Register contributes the Slice Array node: it cuts a section out of a list
// starting at a zero-based Start index with an optional Count length. Without
// a Count the slice runs to the end of the list, so the node doubles as a
// skip operation.
func Register(registrar nodes.Registrar) error {
	start := datanodes.Pin("start", "Start", domain.PinInput, domain.DataNumber)
	start.Default = 0.0
	count := datanodes.Pin("count", "Count", domain.PinInput, domain.DataNumber)
	inputs := []domain.NodePort{
		datanodes.Pin("array", "Array", domain.PinInput, domain.DataList),
		start,
		count,
	}
	return registrar.Register(Node{Metadata: datanodes.Node("data:array_slice", "Arrays", "Slice Array",
		"Cut a section out of a list from a start index with an optional length.",
		"scissors", inputs,
		[]domain.NodePort{datanodes.Pin("array", "Array", domain.PinOutput, domain.DataList)},
		[]domain.ConfigField{
			datanodes.Field("start", "Start", "number", "0", false),
			datanodes.Field("count", "Count", "number", "all remaining elements", false),
		},
		map[string]any{"start": 0.0}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate copies the requested section. A Start beyond the list length
// yields an empty result, and a Count that runs past the end is clamped, so
// slicing never fails on bounds — only on non-numeric or negative settings.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	items, ok := invocation.Inputs["array"].([]any)
	if !ok {
		return nil, fmt.Errorf("slice array requires an Array list")
	}
	start, err := wholeNumber(invocation, "start", 0.0)
	if err != nil {
		return nil, err
	}
	if start < 0 {
		return nil, fmt.Errorf("slice array requires a Start of zero or more")
	}
	if start > len(items) {
		start = len(items)
	}
	end := len(items)
	if configured(invocation, "count") {
		count, err := wholeNumber(invocation, "count", 0.0)
		if err != nil {
			return nil, err
		}
		if count < 0 {
			return nil, fmt.Errorf("slice array requires a Count of zero or more")
		}
		end = start + count
		if end > len(items) {
			end = len(items)
		}
	}
	result := make([]any, end-start)
	copy(result, items[start:end])
	return map[string]any{"array": result}, nil
}

// configured reports whether a setting arrives from a wire or the inspector.
func configured(invocation nodes.Invocation, key string) bool {
	if value, exists := invocation.Inputs[key]; exists && value != nil {
		return true
	}
	if value, ok := invocation.Config[key]; ok && value != nil {
		return true
	}
	return false
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
		return 0, fmt.Errorf("slice array requires a numeric %s", key)
	}
	if number != float64(int(number)) {
		return 0, fmt.Errorf("slice array requires a whole-number %s, got %v", key, number)
	}
	return int(number), nil
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

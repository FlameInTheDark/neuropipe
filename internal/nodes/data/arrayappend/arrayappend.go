// Package arrayappend registers the strict Append to Array node.
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

// Register contributes the Append to Array node. The Append mode decides what
// the Value input means: "item" appends the value as one element (an array
// value nests as a single element), while "array" concatenates the wired
// list's elements onto the base list — appending one array to another.
func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{
		datanodes.Pin("array", "Array", domain.PinInput, domain.DataList),
		datanodes.Pin("value", "Value", domain.PinInput, domain.DataAny),
	}
	fields := []domain.ConfigField{{
		Name:     "mode",
		Label:    "Append",
		Kind:     "select",
		Required: true,
		Options: []domain.Option{
			{Value: "item", Label: "Single item"},
			{Value: "array", Label: "Array elements"},
		},
	}}
	return registrar.Register(Node{Metadata: datanodes.Node("data:array_append", "Arrays", "Append to Array",
		"Append one value or a whole list of elements onto a list.", "list-plus", inputs,
		[]domain.NodePort{datanodes.Pin("array", "Array", domain.PinOutput, domain.DataList)},
		fields, map[string]any{"mode": "item"}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate appends without mutating the input list cached by the graph host.
// Item mode appends the value as one element; array mode requires a list and
// concatenates its elements.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	items, ok := invocation.Inputs["array"].([]any)
	if !ok {
		return nil, fmt.Errorf("append to array requires an Array list")
	}
	appendMode, err := mode(invocation)
	if err != nil {
		return nil, err
	}
	result := make([]any, 0, len(items)+1)
	result = append(result, items...)
	if appendMode == "array" {
		extra, ok := invocation.Inputs["value"].([]any)
		if !ok {
			return nil, fmt.Errorf("appending an array requires the Value input to be an Array list")
		}
		result = append(result, extra...)
		return map[string]any{"array": result}, nil
	}
	result = append(result, invocation.Inputs["value"])
	return map[string]any{"array": result}, nil
}

// mode resolves the configured append mode. Legacy graphs that predate the
// field fall back to the single-item default; unknown values are hard errors
// so a typo never silently picks a behavior.
func mode(invocation nodes.Invocation) (string, error) {
	value, configured := invocation.Config["mode"].(string)
	if !configured || value == "" {
		if fallback, ok := invocation.Definition.DefaultConfig["mode"].(string); ok && fallback != "" {
			value = fallback
		} else {
			value = "item"
		}
	}
	switch value {
	case "item", "array":
		return value, nil
	default:
		return "", fmt.Errorf("append mode %q is not one of: Single item, Array elements", value)
	}
}

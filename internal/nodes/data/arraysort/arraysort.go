// Package arraysort registers the strict Sort Array node.
package arraysort

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// Register contributes the Sort Array node: numbers sort numerically, text
// sorts lexicographically, and Booleans sort false before true. Mixed scalar
// lists sort by type rank (numbers, then text, then Booleans) so the result
// is deterministic; objects, lists, bytes, and null fail with the kind named.
func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{datanodes.Pin("array", "Array", domain.PinInput, domain.DataList)}
	return registrar.Register(Node{Metadata: datanodes.Node("data:array_sort", "Arrays", "Sort Array",
		"Sort a list's number, text, or Boolean elements in ascending or descending order.",
		"chevrons-up-down", inputs,
		[]domain.NodePort{datanodes.Pin("array", "Array", domain.PinOutput, domain.DataList)},
		[]domain.ConfigField{datanodes.Select("order", "Order", "ascending", "descending")},
		map[string]any{"order": "ascending"}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate returns a sorted copy; equal elements keep their original order.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	items, ok := invocation.Inputs["array"].([]any)
	if !ok {
		return nil, fmt.Errorf("sort array requires an Array list")
	}
	direction, err := order(invocation)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if _, _, err := rank(item); err != nil {
			return nil, err
		}
	}
	sorted := append(make([]any, 0, len(items)), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		comparison := compare(sorted[i], sorted[j])
		if direction == "descending" {
			return comparison > 0
		}
		return comparison < 0
	})
	return map[string]any{"array": sorted}, nil
}

// order resolves the configured sort direction with the ascending default.
func order(invocation nodes.Invocation) (string, error) {
	value, configured := invocation.Config["order"].(string)
	if !configured || value == "" {
		value = "ascending"
	}
	switch value {
	case "ascending", "descending":
		return value, nil
	default:
		return "", fmt.Errorf("sort order %q is not ascending or descending", value)
	}
}

// compare orders two already-ranked elements: within one type by value,
// across scalar types by rank (number, then text, then Boolean).
func compare(left, right any) int {
	leftRank, leftValue, _ := rank(left)
	rightRank, rightValue, _ := rank(right)
	if leftRank != rightRank {
		return leftRank - rightRank
	}
	switch leftValue.(type) {
	case string:
		return strings.Compare(leftValue.(string), rightValue.(string))
	case bool:
		if leftValue.(bool) == rightValue.(bool) {
			return 0
		}
		if leftValue.(bool) {
			return 1
		}
		return -1
	default:
		difference := numberValue(leftValue) - numberValue(rightValue)
		if difference < 0 {
			return -1
		}
		if difference > 0 {
			return 1
		}
		return 0
	}
}

// rank classifies one element as a comparable scalar. It returns the type
// rank and a normalized value (float64 for numbers) for compare.
func rank(value any) (int, any, error) {
	switch typed := value.(type) {
	case float64:
		return 0, typed, nil
	case float32:
		return 0, float64(typed), nil
	case int:
		return 0, float64(typed), nil
	case int64:
		return 0, float64(typed), nil
	case string:
		return 1, typed, nil
	case bool:
		return 2, typed, nil
	default:
		return 0, nil, fmt.Errorf("sort array cannot order %s elements", kindOf(value))
	}
}

// kindOf names an unsupported element kind for the error message.
func kindOf(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "list"
	case []byte:
		return "bytes"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// numberValue extracts the numeric payload of a ranked element.
func numberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

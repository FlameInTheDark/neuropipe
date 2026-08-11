package equals

import (
	"context"
	"encoding/json"
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

// Evaluate compares values without coercing textual values through numbers or
// strings into each other. JSON has a single number domain, so two numeric
// values compare by value regardless of which Go integer or float kind holds
// them; every other type pair compares structurally.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	left, leftNumber := numericValue(invocation.Inputs["left"])
	right, rightNumber := numericValue(invocation.Inputs["right"])
	if leftNumber && rightNumber {
		return map[string]any{"value": left == right}, nil
	}
	return map[string]any{"value": reflect.DeepEqual(invocation.Inputs["left"], invocation.Inputs["right"])}, nil
}

// numericValue reports whether value belongs to JSON's number domain and
// returns its canonical float64 form for comparison.
func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

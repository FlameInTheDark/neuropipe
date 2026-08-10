package arrayget

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
	index := datanodes.Pin("index", "Index", domain.PinInput, domain.DataNumber)
	index.Default = 0.0
	inputs := []domain.NodePort{datanodes.Pin("array", "Array", domain.PinInput, domain.DataList), index}
	return registrar.Register(Node{Metadata: datanodes.Node("data:array_get", "Data", "Pick from Array", "Read the element at a zero-based index from a list.", "list", inputs, []domain.NodePort{datanodes.Pin("value", "Value", domain.PinOutput, domain.DataAny)}, []domain.ConfigField{datanodes.Field("index", "Index", "number", "0", false)}, map[string]any{"index": 0.0}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate reads one zero-based item from a JSON-compatible list.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	items, ok := invocation.Inputs["array"].([]any)
	if !ok {
		return nil, fmt.Errorf("pick requires an Array list")
	}
	index, ok := integer(invocation.Inputs["index"])
	if !ok {
		return nil, fmt.Errorf("pick requires a numeric Index")
	}
	if index < 0 || index >= len(items) {
		return nil, fmt.Errorf("array index %d is out of range (length %d)", index, len(items))
	}
	return map[string]any{"value": items[index]}, nil
}

func integer(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), number == float64(int(number))
	case float32:
		return int(number), number == float32(int(number))
	case int:
		return number, true
	case int64:
		return int(number), int64(int(number)) == number
	default:
		return 0, false
	}
}

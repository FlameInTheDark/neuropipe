package gettype

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
	outputs := []domain.NodePort{datanodes.Pin("type", "Type", domain.PinOutput, domain.DataText), datanodes.Pin("elementType", "Element Type", domain.PinOutput, domain.DataText)}
	return registrar.Register(Node{Metadata: datanodes.Node("data:get_type", "Data", "Get Type", "Report the JSON type of a value and the element type of a list.", "type", []domain.NodePort{datanodes.Pin("value", "Value", domain.PinInput, domain.DataAny)}, outputs, nil, map[string]any{}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate reports stable graph value kinds without converting the value.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	value := invocation.Inputs["value"]
	valueType := valueKind(value)
	elementType := ""
	if items, ok := value.([]any); ok {
		elementType = elementKind(items)
	}
	return map[string]any{"type": valueType, "elementType": elementType}, nil
}

func valueKind(value any) string {
	if value == nil {
		return "null"
	}
	switch value.(type) {
	case string:
		return "text"
	case bool:
		return "boolean"
	case []any:
		return "list"
	case float64, float32, int, int64:
		return "number"
	}
	resolved := dereference(reflect.ValueOf(value))
	if resolved.IsValid() && (resolved.Kind() == reflect.Struct || (resolved.Kind() == reflect.Map && resolved.Type().Key().Kind() == reflect.String)) {
		return "object"
	}
	return "unknown"
}

func elementKind(items []any) string {
	if len(items) == 0 {
		return "any"
	}
	kind := valueKind(items[0])
	for _, item := range items[1:] {
		if valueKind(item) != kind {
			return "mixed"
		}
	}
	return kind
}

func dereference(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

package length

import (
	"context"
	"fmt"
	"reflect"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	return registrar.Register(Node{Metadata: datanodes.Node("data:length", "Data", "Length", "Count list elements, text characters, or object keys.", "ruler", []domain.NodePort{datanodes.Pin("value", "Value", domain.PinInput, domain.DataAny)}, []domain.NodePort{datanodes.Pin("length", "Length", domain.PinOutput, domain.DataNumber)}, nil, map[string]any{}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate counts UTF-8 characters, list elements, or string-keyed object
// members without coercing another Go value to one of those forms.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	value := invocation.Inputs["value"]
	switch typed := value.(type) {
	case []any:
		return map[string]any{"length": float64(len(typed))}, nil
	case string:
		return map[string]any{"length": float64(utf8.RuneCountInString(typed))}, nil
	}
	resolved := dereference(reflect.ValueOf(value))
	if resolved.IsValid() && resolved.Kind() == reflect.Map && resolved.Type().Key().Kind() == reflect.String {
		return map[string]any{"length": float64(resolved.Len())}, nil
	}
	return nil, fmt.Errorf("length requires a list, text, or object value, received %T", value)
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

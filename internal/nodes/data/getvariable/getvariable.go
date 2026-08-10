package getvariable

import (
	"context"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	return registrar.Register(Node{Metadata: datanodes.Node("data:get_variable", "Data", "Get Variable", "Read a value stored during this execution.", "bookmark-check", nil, []domain.NodePort{datanodes.Pin("value", "Value", domain.PinOutput, domain.DataAny)}, []domain.ConfigField{datanodes.Field("name", "Variable name", "string", "Result", true)}, map[string]any{}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate reads a value through the narrow variable-reader abstraction.
func Evaluate(_ context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (map[string]any, error) {
	variables, ok := runtime.(nodes.VariableReader)
	if !ok {
		return nil, fmt.Errorf("get variable runtime is unavailable")
	}
	name, _ := invocation.Config["name"].(string)
	name = strings.TrimSpace(name)
	value, exists := variables.LookupVariable(name)
	if !exists {
		return nil, fmt.Errorf("variable %q has not been set in this execution", name)
	}
	return map[string]any{"value": value}, nil
}

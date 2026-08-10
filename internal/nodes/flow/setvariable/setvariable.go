package setvariable

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	flow "github.com/FlameInTheDark/neuropipe/internal/nodes/flow"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput), flow.Data("value", "Value", domain.PinInput, domain.DataAny)}
	outputs := []domain.NodePort{flow.Exec("out", "Then", domain.PinOutput), flow.Data("result", "Value", domain.PinOutput, domain.DataAny)}
	return registrar.Register(Node{Metadata: flow.Node("flow:set_variable", "Data", "Set Variable", "Store a data value for this execution.", "bookmark-plus", inputs, outputs, []domain.ConfigField{flow.Field("name", "Variable name", "string", "Result", true)}, map[string]any{}), Executor: Execute})
}

var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Execute stores the exact graph value through VariableWriter.
func Execute(_ context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	variables, ok := runtime.(nodes.VariableWriter)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("set variable runtime is unavailable")
	}
	name, _ := invocation.Config["name"].(string)
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return nodes.ExecutionResult{}, fmt.Errorf("variable name must start with a letter or underscore and contain only letters, numbers, or underscores")
	}
	value := invocation.Inputs["value"]
	variables.StoreVariable(name, value)
	return nodes.ExecutionResult{Outputs: map[string]any{"result": value}, Ports: []string{"out"}}, nil
}

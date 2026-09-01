package getfield

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/jsonquery"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	defaults := []any{map[string]any{"id": "value", "label": "Value", "path": "value", "dataType": "any"}}
	definition := datanodes.Node("data:get_field", "Data", "Get Field", "Read a field from an object or list value.", "braces", []domain.NodePort{datanodes.Pin("source", "Source", domain.PinInput, domain.DataAny)}, nil, []domain.ConfigField{datanodes.FieldOutputs("outputs", "Outputs")}, map[string]any{"outputs": defaults})
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		return ResolveDefinition(definition, node)
	}, Executor: nodes.Outputs(Evaluate)})
}

// ResolveDefinition expands configured output pins for Get Field-like nodes.
func ResolveDefinition(definition domain.NodeDefinition, node domain.FlowNode) (domain.NodeDefinition, error) {
	outputs, err := datanodes.FieldOutputsFor(config(node), definition.DefaultConfig)
	if err != nil {
		return definition, err
	}
	resolved := definition
	resolved.Outputs = make([]domain.NodePort, 0, len(outputs))
	for _, output := range outputs {
		resolved.Outputs = append(resolved.Outputs, datanodes.Pin(output.ID, output.Label, domain.PinOutput, output.DataType))
	}
	return resolved, nil
}

// Evaluate resolves every configured output path and validates its declared
// dynamic pin type before exposing it to the graph.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	outputs, err := datanodes.FieldOutputsFor(invocation.Config, invocation.Definition.DefaultConfig)
	if err != nil {
		return nil, err
	}
	result := make(map[string]any, len(outputs))
	for _, output := range outputs {
		value := jsonquery.ValueAt(invocation.Inputs["source"], output.Path)
		if err := typespec.ValidateValue(value, typespec.FromDataType(output.DataType)); err != nil {
			return nil, fmt.Errorf("get field output %q is declared %s, but %q is incompatible: %w", output.Label, output.DataType, output.Path, err)
		}
		result[output.ID] = value
	}
	return result, nil
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

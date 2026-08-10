package buildobject

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
	defaults := []any{map[string]any{"id": "value", "label": "Value", "key": "value", "dataType": "any"}}
	definition := datanodes.Node("data:build_object", "Data", "Build Object", "Build an object from configurable typed input pins.", "braces", []domain.NodePort{datanodes.Pin("key", "Key", domain.PinInput, domain.DataText), datanodes.Pin("value", "Value", domain.PinInput, domain.DataAny)}, []domain.NodePort{datanodes.Pin("object", "Object", domain.PinOutput, domain.DataObject)}, []domain.ConfigField{datanodes.ObjectFields("fields", "Fields")}, map[string]any{"fields": defaults})
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		return ResolveDefinition(definition, node)
	}, Executor: nodes.Outputs(Evaluate)})
}

// ResolveDefinition expands Build Object's dynamic input field contract.
func ResolveDefinition(definition domain.NodeDefinition, node domain.FlowNode) (domain.NodeDefinition, error) {
	config := config(node)
	if _, dynamic := config["fields"]; !dynamic {
		return definition, nil
	}
	fields, err := datanodes.ObjectFieldsFor(config, definition.DefaultConfig)
	if err != nil {
		return definition, err
	}
	resolved := definition
	resolved.Inputs = make([]domain.NodePort, 0, len(fields))
	for _, field := range fields {
		resolved.Inputs = append(resolved.Inputs, datanodes.Pin(field.ID, field.Label, domain.PinInput, field.DataType))
	}
	return resolved, nil
}

// Evaluate creates the exact configured structural object without silently
// adding or coercing fields.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	if _, dynamic := invocation.Config["fields"]; !dynamic {
		key, ok := invocation.Inputs["key"].(string)
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("build object requires a non-empty Key")
		}
		return map[string]any{"object": map[string]any{key: invocation.Inputs["value"]}}, nil
	}
	fields, err := datanodes.ObjectFieldsFor(invocation.Config, invocation.Definition.DefaultConfig)
	if err != nil {
		return nil, err
	}
	object := make(map[string]any, len(fields))
	for _, field := range fields {
		if err := datanodes.SetObjectPath(object, field.Key, invocation.Inputs[field.ID]); err != nil {
			return nil, fmt.Errorf("build object field %q: %w", field.Label, err)
		}
	}
	return map[string]any{"object": object}, nil
}

func config(node domain.FlowNode) map[string]any {
	if value, ok := node.Data["config"].(map[string]any); ok {
		return value
	}
	return node.Data
}

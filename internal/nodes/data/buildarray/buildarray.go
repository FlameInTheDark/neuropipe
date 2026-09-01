package buildarray

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// elementTypes lists every supported element type in inspector order. Like
// an array in a typed language, all elements share one type; "any" is the
// explicit mixed-type escape hatch.
var elementTypes = []string{"any", "text", "number", "boolean", "object", "list", "bytes"}

// Register contributes the Build Array node: configurable ordered input pins
// that assemble into one list output.
func Register(registrar nodes.Registrar) error {
	defaults := []any{map[string]any{"id": "first", "label": "First"}}
	definition := datanodes.Node("data:build_array", "Arrays", "Build Array",
		"Build an array of one shared element type from configurable input pins.", "list",
		nil,
		[]domain.NodePort{datanodes.Pin("array", "Array", domain.PinOutput, domain.DataList)},
		[]domain.ConfigField{
			datanodes.Select("elementType", "Element type", elementTypes...),
			datanodes.Field("items", "Items", "array-items", "first", true),
		},
		map[string]any{"elementType": "any", "items": defaults})
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		return ResolveDefinition(definition, node)
	}, Executor: nodes.Outputs(Evaluate)})
}

// ResolveDefinition expands Build Array's dynamic item contract. Every row is
// one input pin typed by the node's element type under the "item_" namespace:
// a row with a constant applies it as the pin default (the engine feeds it
// when no wire lands), while a row without one marks the pin required so an
// unwired element fails with the item's name instead of silently producing
// null. A concrete element type also refines the output contract to list<T>,
// so typed consumers reject mismatched arrays at validation time.
func ResolveDefinition(definition domain.NodeDefinition, node domain.FlowNode) (domain.NodeDefinition, error) {
	elementType, err := datanodes.CollectionTypeFor(config(node), definition.DefaultConfig, "elementType")
	if err != nil {
		return definition, err
	}
	items, err := datanodes.ArrayItemsFor(config(node), definition.DefaultConfig, elementType)
	if err != nil {
		return definition, err
	}
	resolved := definition
	resolved.Inputs = make([]domain.NodePort, 0, len(items))
	for _, item := range items {
		pin := datanodes.Pin(datanodes.ArrayItemPinID(item.ID), item.Label, domain.PinInput, elementType)
		if item.Literal != nil {
			pin.Default = item.Literal
		} else {
			pin.Required = true
		}
		resolved.Inputs = append(resolved.Inputs, pin)
	}
	resolved.Outputs = []domain.NodePort{arrayOutput(elementType)}
	return resolved, nil
}

// arrayOutput declares the list output refined by the element type: list<T>
// when the type is concrete, the plain list contract (list<any>) otherwise.
func arrayOutput(elementType domain.DataType) domain.NodePort {
	pin := datanodes.Pin("array", "Array", domain.PinOutput, domain.DataList)
	if elementType == "" || elementType == domain.DataAny {
		return pin
	}
	element := typespec.FromDataType(elementType)
	list := domain.TypeSpec{Kind: domain.TypeList, Element: &element}
	pin.Type = &list
	return pin
}

// Evaluate assembles the array in row order: the wired value when a wire
// feeds the pin, the row's constant otherwise. The engine already applies
// pin defaults, so the constant here is a backstop for direct execution.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	elementType, err := datanodes.CollectionTypeFor(invocation.Config, invocation.Definition.DefaultConfig, "elementType")
	if err != nil {
		return nil, err
	}
	items, err := datanodes.ArrayItemsFor(invocation.Config, invocation.Definition.DefaultConfig, elementType)
	if err != nil {
		return nil, err
	}
	array := make([]any, 0, len(items))
	for _, item := range items {
		value, exists := invocation.Inputs[datanodes.ArrayItemPinID(item.ID)]
		if !exists {
			if item.Literal == nil {
				return nil, fmt.Errorf("item %q has no value: wire its pin or set a constant", item.Label)
			}
			value = item.Literal
		}
		array = append(array, value)
	}
	return map[string]any{"array": array}, nil
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

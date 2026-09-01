package buildmap

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

// valueTypes lists every supported value type in inspector order. Like a map
// in a typed language, all values share one type; "any" is the explicit
// mixed-type escape hatch. Keys are always text — JSON object keys.
var valueTypes = []string{"any", "text", "number", "boolean", "object", "list", "bytes"}

// Register contributes the Build Map node: configurable key-value input pins
// that assemble into one flat object output.
func Register(registrar nodes.Registrar) error {
	defaults := []any{map[string]any{"id": "value", "label": "Value", "key": "value"}}
	definition := datanodes.Node("data:build_map", "Data", "Build Map",
		"Build a map object from key-value pins sharing one value type.", "braces",
		nil,
		[]domain.NodePort{mapOutputPin(domain.DataAny)},
		[]domain.ConfigField{
			datanodes.Select("valueType", "Value type", valueTypes...),
			datanodes.Field("entries", "Entries", "map-entries", "id", true),
		},
		map[string]any{"valueType": "any", "entries": defaults})
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		return ResolveDefinition(definition, node)
	}, Executor: nodes.Outputs(Evaluate)})
}

// mapOutputPin declares the map-typed output: text keys to values of the
// node's value type, so the assembled dictionary connects to every
// map-typed consumer (Values, Fields, Rows, KV payloads) without a Cast.
func mapOutputPin(valueType domain.DataType) domain.NodePort {
	keyType := domain.TypeSpec{Kind: domain.TypeString}
	valueSpec := typespec.FromDataType(valueType)
	mapType := domain.TypeSpec{Kind: domain.TypeMap, Key: &keyType, Value: &valueSpec}
	return domain.NodePort{ID: "map", Label: "Map", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &mapType, Color: "#60a5fa", MaxConnections: 1}
}

// ResolveDefinition expands Build Map's dynamic entry contract. Keys are
// verbatim — dots and spaces stay literal, unlike Build Object's dotted
// paths — and each row is one typed input pin under the "entry_" namespace,
// typed by the node's value type. Constants become pin defaults; rows
// without one mark the pin required.
func ResolveDefinition(definition domain.NodeDefinition, node domain.FlowNode) (domain.NodeDefinition, error) {
	valueType, err := datanodes.CollectionTypeFor(config(node), definition.DefaultConfig, "valueType")
	if err != nil {
		return definition, err
	}
	entries, err := datanodes.MapEntriesFor(config(node), definition.DefaultConfig, valueType)
	if err != nil {
		return definition, err
	}
	resolved := definition
	resolved.Inputs = make([]domain.NodePort, 0, len(entries))
	for _, entry := range entries {
		pin := datanodes.Pin(datanodes.MapEntryPinID(entry.ID), entry.Label, domain.PinInput, valueType)
		if entry.Literal != nil {
			pin.Default = entry.Literal
		} else {
			pin.Required = true
		}
		resolved.Inputs = append(resolved.Inputs, pin)
	}
	resolved.Outputs = []domain.NodePort{mapOutputPin(valueType)}
	return resolved, nil
}

// Evaluate assembles the flat map: the wired value when a wire feeds the
// pin, the row's constant otherwise. The engine already applies pin
// defaults, so the constant here is a backstop for direct execution.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	valueType, err := datanodes.CollectionTypeFor(invocation.Config, invocation.Definition.DefaultConfig, "valueType")
	if err != nil {
		return nil, err
	}
	entries, err := datanodes.MapEntriesFor(invocation.Config, invocation.Definition.DefaultConfig, valueType)
	if err != nil {
		return nil, err
	}
	object := make(map[string]any, len(entries))
	for _, entry := range entries {
		value, exists := invocation.Inputs[datanodes.MapEntryPinID(entry.ID)]
		if !exists {
			if entry.Literal == nil {
				return nil, fmt.Errorf("entry %q has no value: wire its pin or set a constant", entry.Label)
			}
			value = entry.Literal
		}
		object[entry.Key] = value
	}
	return map[string]any{"map": object}, nil
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

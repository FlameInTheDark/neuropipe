// Package typeassert owns metadata for narrowing an any value with a runtime
// contract. Evaluation remains hosted by the cancellable Blueprint runtime.
package typeassert

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// Register contributes the Type Assert node to the first-party node registry.
func Register(registrar nodes.Registrar) error {
	definition := definition()
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		typeSpec, err := contract(config(node), definition.DefaultConfig)
		if err != nil {
			return definition, err
		}
		resolved := definition
		resolved.Outputs = append([]domain.NodePort(nil), definition.Outputs...)
		for index := range resolved.Outputs {
			resolved.Outputs[index].DataType = dataTypeForSpec(typeSpec)
			resolved.Outputs[index].Type = &typeSpec
			resolved.Outputs[index].Color = color(resolved.Outputs[index].DataType)
		}
		return resolved, nil
	}, Executor: nodes.Outputs(Evaluate)})
}

// Evaluate validates and returns the original value. Unlike Cast, it never
// converts data; that makes an any-to-concrete wire explicit and safe.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	typeSpec, err := contract(invocation.Config, invocation.Definition.DefaultConfig)
	if err != nil {
		return nil, err
	}
	if err := typespec.ValidateValue(invocation.Inputs["value"], typeSpec); err != nil {
		return nil, fmt.Errorf("type assertion failed: %w", err)
	}
	return map[string]any{"value": invocation.Inputs["value"]}, nil
}

func contract(config, defaults map[string]any) (domain.TypeSpec, error) {
	raw, exists := config["typeSpec"]
	if !exists {
		raw = defaults["typeSpec"]
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return domain.TypeSpec{}, fmt.Errorf("type contract must be JSON data: %w", err)
	}
	var typeSpec domain.TypeSpec
	if err := json.Unmarshal(encoded, &typeSpec); err != nil {
		return domain.TypeSpec{}, fmt.Errorf("type contract is invalid: %w", err)
	}
	if err := typespec.ValidateSpec(typeSpec); err != nil {
		return domain.TypeSpec{}, fmt.Errorf("type contract is invalid: %w", err)
	}
	return typeSpec, nil
}

func definition() domain.NodeDefinition {
	return domain.NodeDefinition{
		Type:        "data:type_assert",
		Category:    "Data",
		Label:       "Type Assert",
		Description: "Validate an any value against a type contract without converting it.",
		Icon:        "shield-check",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs:      []domain.NodePort{dataPin("value", "Value", domain.PinInput)},
		Outputs:     []domain.NodePort{dataPin("value", "Value", domain.PinOutput)},
		Fields:      []domain.ConfigField{{Name: "typeSpec", Label: "Type", Kind: "type-spec", Required: true}},
		DefaultConfig: map[string]any{
			"typeSpec": map[string]any{"kind": "any"},
		},
		Source: "builtin",
	}
}

func dataPin(id, label string, direction domain.PinDirection) domain.NodePort {
	typeSpec := typespec.Any()
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataAny, Type: &typeSpec, Color: "#a1a1aa", MaxConnections: 1}
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

func dataTypeForSpec(typeSpec domain.TypeSpec) domain.DataType {
	switch typeSpec.Kind {
	case domain.TypeString:
		return domain.DataText
	case domain.TypeInt, domain.TypeFloat:
		return domain.DataNumber
	case domain.TypeBool:
		return domain.DataBoolean
	case domain.TypeList:
		return domain.DataList
	case domain.TypeMap, domain.TypeRecord:
		return domain.DataObject
	default:
		return domain.DataAny
	}
}

func color(dataType domain.DataType) string {
	switch dataType {
	case domain.DataText:
		return "#e879f9"
	case domain.DataNumber:
		return "#86efac"
	case domain.DataBoolean:
		return "#f87171"
	case domain.DataList:
		return "#facc15"
	case domain.DataObject:
		return "#60a5fa"
	default:
		return "#a1a1aa"
	}
}

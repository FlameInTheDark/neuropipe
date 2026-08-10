// Package cast owns the metadata contract for the explicit primitive Cast node.
package cast

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// Register contributes the Cast node to the first-party node registry.
func Register(registrar nodes.Registrar) error {
	definition := definition()
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		config := config(node)
		dataType, ok := outputType(strings.TrimSpace(fmt.Sprint(config["target"])))
		if !ok {
			return definition, nil
		}
		resolved := definition
		resolved.Outputs = append([]domain.NodePort(nil), definition.Outputs...)
		for index := range resolved.Outputs {
			resolved.Outputs[index].DataType = dataType
			typeSpec := typespec.FromDataType(dataType)
			resolved.Outputs[index].Type = &typeSpec
			resolved.Outputs[index].Color = color(dataType)
		}
		return resolved, nil
	}, Executor: nodes.Outputs(Evaluate)})
}

// Evaluate applies the documented, explicit primitive conversions. It never
// changes a value without a Cast node and returns an error for invalid input.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	target := strings.TrimSpace(fmt.Sprint(invocation.Config["target"]))
	if target == "" {
		target = "text"
	}
	value, err := convert(invocation.Inputs["value"], target)
	if err != nil {
		return nil, err
	}
	return map[string]any{"value": value}, nil
}

func convert(value any, target string) (any, error) {
	switch target {
	case "text":
		return fmt.Sprint(value), nil
	case "number":
		if number, ok := numberValue(value); ok {
			return number, nil
		}
		number, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		if err != nil {
			return nil, fmt.Errorf("cannot cast %T to number", value)
		}
		return number, nil
	case "boolean":
		if result, ok := value.(bool); ok {
			return result, nil
		}
		result, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(value)))
		if err != nil {
			return nil, fmt.Errorf("cannot cast %T to Boolean", value)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unknown cast target %q", target)
	}
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func definition() domain.NodeDefinition {
	return domain.NodeDefinition{
		Type:        "data:cast",
		Category:    "Data",
		Label:       "Cast",
		Description: "Explicitly cast a value to text, number, or Boolean.",
		Icon:        "arrow-right-left",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs:      []domain.NodePort{dataPin("value", "Value", domain.PinInput, domain.DataAny)},
		Outputs:     []domain.NodePort{dataPin("value", "Value", domain.PinOutput, domain.DataAny)},
		Fields: []domain.ConfigField{{
			Name: "target", Label: "Target type", Kind: "select", Required: true,
			Options: []domain.Option{{Value: "text", Label: "text"}, {Value: "number", Label: "number"}, {Value: "boolean", Label: "boolean"}},
		}},
		DefaultConfig: map[string]any{"target": "text"},
		Source:        "builtin",
	}
}

func dataPin(id, label string, direction domain.PinDirection, dataType domain.DataType) domain.NodePort {
	typeSpec := typespec.FromDataType(dataType)
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: dataType, Type: &typeSpec, Color: "#a1a1aa", MaxConnections: 1}
}

func config(node domain.FlowNode) map[string]any {
	if value, ok := node.Data["config"].(map[string]any); ok {
		return value
	}
	return node.Data
}

func outputType(target string) (domain.DataType, bool) {
	switch target {
	case "text":
		return domain.DataText, true
	case "number":
		return domain.DataNumber, true
	case "boolean":
		return domain.DataBoolean, true
	default:
		return "", false
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
	default:
		return "#a1a1aa"
	}
}

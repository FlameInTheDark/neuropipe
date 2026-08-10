// Package constant registers a typed literal data source.
package constant

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// Register contributes the typed Constant node and the behavior that owns its
// explicit literal contract.
func Register(registrar nodes.Registrar) error {
	definition := datanodes.Node("data:constant", "Data", "Constant", "Provide a literal value cast to a fixed type.", "circle-dot", nil, []domain.NodePort{datanodes.Pin("value", "Value", domain.PinOutput, domain.DataAny)}, []domain.ConfigField{datanodes.Field("value", "Value", "string", "", false), datanodes.Select("type", "Type", "text", "number", "boolean")}, map[string]any{"type": "text"})
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		return resolve(definition, node), nil
	}, Executor: nodes.Outputs(Evaluate)})
}

func resolve(definition domain.NodeDefinition, node domain.FlowNode) domain.NodeDefinition {
	config := config(node)
	target, _ := config["type"].(string)
	dataType, ok := outputType(target)
	if !ok {
		return definition
	}
	resolved := definition
	resolved.Outputs = append([]domain.NodePort(nil), definition.Outputs...)
	for index := range resolved.Outputs {
		resolved.Outputs[index].DataType = dataType
		typeSpec := typespec.FromDataType(dataType)
		resolved.Outputs[index].Type = &typeSpec
		resolved.Outputs[index].Color = color(dataType)
	}
	return resolved
}

// Evaluate validates a canonical V3 literal. Only legacy V2 values retain
// text parsing, and only inside this explicit Constant node.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	value, exists := invocation.Inputs["value"]
	if !exists {
		value = invocation.Config["value"]
	}
	target, _ := invocation.Config["type"].(string)
	if target == "" && invocation.SchemaVersion >= domain.GraphSchemaV3 {
		target = "text"
	}
	typed, err := typedValue(target, value, invocation.SchemaVersion >= domain.GraphSchemaV3)
	if err != nil {
		return nil, err
	}
	return map[string]any{"value": typed}, nil
}

func typedValue(target string, value any, strict bool) (any, error) {
	switch target {
	case "":
		return value, nil
	case "text":
		if strict {
			if _, ok := value.(string); !ok {
				return nil, fmt.Errorf("constant text value must be a string")
			}
			return value, nil
		}
		return fmt.Sprint(value), nil
	case "number":
		if strict {
			if err := typespec.ValidateValue(value, typespec.Float()); err != nil {
				return nil, fmt.Errorf("constant number value: %w", err)
			}
			return value, nil
		}
		return castNumber(value)
	case "boolean":
		if strict {
			if _, ok := value.(bool); !ok {
				return nil, fmt.Errorf("constant Boolean value must be a bool")
			}
			return value, nil
		}
		return castBoolean(value)
	default:
		return nil, fmt.Errorf("unknown constant type %q", target)
	}
}

func castNumber(value any) (float64, error) {
	switch number := value.(type) {
	case float64:
		return number, nil
	case float32:
		return float64(number), nil
	case int:
		return float64(number), nil
	case int64:
		return float64(number), nil
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
	if err != nil {
		return 0, fmt.Errorf("cannot cast %T to number", value)
	}
	return parsed, nil
}

func castBoolean(value any) (bool, error) {
	if boolean, ok := value.(bool); ok {
		return boolean, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(value)))
	if err != nil {
		return false, fmt.Errorf("cannot cast %T to Boolean", value)
	}
	return parsed, nil
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

// Package cast owns the metadata contract for the explicit Cast node.
package cast

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// targets lists every supported cast target in inspector order. They cover
// the complete concrete data-type system: any never needs a cast because an
// untyped value already flows into any input.
var targets = []string{"text", "number", "boolean", "object", "list", "bytes"}

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// Register contributes the Cast node to the first-party node registry.
func Register(registrar nodes.Registrar) error {
	definition := definition()
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		target, _ := config(node)["target"].(string)
		dataType, ok := outputType(strings.TrimSpace(target))
		if !ok {
			return definition, nil
		}
		resolved := definition
		resolved.Outputs = append([]domain.NodePort(nil), definition.Outputs...)
		for index := range resolved.Outputs {
			resolved.Outputs[index].DataType = dataType
			typeSpec := OutputSpec(dataType)
			resolved.Outputs[index].Type = &typeSpec
			resolved.Outputs[index].Color = color(dataType)
		}
		return resolved, nil
	}, Executor: nodes.Outputs(Evaluate)})
}

// Evaluate applies the documented, explicit conversions. It never changes a
// value without a Cast node and returns an error for invalid input.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	target, _ := invocation.Config["target"].(string)
	target = strings.TrimSpace(target)
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
		return textValue(value)
	case "number":
		if number, ok := numberValue(value); ok {
			return number, nil
		}
		text, err := textValue(value)
		if err != nil {
			return nil, err
		}
		number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return nil, fmt.Errorf("cannot cast %T to number", value)
		}
		return number, nil
	case "boolean":
		if result, ok := value.(bool); ok {
			return result, nil
		}
		text, err := textValue(value)
		if err != nil {
			return nil, err
		}
		result, err := strconv.ParseBool(strings.TrimSpace(text))
		if err != nil {
			return nil, fmt.Errorf("cannot cast %T to Boolean", value)
		}
		return result, nil
	case "object":
		return toObject(value)
	case "list":
		return toList(value)
	case "bytes":
		return toBytes(value)
	default:
		return nil, fmt.Errorf("unknown cast target %q", target)
	}
}

// textValue renders a value as text. Bytes decode as raw text and JSON
// structures serialize compactly, so text casts round-trip with object,
// list, and bytes casts.
func textValue(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	case map[string]any, []any:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "", fmt.Errorf("cannot cast %T to text", value)
		}
		return string(encoded), nil
	default:
		return fmt.Sprint(value), nil
	}
}

// toObject narrows structured values to the graph's object shape. JSON text
// must decode to an object; native structs and typed maps convert through a
// JSON round-trip into graph-safe map[string]any values.
func toObject(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, nil
	case string:
		var decoded map[string]any
		if err := json.Unmarshal([]byte(typed), &decoded); err != nil {
			return nil, fmt.Errorf("cannot cast text to object: %w", err)
		}
		if decoded == nil {
			return nil, fmt.Errorf("cannot cast text to object: JSON null is not an object")
		}
		return decoded, nil
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("cannot cast %T to object", value)
		}
		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil || decoded == nil {
			return nil, fmt.Errorf("cannot cast %T to object", value)
		}
		return decoded, nil
	}
}

// toList narrows sequence values to the graph's list shape. JSON text must
// decode to an array; typed slices from native producers convert through a
// JSON round-trip. Bytes never become lists because JSON encodes them as
// base64 text.
func toList(value any) (any, error) {
	switch typed := value.(type) {
	case []any:
		return typed, nil
	case string:
		var decoded []any
		if err := json.Unmarshal([]byte(typed), &decoded); err != nil {
			return nil, fmt.Errorf("cannot cast text to list: %w", err)
		}
		if decoded == nil {
			return nil, fmt.Errorf("cannot cast text to list: JSON null is not a list")
		}
		return decoded, nil
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("cannot cast %T to list", value)
		}
		var decoded []any
		if err := json.Unmarshal(encoded, &decoded); err != nil || decoded == nil {
			return nil, fmt.Errorf("cannot cast %T to list", value)
		}
		return decoded, nil
	}
}

// toBytes encodes text as raw bytes and passes existing bytes through. Other
// types have no unambiguous byte encoding and fail loudly instead.
func toBytes(value any) (any, error) {
	switch typed := value.(type) {
	case []byte:
		return typed, nil
	case string:
		return []byte(typed), nil
	default:
		return nil, fmt.Errorf("cannot cast %T to bytes", value)
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
	options := make([]domain.Option, 0, len(targets))
	for _, target := range targets {
		options = append(options, domain.Option{Value: target, Label: target})
	}
	return domain.NodeDefinition{
		Type:        "data:cast",
		Category:    "Data",
		Label:       "Cast",
		Description: "Explicitly cast a value to text, number, Boolean, object, list, or bytes.",
		Icon:        "arrow-right-left",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs:      []domain.NodePort{dataPin("value", "Value", domain.PinInput, domain.DataAny)},
		Outputs:     []domain.NodePort{dataPin("value", "Value", domain.PinOutput, domain.DataAny)},
		Fields: []domain.ConfigField{{
			Name: "target", Label: "Target type", Kind: "select", Required: true,
			Options: options,
		}},
		DefaultConfig: map[string]any{"target": "text"},
		Source:        "builtin",
	}
}

func dataPin(id, label string, direction domain.PinDirection, dataType domain.DataType) domain.NodePort {
	typeSpec := typespec.FromDataType(dataType)
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: dataType, Type: &typeSpec, Color: "#a1a1aa", MaxConnections: 1}
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

func outputType(target string) (domain.DataType, bool) {
	switch target {
	case "text":
		return domain.DataText, true
	case "number":
		return domain.DataNumber, true
	case "boolean":
		return domain.DataBoolean, true
	case "object":
		return domain.DataObject, true
	case "list":
		return domain.DataList, true
	case "bytes":
		return domain.DataBytes, true
	default:
		return "", false
	}
}

// OutputSpec returns the wire contract for a cast target's output pin.
// Objects use the graph-wide map<string, any> shape — the same contract as
// KV Hash Set fields, SQL rows, and storage entries — so a cast output
// connects to first-party object inputs. Lists stay list<any> and inherit
// element assignability; precise element contracts belong to Type Assert.
func OutputSpec(dataType domain.DataType) domain.TypeSpec {
	if dataType == domain.DataObject {
		key := domain.TypeSpec{Kind: domain.TypeString}
		value := domain.TypeSpec{Kind: domain.TypeAny}
		return domain.TypeSpec{Kind: domain.TypeMap, Key: &key, Value: &value}
	}
	return typespec.FromDataType(dataType)
}

func color(dataType domain.DataType) string {
	switch dataType {
	case domain.DataText:
		return "#e879f9"
	case domain.DataNumber:
		return "#86efac"
	case domain.DataBoolean:
		return "#f87171"
	case domain.DataObject:
		return "#60a5fa"
	case domain.DataList:
		return "#facc15"
	case domain.DataBytes:
		return "#fbbf24"
	default:
		return "#a1a1aa"
	}
}

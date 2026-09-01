package jsonparse

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// rootTypes lists every supported JSON root type in inspector order. Bytes is
// absent because JSON never produces raw bytes; "any" is the escape hatch for
// mixed roots and for graphs saved before the field existed.
var rootTypes = []string{"any", "object", "list", "text", "number", "boolean"}

// Register contributes the Parse JSON node: JSON text in, a value out whose
// wire contract follows the configured root type.
func Register(registrar nodes.Registrar) error {
	definition := datanodes.Node("data:json_parse", "Data", "Parse JSON",
		"Parse JSON text into a typed object, list, or scalar.",
		"file-json",
		[]domain.NodePort{datanodes.Pin("text", "Text", domain.PinInput, domain.DataText)},
		[]domain.NodePort{outputPin(domain.DataAny)},
		[]domain.ConfigField{datanodes.Select("type", "Root type", rootTypes...)},
		map[string]any{"type": "object"})
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		return ResolveDefinition(definition, node)
	}, Executor: nodes.Outputs(Evaluate)})
}

// ResolveDefinition types the value output by the node's configured root
// type. Blank configuration — every graph saved before the field existed —
// keeps the untyped any contract so existing wires stay valid.
func ResolveDefinition(definition domain.NodeDefinition, node domain.FlowNode) (domain.NodeDefinition, error) {
	rootType, err := RootTypeFor(config(node))
	if err != nil {
		return definition, err
	}
	resolved := definition
	if rootType != domain.DataAny {
		resolved.Outputs = []domain.NodePort{outputPin(rootType)}
	}
	return resolved, nil
}

// RootTypeFor reads the configured JSON root type. Blank or missing values
// resolve to any — the historical contract — while unsupported values are
// hard errors. It deliberately ignores the definition defaults so legacy
// nodes without the key stay untyped.
func RootTypeFor(config map[string]any) (domain.DataType, error) {
	raw, exists := config["type"]
	if !exists || raw == nil {
		return domain.DataAny, nil
	}
	value := strings.TrimSpace(fmt.Sprint(raw))
	if value == "" || value == "<nil>" {
		return domain.DataAny, nil
	}
	if value == "bytes" || !datanodes.ValidDataType(domain.DataType(value)) {
		return domain.DataAny, fmt.Errorf("type %q is not a supported JSON root type", value)
	}
	return domain.DataType(value), nil
}

// outputPin declares the value output for a root type. Objects use the
// graph-wide map<string, any> shape — the same contract as Cast's object
// target, Build Map, Word template values, and Excel rows — so a parsed
// object connects to first-party object inputs without a Cast.
func outputPin(dataType domain.DataType) domain.NodePort {
	if dataType == domain.DataObject {
		key := domain.TypeSpec{Kind: domain.TypeString}
		value := domain.TypeSpec{Kind: domain.TypeAny}
		mapType := domain.TypeSpec{Kind: domain.TypeMap, Key: &key, Value: &value}
		return domain.NodePort{ID: "value", Label: "Value", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &mapType, Color: "#60a5fa", MaxConnections: 1}
	}
	return datanodes.Pin("value", "Value", domain.PinOutput, dataType)
}

// Evaluate decodes JSON text into graph-safe JSON values and checks the
// decoded root against the configured root type, so the value always honors
// the pin's wire contract.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	text, ok := invocation.Inputs["text"].(string)
	if !ok {
		return nil, fmt.Errorf("parse JSON requires text input")
	}
	rootType, err := RootTypeFor(invocation.Config)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	if err := validateRoot(value, rootType); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return map[string]any{"value": value}, nil
}

// validateRoot checks the decoded value's kind against the declared root
// type. "any" accepts everything, matching the historical contract; every
// concrete type names the actual kind and the fix in its error.
func validateRoot(value any, rootType domain.DataType) error {
	switch rootType {
	case domain.DataAny:
		return nil
	case domain.DataObject:
		if _, ok := value.(map[string]any); ok {
			return nil
		}
	case domain.DataList:
		if _, ok := value.([]any); ok {
			return nil
		}
	case domain.DataText:
		if _, ok := value.(string); ok {
			return nil
		}
	case domain.DataNumber:
		if _, ok := value.(float64); ok {
			return nil
		}
	case domain.DataBoolean:
		if _, ok := value.(bool); ok {
			return nil
		}
	default:
		return fmt.Errorf("root type %q is not a supported JSON root type", string(rootType))
	}
	actual := jsonKind(value)
	if actual == "null" {
		return fmt.Errorf("root is null, but Root type is %s; set Root type to any", rootType)
	}
	return fmt.Errorf("root is %s, but Root type is %s; set Root type to %s or any", actual, rootType, actual)
}

// jsonKind names a decoded JSON value's kind using the graph's data-type
// labels, so error messages read in the inspector's vocabulary.
func jsonKind(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "text"
	case []any:
		return "list"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

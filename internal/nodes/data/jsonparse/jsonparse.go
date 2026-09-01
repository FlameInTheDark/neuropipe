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

type Node = nodes.Implementation

var _ nodes.Node = Node{}

var supportedRootTypes = []string{"object", "list"}

func Register(registrar nodes.Registrar) error {
	return registrar.Register(Node{Metadata: datanodes.Node("data:json_parse", "Data", "Parse JSON", "Parse JSON text into an object or list.", "file-json", []domain.NodePort{datanodes.Pin("text", "Text", domain.PinInput, domain.DataText)}, []domain.NodePort{datanodes.Pin("value", "Value", domain.PinOutput, domain.DataAny)}, nil, map[string]any{}), Resolver: resolveJSONParse, Executor: nodes.Outputs(Evaluate)})
}

func resolveJSONParse(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := datanodes.Node("data:json_parse", "Data", "Parse JSON", "Parse JSON text into an object or list.", "file-json", []domain.NodePort{datanodes.Pin("text", "Text", domain.PinInput, domain.DataText)}, []domain.NodePort{datanodes.Pin("value", "Value", domain.PinOutput, domain.DataAny)}, nil, map[string]any{})
	config := configFor(node)
	rootType, _ := config["type"].(string)
	rootType = strings.TrimSpace(strings.ToLower(rootType))
	if rootType == "" {
		rootType = "object"
	}
	if !isSupportedRootType(rootType) {
		return definition, fmt.Errorf("%q is not a supported JSON root type", rootType)
	}
	resolved := definition
	resolved.Outputs = append([]domain.NodePort(nil), definition.Outputs...)
	resolved.Outputs[0].DataType = dataType(rootType)
	resolved.Outputs[0].Type = outputSpec(rootType)
	resolved.Outputs[0].Color = color(rootType)
	return resolved, nil
}

func configFor(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

func isSupportedRootType(t string) bool {
	for _, supported := range supportedRootTypes {
		if t == supported {
			return true
		}
	}
	return false
}

func dataType(rootType string) domain.DataType {
	switch rootType {
	case "object":
		return domain.DataObject
	case "list":
		return domain.DataList
	default:
		return domain.DataAny
	}
}

func outputSpec(rootType string) *domain.TypeSpec {
	if rootType == "object" {
		key := domain.TypeSpec{Kind: domain.TypeString}
		value := domain.TypeSpec{Kind: domain.TypeAny}
		return &domain.TypeSpec{Kind: domain.TypeMap, Key: &key, Value: &value}
	}
	if rootType == "list" {
		element := domain.TypeSpec{Kind: domain.TypeAny}
		return &domain.TypeSpec{Kind: domain.TypeList, Element: &element}
	}
	return &domain.TypeSpec{Kind: domain.TypeAny}
}

func color(rootType string) string {
	switch rootType {
	case "object":
		return "#60a5fa"
	case "list":
		return "#facc15"
	default:
		return "#a1a1aa"
	}
}

func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	text, ok := invocation.Inputs["text"].(string)
	if !ok {
		return nil, fmt.Errorf("parse JSON requires text input")
	}
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	config := invocation.Config
	rootType, _ := config["type"].(string)
	rootType = strings.TrimSpace(strings.ToLower(rootType))
	if rootType != "" && isSupportedRootType(rootType) {
		switch rootType {
		case "object":
			if _, ok := value.(map[string]any); !ok {
				return nil, fmt.Errorf("root is %s, but Root type is %s", valueKind(value), rootType)
			}
		case "list":
			if _, ok := value.([]any); !ok {
				return nil, fmt.Errorf("root is %s, but Root type is %s", valueKind(value), rootType)
			}
		}
	}
	return map[string]any{"value": value}, nil
}

func valueKind(value any) string {
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "list"
	case string:
		return "text"
	case float64:
		return "number"
	case bool:
		return "boolean"
	default:
		return fmt.Sprintf("%T", value)
	}
}

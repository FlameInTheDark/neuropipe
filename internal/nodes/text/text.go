// Package text provides strict shared contracts for the focused Text nodes.
// It intentionally contains no registered node: each operation owns its
// metadata and behaviour in a sibling package.
package text

import (
	"fmt"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

const Color = "#e879f9"

func Definition(nodeType, label, description, icon string, inputs, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any) domain.NodeDefinition {
	return domain.NodeDefinition{Type: nodeType, Category: "Text", Label: label, Description: description, Icon: icon, Color: Color, Mode: domain.NodePure, Inputs: inputs, Outputs: outputs, Fields: fields, DefaultConfig: defaults, Source: "builtin"}
}

func TextPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	spec := typespec.String()
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataText, Type: &spec, Color: Color, Required: required, MaxConnections: 1}
}

func IntPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	spec := typespec.Int()
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataNumber, Type: &spec, Color: "#86efac", Required: required, MaxConnections: 1}
}

func BoolPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	spec := typespec.Bool()
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataBoolean, Type: &spec, Color: "#f87171", Required: required, MaxConnections: 1}
}

func TextListPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	element := typespec.String()
	spec := domain.TypeSpec{Kind: domain.TypeList, Element: &element}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataList, Type: &spec, Color: "#facc15", Required: required, MaxConnections: 1}
}

func String(inputs map[string]any, name string) (string, error) {
	value, ok := inputs[name].(string)
	if !ok {
		return "", fmt.Errorf("%s must be text", name)
	}
	return value, nil
}

func Int(inputs map[string]any, name string) (int, error) {
	value, ok := inputs[name].(int)
	if !ok {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return value, nil
}

// Strings reads the JSON-safe list representation without changing values.
func Strings(inputs map[string]any, name string) ([]string, error) {
	switch value := inputs[name].(type) {
	case []string:
		return append([]string(nil), value...), nil
	case []any:
		result := make([]string, len(value))
		for index, item := range value {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s[%d] must be text", name, index)
			}
			result[index] = text
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%s must be a list of text", name)
	}
}

func RuneOffset(value string, byteOffset int) int { return utf8.RuneCountInString(value[:byteOffset]) }

func Runes(value string) []rune { return []rune(value) }

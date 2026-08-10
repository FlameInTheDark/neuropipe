// Package datanodes provides small metadata builders shared by data-node
// modules. Each node package owns the actual ports, fields, and defaults.
package datanodes

import (
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

func Node(nodeType, category, label, description, icon string, inputs, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any) domain.NodeDefinition {
	color := "#22c55e"
	if category == "Chat" {
		color = "#a78bfa"
	}
	return domain.NodeDefinition{Type: nodeType, Category: category, Label: label, Description: description, Icon: icon, Color: color, Mode: domain.NodePure, Inputs: inputs, Outputs: outputs, Fields: fields, DefaultConfig: defaults, Source: "builtin"}
}

func Pin(id, label string, direction domain.PinDirection, dataType domain.DataType) domain.NodePort {
	typeSpec := typespec.FromDataType(dataType)
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: dataType, Type: &typeSpec, Color: color(dataType), MaxConnections: 1}
}

func Field(name, label, kind, placeholder string, required bool) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: kind, Placeholder: placeholder, Required: required}
}

func Select(name, label string, values ...string) domain.ConfigField {
	options := make([]domain.Option, 0, len(values))
	for _, value := range values {
		options = append(options, domain.Option{Value: value, Label: value})
	}
	return domain.ConfigField{Name: name, Label: label, Kind: "select", Options: options, Required: true}
}

func FieldOutputs(name, label string) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: "field-outputs", Required: true}
}

func ObjectFields(name, label string) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: "object-fields", Required: true}
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
	default:
		return "#a1a1aa"
	}
}

// Package flownodes contains small metadata builders shared by control-flow
// node modules. The engine retains traversal and loop-safety ownership.
package flownodes

import (
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

func Node(nodeType, category, label, description, icon string, inputs, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any) domain.NodeDefinition {
	return domain.NodeDefinition{Type: nodeType, Category: category, Label: label, Description: description, Icon: icon, Color: "#fbbf24", Mode: domain.NodeImpure, Inputs: inputs, Outputs: outputs, Fields: fields, DefaultConfig: defaults, Source: "builtin"}
}

func Exec(id, label string, direction domain.PinDirection) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: direction, Color: "#fafafa", MaxConnections: 1}
}

func Data(id, label string, direction domain.PinDirection, dataType domain.DataType) domain.NodePort {
	typeSpec := typespec.FromDataType(dataType)
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: dataType, Type: &typeSpec, Color: "#a1a1aa", MaxConnections: 1}
}

func Field(name, label, kind, placeholder string, required bool) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: kind, Placeholder: placeholder, Required: required}
}

func SwitchCases(name, label string) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: "switch-cases", Required: true}
}

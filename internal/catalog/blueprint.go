package catalog

import "github.com/FlameInTheDark/neuropipe/internal/domain"

// blueprintBuiltins are the explicit value and control nodes used by graph v2.
// Existing first-party nodes are also normalised into Blueprint pins, but these
// nodes make common data and control-flow work discoverable without templates.
func blueprintBuiltins() []domain.NodeDefinition {
	definitions := blueprintChatBuiltins()
	return append(definitions, blueprintFunctionBuiltins()...)
}

func execInput() []domain.NodePort {
	return []domain.NodePort{execPin("in", "Exec", domain.PinInput)}
}

func execOutput() []domain.NodePort {
	return []domain.NodePort{execPin("out", "Then", domain.PinOutput)}
}

func blueprintNode(nodeType, category, label, description, icon, color string, mode domain.NodeExecutionMode, inputs, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any) domain.NodeDefinition {
	return normalizeDefinition(domain.NodeDefinition{Type: nodeType, Category: category, Label: label, Description: description, Icon: icon, Color: color, Mode: mode, Inputs: inputs, Outputs: outputs, Fields: fields, DefaultConfig: defaults, Source: "builtin"})
}

func execPin(id, label string, direction domain.PinDirection) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: direction, Color: "#fafafa", MaxConnections: 1}
}

func dataPin(id, label string, direction domain.PinDirection, dataType domain.DataType) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: dataType, Type: typeSpecForDataType(dataType), Color: dataColor(dataType), MaxConnections: 1}
}

// typeSpecForDataType gives existing built-ins a V3 contract while their
// persisted DataType remains available for v2 migration diagnostics.
func typeSpecForDataType(dataType domain.DataType) *domain.TypeSpec {
	var spec domain.TypeSpec
	switch dataType {
	case domain.DataText:
		spec.Kind = domain.TypeString
	case domain.DataNumber:
		spec.Kind = domain.TypeFloat
	case domain.DataBoolean:
		spec.Kind = domain.TypeBool
	case domain.DataObject:
		spec.Kind = domain.TypeRecord
	case domain.DataList:
		spec.Kind = domain.TypeList
		spec.Element = &domain.TypeSpec{Kind: domain.TypeAny}
	default:
		spec.Kind = domain.TypeAny
	}
	return &spec
}

func dataColor(dataType domain.DataType) string {
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

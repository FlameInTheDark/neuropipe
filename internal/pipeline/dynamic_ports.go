package pipeline

import (
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// definitionForNode expands configuration-driven pins. Keeping this in the
// runtime layer makes the editor, validator, and interpreter agree on the
// same stable IDs.
func definitionForNode(definition domain.NodeDefinition, node domain.FlowNode) (domain.NodeDefinition, error) {
	config := configFor(node)
	switch node.Type {
	case "flow:switch":
		ports, err := switchCasePorts(definition.DefaultConfig, node)
		if err != nil {
			return definition, err
		}
		definition.Outputs = append(ports, definition.Outputs...)
	case "llm:choice":
		options, exists := config["options"]
		if !exists {
			options = definition.DefaultConfig["options"]
		}
		ports, err := routeOptionPorts(options)
		if err != nil {
			return definition, err
		}
		definition.Outputs = append(ports, definition.Outputs...)
	case "data:get_field", "data:break_object":
		ports, err := getFieldOutputPorts(config, definition.DefaultConfig)
		if err != nil {
			return definition, err
		}
		definition.Outputs = ports
	case "data:build_object":
		// Graphs saved before configurable Build Object inputs retain their
		// explicit Key/Value pins until a user upgrades the node in the editor.
		if _, legacy := config["fields"]; !legacy {
			return definition, nil
		}
		ports, err := objectFieldInputPorts(config, definition.DefaultConfig)
		if err != nil {
			return definition, err
		}
		definition.Inputs = ports
	}
	return definition, nil
}

type fieldOutput struct {
	ID       string
	Label    string
	Path     string
	DataType domain.DataType
}

type objectField struct {
	ID       string
	Label    string
	Key      string
	DataType domain.DataType
}

// getFieldOutputs accepts the persisted data of the configurable Get Field
// node. Existing v2 drafts with the former single `path` setting keep their
// stable `value` port until a user edits the node.
func getFieldOutputs(config, defaults map[string]any) ([]fieldOutput, error) {
	configured, exists := config["outputs"]
	if !exists {
		if _, legacy := config["path"]; legacy {
			return []fieldOutput{{ID: "value", Label: "Value", Path: text(config, "path"), DataType: domain.DataAny}}, nil
		}
		configured = defaults["outputs"]
	}
	items, ok := configured.([]any)
	if !ok {
		return nil, fmt.Errorf("outputs must be a list")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("add at least one output")
	}
	outputs := make([]fieldOutput, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("output %d must be an object", index+1)
		}
		id := strings.TrimSpace(fmt.Sprint(item["id"]))
		if id == "" {
			return nil, fmt.Errorf("output %d needs an ID", index+1)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("outputs contains duplicate ID %q", id)
		}
		seen[id] = struct{}{}
		dataType := domain.DataType(strings.TrimSpace(fmt.Sprint(item["dataType"])))
		if !validDataType(dataType) {
			return nil, fmt.Errorf("output %q has unsupported data type %q", id, dataType)
		}
		label := strings.TrimSpace(fmt.Sprint(item["label"]))
		if label == "" {
			label = id
		}
		outputs = append(outputs, fieldOutput{ID: id, Label: label, Path: strings.TrimSpace(fmt.Sprint(item["path"])), DataType: dataType})
	}
	return outputs, nil
}

func getFieldOutputPorts(config, defaults map[string]any) ([]domain.NodePort, error) {
	outputs, err := getFieldOutputs(config, defaults)
	if err != nil {
		return nil, err
	}
	ports := make([]domain.NodePort, 0, len(outputs))
	for _, output := range outputs {
		ports = append(ports, domain.NodePort{ID: output.ID, Label: output.Label, Kind: domain.PinData, Direction: domain.PinOutput, DataType: output.DataType, Color: dataTypeColor(output.DataType), MaxConnections: 1})
	}
	return ports, nil
}

func objectFields(config, defaults map[string]any) ([]objectField, error) {
	configured, exists := config["fields"]
	if !exists {
		configured = defaults["fields"]
	}
	items, ok := configured.([]any)
	if !ok {
		return nil, fmt.Errorf("fields must be a list")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("add at least one field")
	}
	fields := make([]objectField, 0, len(items))
	ids := make(map[string]struct{}, len(items))
	keys := make(map[string]struct{}, len(items))
	for index, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("field %d must be an object", index+1)
		}
		id := strings.TrimSpace(fmt.Sprint(item["id"]))
		if id == "" {
			return nil, fmt.Errorf("field %d needs an ID", index+1)
		}
		if _, duplicate := ids[id]; duplicate {
			return nil, fmt.Errorf("fields contains duplicate ID %q", id)
		}
		ids[id] = struct{}{}
		key := strings.TrimSpace(fmt.Sprint(item["key"]))
		if err := validateObjectKeyPath(key); err != nil {
			return nil, fmt.Errorf("field %q %w", id, err)
		}
		if _, duplicate := keys[key]; duplicate {
			return nil, fmt.Errorf("fields contains duplicate object key %q", key)
		}
		keys[key] = struct{}{}
		dataType := domain.DataType(strings.TrimSpace(fmt.Sprint(item["dataType"])))
		if !validDataType(dataType) {
			return nil, fmt.Errorf("field %q has unsupported data type %q", id, dataType)
		}
		label := strings.TrimSpace(fmt.Sprint(item["label"]))
		if label == "" {
			label = id
		}
		fields = append(fields, objectField{ID: id, Label: label, Key: key, DataType: dataType})
	}
	for _, field := range fields {
		for _, other := range fields {
			if field.ID == other.ID {
				continue
			}
			if strings.HasPrefix(other.Key, field.Key+".") {
				return nil, fmt.Errorf("object keys %q and %q overlap", field.Key, other.Key)
			}
		}
	}
	return fields, nil
}

func objectFieldInputPorts(config, defaults map[string]any) ([]domain.NodePort, error) {
	fields, err := objectFields(config, defaults)
	if err != nil {
		return nil, err
	}
	ports := make([]domain.NodePort, 0, len(fields))
	for _, field := range fields {
		ports = append(ports, domain.NodePort{ID: field.ID, Label: field.Label, Kind: domain.PinData, Direction: domain.PinInput, DataType: field.DataType, Color: dataTypeColor(field.DataType), MaxConnections: 1})
	}
	return ports, nil
}

func validateObjectKeyPath(key string) error {
	if key == "" {
		return fmt.Errorf("needs an object key")
	}
	for _, part := range strings.Split(key, ".") {
		if strings.TrimSpace(part) == "" {
			return fmt.Errorf("has invalid object key path %q", key)
		}
	}
	return nil
}

func dataTypeColor(dataType domain.DataType) string {
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

func validDataType(dataType domain.DataType) bool {
	switch dataType {
	case domain.DataAny, domain.DataText, domain.DataNumber, domain.DataBoolean, domain.DataObject, domain.DataList:
		return true
	default:
		return false
	}
}

func routeOptionPorts(value any) ([]domain.NodePort, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("options must be a list")
	}
	ports := make([]domain.NodePort, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, raw := range items {
		option, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("options item %d must be an object", index+1)
		}
		id := strings.TrimSpace(fmt.Sprint(option["id"]))
		if id == "" {
			return nil, fmt.Errorf("options item %d needs an ID", index+1)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("options contains duplicate ID %q", id)
		}
		seen[id] = struct{}{}
		label := strings.TrimSpace(fmt.Sprint(option["label"]))
		if label == "" {
			label = id
		}
		ports = append(ports, domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1})
	}
	return ports, nil
}

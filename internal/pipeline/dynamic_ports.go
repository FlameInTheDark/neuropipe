package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// definitionForRegisteredNode asks a first-party module to resolve its own
// dynamic ports. The legacy fallback remains only for plugin and custom
// function definitions that do not carry a registered module handler.
func definitionForRegisteredNode(registry *catalog.Registry, definition domain.NodeDefinition, node domain.FlowNode) (domain.NodeDefinition, error) {
	if module, exists := registry.Node(node.Type); exists {
		return module.Resolve(node)
	}
	return definitionForNode(definition, node)
}

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
		definition.Inputs = filterChatContextPins(node, config, definition.Inputs)
	case "data:get_field", "data:break_object":
		ports, err := getFieldOutputPorts(config, definition.DefaultConfig)
		if err != nil {
			return definition, err
		}
		definition.Outputs = ports
	case "data:constant":
		if dataType, ok := constantOutputType(config, definition.DefaultConfig); ok {
			outputs := make([]domain.NodePort, len(definition.Outputs))
			copy(outputs, definition.Outputs)
			for index := range outputs {
				outputs[index].DataType = dataType
				typeSpec := typespec.FromDataType(dataType)
				outputs[index].Type = &typeSpec
				outputs[index].Color = dataTypeColor(dataType)
			}
			definition.Outputs = outputs
		}
	case "data:cast":
		if dataType, ok := castOutputType(config); ok {
			outputs := make([]domain.NodePort, len(definition.Outputs))
			copy(outputs, definition.Outputs)
			for index := range outputs {
				outputs[index].DataType = dataType
				typeSpec := typespec.FromDataType(dataType)
				outputs[index].Type = &typeSpec
				outputs[index].Color = dataTypeColor(dataType)
			}
			definition.Outputs = outputs
		}
	case "data:type_assert":
		typeSpec, err := typeAssertOutputSpec(config, definition.DefaultConfig)
		if err != nil {
			return definition, err
		}
		outputs := make([]domain.NodePort, len(definition.Outputs))
		copy(outputs, definition.Outputs)
		for index := range outputs {
			outputs[index].DataType = dataTypeForSpec(typeSpec)
			outputs[index].Type = &typeSpec
			outputs[index].Color = dataTypeColor(outputs[index].DataType)
		}
		definition.Outputs = outputs
	case "llm:prompt", "llm:extract", "llm:boolean", "llm:summarize", "llm:agent", "llm:coding_agent":
		definition.Inputs = filterChatContextPins(node, config, definition.Inputs)
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

// filterNodePort returns the ports without the named pin, sharing the original
// slice whenever the pin is absent so callers keep ownership semantics.
func filterNodePort(ports []domain.NodePort, id string) []domain.NodePort {
	filtered := make([]domain.NodePort, 0, len(ports))
	for _, port := range ports {
		if port.ID == id {
			continue
		}
		filtered = append(filtered, port)
	}
	return filtered
}

// filterChatContextPins hides the toggle-gated chat pins of LLM nodes: the
// Chat Run ID pin exists only while status updates are enabled, and the Chat
// ID pin only in the agents' chat-history mode. The editor mirrors this so
// validation and connections always agree.
func filterChatContextPins(node domain.FlowNode, config map[string]any, inputs []domain.NodePort) []domain.NodePort {
	if !boolValue(config["updateChatStatus"]) {
		inputs = filterNodePort(inputs, "chatRunId")
	}
	if (node.Type == "llm:agent" || node.Type == "llm:coding_agent") && !chatHistoryMode(config) {
		inputs = filterNodePort(inputs, "chatId")
	}
	return inputs
}

func typeAssertOutputSpec(config, defaults map[string]any) (domain.TypeSpec, error) {
	raw, exists := config["typeSpec"]
	if !exists {
		raw = defaults["typeSpec"]
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return domain.TypeSpec{}, fmt.Errorf("type contract must be JSON data: %w", err)
	}
	var typeSpec domain.TypeSpec
	if err := json.Unmarshal(encoded, &typeSpec); err != nil {
		return domain.TypeSpec{}, fmt.Errorf("type contract is invalid: %w", err)
	}
	if err := typespec.ValidateSpec(typeSpec); err != nil {
		return domain.TypeSpec{}, fmt.Errorf("type contract is invalid: %w", err)
	}
	return typeSpec, nil
}

func dataTypeForSpec(typeSpec domain.TypeSpec) domain.DataType {
	switch typeSpec.Kind {
	case domain.TypeString:
		return domain.DataText
	case domain.TypeInt, domain.TypeFloat:
		return domain.DataNumber
	case domain.TypeBool:
		return domain.DataBoolean
	case domain.TypeList:
		return domain.DataList
	case domain.TypeMap, domain.TypeRecord:
		return domain.DataObject
	default:
		return domain.DataAny
	}
}

func castOutputType(config map[string]any) (domain.DataType, bool) {
	switch target := strings.TrimSpace(fmt.Sprint(config["target"])); target {
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
		typeSpec := typespec.FromDataType(output.DataType)
		ports = append(ports, domain.NodePort{ID: output.ID, Label: output.Label, Kind: domain.PinData, Direction: domain.PinOutput, DataType: output.DataType, Type: &typeSpec, Color: dataTypeColor(output.DataType), MaxConnections: 1})
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
		typeSpec := typespec.FromDataType(field.DataType)
		ports = append(ports, domain.NodePort{ID: field.ID, Label: field.Label, Kind: domain.PinData, Direction: domain.PinInput, DataType: field.DataType, Type: &typeSpec, Color: dataTypeColor(field.DataType), MaxConnections: 1})
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

// constantOutputType maps the Constant node's inspector Type to the output pin
// DataType. Only an explicit node-level type is honoured: legacy nodes without
// a Type keep their untyped pin so existing graphs stay valid.
func constantOutputType(config, defaults map[string]any) (domain.DataType, bool) {
	target, exists := config["type"]
	if !exists {
		return "", false
	}
	if typed, ok := target.(string); ok {
		switch domain.DataType(typed) {
		case domain.DataText, domain.DataNumber, domain.DataBoolean:
			return domain.DataType(typed), true
		}
	}
	return "", false
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

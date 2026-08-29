package datanodes

import (
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// FieldOutput describes one configured output owned by Get Field or Break
// Object. It is intentionally independent of pipeline execution state.
type FieldOutput struct {
	ID       string
	Label    string
	Path     string
	DataType domain.DataType
}

// ObjectField describes one configured Build Object input.
type ObjectField struct {
	ID       string
	Label    string
	Key      string
	DataType domain.DataType
}

// FieldOutputsFor parses and validates a node's dynamic field-output
// configuration, preserving the old single-path V2 form for migration only.
func FieldOutputsFor(config, defaults map[string]any) ([]FieldOutput, error) {
	configured, exists := config["outputs"]
	if !exists {
		if _, legacy := config["path"]; legacy {
			return []FieldOutput{{ID: "value", Label: "Value", Path: text(config, "path"), DataType: domain.DataAny}}, nil
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
	outputs := make([]FieldOutput, 0, len(items))
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
		if !ValidDataType(dataType) {
			return nil, fmt.Errorf("output %q has unsupported data type %q", id, dataType)
		}
		label := strings.TrimSpace(fmt.Sprint(item["label"]))
		if label == "" {
			label = id
		}
		outputs = append(outputs, FieldOutput{ID: id, Label: label, Path: strings.TrimSpace(fmt.Sprint(item["path"])), DataType: dataType})
	}
	return outputs, nil
}

// ObjectFieldsFor parses and validates Build Object's configurable input
// contract, including collision-free nested field paths.
func ObjectFieldsFor(config, defaults map[string]any) ([]ObjectField, error) {
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
	fields := make([]ObjectField, 0, len(items))
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
		if !ValidDataType(dataType) {
			return nil, fmt.Errorf("field %q has unsupported data type %q", id, dataType)
		}
		label := strings.TrimSpace(fmt.Sprint(item["label"]))
		if label == "" {
			label = id
		}
		fields = append(fields, ObjectField{ID: id, Label: label, Key: key, DataType: dataType})
	}
	for _, field := range fields {
		for _, other := range fields {
			if field.ID != other.ID && strings.HasPrefix(other.Key, field.Key+".") {
				return nil, fmt.Errorf("object keys %q and %q overlap", field.Key, other.Key)
			}
		}
	}
	return fields, nil
}

// ValidDataType reports the set of legacy data-type labels supported by the
// dynamic node editor. V3 pin contracts are resolved from these labels.
func ValidDataType(dataType domain.DataType) bool {
	switch dataType {
	case domain.DataAny, domain.DataText, domain.DataNumber, domain.DataBoolean, domain.DataObject, domain.DataList, domain.DataBytes:
		return true
	default:
		return false
	}
}

// SetObjectPath places a value at a validated dotted object path.
func SetObjectPath(object map[string]any, path string, value any) error {
	parts := strings.Split(path, ".")
	current := object
	for index, part := range parts {
		if index == len(parts)-1 {
			current[part] = value
			return nil
		}
		next, exists := current[part]
		if !exists {
			child := make(map[string]any)
			current[part] = child
			current = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("key path conflicts at %q", strings.Join(parts[:index+1], "."))
		}
		current = child
	}
	return nil
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

func text(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return strings.TrimSpace(value)
}

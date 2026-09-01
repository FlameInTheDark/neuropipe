package datanodes

import (
	"encoding/json"
	"fmt"
	"strconv"
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

// ArrayItem describes one configured Build Array input. Value is the raw
// persisted constant (empty when the pin must be wired); Literal is the
// constant coerced to the node's element type, nil when absent.
type ArrayItem struct {
	ID      string
	Label   string
	Value   string
	Literal any
}

// MapEntry describes one configured Build Map key-value input. Keys are used
// verbatim — dots never nest, unlike Build Object's dotted paths — and
// Literal is the constant coerced to the node's value type.
type MapEntry struct {
	ID      string
	Label   string
	Key     string
	Value   string
	Literal any
}

// MaxBuildRows caps Build Array items and Build Map entries so a pathological
// config cannot mint thousands of pins and stall the editor or validator.
const MaxBuildRows = 32

// ArrayItemPinID returns the stable edge-handle ID of one item's pin. The
// "item_" namespace keeps resolver pins clear of the engine's config-fallback
// lookup, which reads config[pinID].
func ArrayItemPinID(rowID string) string { return "item_" + rowID }

// MapEntryPinID returns the stable edge-handle ID of one entry's pin under
// the collision-free "entry_" namespace.
func MapEntryPinID(rowID string) string { return "entry_" + rowID }

// CollectionTypeFor parses the node-level collection data type: the element
// type for Build Array and the value type for Build Map. Like an array or
// map in a typed language, every pin, constant, and the output share it, so
// the collection stays homogeneous; "any" is the explicit mixed-type escape
// hatch and the default when nothing is configured.
func CollectionTypeFor(config, defaults map[string]any, key string) (domain.DataType, error) {
	raw, exists := config[key]
	if !exists || raw == nil {
		raw, exists = defaults[key]
	}
	if !exists || raw == nil {
		return domain.DataAny, nil
	}
	dataType := domain.DataType(strings.TrimSpace(fmt.Sprint(raw)))
	if dataType == "" || dataType == "<nil>" {
		return domain.DataAny, nil
	}
	if !ValidDataType(dataType) {
		return domain.DataAny, fmt.Errorf("%s %q is not a supported data type", key, dataType)
	}
	return dataType, nil
}

// ArrayItemsFor parses and validates Build Array's configurable item list,
// coercing each row's constant to the node's element type. It falls back to
// the definition defaults when nothing is configured yet.
func ArrayItemsFor(config, defaults map[string]any, elementType domain.DataType) ([]ArrayItem, error) {
	configured, exists := config["items"]
	if !exists {
		configured = defaults["items"]
	}
	items, err := buildRows(configured, "items", "item")
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("add at least one item")
	}
	rows := make([]ArrayItem, 0, len(items))
	for _, item := range items {
		row := ArrayItem{ID: item.id, Label: item.label, Value: item.value}
		if row.Label == "" {
			row.Label = row.ID
		}
		literal, err := LiteralValue(elementType, item.value)
		if err != nil {
			return nil, fmt.Errorf("item %q: %w", row.Label, err)
		}
		row.Literal = literal
		rows = append(rows, row)
	}
	return rows, nil
}

// MapEntriesFor parses and validates Build Map's configurable entry list,
// coercing each row's constant to the node's value type. Rows with blank keys
// are dropped as mid-edit editor state; duplicate keys are hard errors
// because they would silently overwrite each other.
func MapEntriesFor(config, defaults map[string]any, valueType domain.DataType) ([]MapEntry, error) {
	configured, exists := config["entries"]
	if !exists {
		configured = defaults["entries"]
	}
	items, err := buildRows(configured, "entries", "entry")
	if err != nil {
		return nil, err
	}
	rows := make([]MapEntry, 0, len(items))
	keys := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.key == "" {
			// Blank-key rows are mid-edit editor state, not a runtime contract.
			continue
		}
		if _, duplicate := keys[item.key]; duplicate {
			return nil, fmt.Errorf("entries contain duplicate key %q", item.key)
		}
		keys[item.key] = struct{}{}
		row := MapEntry{ID: item.id, Label: item.label, Key: item.key, Value: item.value}
		if row.Label == "" {
			row.Label = row.Key
		}
		literal, err := LiteralValue(valueType, item.value)
		if err != nil {
			return nil, fmt.Errorf("entry %q: %w", row.Label, err)
		}
		row.Literal = literal
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("add at least one entry")
	}
	return rows, nil
}

// buildRow is the raw persisted shape shared by both builders. Rows carry no
// data type: the node-level collection type owns typing, and any legacy
// per-row dataType keys are ignored.
type buildRow struct {
	id    string
	label string
	key   string
	value string
}

// buildRows decodes the persisted row list shared by Build Array and Build
// Map: {id, label, key, value}. IDs are generated when missing so hand-
// written configs stay valid; duplicate IDs and reserved-prefix IDs are
// rejected because they would merge two pins into one edge handle.
func buildRows(configured any, key, prefix string) ([]buildRow, error) {
	items, ok := configured.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a list", key)
	}
	if len(items) > MaxBuildRows {
		return nil, fmt.Errorf("%s accept at most %d rows", key, MaxBuildRows)
	}
	rows := make([]buildRow, 0, len(items))
	ids := make(map[string]struct{}, len(items))
	for index, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s entry %d must be an object", key, index+1)
		}
		id := fieldText(item, "id")
		if id == "" {
			id = fmt.Sprintf("row_%d", index+1)
		}
		if strings.HasPrefix(id, prefix+"_") {
			return nil, fmt.Errorf("%s entry %d: row IDs are generated, remove the %q prefix", key, index+1, prefix+"_")
		}
		if _, duplicate := ids[id]; duplicate {
			return nil, fmt.Errorf("%s contain duplicate row ID %q", key, id)
		}
		ids[id] = struct{}{}
		rows = append(rows, buildRow{
			id:    id,
			label: fieldText(item, "label"),
			key:   fieldText(item, "key"),
			value: fieldText(item, "value"),
		})
	}
	return rows, nil
}

// fieldText reads a trimmed string field; absent and null fields read as
// empty so hand-edited partial rows stay valid.
func fieldText(item map[string]any, key string) string {
	value, exists := item[key]
	if !exists || value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

// LiteralValue coerces a persisted constant to the declared data type —
// for Build Array and Build Map this is the node-level collection type. An
// absent constant (nil or empty) yields (nil, nil) so the caller can mark
// the pin as required instead. Bytes have no textual constant form.
func LiteralValue(dataType domain.DataType, raw any) (any, error) {
	switch value := raw.(type) {
	case nil:
		return nil, nil
	case string:
		if value == "" {
			return nil, nil
		}
		return coerceLiteral(dataType, value)
	default:
		// Hand-edited configs may persist already-typed constants.
		return canonicalLiteral(dataType, value)
	}
}

func coerceLiteral(dataType domain.DataType, value string) (any, error) {
	switch dataType {
	case domain.DataNumber:
		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("constant %q is not a number", value)
		}
		return number, nil
	case domain.DataBoolean:
		switch strings.ToLower(value) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		return nil, fmt.Errorf("constant %q is not a boolean", value)
	case domain.DataObject:
		decoded := map[string]any{}
		if err := json.Unmarshal([]byte(value), &decoded); err != nil {
			return nil, fmt.Errorf("constant is not a JSON object: %w", err)
		}
		return decoded, nil
	case domain.DataList:
		decoded := []any{}
		if err := json.Unmarshal([]byte(value), &decoded); err != nil {
			return nil, fmt.Errorf("constant is not a JSON list: %w", err)
		}
		return decoded, nil
	case domain.DataBytes:
		return nil, fmt.Errorf("constants are not supported for bytes items")
	default:
		return value, nil
	}
}

func canonicalLiteral(dataType domain.DataType, value any) (any, error) {
	switch dataType {
	case domain.DataNumber:
		switch number := value.(type) {
		case float64:
			return number, nil
		case int:
			return float64(number), nil
		case json.Number:
			parsed, err := number.Float64()
			if err != nil {
				return nil, fmt.Errorf("constant %q is not a number", number.String())
			}
			return parsed, nil
		}
		return nil, fmt.Errorf("constant %v is not a number", value)
	case domain.DataBoolean:
		if flag, ok := value.(bool); ok {
			return flag, nil
		}
		return nil, fmt.Errorf("constant %v is not a boolean", value)
	case domain.DataObject:
		if decoded, ok := value.(map[string]any); ok {
			return decoded, nil
		}
		return nil, fmt.Errorf("constant is not an object")
	case domain.DataList:
		if decoded, ok := value.([]any); ok {
			return decoded, nil
		}
		return nil, fmt.Errorf("constant is not a list")
	case domain.DataBytes:
		return nil, fmt.Errorf("constants are not supported for bytes items")
	default:
		return value, nil
	}
}

// FieldOutputsFor parses and validates a node's dynamic field-output
// configuration, falling back to the definition defaults when the node has
// not configured its own outputs yet.
func FieldOutputsFor(config, defaults map[string]any) ([]FieldOutput, error) {
	configured, exists := config["outputs"]
	if !exists {
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

// ValidDataType reports the set of data-type labels supported by the dynamic
// node editor. Pin contracts are resolved from these labels.
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

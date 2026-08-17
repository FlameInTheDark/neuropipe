// Package form owns the Form node's data model and layout parser. The node
// shows a full-screen form modal built from a grid-based layout the user
// designs in the inspector. Each input and dropdown becomes a typed output
// data pin; text panels produce no pin.
package form

import (
	"fmt"
	"strings"
)

// FormItemKind enumerates the placeable form items.
type FormItemKind string

const (
	ItemText     FormItemKind = "text"     // rich text panel, no output pin
	ItemInput    FormItemKind = "input"    // text or number input -> one output pin
	ItemDropdown FormItemKind = "dropdown" // select -> one output pin (value)
)

// FormInputType narrows input field value types.
type FormInputType string

const (
	InputText   FormInputType = "text"
	InputNumber FormInputType = "number"
)

// FormOption is one dropdown option. If Label is empty, the Value is shown
// to the user in the dropdown. The Value is what the output pin emits.
type FormOption struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

// FormItem is one cell in the form builder grid. ID is stable and used as the
// output pin ID; Label is user-editable display text. Col/Row are 0-indexed
// grid positions. Span is 1..4 (column width). RowSpan is 1..N (row height,
// useful for multiline text panels).
type FormItem struct {
	ID          string        `json:"id"`
	Kind        FormItemKind  `json:"kind"`
	Label       string        `json:"label"`
	Col         int           `json:"col"`
	Row         int           `json:"row"`
	Span        int           `json:"span"`
	RowSpan     int           `json:"rowSpan,omitempty"`
	InputType   FormInputType `json:"inputType,omitempty"`
	Placeholder string        `json:"placeholder,omitempty"`
	Options     []FormOption  `json:"options,omitempty"`
}

// FormLayout is the full builder state stored under config["form"].
type FormLayout struct {
	Items []FormItem `json:"items"`
}

// formLayoutFor extracts and validates the form layout from the node config.
func formLayoutFor(config map[string]any, defaults map[string]any) (FormLayout, error) {
	raw := config["form"]
	if raw == nil {
		if defaults != nil {
			raw = defaults["form"]
		}
		if raw == nil {
			return defaultLayout(), nil
		}
	}
	layout, ok := raw.(map[string]any)
	if !ok {
		return defaultLayout(), nil
	}
	itemsRaw, ok := layout["items"].([]any)
	if !ok || len(itemsRaw) == 0 {
		return defaultLayout(), nil
	}
	items := make([]FormItem, 0, len(itemsRaw))
	ids := make(map[string]struct{})
	for index, raw := range itemsRaw {
		entry, ok := raw.(map[string]any)
		if !ok {
			return FormLayout{}, fmt.Errorf("form item %d is not an object", index)
		}
		item, err := parseFormItem(entry, index)
		if err != nil {
			return FormLayout{}, err
		}
		if _, exists := ids[item.ID]; exists {
			return FormLayout{}, fmt.Errorf("duplicate form item id %q", item.ID)
		}
		ids[item.ID] = struct{}{}
		items = append(items, item)
	}
	return FormLayout{Items: items}, nil
}

func parseFormItem(entry map[string]any, index int) (FormItem, error) {
	id, _ := entry["id"].(string)
	if strings.TrimSpace(id) == "" {
		return FormItem{}, fmt.Errorf("form item %d has no id", index)
	}
	kindStr, _ := entry["kind"].(string)
	kind := FormItemKind(kindStr)
	switch kind {
	case ItemText, ItemInput, ItemDropdown:
	default:
		return FormItem{}, fmt.Errorf("form item %q has unknown kind %q", id, kindStr)
	}
	item := FormItem{
		ID:    id,
		Kind:  kind,
		Label: asString(entry["label"]),
		Col:   asInt(entry["col"], 0),
		Row:   asInt(entry["row"], 0),
		Span:  clampInt(asInt(entry["span"], 1), 1, 4),
	}
	if item.Span == 0 {
		item.Span = 1
	}
	item.RowSpan = clampInt(asInt(entry["rowSpan"], 1), 1, 20)
	if item.RowSpan == 0 {
		item.RowSpan = 1
	}
	if kind == ItemInput {
		inputTypeStr, _ := entry["inputType"].(string)
		if inputTypeStr == "" {
			inputTypeStr = string(InputText)
		}
		item.InputType = FormInputType(inputTypeStr)
		if item.InputType != InputText && item.InputType != InputNumber {
			return FormItem{}, fmt.Errorf("form item %q has unknown inputType %q", id, inputTypeStr)
		}
		item.Placeholder = asString(entry["placeholder"])
	}
	if kind == ItemDropdown {
		optsRaw, ok := entry["options"].([]any)
		if !ok || len(optsRaw) == 0 {
			return FormItem{}, fmt.Errorf("form item %q (dropdown) needs at least one option", id)
		}
		for optIndex, optRaw := range optsRaw {
			optEntry, ok := optRaw.(map[string]any)
			if !ok {
				return FormItem{}, fmt.Errorf("form item %q option %d is not an object", id, optIndex)
			}
			opt := FormOption{
				Value: asString(optEntry["value"]),
				Label: asString(optEntry["label"]),
			}
			if strings.TrimSpace(opt.Value) == "" {
				return FormItem{}, fmt.Errorf("form item %q option %d has no value", id, optIndex)
			}
			item.Options = append(item.Options, opt)
		}
	}
	return item, nil
}

func defaultLayout() FormLayout {
	return FormLayout{
		Items: []FormItem{
			{ID: "field_1", Kind: ItemInput, Label: "Input", Col: 0, Row: 0, Span: 4, RowSpan: 1, InputType: InputText},
		},
	}
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func asInt(value any, fallback int) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	default:
		return fallback
	}
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

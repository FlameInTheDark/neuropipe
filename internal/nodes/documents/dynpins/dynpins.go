// Package dynpins is the shared configuration-driven pin machinery of the
// Documents nodes. Word and Excel actions expose user-defined value bindings
// — placeholder values, table columns, worksheet cells — as ordinary data
// pins so upstream nodes can wire values directly instead of assembling a
// JSON object first.
//
// The persisted row contract is shared by every consumer:
//
//	valuePins / cellPins / columnPins / fieldPins: [
//	  { "id": "field_1", "name": "customer", "label": "Customer", "value": "Contoso" }
//	]
//
// Pin IDs are namespaced ("pin_" + row ID) so a hand-edited row can never
// collide with a node's static pins, and the engine's config-fallback lookup
// (config[pinID]) can never accidentally read persisted row data.
package dynpins

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

// MaxRows caps one binding list so a pathological config cannot mint
// thousands of pins and stall the editor or the validator.
const MaxRows = 32

// Row is one configured value binding. Name is the domain key (placeholder,
// column header, cell reference); Label overrides the pin label; Value is the
// literal used when no wire feeds the pin.
type Row struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
	Value any    `json:"value,omitempty"`
}

// PinID returns the stable edge-handle ID of one row's pin.
func PinID(rowID string) string { return "pin_" + rowID }

// Configured parses and validates the binding list persisted under key.
// Rows with blank names are dropped as editor noise; ids are generated when
// missing so hand-written configs stay valid. Duplicate ids, duplicate names,
// and reserved-id collisions are hard errors because they would silently
// merge two pins into one.
func Configured(config map[string]any, key string) ([]Row, error) {
	raw, exists := config[key]
	if !exists || raw == nil {
		return []Row{}, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a list of pin bindings", key)
	}
	rows := make([]Row, 0, len(items))
	ids := make(map[string]struct{}, len(items))
	names := make(map[string]struct{}, len(items))
	for index, item := range items {
		encoded, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("%s entry %d: %w", key, index+1, err)
		}
		var row Row
		if err := json.Unmarshal(encoded, &row); err != nil {
			return nil, fmt.Errorf("%s entry %d: %w", key, index+1, err)
		}
		row.ID = strings.TrimSpace(row.ID)
		row.Name = strings.TrimSpace(row.Name)
		row.Label = strings.TrimSpace(row.Label)
		if row.Name == "" {
			// Blank rows are mid-edit editor state, not a runtime contract.
			continue
		}
		if row.ID == "" {
			row.ID = fmt.Sprintf("row_%d", index+1)
		}
		if strings.HasPrefix(row.ID, "pin_") {
			return nil, fmt.Errorf("%s entry %d: pin IDs are generated, remove the %q prefix", key, index+1, "pin_")
		}
		if _, duplicate := ids[row.ID]; duplicate {
			return nil, fmt.Errorf("%s contain duplicate pin ID %q", key, row.ID)
		}
		if _, duplicate := names[row.Name]; duplicate {
			return nil, fmt.Errorf("%s contain duplicate name %q", key, row.Name)
		}
		if row.Label == "" {
			row.Label = row.Name
		}
		ids[row.ID] = struct{}{}
		names[row.Name] = struct{}{}
		rows = append(rows, row)
	}
	if len(rows) > MaxRows {
		return nil, fmt.Errorf("%s accepts at most %d pins", key, MaxRows)
	}
	return rows, nil
}

// anyType is the shared pin contract: every document value accepts any wire
// type. Placeholders stringify, cells coerce, columns coerce — restricting
// the pin would only force Cast nodes into simple flows.
var anyType = domain.TypeSpec{Kind: domain.TypeAny}

// InputPins renders rows as data input pins. The row's literal Value becomes
// the pin default, which the engine applies when no wire is connected.
func InputPins(rows []Row, color string) []domain.NodePort {
	pins := make([]domain.NodePort, 0, len(rows))
	for _, row := range rows {
		pinType := anyType
		pins = append(pins, domain.NodePort{
			ID: PinID(row.ID), Label: row.Label, Kind: domain.PinData, Direction: domain.PinInput,
			DataType: domain.DataAny, Type: &pinType, Color: color,
			MaxConnections: 1, Default: row.Value,
		})
	}
	return pins
}

// OutputPins renders rows as data output pins. Output defaults are metadata
// only, so row literals are ignored here.
func OutputPins(rows []Row, color string) []domain.NodePort {
	pins := make([]domain.NodePort, 0, len(rows))
	for _, row := range rows {
		pinType := anyType
		pins = append(pins, domain.NodePort{
			ID: PinID(row.ID), Label: row.Label, Kind: domain.PinData, Direction: domain.PinOutput,
			DataType: domain.DataAny, Type: &pinType, Color: color, MaxConnections: 1,
		})
	}
	return pins
}

// Values collects the effective value of every row: the wired value when a
// connection feeds the pin, otherwise the row literal. Rows with neither are
// skipped — the node decides whether a missing binding is an error. The map
// is keyed by row Name (the placeholder, column, or cell), which is what the
// document operations consume.
func Values(invocation nodes.Invocation, rows []Row) map[string]any {
	values := make(map[string]any, len(rows))
	for _, row := range rows {
		value, wired := invocation.Inputs[PinID(row.ID)]
		if wired {
			values[row.Name] = value
			continue
		}
		if row.Value != nil && row.Value != "" {
			values[row.Name] = row.Value
		}
	}
	return values
}

// WiredValues returns only the values fed through actual graph wires, keyed
// by row name. Row literals are deliberately excluded: nodes that merge pins
// with an existing object treat literals as gap fillers, never as overrides
// of explicitly configured object entries.
func WiredValues(invocation nodes.Invocation, rows []Row) map[string]any {
	values := make(map[string]any, len(rows))
	for _, row := range rows {
		if !invocation.ConnectedInputs[PinID(row.ID)] {
			continue
		}
		if value, exists := invocation.Inputs[PinID(row.ID)]; exists {
			values[row.Name] = value
		}
	}
	return values
}

// FallbackLiterals returns the literals of rows whose pin carries no wire.
// Merging nodes apply them only to names the primary object does not define.
func FallbackLiterals(rows []Row) map[string]any {
	values := make(map[string]any, len(rows))
	for _, row := range rows {
		if row.Value == nil || row.Value == "" {
			continue
		}
		values[row.Name] = row.Value
	}
	return values
}

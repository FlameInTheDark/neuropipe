// Package excel registers the local Excel workbook Blueprint nodes.
//
// The module set covers the full table workflow — list rows, add rows,
// update and delete rows by key column, cell reads and writes, and
// worksheet management — while operating on local .xlsx files instead of
// cloud drives.
package excel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/documents/dynpins"
	"github.com/xuri/excelize/v2"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

const excelColor = "#217346"

// register contributes one complete node implementation to the registry.
func register(registrar nodes.Registrar, definition domain.NodeDefinition, executor func(context.Context, nodes.Invocation, nodes.Runtime) (nodes.ExecutionResult, error)) error {
	return registrar.Register(Node{Metadata: definition, Executor: executor})
}

// registerResolved additionally contributes a dynamic port resolver so a
// node's configuration-driven pins (cells, columns) appear in the editor,
// validator, and engine under stable IDs.
func registerResolved(registrar nodes.Registrar, definition domain.NodeDefinition, resolver func(domain.FlowNode) (domain.NodeDefinition, error), executor func(context.Context, nodes.Invocation, nodes.Runtime) (nodes.ExecutionResult, error)) error {
	return registrar.Register(Node{Metadata: definition, Resolver: resolver, Executor: executor})
}

// Definition assembles the common NodeDefinition skeleton for workbook nodes.
func definition(nodeType, label, description, icon string, inputs []domain.NodePort, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any, capabilities ...domain.Capability) domain.NodeDefinition {
	if defaults == nil {
		defaults = map[string]any{}
	}
	return domain.NodeDefinition{
		Type: nodeType, Category: "Documents", Label: label, Description: description,
		Icon: icon, Color: excelColor, Mode: domain.NodeImpure, PortContractOwned: true,
		Capabilities:  capabilities,
		Inputs:        inputs,
		Outputs:       outputs,
		Fields:        fields,
		DefaultConfig: defaults,
		Source:        "builtin",
	}
}

/* ---------------- Pin builders ---------------- */

func execPin(id, label string, direction domain.PinDirection) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: direction, Color: "#fafafa", MaxConnections: 1}
}

func textPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	pinType := domain.TypeSpec{Kind: domain.TypeString}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataText, Type: &pinType, Color: "#e879f9", Required: required, MaxConnections: 1}
}

func numberPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	pinType := domain.TypeSpec{Kind: domain.TypeFloat}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataNumber, Type: &pinType, Color: "#86efac", Required: required, MaxConnections: 1}
}

func objectPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	keyType := domain.TypeSpec{Kind: domain.TypeString}
	valueType := domain.TypeSpec{Kind: domain.TypeAny}
	pinType := domain.TypeSpec{Kind: domain.TypeMap, Key: &keyType, Value: &valueType}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataObject, Type: &pinType, Color: "#60a5fa", Required: required, MaxConnections: 1}
}

func boolPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	pinType := domain.TypeSpec{Kind: domain.TypeBool}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataBoolean, Type: &pinType, Color: "#f87171", Required: required, MaxConnections: 1}
}

func anyPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	pinType := domain.TypeSpec{Kind: domain.TypeAny}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataAny, Type: &pinType, Color: "#a1a1aa", Required: required, MaxConnections: 1}
}

func listPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	element := domain.TypeSpec{Kind: domain.TypeAny}
	pinType := domain.TypeSpec{Kind: domain.TypeList, Element: &element}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataList, Type: &pinType, Color: "#facc15", Required: required, MaxConnections: 1}
}

func thenPin() domain.NodePort { return execPin("out", "Then", domain.PinOutput) }

/* ---------------- Invocation helpers ---------------- */

// cleanPath reads a text input or configured field and returns its cleaned
// filesystem path. Empty paths fail with a single consistent message.
func cleanPath(invocation nodes.Invocation, name string) (string, error) {
	raw, _ := invocation.Inputs[name].(string)
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		return "", fmt.Errorf("%s must point to an .xlsx workbook", name)
	}
	return filepath.Clean(path), nil
}

// textValue reads a trimmed string from a connected pin or the inspector.
func textValue(invocation nodes.Invocation, name string) string {
	raw, _ := invocation.Inputs[name].(string)
	return strings.TrimSpace(raw)
}

// configured reads a trimmed string from the node configuration only.
func configured(invocation nodes.Invocation, name string) string {
	raw, _ := invocation.Config[name].(string)
	return strings.TrimSpace(raw)
}

// configOf reads the persisted node configuration map from a FlowNode.
func configOf(node domain.FlowNode) map[string]any {
	if config, ok := node.Data["config"].(map[string]any); ok {
		return config
	}
	return map[string]any{}
}

// cellReference validates and canonicalizes one A1-style cell reference.
func cellReference(name string, value string) (string, error) {
	cell := strings.ToUpper(strings.TrimSpace(value))
	if _, _, err := excelize.CellNameToCoordinates(cell); err != nil {
		return "", fmt.Errorf("%s %q is not a valid cell reference", name, value)
	}
	return cell, nil
}

// configuredBool reads a boolean from the node configuration with a default.
func configuredBool(invocation nodes.Invocation, name string, fallback bool) bool {
	if value, ok := invocation.Config[name].(bool); ok {
		return value
	}
	if raw, ok := invocation.Config[name].(string); ok {
		if parsed, err := strconv.ParseBool(strings.TrimSpace(raw)); err == nil {
			return parsed
		}
	}
	return fallback
}

// rowsInput normalizes the Rows input into a list of row objects. A single
// connected object is treated as one row; the inspector fallback parses a
// JSON object or array. An
// absent or blank value is not an error — column pins may carry the row
// instead, so the caller combines both sources before validating.
func rowsInput(invocation nodes.Invocation) ([]map[string]any, error) {
	value := invocation.Inputs["rows"]
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for index, item := range typed {
			row, err := rowObject(item)
			if err != nil {
				return nil, fmt.Errorf("rows[%d]: %w", index, err)
			}
			rows = append(rows, row)
		}
		return rows, nil
	case map[string]any:
		row, err := rowObject(typed)
		if err != nil {
			return nil, err
		}
		return []map[string]any{row}, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, nil
		}
		return rowsFromJSON(typed)
	default:
		return nil, fmt.Errorf("rows must be a row object or a list of row objects")
	}
}

// rowObject validates one row value.
func rowObject(value any) (map[string]any, error) {
	if row, ok := value.(map[string]any); ok {
		return row, nil
	}
	return nil, fmt.Errorf("row must be an object of column values")
}

// fieldsInput reads the Fields object from a connected pin or the inspector
// JSON fallback. An absent or blank value is not an error — field pins may
// carry the payload instead, so the caller combines both sources before
// validating.
func fieldsInput(invocation nodes.Invocation) (map[string]any, error) {
	raw, exists := invocation.Inputs["fields"]
	if !exists || raw == nil {
		return map[string]any{}, nil
	}
	switch typed := raw.(type) {
	case map[string]any:
		return typed, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return map[string]any{}, nil
		}
		var fields map[string]any
		if err := decodeJSON(typed, &fields); err != nil {
			return nil, fmt.Errorf("fields: %w", err)
		}
		if fields == nil {
			fields = map[string]any{}
		}
		return fields, nil
	default:
		return nil, fmt.Errorf("fields must be an object of column values")
	}
}

// updateFields combines the Fields object and the configured field pins
// into one update payload. A wired pin overrides the object entry for the
// same column; a row literal only fills columns the object leaves open.
func updateFields(invocation nodes.Invocation) (map[string]any, error) {
	fields, err := fieldsInput(invocation)
	if err != nil {
		return nil, err
	}
	rows, err := dynpins.Configured(invocation.Config, "fieldPins")
	if err != nil {
		return nil, fmt.Errorf("field pins: %w", err)
	}
	for name, value := range dynpins.WiredValues(invocation, rows) {
		fields[name] = value
	}
	for name, value := range dynpins.FallbackLiterals(rows) {
		if _, known := fields[name]; !known {
			fields[name] = value
		}
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("at least one field or field pin is required")
	}
	return fields, nil
}

// openWorkbook opens an existing .xlsx file for reading and mutation.
func openWorkbook(path string) (*excelize.File, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("workbook %s does not exist", path)
		}
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	file, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	return file, nil
}

// saveWorkbook persists the workbook back to its original path.
func saveWorkbook(path string, file *excelize.File) error {
	if err := file.Save(); err != nil {
		// Fall back to an explicit SaveAs for read-only filesystems or files
		// opened from a stream; both write identical content.
		if saveErr := file.SaveAs(path); saveErr != nil {
			return fmt.Errorf("save workbook: %w", err)
		}
	}
	return nil
}

// resolveSheet returns the sheet to operate on: an explicit name, or the
// workbook's first sheet. The name is validated against the sheet list.
func resolveSheet(file *excelize.File, sheet string) (string, error) {
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return "", fmt.Errorf("workbook has no worksheets")
	}
	if sheet == "" {
		return sheets[0], nil
	}
	for _, name := range sheets {
		if name == sheet {
			return sheet, nil
		}
	}
	return "", fmt.Errorf("worksheet %q does not exist", sheet)
}

// findTable locates a named table and returns its sheet and range.
func findTable(file *excelize.File, table string) (sheet string, tableRange string, found bool, err error) {
	for _, name := range file.GetSheetList() {
		tables, tableErr := file.GetTables(name)
		if tableErr != nil {
			continue
		}
		for _, entry := range tables {
			if entry.Name == table {
				return name, entry.Range, true, nil
			}
		}
	}
	return "", "", false, nil
}

// tableRangeBounds converts an A1:B9 style range into zero-based bounds.
func tableRangeBounds(tableRange string) (startCol, startRow, endCol, endRow int, err error) {
	parts := strings.SplitN(tableRange, ":", 2)
	startCol, startRow, err = excelize.CellNameToCoordinates(parts[0])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("table range %q: %w", tableRange, err)
	}
	endCol, endRow = startCol, startRow
	if len(parts) == 2 {
		endCol, endRow, err = excelize.CellNameToCoordinates(parts[1])
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("table range %q: %w", tableRange, err)
		}
	}
	return startCol - 1, startRow - 1, endCol - 1, endRow - 1, nil
}

// typedValue converts a raw cell string into a JSON-safe value: numbers and
// booleans keep their wire types, everything else stays text.
func typedValue(raw string) any {
	if raw == "" {
		return ""
	}
	if number, err := strconv.ParseFloat(raw, 64); err == nil {
		return number
	}
	switch strings.ToUpper(raw) {
	case "TRUE":
		return true
	case "FALSE":
		return false
	}
	return raw
}

// writeCellValue writes a JSON value into a cell. Strings starting with "="
// are stored as formulas, mirroring interactive spreadsheet behaviour.
func writeCellValue(file *excelize.File, sheet, cell string, value any) error {
	if text, ok := value.(string); ok && strings.HasPrefix(text, "=") {
		return file.SetCellFormula(sheet, cell, text)
	}
	return file.SetCellValue(sheet, cell, value)
}

// firstSheetRows reads the used-range rows of a sheet with the requested
// value representation.
func sheetRows(file *excelize.File, sheet string, raw bool) ([][]string, error) {
	options := excelize.Options{RawCellValue: raw}
	return file.GetRows(sheet, options)
}

// decodeJSON parses an inspector-entered JSON document.
func decodeJSON(text string, target any) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return fmt.Errorf("empty JSON")
	}
	if err := json.Unmarshal([]byte(trimmed), target); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// rowsFromJSON parses a JSON object or array entered in the inspector.
func rowsFromJSON(text string) ([]map[string]any, error) {
	var parsed any
	if err := decodeJSON(text, &parsed); err != nil {
		return nil, err
	}
	switch typed := parsed.(type) {
	case map[string]any:
		return []map[string]any{typed}, nil
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for index, item := range typed {
			row, err := rowObject(item)
			if err != nil {
				return nil, fmt.Errorf("rows[%d]: %w", index, err)
			}
			rows = append(rows, row)
		}
		return rows, nil
	default:
		return nil, fmt.Errorf("JSON must be a row object or an array of row objects")
	}
}

// coerceString converts an inspector-entered string into a typed cell value
// so "42" lands in a numeric cell like interactively typed spreadsheet text.
// Non-numeric and non-boolean text is kept verbatim.
func coerceString(value any) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	if number, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil {
		return number
	}
	switch strings.ToUpper(strings.TrimSpace(text)) {
	case "TRUE":
		return true
	case "FALSE":
		return false
	}
	return value
}

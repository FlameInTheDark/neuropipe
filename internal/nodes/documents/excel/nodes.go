package excel

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/documents/dynpins"
	"github.com/xuri/excelize/v2"
)

/* ------------------------------------------------------------------ */
/* Read Excel Rows — emit tables, ranges, and sheets as row objects */
/* ------------------------------------------------------------------ */

// RegisterReadRows contributes the Read Excel Rows node.
func RegisterReadRows(registrar nodes.Registrar) error {
	definition := definition("action:excel_read_rows", "Read Excel Rows",
		"Read a named Excel table (or a sheet range) as a list of row objects.",
		"table-2",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
			textPin("table", "Table", domain.PinInput, false),
			textPin("sheet", "Sheet", domain.PinInput, false),
			textPin("range", "Range", domain.PinInput, false),
		},
		[]domain.NodePort{
			thenPin(),
			listPin("rows", "Rows", domain.PinOutput, false),
			numberPin("count", "Count", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\orders.xlsx", Required: true},
			{Name: "table", Label: "Table", Kind: "string", Placeholder: "Table1 (empty reads the sheet range)"},
			{Name: "sheet", Label: "Sheet", Kind: "string", Placeholder: "Sheet1 (empty uses the first sheet)"},
			{Name: "range", Label: "Range", Kind: "string", Placeholder: "A1:C10 (used when no table is set)"},
			{Name: "valueMode", Label: "Values", Kind: "select", Options: []domain.Option{
				{Value: "raw", Label: "Raw (typed numbers and booleans)"},
				{Value: "formatted", Label: "Formatted (displayed text)"},
			}},
		},
		map[string]any{"table": "", "sheet": "", "range": "", "valueMode": "raw"},
		domain.CapabilityFileRead,
	)
	return register(registrar, definition, executeReadRows)
}

func executeReadRows(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read excel rows cancelled: %w", err)
	}
	path, err := cleanPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	file, err := openWorkbook(path)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	defer file.Close()

	table := textValue(invocation, "table")
	raw := configured(invocation, "valueMode") != "formatted"

	var sheet string
	var startCol, startRow, endCol, endRow int
	if table != "" {
		tableSheet, tableRange, found, findErr := findTable(file, table)
		if findErr != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("find table %q: %w", table, findErr)
		}
		if !found {
			return nodes.ExecutionResult{}, fmt.Errorf("table %q does not exist in %s", table, path)
		}
		sheet = tableSheet
		startCol, startRow, endCol, endRow, err = tableRangeBounds(tableRange)
		if err != nil {
			return nodes.ExecutionResult{}, err
		}
	} else {
		sheet, err = resolveSheet(file, textValue(invocation, "sheet"))
		if err != nil {
			return nodes.ExecutionResult{}, err
		}
		if region := textValue(invocation, "range"); region != "" {
			startCol, startRow, endCol, endRow, err = tableRangeBounds(region)
			if err != nil {
				return nodes.ExecutionResult{}, fmt.Errorf("range: %w", err)
			}
		} else {
			startCol, startRow, endCol, endRow = 0, 0, -1, -1
		}
	}

	grid, err := sheetRows(file, sheet, raw)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read sheet %q: %w", sheet, err)
	}
	if endRow < 0 {
		endRow = len(grid) - 1
	}
	if endCol < 0 {
		endCol = widestRow(grid, startRow) - 1
	}
	if startRow >= len(grid) || startRow > endRow {
		return nodes.ExecutionResult{Outputs: map[string]any{"rows": []any{}, "count": 0.0}, Ports: []string{"out"}}, nil
	}

	headers := headerNames(grid[startRow], startCol, endCol)
	rows := make([]any, 0, endRow-startRow)
	for rowIndex := startRow + 1; rowIndex <= endRow && rowIndex < len(grid); rowIndex++ {
		line := grid[rowIndex]
		object := make(map[string]any, len(headers))
		empty := true
		for offset, header := range headers {
			rawText := cellAt(line, startCol+offset)
			if rawText != "" {
				empty = false
			}
			if raw {
				object[header] = typedValue(rawText)
			} else {
				object[header] = rawText
			}
		}
		if empty {
			continue
		}
		rows = append(rows, object)
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"rows": rows, "count": float64(len(rows))}, Ports: []string{"out"}}, nil
}

/* ------------------------------------------------------------------ */
/* Append Excel Rows — append row objects to a table or sheet        */
/* ------------------------------------------------------------------ */

// RegisterAppendRows contributes the Append Excel Rows node.
func RegisterAppendRows(registrar nodes.Registrar) error {
	return registerResolved(registrar, appendRowsDefinition(), resolveAppendRows, executeAppendRows)
}

// appendRowsDefinition is the static contract. The rows input is optional
// because column pins can assemble one row directly from wired values.
func appendRowsDefinition() domain.NodeDefinition {
	return definition("action:excel_append_rows", "Append Excel Rows",
		"Append one or many row objects to an Excel table (or a sheet), creating the table when it is missing.",
		"table-properties",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
			textPin("table", "Table", domain.PinInput, false),
			anyPin("rows", "Rows", domain.PinInput, false),
		},
		[]domain.NodePort{
			thenPin(),
			numberPin("added", "Added", domain.PinOutput, false),
			textPin("path", "Path", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\orders.xlsx", Required: true},
			{Name: "table", Label: "Table", Kind: "string", Placeholder: "Table1 (empty appends to the sheet)"},
			{Name: "rows", Label: "Rows (JSON)", Kind: "textarea", Placeholder: `{"Order": "A-101", "Amount": 3} (or use column pins)`},
			{Name: "columnPins", Label: "Column pins", Kind: "pin-bindings", Placeholder: `Order`},
			{Name: "sheet", Label: "Sheet", Kind: "string", Placeholder: "Sheet1 (used when the table is created)"},
			{Name: "createTableIfMissing", Label: "Create table when missing", Kind: "boolean"},
			{Name: "createWorkbookIfMissing", Label: "Create workbook when missing", Kind: "boolean"},
			{Name: "coerceNumbers", Label: "Convert numeric text to numbers", Kind: "boolean"},
		},
		map[string]any{"table": "", "rows": "", "columnPins": []any{}, "sheet": "", "createTableIfMissing": true, "createWorkbookIfMissing": false, "coerceNumbers": true},
		domain.CapabilityFileWrite,
	)
}

// resolveAppendRows expands configured column pins into data input pins.
// Each execution appends one row assembled from the pins after any rows
// supplied through the rows input — the pin analogue of per-column
// form fields.
func resolveAppendRows(node domain.FlowNode) (domain.NodeDefinition, error) {
	result := appendRowsDefinition()
	rows, err := dynpins.Configured(configOf(node), "columnPins")
	if err != nil {
		return result, err
	}
	result.Inputs = append(result.Inputs, dynpins.InputPins(rows, "#a1a1aa")...)
	return result, nil
}

func executeAppendRows(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("append excel rows cancelled: %w", err)
	}
	path, err := cleanPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	rows, err := rowsInput(invocation)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("rows: %w", err)
	}
	// One row assembled from the column pins joins the rows input: wired
	// values first, row literals as fallback, absent columns skipped.
	columnRows, colErr := dynpins.Configured(invocation.Config, "columnPins")
	if colErr != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("column pins: %w", colErr)
	}
	if columnValues := dynpins.Values(invocation, columnRows); len(columnValues) > 0 {
		rows = append(rows, columnValues)
	}
	if len(rows) == 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("at least one row is required: wire the rows pin, enter rows JSON, or configure column pins")
	}

	file, err := workbookForWrite(invocation, path)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	defer file.Close()

	table := textValue(invocation, "table")
	coerce := configuredBool(invocation, "coerceNumbers", true)
	var sheet string

	if table != "" {
		var tableRange string
		var found bool
		var findErr error
		sheet, tableRange, found, findErr = findTable(file, table)
		if findErr != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("find table %q: %w", table, findErr)
		}
		if found {
			added, err := appendToTable(file, sheet, tableRange, rows, coerce)
			if err != nil {
				return nodes.ExecutionResult{}, err
			}
			if err := saveWorkbook(path, file); err != nil {
				return nodes.ExecutionResult{}, err
			}
			return nodes.ExecutionResult{Outputs: map[string]any{"added": added, "path": path}, Ports: []string{"out"}}, nil
		}
		if !configuredBool(invocation, "createTableIfMissing", true) {
			return nodes.ExecutionResult{}, fmt.Errorf("table %q does not exist in %s", table, path)
		}
		sheet, err = resolveSheet(file, textValue(invocation, "sheet"))
		if err != nil {
			return nodes.ExecutionResult{}, err
		}
		if err := createTableWithData(file, sheet, table, rows, coerce); err != nil {
			return nodes.ExecutionResult{}, err
		}
		if err := saveWorkbook(path, file); err != nil {
			return nodes.ExecutionResult{}, err
		}
		return nodes.ExecutionResult{Outputs: map[string]any{"added": float64(len(rows)), "path": path}, Ports: []string{"out"}}, nil
	}

	// Sheet mode: the first row of the sheet is treated as the header row,
	// matching how exported CSV-style sheets are organized.
	sheet, err = resolveSheet(file, textValue(invocation, "sheet"))
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	grid, err := sheetRows(file, sheet, true)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read sheet %q: %w", sheet, err)
	}
	if len(grid) == 0 {
		if err := writeHeaderRow(file, sheet, 1, sortedKeys(rows[0])); err != nil {
			return nodes.ExecutionResult{}, err
		}
	}
	grid, err = sheetRows(file, sheet, true)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read sheet %q: %w", sheet, err)
	}
	headers := headerNames(grid[0], 0, widestRow(grid, 0)-1)
	writeRow := firstEmptyRow(grid)
	for _, row := range rows {
		if err := writeRowCells(file, sheet, writeRow, headers, row, coerce); err != nil {
			return nodes.ExecutionResult{}, err
		}
		writeRow++
	}
	if err := saveWorkbook(path, file); err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"added": float64(len(rows)), "path": path}, Ports: []string{"out"}}, nil
}

// workbookForWrite opens the workbook for mutation, optionally creating a
// new single-sheet workbook when the file does not exist yet.
func workbookForWrite(invocation nodes.Invocation, path string) (*excelize.File, error) {
	if _, statErr := os.Stat(path); statErr == nil {
		return openWorkbook(path)
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("open workbook: %w", statErr)
	}
	if !configuredBool(invocation, "createWorkbookIfMissing", false) {
		return nil, fmt.Errorf("workbook %s does not exist", path)
	}
	return excelize.NewFile(), nil
}

// appendToTable writes rows directly below an existing table and extends the
// table range to cover them.
func appendToTable(file *excelize.File, sheet, tableRange string, rows []map[string]any, coerce bool) (float64, error) {
	startCol, startRow, endCol, endRow, err := tableRangeBounds(tableRange)
	if err != nil {
		return 0, err
	}
	grid, err := sheetRows(file, sheet, true)
	if err != nil {
		return 0, fmt.Errorf("read sheet %q: %w", sheet, err)
	}
	if startRow >= len(grid) {
		return 0, fmt.Errorf("table header row is missing on sheet %q", sheet)
	}
	headers := headerNames(grid[startRow], startCol, endCol)
	writeRow := endRow + 2
	for _, row := range rows {
		if err := writeRowCells(file, sheet, writeRow, headers, row, coerce); err != nil {
			return 0, err
		}
		writeRow++
	}
	newEndRow := endRow + len(rows) + 1
	if err := reseatTable(file, sheet, tableRange, startCol+1, startRow+1, endCol+1, newEndRow); err != nil {
		return 0, err
	}
	return float64(len(rows)), nil
}

// createTableWithData writes a header row from the union of row keys and
// registers a new table over the written block.
func createTableWithData(file *excelize.File, sheet, table string, rows []map[string]any, coerce bool) error {
	grid, err := sheetRows(file, sheet, true)
	if err != nil {
		return fmt.Errorf("read sheet %q: %w", sheet, err)
	}
	startRow := firstEmptyRow(grid)
	if startRow < 1 {
		startRow = 1
	}
	headers := sortedUnionKeys(rows)
	if err := writeHeaderRow(file, sheet, startRow, headers); err != nil {
		return err
	}
	for offset, row := range rows {
		if err := writeRowCells(file, sheet, startRow+1+offset, headers, row, coerce); err != nil {
			return err
		}
	}
	endRow := startRow + len(rows)
	endColumn := excelColumn(len(headers))
	if err := file.AddTable(sheet, &excelize.Table{
		Name:      table,
		Range:     fmt.Sprintf("A%d:%s%d", startRow, endColumn, endRow),
		StyleName: "TableStyleMedium2",
	}); err != nil {
		return fmt.Errorf("create table %q: %w", table, err)
	}
	return nil
}

// reseatTable widens or shrinks a table range by re-registering the table
// with the same name over a new range. excelize has no resize API, but
// deleting and re-adding the table preserves the header cells on the sheet.
func reseatTable(file *excelize.File, sheet, tableRange string, startCol, startRow, endCol, endRow int) error {
	table, style, err := tableStyle(file, sheet, tableRange)
	if err != nil {
		return err
	}
	if err := file.DeleteTable(table); err != nil {
		return fmt.Errorf("extend table %q: %w", table, err)
	}
	if style == "" {
		style = "TableStyleMedium2"
	}
	if err := file.AddTable(sheet, &excelize.Table{
		Name:      table,
		Range:     fmt.Sprintf("%s%d:%s%d", excelColumn(startCol), startRow, excelColumn(endCol), endRow),
		StyleName: style,
	}); err != nil {
		return fmt.Errorf("extend table %q: %w", table, err)
	}
	return nil
}

// tableStyle finds the table covering tableRange on sheet and returns its
// name and style.
func tableStyle(file *excelize.File, sheet, tableRange string) (string, string, error) {
	tables, err := file.GetTables(sheet)
	if err != nil {
		return "", "", err
	}
	for _, table := range tables {
		if table.Range == tableRange {
			return table.Name, table.StyleName, nil
		}
	}
	return "", "", fmt.Errorf("table with range %q was not found on sheet %q", tableRange, sheet)
}

// writeRowCells writes one row object under the given header names. Columns
// not present in the object keep their existing cell content.
func writeRowCells(file *excelize.File, sheet string, rowNumber int, headers []string, row map[string]any, coerce bool) error {
	for index, header := range headers {
		value, present := row[header]
		if !present {
			continue
		}
		if coerce {
			value = coerceString(value)
		}
		cell := fmt.Sprintf("%s%d", excelColumn(index+1), rowNumber)
		if err := writeCellValue(file, sheet, cell, value); err != nil {
			return fmt.Errorf("write cell %s: %w", cell, err)
		}
	}
	return nil
}

// writeHeaderRow writes header names across one row starting at column A.
func writeHeaderRow(file *excelize.File, sheet string, rowNumber int, headers []string) error {
	for index, header := range headers {
		cell := fmt.Sprintf("%s%d", excelColumn(index+1), rowNumber)
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			return fmt.Errorf("write header %s: %w", cell, err)
		}
	}
	return nil
}

/* ------------------------------------------------------------------ */
/* Update Excel Row — rewrite table rows matched by a key column     */
/* ------------------------------------------------------------------ */

// RegisterUpdateRow contributes the Update Excel Row node.
func RegisterUpdateRow(registrar nodes.Registrar) error {
	return registerResolved(registrar, updateRowDefinition(), resolveUpdateRow, executeUpdateRow)
}

// updateRowDefinition is the static contract. The fields object is optional
// because field pins can carry the update payload directly.
func updateRowDefinition() domain.NodeDefinition {
	return definition("action:excel_update_row", "Update Excel Row",
		"Update the row of an Excel table whose key column equals a value, with optional insert when missing.",
		"pencil",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
			textPin("table", "Table", domain.PinInput, true),
			textPin("keyColumn", "Key column", domain.PinInput, true),
			textPin("keyValue", "Key value", domain.PinInput, true),
			objectPin("fields", "Fields", domain.PinInput, false),
		},
		[]domain.NodePort{
			thenPin(),
			numberPin("updated", "Updated", domain.PinOutput, false),
			numberPin("created", "Created", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\orders.xlsx", Required: true},
			{Name: "table", Label: "Table", Kind: "string", Placeholder: "Table1", Required: true},
			{Name: "keyColumn", Label: "Key column", Kind: "string", Placeholder: "Order", Required: true},
			{Name: "keyValue", Label: "Key value", Kind: "string", Placeholder: "A-101", Required: true},
			{Name: "fields", Label: "Fields", Kind: "kv-hash-fields", Placeholder: "Amount = 5"},
			{Name: "fieldPins", Label: "Field pins", Kind: "pin-bindings", Placeholder: `Amount`},
			{Name: "upsert", Label: "Insert when no row matches", Kind: "boolean"},
			{Name: "coerceNumbers", Label: "Convert numeric text to numbers", Kind: "boolean"},
		},
		map[string]any{"path": "", "table": "", "keyColumn": "", "keyValue": "", "fields": map[string]any{}, "fieldPins": []any{}, "upsert": false, "coerceNumbers": true},
		domain.CapabilityFileWrite,
	)
}

// resolveUpdateRow expands configured field pins into data input pins.
// Pin values merge over the Fields object for the same column so both
// sources compose in one update.
func resolveUpdateRow(node domain.FlowNode) (domain.NodeDefinition, error) {
	result := updateRowDefinition()
	rows, err := dynpins.Configured(configOf(node), "fieldPins")
	if err != nil {
		return result, err
	}
	result.Inputs = append(result.Inputs, dynpins.InputPins(rows, "#a1a1aa")...)
	return result, nil
}

func executeUpdateRow(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("update excel row cancelled: %w", err)
	}
	path, err := cleanPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	table := textValue(invocation, "table")
	if table == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("table is required")
	}
	keyColumn := textValue(invocation, "keyColumn")
	if keyColumn == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key column is required")
	}
	keyValue := textValue(invocation, "keyValue")
	if keyValue == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key value is required")
	}
	fields, err := updateFields(invocation)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}

	file, err := openWorkbook(path)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	defer file.Close()

	sheet, tableRange, found, err := findTable(file, table)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("find table %q: %w", table, err)
	}
	if !found {
		return nodes.ExecutionResult{}, fmt.Errorf("table %q does not exist in %s", table, path)
	}
	startCol, startRow, endCol, endRow, err := tableRangeBounds(tableRange)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	grid, err := sheetRows(file, sheet, true)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read sheet %q: %w", sheet, err)
	}
	if startRow >= len(grid) {
		return nodes.ExecutionResult{}, fmt.Errorf("table header row is missing on sheet %q", sheet)
	}
	headers := headerNames(grid[startRow], startCol, endCol)
	keyIndex := -1
	for index, header := range headers {
		if header == keyColumn {
			keyIndex = index
		}
	}
	if keyIndex < 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("key column %q is not a column of table %q (%s)", keyColumn, table, strings.Join(headers, ", "))
	}

	coerce := configuredBool(invocation, "coerceNumbers", true)
	updated := 0
	for rowIndex := startRow + 1; rowIndex <= endRow && rowIndex < len(grid); rowIndex++ {
		if cellAt(grid[rowIndex], startCol+keyIndex) != keyValue {
			continue
		}
		updated++
		for name, value := range fields {
			if err := writeCellByHeader(file, sheet, headers, startCol, rowIndex, name, value, coerce); err != nil {
				return nodes.ExecutionResult{}, err
			}
		}
	}

	created := 0
	if updated == 0 && configuredBool(invocation, "upsert", false) {
		row := map[string]any{keyColumn: keyValue}
		for name, value := range fields {
			row[name] = value
		}
		if err := appendOneToTable(file, sheet, tableRange, row, coerce); err != nil {
			return nodes.ExecutionResult{}, err
		}
		created = 1
	}
	if err := saveWorkbook(path, file); err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"updated": float64(updated), "created": float64(created)}, Ports: []string{"out"}}, nil
}

// appendOneToTable appends a single row object to an existing table.
func appendOneToTable(file *excelize.File, sheet, tableRange string, row map[string]any, coerce bool) error {
	_, err := appendToTable(file, sheet, tableRange, []map[string]any{row}, coerce)
	return err
}

// writeCellByHeader writes one named field into the row identified by its
// position on the sheet.
func writeCellByHeader(file *excelize.File, sheet string, headers []string, startCol, rowIndex int, name string, value any, coerce bool) error {
	for index, header := range headers {
		if header != name {
			continue
		}
		if coerce {
			value = coerceString(value)
		}
		cell := cellName(startCol+index+1, rowIndex+1)
		if err := writeCellValue(file, sheet, cell, value); err != nil {
			return fmt.Errorf("write cell %s: %w", cell, err)
		}
		return nil
	}
	return fmt.Errorf("column %q is not a column of the table (%s)", name, strings.Join(headers, ", "))
}

/* ------------------------------------------------------------------ */
/* Delete Excel Row — remove table rows matched by a key column      */
/* ------------------------------------------------------------------ */

// RegisterDeleteRow contributes the Delete Excel Row node.
func RegisterDeleteRow(registrar nodes.Registrar) error {
	definition := definition("action:excel_delete_row", "Delete Excel Row",
		"Delete the first (or every) row of an Excel table whose key column equals a value.",
		"trash-2",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
			textPin("table", "Table", domain.PinInput, true),
			textPin("keyColumn", "Key column", domain.PinInput, true),
			textPin("keyValue", "Key value", domain.PinInput, true),
		},
		[]domain.NodePort{
			thenPin(),
			numberPin("deleted", "Deleted", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\orders.xlsx", Required: true},
			{Name: "table", Label: "Table", Kind: "string", Placeholder: "Table1", Required: true},
			{Name: "keyColumn", Label: "Key column", Kind: "string", Placeholder: "Order", Required: true},
			{Name: "keyValue", Label: "Key value", Kind: "string", Placeholder: "A-101", Required: true},
			{Name: "deleteAll", Label: "Delete every matching row", Kind: "boolean"},
		},
		map[string]any{"path": "", "table": "", "keyColumn": "", "keyValue": "", "deleteAll": false},
		domain.CapabilityFileWrite,
	)
	return register(registrar, definition, executeDeleteRow)
}

func executeDeleteRow(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("delete excel row cancelled: %w", err)
	}
	path, err := cleanPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	table := textValue(invocation, "table")
	if table == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("table is required")
	}
	keyColumn := textValue(invocation, "keyColumn")
	if keyColumn == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key column is required")
	}
	keyValue := textValue(invocation, "keyValue")
	if keyValue == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key value is required")
	}

	file, err := openWorkbook(path)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	defer file.Close()

	sheet, tableRange, found, err := findTable(file, table)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("find table %q: %w", table, err)
	}
	if !found {
		return nodes.ExecutionResult{}, fmt.Errorf("table %q does not exist in %s", table, path)
	}
	startCol, startRow, endCol, endRow, err := tableRangeBounds(tableRange)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	grid, err := sheetRows(file, sheet, true)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read sheet %q: %w", sheet, err)
	}
	if startRow >= len(grid) {
		return nodes.ExecutionResult{}, fmt.Errorf("table header row is missing on sheet %q", sheet)
	}
	headers := headerNames(grid[startRow], startCol, endCol)
	keyIndex := -1
	for index, header := range headers {
		if header == keyColumn {
			keyIndex = index
		}
	}
	if keyIndex < 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("key column %q is not a column of table %q (%s)", keyColumn, table, strings.Join(headers, ", "))
	}

	deleteAll := configuredBool(invocation, "deleteAll", false)
	matches := make([]int, 0, 4)
	for rowIndex := startRow + 1; rowIndex <= endRow && rowIndex < len(grid); rowIndex++ {
		if cellAt(grid[rowIndex], startCol+keyIndex) == keyValue {
			matches = append(matches, rowIndex)
			if !deleteAll {
				break
			}
		}
	}
	if len(matches) == 0 {
		return nodes.ExecutionResult{Outputs: map[string]any{"deleted": 0.0}, Ports: []string{"out"}}, nil
	}
	// Remove from the bottom up so earlier row numbers stay valid. excelize
	// shrinks the table range itself while removing rows, and drops a table
	// that would be left with only its header row.
	for index := len(matches) - 1; index >= 0; index-- {
		if err := file.RemoveRow(sheet, matches[index]+1); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("delete row %d: %w", matches[index]+1, err)
		}
	}
	if err := saveWorkbook(path, file); err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"deleted": float64(len(matches))}, Ports: []string{"out"}}, nil
}

/* ------------------------------------------------------------------ */
/* Read / Write Excel Cell                                            */
/* ------------------------------------------------------------------ */

// RegisterReadCell contributes the Read Excel Cell node.
func RegisterReadCell(registrar nodes.Registrar) error {
	return registerResolved(registrar, readCellDefinition(), resolveReadCell, executeReadCell)
}

// readCellDefinition is the static contract. The single cell is optional
// because configured cell pins can expose several cells at once.
func readCellDefinition() domain.NodeDefinition {
	return definition("action:excel_read_cell", "Read Excel Cell",
		"Read cells of an Excel worksheet as typed values or displayed text; configured cells become output pins.",
		"crosshair",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
			textPin("sheet", "Sheet", domain.PinInput, false),
			textPin("cell", "Cell", domain.PinInput, false),
		},
		[]domain.NodePort{
			thenPin(),
			anyPin("value", "Value", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\orders.xlsx", Required: true},
			{Name: "sheet", Label: "Sheet", Kind: "string", Placeholder: "Sheet1 (empty uses the first sheet)"},
			{Name: "cell", Label: "Cell", Kind: "string", Placeholder: "B4 (empty reads only the cell pins)"},
			{Name: "cellPins", Label: "Cell pins", Kind: "pin-bindings-output", Placeholder: `B4`},
			{Name: "valueMode", Label: "Values", Kind: "select", Options: []domain.Option{
				{Value: "raw", Label: "Raw (typed numbers and booleans)"},
				{Value: "formatted", Label: "Formatted (displayed text)"},
			}},
		},
		map[string]any{"sheet": "", "cell": "", "cellPins": []any{}, "valueMode": "raw"},
		domain.CapabilityFileRead,
	)
}

// resolveReadCell expands the configured cell pins into data output pins,
// one per referenced cell, so downstream nodes can wire single cells
// directly.
func resolveReadCell(node domain.FlowNode) (domain.NodeDefinition, error) {
	result := readCellDefinition()
	rows, err := dynpins.Configured(configOf(node), "cellPins")
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		if _, cellErr := cellReference("cell pin", row.Name); cellErr != nil {
			return result, fmt.Errorf("cell pins: %w", cellErr)
		}
	}
	result.Outputs = append(result.Outputs, dynpins.OutputPins(rows, "#a1a1aa")...)
	return result, nil
}

func executeReadCell(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read excel cell cancelled: %w", err)
	}
	path, err := cleanPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	cellPins, pinsErr := dynpins.Configured(invocation.Config, "cellPins")
	if pinsErr != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("cell pins: %w", pinsErr)
	}
	cell := strings.ToUpper(strings.TrimSpace(textValue(invocation, "cell")))
	if cell == "" && len(cellPins) == 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("cell is required: set a cell or configure cell pins")
	}
	if cell != "" {
		if _, _, coordErr := excelize.CellNameToCoordinates(cell); coordErr != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("cell reference %q is invalid", cell)
		}
	}
	file, err := openWorkbook(path)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	defer file.Close()

	sheet, err := resolveSheet(file, textValue(invocation, "sheet"))
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	raw := configured(invocation, "valueMode") != "formatted"
	readCell := func(reference string) (any, error) {
		text, readErr := file.GetCellValue(sheet, reference, excelize.Options{RawCellValue: raw})
		if readErr != nil {
			return nil, fmt.Errorf("read cell %s: %w", reference, readErr)
		}
		if raw {
			return typedValue(text), nil
		}
		return text, nil
	}
	outputs := map[string]any{"value": nil}
	if cell != "" {
		value, readErr := readCell(cell)
		if readErr != nil {
			return nodes.ExecutionResult{}, readErr
		}
		outputs["value"] = value
	}
	for _, row := range cellPins {
		reference, refErr := cellReference("cell pin", row.Name)
		if refErr != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("cell pins: %w", refErr)
		}
		value, readErr := readCell(reference)
		if readErr != nil {
			return nodes.ExecutionResult{}, readErr
		}
		outputs[dynpins.PinID(row.ID)] = value
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"out"}}, nil
}

// RegisterWriteCell contributes the Write Excel Cell node.
func RegisterWriteCell(registrar nodes.Registrar) error {
	return registerResolved(registrar, writeCellDefinition(), resolveWriteCell, executeWriteCell)
}

// writeCellDefinition is the static contract. The single cell and value are
// optional because configured cell pins can write several cells at once.
func writeCellDefinition() domain.NodeDefinition {
	return definition("action:excel_write_cell", "Write Excel Cell",
		"Write typed values into Excel worksheet cells; text starting with = becomes a formula.",
		"text-cursor-input",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
			textPin("sheet", "Sheet", domain.PinInput, true),
			textPin("cell", "Cell", domain.PinInput, false),
			anyPin("value", "Value", domain.PinInput, false),
		},
		[]domain.NodePort{
			thenPin(),
			textPin("path", "Path", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\orders.xlsx", Required: true},
			{Name: "sheet", Label: "Sheet", Kind: "string", Placeholder: "Sheet1", Required: true},
			{Name: "cell", Label: "Cell", Kind: "string", Placeholder: "B4 (or use cell pins)"},
			{Name: "value", Label: "Value", Kind: "string", Placeholder: "42, true, or =SUM(A1:A9)"},
			{Name: "cellPins", Label: "Cell pins", Kind: "pin-bindings", Placeholder: `B4`},
			{Name: "createSheetIfMissing", Label: "Create sheet when missing", Kind: "boolean"},
			{Name: "coerceNumbers", Label: "Convert numeric text to numbers", Kind: "boolean"},
		},
		map[string]any{"sheet": "", "cell": "", "value": "", "cellPins": []any{}, "createSheetIfMissing": false, "coerceNumbers": true},
		domain.CapabilityFileWrite,
	)
}

// resolveWriteCell expands the configured cell pins into data input pins;
// a wired value lands in the referenced cell.
func resolveWriteCell(node domain.FlowNode) (domain.NodeDefinition, error) {
	result := writeCellDefinition()
	rows, err := dynpins.Configured(configOf(node), "cellPins")
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		if _, cellErr := cellReference("cell pin", row.Name); cellErr != nil {
			return result, fmt.Errorf("cell pins: %w", cellErr)
		}
	}
	result.Inputs = append(result.Inputs, dynpins.InputPins(rows, "#a1a1aa")...)
	return result, nil
}

func executeWriteCell(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("write excel cell cancelled: %w", err)
	}
	path, err := cleanPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	sheet := textValue(invocation, "sheet")
	if sheet == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("sheet is required")
	}
	cellPins, pinsErr := dynpins.Configured(invocation.Config, "cellPins")
	if pinsErr != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("cell pins: %w", pinsErr)
	}
	cell := strings.ToUpper(strings.TrimSpace(textValue(invocation, "cell")))
	if cell == "" && len(cellPins) == 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("cell is required: set a cell or configure cell pins")
	}
	value := invocation.Inputs["value"]
	if cell == "" {
		// Pins only: the single-cell contract stays dormant.
		value = nil
	} else {
		if _, _, coordErr := excelize.CellNameToCoordinates(cell); coordErr != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("cell reference %q is invalid", cell)
		}
		if value == nil {
			return nodes.ExecutionResult{}, fmt.Errorf("value is required")
		}
	}

	file, err := openWorkbook(path)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	defer file.Close()

	if index, indexErr := file.GetSheetIndex(sheet); indexErr != nil || index < 0 {
		if !configuredBool(invocation, "createSheetIfMissing", false) {
			return nodes.ExecutionResult{}, fmt.Errorf("worksheet %q does not exist in %s", sheet, path)
		}
		if _, sheetErr := file.NewSheet(sheet); sheetErr != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("create worksheet %q: %w", sheet, sheetErr)
		}
	}
	coerce := configuredBool(invocation, "coerceNumbers", true)
	for _, row := range cellPins {
		reference, refErr := cellReference("cell pin", row.Name)
		if refErr != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("cell pins: %w", refErr)
		}
		pinValue, exists := invocation.Inputs[dynpins.PinID(row.ID)]
		if !exists || pinValue == nil {
			continue
		}
		if coerce {
			pinValue = coerceString(pinValue)
		}
		if err := writeCellValue(file, sheet, reference, pinValue); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("write cell %s: %w", reference, err)
		}
	}
	if cell != "" {
		if coerce {
			value = coerceString(value)
		}
		if err := writeCellValue(file, sheet, cell, value); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("write cell %s: %w", cell, err)
		}
	}
	if err := saveWorkbook(path, file); err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"path": path}, Ports: []string{"out"}}, nil
}

/* ------------------------------------------------------------------ */
/* List / Add Worksheet — worksheet enumeration and creation         */
/* ------------------------------------------------------------------ */

// RegisterListWorksheets contributes the List Excel Worksheets node.
func RegisterListWorksheets(registrar nodes.Registrar) error {
	definition := definition("action:excel_list_worksheets", "List Excel Worksheets",
		"List the worksheets of an Excel workbook.",
		"list",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
		},
		[]domain.NodePort{
			thenPin(),
			listPin("worksheets", "Worksheets", domain.PinOutput, false),
			numberPin("count", "Count", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\orders.xlsx", Required: true},
		},
		map[string]any{},
		domain.CapabilityFileRead,
	)
	return register(registrar, definition, executeListWorksheets)
}

func executeListWorksheets(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("list excel worksheets cancelled: %w", err)
	}
	path, err := cleanPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	file, err := openWorkbook(path)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	defer file.Close()

	sheets := file.GetSheetList()
	names := make([]any, 0, len(sheets))
	for _, sheet := range sheets {
		names = append(names, sheet)
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"worksheets": names, "count": float64(len(names))}, Ports: []string{"out"}}, nil
}

// RegisterAddWorksheet contributes the Add Excel Worksheet node.
func RegisterAddWorksheet(registrar nodes.Registrar) error {
	definition := definition("action:excel_add_worksheet", "Add Excel Worksheet",
		"Add a new worksheet to an Excel workbook, or report the existing one with the same name.",
		"columns-3",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
			textPin("name", "Name", domain.PinInput, true),
		},
		[]domain.NodePort{
			thenPin(),
			textPin("name", "Name", domain.PinOutput, false),
			numberPin("index", "Index", domain.PinOutput, false),
			boolPin("created", "Created", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\orders.xlsx", Required: true},
			{Name: "name", Label: "Name", Kind: "string", Placeholder: "February", Required: true},
			{Name: "failIfExists", Label: "Fail when the sheet already exists", Kind: "boolean"},
		},
		map[string]any{"name": "", "failIfExists": false},
		domain.CapabilityFileWrite,
	)
	return register(registrar, definition, executeAddWorksheet)
}

func executeAddWorksheet(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("add excel worksheet cancelled: %w", err)
	}
	path, err := cleanPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	name := textValue(invocation, "name")
	if name == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("worksheet name is required")
	}
	if err := validateSheetName(name); err != nil {
		return nodes.ExecutionResult{}, err
	}

	file, err := openWorkbook(path)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	defer file.Close()

	if index, indexErr := file.GetSheetIndex(name); indexErr == nil && index >= 0 {
		if configuredBool(invocation, "failIfExists", false) {
			return nodes.ExecutionResult{}, fmt.Errorf("worksheet %q already exists in %s", name, path)
		}
		return nodes.ExecutionResult{Outputs: map[string]any{"name": name, "index": float64(index), "created": false}, Ports: []string{"out"}}, nil
	}
	index, sheetErr := file.NewSheet(name)
	if sheetErr != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create worksheet %q: %w", name, sheetErr)
	}
	if err := saveWorkbook(path, file); err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"name": name, "index": float64(index), "created": true}, Ports: []string{"out"}}, nil
}

/* ------------------------------------------------------------------ */
/* Shared grid helpers                                                */
/* ------------------------------------------------------------------ */

// cellAt returns the value at a zero-based column offset of a grid row.
func cellAt(row []string, col int) string {
	if col < 0 || col >= len(row) {
		return ""
	}
	return row[col]
}

// widestRow returns the width of the widest row at or after startRow.
func widestRow(grid [][]string, startRow int) int {
	width := 0
	for index := startRow; index < len(grid); index++ {
		if len(grid[index]) > width {
			width = len(grid[index])
		}
	}
	return width
}

// headerNames normalizes one grid row into unique column names. Empty cells
// become generated names; duplicates gain numeric suffixes.
func headerNames(row []string, startCol, endCol int) []string {
	seen := make(map[string]int)
	headers := make([]string, 0, endCol-startCol+1)
	for col := startCol; col <= endCol; col++ {
		name := strings.TrimSpace(cellAt(row, col))
		if name == "" {
			name = fmt.Sprintf("col%d", col+1)
		}
		if count, exists := seen[name]; exists {
			seen[name] = count + 1
			name = fmt.Sprintf("%s_%d", name, count+1)
		} else {
			seen[name] = 1
		}
		headers = append(headers, name)
	}
	return headers
}

// firstEmptyRow returns the one-based number of the first fully-empty row
// after the last non-empty row of a grid.
func firstEmptyRow(grid [][]string) int {
	last := 0
	for index, row := range grid {
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				last = index + 1
				break
			}
		}
	}
	return last + 1
}

// sortedKeys returns the alphabetically ordered keys of a row object.
func sortedKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sortedUnionKeys merges the keys of all rows in deterministic order.
func sortedUnionKeys(rows []map[string]any) []string {
	union := make(map[string]struct{})
	for _, row := range rows {
		for key := range row {
			union[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(union))
	for key := range union {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// cellName builds an A1 cell reference from one-based coordinates.
func cellName(col, row int) string {
	return fmt.Sprintf("%s%d", excelColumn(col), row)
}

// excelColumn converts a one-based column number into its letters.
func excelColumn(col int) string {
	if col < 1 {
		return "A"
	}
	name, err := excelize.ColumnNumberToName(col)
	if err != nil {
		return "A"
	}
	return name
}

// coerced applies the numeric text conversion when enabled.
func coerced(value any, coerce bool) any {
	if !coerce {
		return value
	}
	return coerceString(value)
}

// validateSheetName enforces the Excel worksheet naming rules before the
// library rejects the name with a generic error.
func validateSheetName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("worksheet name is required")
	}
	if len(name) > 31 {
		return fmt.Errorf("worksheet name must be 31 characters or fewer")
	}
	if strings.ContainsAny(name, `[]:*?/\`) {
		return fmt.Errorf("worksheet name must not contain any of []:*?/\\")
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("worksheet name must not be blank")
	}
	return nil
}

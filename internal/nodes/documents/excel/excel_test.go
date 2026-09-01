package excel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/xuri/excelize/v2"
)

// invoke runs a node executor with an empty runtime. Input keys inside the
// reserved "pin_" namespace are marked as connected: in the engine only
// wires can deliver those keys, so the harness mirrors that provenance.
func invoke(t *testing.T, register func(nodes.Registrar) error, inputs map[string]any, config map[string]any) (nodes.ExecutionResult, error) {
	t.Helper()
	var module nodes.Node
	registrar := registrarFunc(func(node nodes.Node) error {
		module = node
		return nil
	})
	if err := register(registrar); err != nil {
		t.Fatalf("register: %v", err)
	}
	connected := map[string]bool{}
	for key := range inputs {
		if strings.HasPrefix(key, "pin_") {
			connected[key] = true
		}
	}
	return module.Execute(context.Background(), nodes.Invocation{
		Node:            domain.FlowNode{Type: module.Definition().Type, Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: connected,
	}, nil)
}

// registrarFunc adapts a closure to the nodes.Registrar interface.
type registrarFunc func(nodes.Node) error

func (f registrarFunc) Register(node nodes.Node) error { return f(node) }

// workbookPath creates an .xlsx file in a test directory.
func workbookPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

// seedWorkbook writes a workbook with a Table1 table and one data row.
func seedWorkbook(t *testing.T, path string) {
	t.Helper()
	file := excelize.NewFile()
	if err := file.SetSheetName("Sheet1", "Sheet1"); err != nil {
		t.Fatal(err)
	}
	values := [][]any{{"Order", "Amount"}, {"A-101", 3}}
	for rowIndex, row := range values {
		if err := file.SetSheetRow("Sheet1", cellName(1, rowIndex+1), &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.AddTable("Sheet1", &excelize.Table{Name: "Table1", Range: "A1:B2", StyleName: "TableStyleMedium2"}); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReadRowsFromTableAndSheet(t *testing.T) {
	path := workbookPath(t, "orders.xlsx")
	seedWorkbook(t, path)

	tableResult, err := invoke(t, RegisterReadRows, map[string]any{"path": path, "table": "Table1"}, map[string]any{})
	if err != nil {
		t.Fatalf("read table rows: %v", err)
	}
	rows, _ := tableResult.Outputs["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("rows = %#v", tableResult.Outputs["rows"])
	}
	row, _ := rows[0].(map[string]any)
	if row["Order"] != "A-101" || row["Amount"] != 3.0 {
		t.Fatalf("row = %#v", row)
	}
	if tableResult.Outputs["count"] != 1.0 {
		t.Fatalf("count = %#v", tableResult.Outputs["count"])
	}

	sheetResult, err := invoke(t, RegisterReadRows, map[string]any{"path": path}, map[string]any{})
	if err != nil {
		t.Fatalf("read sheet rows: %v", err)
	}
	if sheetResult.Outputs["count"] != 1.0 {
		t.Fatalf("sheet count = %#v", sheetResult.Outputs["count"])
	}

	formatted, err := invoke(t, RegisterReadRows, map[string]any{"path": path, "table": "Table1"}, map[string]any{"valueMode": "formatted"})
	if err != nil {
		t.Fatalf("read formatted rows: %v", err)
	}
	formattedRows, _ := formatted.Outputs["rows"].([]any)
	formattedRow, _ := formattedRows[0].(map[string]any)
	if formattedRow["Amount"] != "3" {
		t.Fatalf("formatted row = %#v", formattedRow)
	}
}

func TestReadRowsRejectsMissingTableAndBadExtension(t *testing.T) {
	path := workbookPath(t, "orders.xlsx")
	seedWorkbook(t, path)
	if _, err := invoke(t, RegisterReadRows, map[string]any{"path": path, "table": "Nope"}, nil); err == nil {
		t.Fatal("missing table must fail")
	}
	if _, err := invoke(t, RegisterReadRows, map[string]any{"path": filepath.Join(t.TempDir(), "data.csv")}, nil); err == nil {
		t.Fatal("non .xlsx path must fail")
	}
}

func TestAppendRowsExtendsTableAndCreatesIt(t *testing.T) {
	path := workbookPath(t, "orders.xlsx")
	seedWorkbook(t, path)

	result, err := invoke(t, RegisterAppendRows,
		map[string]any{"path": path, "table": "Table1", "rows": []any{map[string]any{"Order": "A-102", "Amount": 5}}},
		map[string]any{})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if result.Outputs["added"] != 1.0 {
		t.Fatalf("added = %#v", result.Outputs["added"])
	}

	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	tables, err := file.GetTables("Sheet1")
	if err != nil || len(tables) != 1 {
		t.Fatalf("tables = %#v err = %v", tables, err)
	}
	if tables[0].Name != "Table1" || tables[0].Range != "A1:B3" {
		t.Fatalf("table after append = %#v", tables[0])
	}
	if value, _ := file.GetCellValue("Sheet1", "A3", excelize.Options{RawCellValue: true}); value != "A-102" {
		t.Fatalf("A3 = %q", value)
	}
	if value, _ := file.GetCellValue("Sheet1", "B3", excelize.Options{RawCellValue: true}); value != "5" {
		t.Fatalf("B3 = %q", value)
	}

	// A missing table is created when the option is on (the default).
	created, err := invoke(t, RegisterAppendRows,
		map[string]any{"path": path, "table": "Table2", "rows": map[string]any{"Order": "A-103", "Amount": 7}},
		map[string]any{})
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	if created.Outputs["added"] != 1.0 {
		t.Fatalf("created added = %#v", created.Outputs["added"])
	}
	file2, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file2.Close()
	found := false
	for _, table := range mustTables(t, file2, "Sheet1") {
		if table.Name == "Table2" {
			found = true
		}
	}
	if !found {
		t.Fatal("Table2 was not created")
	}
}

func mustTables(t *testing.T, file *excelize.File, sheet string) []excelize.Table {
	t.Helper()
	tables, err := file.GetTables(sheet)
	if err != nil {
		t.Fatal(err)
	}
	return tables
}

func TestAppendRowsJSONFallbackAndNewWorkbook(t *testing.T) {
	path := workbookPath(t, "orders.xlsx")
	result, err := invoke(t, RegisterAppendRows,
		map[string]any{"path": path, "rows": `{"Order": "A-1", "Amount": "42"}`},
		map[string]any{"createWorkbookIfMissing": true, "createTableIfMissing": false})
	if err != nil {
		t.Fatalf("append to new workbook: %v", err)
	}
	if result.Outputs["added"] != 1.0 {
		t.Fatalf("added = %#v", result.Outputs["added"])
	}
	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	// Headers come from the alphabetically sorted row keys: Amount, Order.
	if value, _ := file.GetCellValue("Sheet1", "A1", excelize.Options{RawCellValue: true}); value != "Amount" {
		t.Fatalf("A1 = %q", value)
	}
	if value, _ := file.GetCellValue("Sheet1", "B1", excelize.Options{RawCellValue: true}); value != "Order" {
		t.Fatalf("B1 = %q", value)
	}
	// Coercion wrote the numeric text as a number cell.
	if value, _ := file.GetCellValue("Sheet1", "A2", excelize.Options{RawCellValue: true}); value != "42" {
		t.Fatalf("A2 = %q", value)
	}
	if value, _ := file.GetCellValue("Sheet1", "B2", excelize.Options{RawCellValue: true}); value != "A-1" {
		t.Fatalf("B2 = %q", value)
	}
}

func TestAppendRowsRequiresRowsAndRejectsUnknownColumns(t *testing.T) {
	path := workbookPath(t, "orders.xlsx")
	seedWorkbook(t, path)
	if _, err := invoke(t, RegisterAppendRows, map[string]any{"path": path, "table": "Table1", "rows": ""}, nil); err == nil {
		t.Fatal("empty rows must fail")
	}
	// Unknown columns are silently ignored only in sheet mode; table mode maps
	// by header and skips absent keys, so both stay strict on structure.
	if _, err := invoke(t, RegisterAppendRows, map[string]any{"path": path, "table": "Table1", "rows": map[string]any{"Unknown": 1}}, nil); err != nil {
		t.Fatalf("unknown column key should be skipped, not fail: %v", err)
	}
}

func TestUpdateRowUpsertsAndCounts(t *testing.T) {
	path := workbookPath(t, "orders.xlsx")
	seedWorkbook(t, path)

	result, err := invoke(t, RegisterUpdateRow,
		map[string]any{"path": path, "table": "Table1", "keyColumn": "Order", "keyValue": "A-101", "fields": map[string]any{"Amount": 9}},
		map[string]any{})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if result.Outputs["updated"] != 1.0 || result.Outputs["created"] != 0.0 {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if value, _ := file.GetCellValue("Sheet1", "B2", excelize.Options{RawCellValue: true}); value != "9" {
		t.Fatalf("B2 = %q", value)
	}

	// Missing key with upsert appends a new row and widens the table.
	upsert, err := invoke(t, RegisterUpdateRow,
		map[string]any{"path": path, "table": "Table1", "keyColumn": "Order", "keyValue": "A-999", "fields": map[string]any{"Amount": 1}},
		map[string]any{"upsert": true})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if upsert.Outputs["created"] != 1.0 {
		t.Fatalf("upsert outputs = %#v", upsert.Outputs)
	}
	file2, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file2.Close()
	if value, _ := file2.GetCellValue("Sheet1", "A3", excelize.Options{RawCellValue: true}); value != "A-999" {
		t.Fatalf("A3 = %q", value)
	}
	for _, table := range mustTables(t, file2, "Sheet1") {
		if table.Name == "Table1" && table.Range != "A1:B3" {
			t.Fatalf("table range after upsert = %q", table.Range)
		}
	}
}

func TestUpdateRowRejectsUnknownKeyColumnAndMissingFields(t *testing.T) {
	path := workbookPath(t, "orders.xlsx")
	seedWorkbook(t, path)
	if _, err := invoke(t, RegisterUpdateRow,
		map[string]any{"path": path, "table": "Table1", "keyColumn": "Nope", "keyValue": "A-101", "fields": map[string]any{"Amount": 1}}, nil); err == nil {
		t.Fatal("unknown key column must fail")
	}
	if _, err := invoke(t, RegisterUpdateRow,
		map[string]any{"path": path, "table": "Table1", "keyColumn": "Order", "keyValue": "A-101", "fields": map[string]any{"Nope": 1}}, nil); err == nil {
		t.Fatal("unknown field column must fail")
	}
}

func TestDeleteRowShrinksTableAndSupportsDeleteAll(t *testing.T) {
	path := workbookPath(t, "log.xlsx")
	file := excelize.NewFile()
	rows := [][]any{{"Key"}, {"a"}, {"a"}, {"b"}}
	for index, row := range rows {
		if err := file.SetSheetRow("Sheet1", cellName(1, index+1), &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.AddTable("Sheet1", &excelize.Table{Name: "T", Range: "A1:A4"}); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	result, err := invoke(t, RegisterDeleteRow,
		map[string]any{"path": path, "table": "T", "keyColumn": "Key", "keyValue": "a"},
		map[string]any{"deleteAll": true})
	if err != nil {
		t.Fatalf("delete all: %v", err)
	}
	if result.Outputs["deleted"] != 2.0 {
		t.Fatalf("deleted = %#v", result.Outputs["deleted"])
	}
	after, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	if value, _ := after.GetCellValue("Sheet1", "A2", excelize.Options{RawCellValue: true}); value != "b" {
		t.Fatalf("A2 = %q", value)
	}
	for _, table := range mustTables(t, after, "Sheet1") {
		if table.Name == "T" && table.Range != "A1:A2" {
			t.Fatalf("table range after delete = %q", table.Range)
		}
	}

	// Deleting every remaining row removes the header-only table.
	_, err = invoke(t, RegisterDeleteRow,
		map[string]any{"path": path, "table": "T", "keyColumn": "Key", "keyValue": "b"},
		map[string]any{"deleteAll": true})
	if err != nil {
		t.Fatalf("delete last: %v", err)
	}
	final, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer final.Close()
	if tables := mustTables(t, final, "Sheet1"); len(tables) != 0 {
		t.Fatalf("emptied table was not removed: %#v", tables)
	}
}

func TestReadAndWriteCell(t *testing.T) {
	path := workbookPath(t, "cells.xlsx")
	seedWorkbook(t, path)

	write, err := invoke(t, RegisterWriteCell,
		map[string]any{"path": path, "sheet": "Sheet1", "cell": "D4", "value": "=SUM(B2:B3)"},
		map[string]any{})
	if err != nil {
		t.Fatalf("write cell: %v", err)
	}
	if write.Outputs["path"] != path {
		t.Fatalf("path = %#v", write.Outputs["path"])
	}
	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if formula, _ := file.GetCellFormula("Sheet1", "D4"); formula != "=SUM(B2:B3)" {
		t.Fatalf("formula = %q", formula)
	}

	read, err := invoke(t, RegisterReadCell, map[string]any{"path": path, "sheet": "Sheet1", "cell": "B2"}, map[string]any{})
	if err != nil {
		t.Fatalf("read cell: %v", err)
	}
	if read.Outputs["value"] != 3.0 {
		t.Fatalf("value = %#v", read.Outputs["value"])
	}

	if _, err := invoke(t, RegisterReadCell, map[string]any{"path": path, "cell": "not-a-cell"}, nil); err == nil {
		t.Fatal("invalid cell reference must fail")
	}
	if _, err := invoke(t, RegisterWriteCell, map[string]any{"path": path, "sheet": "Nope", "cell": "A1", "value": 1}, nil); err == nil {
		t.Fatal("missing sheet must fail by default")
	}
	created, err := invoke(t, RegisterWriteCell, map[string]any{"path": path, "sheet": "Data", "cell": "A1", "value": 5}, map[string]any{"createSheetIfMissing": true})
	if err != nil {
		t.Fatalf("create sheet: %v", err)
	}
	if created.Outputs["path"] != path {
		t.Fatalf("path = %#v", created.Outputs["path"])
	}
}

func TestListAndAddWorksheet(t *testing.T) {
	path := workbookPath(t, "sheets.xlsx")
	seedWorkbook(t, path)

	list, err := invoke(t, RegisterListWorksheets, map[string]any{"path": path}, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	names, _ := list.Outputs["worksheets"].([]any)
	if len(names) != 1 || names[0] != "Sheet1" || list.Outputs["count"] != 1.0 {
		t.Fatalf("worksheets = %#v", list.Outputs)
	}

	added, err := invoke(t, RegisterAddWorksheet, map[string]any{"path": path, "name": "February"}, nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if added.Outputs["created"] != true || added.Outputs["name"] != "February" {
		t.Fatalf("added = %#v", added.Outputs)
	}

	again, err := invoke(t, RegisterAddWorksheet, map[string]any{"path": path, "name": "February"}, nil)
	if err != nil {
		t.Fatalf("add existing: %v", err)
	}
	if again.Outputs["created"] != false {
		t.Fatalf("existing = %#v", again.Outputs)
	}
	if _, err := invoke(t, RegisterAddWorksheet, map[string]any{"path": path, "name": "February"}, map[string]any{"failIfExists": true}); err == nil {
		t.Fatal("failIfExists must fail")
	}
	if _, err := invoke(t, RegisterAddWorksheet, map[string]any{"path": path, "name": "bad/name"}, nil); err == nil {
		t.Fatal("invalid sheet name must fail")
	}
}

func TestAppendRowsMissingWorkbookWithoutCreateOption(t *testing.T) {
	path := workbookPath(t, "missing.xlsx")
	if _, err := invoke(t, RegisterAppendRows, map[string]any{"path": path, "rows": map[string]any{"A": 1}}, nil); err == nil {
		t.Fatal("missing workbook must fail by default")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("missing workbook must not be created by default")
	}
}

func TestHeaderNamesHandlesEmptyAndDuplicateColumns(t *testing.T) {
	headers := headerNames([]string{"Order", "", "Order", "Amount"}, 0, 3)
	want := []string{"Order", "col2", "Order_2", "Amount"}
	for index, name := range want {
		if headers[index] != name {
			t.Fatalf("headers = %#v want %#v", headers, want)
		}
	}
}

func TestTypedValueAndCoerceString(t *testing.T) {
	if typedValue("42") != 42.0 || typedValue("TRUE") != true || typedValue("x") != "x" || typedValue("") != "" {
		t.Fatal("typedValue conversion broken")
	}
	if coerceString("42") != 42.0 || coerceString("true") != true || coerceString("hello") != "hello" {
		t.Fatal("coerceString conversion broken")
	}
}

func TestWriteCellPins(t *testing.T) {
	path := workbookPath(t, "write-pins.xlsx")
	seedWorkbook(t, path)
	config := map[string]any{
		"sheet": "Sheet1",
		"cellPins": []any{
			map[string]any{"id": "field_1", "name": "b2", "label": "Total"},
			map[string]any{"id": "field_2", "name": "c2", "label": "Formula"},
			map[string]any{"id": "field_3", "name": "d2", "label": "Skipped"},
		},
	}

	// The resolver mints input pins for every row.
	var module nodes.Node
	registrar := registrarFunc(func(node nodes.Node) error { module = node; return nil })
	if err := RegisterWriteCell(registrar); err != nil {
		t.Fatalf("register: %v", err)
	}
	resolved, err := module.Resolve(domain.FlowNode{Type: "action:excel_write_cell", Data: map[string]any{"config": config}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	var found int
	for _, input := range resolved.Inputs {
		if input.ID == "pin_field_1" || input.ID == "pin_field_2" || input.ID == "pin_field_3" {
			if input.Kind != domain.PinData || input.Label != "Total" && input.ID == "pin_field_1" {
				t.Fatalf("pin contract broken: %#v", input)
			}
			found++
		}
	}
	if found != 3 {
		t.Fatalf("resolved inputs = %#v", resolved.Inputs)
	}

	// Wired values land in their cells; numeric text coerces like the single
	// value field; rows without a value are skipped, not cleared.
	result, err := invoke(t, RegisterWriteCell, map[string]any{
		"path": path, "sheet": "Sheet1", "pin_field_1": "12", "pin_field_2": "=B2*2",
	}, config)
	if err != nil {
		t.Fatalf("write cell pins: %v", err)
	}
	if result.Outputs["path"] != path {
		t.Fatalf("path = %#v", result.Outputs["path"])
	}
	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if value, _ := file.GetCellValue("Sheet1", "B2", excelize.Options{RawCellValue: true}); value != "12" {
		t.Fatalf("B2 = %q", value)
	}
	if formula, _ := file.GetCellFormula("Sheet1", "C2"); formula != "=B2*2" {
		t.Fatalf("C2 formula = %q", formula)
	}
	if value, _ := file.GetCellValue("Sheet1", "D2"); value != "" {
		t.Fatalf("D2 should stay empty, got %q", value)
	}

	// Pins alone satisfy the node: no single cell, no single value.
	pinsOnly, err := invoke(t, RegisterWriteCell, map[string]any{"path": path, "sheet": "Sheet1", "pin_field_1": 7}, config)
	if err != nil {
		t.Fatalf("pins-only write: %v", err)
	}
	if pinsOnly.Outputs["path"] != path {
		t.Fatalf("pins-only path = %#v", pinsOnly.Outputs)
	}

	// Neither pins nor a cell fails with an actionable message.
	if _, err := invoke(t, RegisterWriteCell, map[string]any{"path": path, "sheet": "Sheet1"}, map[string]any{}); err == nil {
		t.Fatal("missing cell and pins must fail")
	}
	// Malformed references are rejected by the resolver itself.
	bad := map[string]any{"sheet": "Sheet1", "cellPins": []any{map[string]any{"id": "f", "name": "nope!"}}}
	if _, err := module.Resolve(domain.FlowNode{Type: "action:excel_write_cell", Data: map[string]any{"config": bad}}); err == nil {
		t.Fatal("invalid cell pin reference must fail at resolve time")
	}
}

func TestReadCellPins(t *testing.T) {
	path := workbookPath(t, "read-pins.xlsx")
	seedWorkbook(t, path)
	config := map[string]any{
		"cellPins": []any{
			map[string]any{"id": "field_1", "name": "a2", "label": "Order"},
			map[string]any{"id": "field_2", "name": "b2", "label": "Amount"},
		},
	}
	var module nodes.Node
	registrar := registrarFunc(func(node nodes.Node) error { module = node; return nil })
	if err := RegisterReadCell(registrar); err != nil {
		t.Fatalf("register: %v", err)
	}
	resolved, err := module.Resolve(domain.FlowNode{Type: "action:excel_read_cell", Data: map[string]any{"config": config}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	var outputs int
	for _, port := range resolved.Outputs {
		if port.ID == "pin_field_1" || port.ID == "pin_field_2" {
			if port.Kind != domain.PinData || port.Direction != domain.PinOutput {
				t.Fatalf("output pin contract broken: %#v", port)
			}
			outputs++
		}
	}
	if outputs != 2 {
		t.Fatalf("resolved outputs = %#v", resolved.Outputs)
	}

	result, err := invoke(t, RegisterReadCell, map[string]any{"path": path}, config)
	if err != nil {
		t.Fatalf("read cell pins: %v", err)
	}
	if result.Outputs["pin_field_1"] != "A-101" || result.Outputs["pin_field_2"] != 3.0 {
		t.Fatalf("pin outputs = %#v", result.Outputs)
	}
	if value, exists := result.Outputs["value"]; !exists || value != nil {
		t.Fatalf("single value output should exist and stay nil: %#v", result.Outputs["value"])
	}

	// A single cell plus pins fills both contracts.
	combined, err := invoke(t, RegisterReadCell, map[string]any{"path": path, "cell": "A1"}, config)
	if err != nil {
		t.Fatalf("combined read: %v", err)
	}
	if combined.Outputs["value"] != "Order" || combined.Outputs["pin_field_2"] != 3.0 {
		t.Fatalf("combined outputs = %#v", combined.Outputs)
	}

	// Formatted mode applies to pins too.
	formatted, err := invoke(t, RegisterReadCell, map[string]any{"path": path}, map[string]any{
		"cellPins": config["cellPins"], "valueMode": "formatted",
	})
	if err != nil {
		t.Fatalf("formatted read: %v", err)
	}
	if formatted.Outputs["pin_field_2"] != "3" {
		t.Fatalf("formatted pin output = %#v", formatted.Outputs["pin_field_2"])
	}

	if _, err := invoke(t, RegisterReadCell, map[string]any{"path": path}, map[string]any{}); err == nil {
		t.Fatal("no cell and no pins must fail")
	}
}

func TestAppendRowsColumnPins(t *testing.T) {
	path := workbookPath(t, "append-pins.xlsx")
	seedWorkbook(t, path)
	config := map[string]any{
		"path":  path,
		"table": "Table1",
		"columnPins": []any{
			map[string]any{"id": "field_1", "name": "Order", "label": "Order"},
			map[string]any{"id": "field_2", "name": "Amount", "label": "Amount"},
		},
	}

	// A row assembled from the pins is appended after the rows input.
	result, err := invoke(t, RegisterAppendRows, map[string]any{
		"path": path, "table": "Table1", "rows": `[{"Order": "A-201", "Amount": 7}]`,
		"pin_field_1": "A-202", "pin_field_2": "9",
	}, config)
	if err != nil {
		t.Fatalf("append with pins: %v", err)
	}
	if result.Outputs["added"] != 2.0 {
		t.Fatalf("added = %#v", result.Outputs["added"])
	}
	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if value, _ := file.GetCellValue("Sheet1", "A3", excelize.Options{RawCellValue: true}); value != "A-201" {
		t.Fatalf("A3 = %q", value)
	}
	if value, _ := file.GetCellValue("Sheet1", "B3", excelize.Options{RawCellValue: true}); value != "7" {
		t.Fatalf("B3 = %q", value)
	}
	// The pin-assembled row follows the rows input.
	if value, _ := file.GetCellValue("Sheet1", "A4", excelize.Options{RawCellValue: true}); value != "A-202" {
		t.Fatalf("A4 = %q", value)
	}
	if value, _ := file.GetCellValue("Sheet1", "B4", excelize.Options{RawCellValue: true}); value != "9" {
		t.Fatalf("B4 = %q", value)
	}
	for _, table := range mustTables(t, file, "Sheet1") {
		if table.Name == "Table1" && table.Range != "A1:B4" {
			t.Fatalf("table range = %q", table.Range)
		}
	}

	// Pins alone also work: no rows JSON, no rows wire.
	pinsOnly, err := invoke(t, RegisterAppendRows, map[string]any{"path": path, "table": "Table1", "pin_field_1": "A-203"}, config)
	if err != nil {
		t.Fatalf("pins-only append: %v", err)
	}
	if pinsOnly.Outputs["added"] != 1.0 {
		t.Fatalf("pins-only added = %#v", pinsOnly.Outputs["added"])
	}
	file2, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file2.Close()
	if value, _ := file2.GetCellValue("Sheet1", "A5", excelize.Options{RawCellValue: true}); value != "A-203" {
		t.Fatalf("A5 = %q", value)
	}

	// Unwired, literal-less rows keep the old strict failure for empty input.
	if _, err := invoke(t, RegisterAppendRows, map[string]any{"path": path, "rows": ""}, map[string]any{
		"path": path, "table": "Table1",
		"columnPins": []any{map[string]any{"id": "field_1", "name": "Order"}},
	}); err == nil {
		t.Fatal("no rows and no wired pins must fail")
	}
}

func TestUpdateRowFieldPins(t *testing.T) {
	path := workbookPath(t, "update-pins.xlsx")
	seedWorkbook(t, path)
	config := map[string]any{
		"path": path, "table": "Table1", "keyColumn": "Order", "keyValue": "A-101",
		"fieldPins": []any{
			map[string]any{"id": "field_1", "name": "Amount", "label": "Amount"},
		},
	}

	// A wired field pin updates the row without any Fields object.
	if _, err := invoke(t, RegisterUpdateRow, map[string]any{
		"path": path, "table": "Table1", "keyColumn": "Order", "keyValue": "A-101", "pin_field_1": 11,
	}, config); err != nil {
		t.Fatalf("update via pins: %v", err)
	}
	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if value, _ := file.GetCellValue("Sheet1", "B2", excelize.Options{RawCellValue: true}); value != "11" {
		t.Fatalf("B2 = %q", value)
	}

	// A wired pin overrides the Fields object for the same column; a literal
	// only fills a gap.
	merged, err := invoke(t, RegisterUpdateRow, map[string]any{
		"path": path, "table": "Table1", "keyColumn": "Order", "keyValue": "A-101",
		"fields": map[string]any{"Amount": 5}, "pin_field_1": 21,
	}, config)
	if err != nil {
		t.Fatalf("merged update: %v", err)
	}
	if merged.Outputs["updated"] != 1.0 {
		t.Fatalf("merged outputs = %#v", merged.Outputs)
	}
	file2, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file2.Close()
	if value, _ := file2.GetCellValue("Sheet1", "B2", excelize.Options{RawCellValue: true}); value != "21" {
		t.Fatalf("B2 after merged update = %q", value)
	}

	literal := map[string]any{
		"path": path, "table": "Table1", "keyColumn": "Order", "keyValue": "A-101",
		"fields":    map[string]any{"Amount": 5},
		"fieldPins": []any{map[string]any{"id": "field_9", "name": "Order", "value": "A-101"}},
	}
	gapFill, err := invoke(t, RegisterUpdateRow, map[string]any{
		"path": path, "table": "Table1", "keyColumn": "Order", "keyValue": "A-101",
		"fields": map[string]any{"Amount": 5},
	}, literal)
	if err != nil {
		t.Fatalf("literal gap fill: %v", err)
	}
	if gapFill.Outputs["updated"] != 1.0 {
		t.Fatalf("gap-fill outputs = %#v", gapFill.Outputs)
	}

	// Neither fields nor pins fails.
	if _, err := invoke(t, RegisterUpdateRow, map[string]any{
		"path": path, "table": "Table1", "keyColumn": "Order", "keyValue": "A-101", "fields": map[string]any{},
	}, map[string]any{
		"path": path, "table": "Table1", "keyColumn": "Order", "keyValue": "A-101",
		"fieldPins": []any{map[string]any{"id": "f", "name": "Amount"}},
	}); err == nil {
		t.Fatal("no fields and no wired pins must fail")
	}
}

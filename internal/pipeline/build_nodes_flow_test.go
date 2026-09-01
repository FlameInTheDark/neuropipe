package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// TestBuildArrayFeedsExcelRowsThroughEngine proves the full pure-node path:
// JSON-object item constants become pin defaults, the assembled array flows
// into an action's any-typed input, and the rows land in the workbook.
func TestBuildArrayFeedsExcelRowsThroughEngine(t *testing.T) {
	directory := t.TempDir()
	workbook := filepath.Join(directory, "orders.xlsx")

	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("rows", "data:build_array", map[string]any{"elementType": "object", "items": []any{
			map[string]any{"id": "row_1", "label": "First order", "value": `{"Order": "A-101", "Amount": 3}`},
			map[string]any{"id": "row_2", "label": "Second order", "value": `{"Order": "A-102", "Amount": 5}`},
		}}),
		cfgNode("append", "action:excel_append_rows", map[string]any{
			"path": workbook, "table": "Table1", "createWorkbookIfMissing": true,
		}),
		cfgNode("read", "action:excel_read_rows", map[string]any{"path": workbook, "table": "Table1"}),
	}, Edges: []domain.FlowEdge{
		dataEdge("rows-append", "rows", "array", "append", "rows"),
		execEdge("start-append", "start", "out", "append", "in"),
		execEdge("append-read", "append", "out", "read", "in"),
	}}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var rows []any
	for _, run := range result.NodeRuns {
		if run.NodeID != "read" {
			continue
		}
		output, _ := run.Output.(map[string]any)
		rows, _ = output["rows"].([]any)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want the two assembled rows", rows)
	}
	first, _ := rows[0].(map[string]any)
	if first["Order"] != "A-101" || first["Amount"] != 3.0 {
		t.Fatalf("first row = %#v", first)
	}
}

// TestBuildMapFeedsTemplateValuesThroughEngine wires a constant into one
// entry pin while the other entry falls back to its constant, and feeds the
// assembled map straight into the template fill's Values input.
func TestBuildMapFeedsTemplateValuesThroughEngine(t *testing.T) {
	directory := t.TempDir()
	template := filepath.Join(directory, "offer.docx")
	filled := filepath.Join(directory, "offer-filled.docx")

	create := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("create", "action:word_create", map[string]any{
			"path": template, "title": "Offer", "content": "Dear {{customer}}, your total is {{amount}}.",
		}),
	}, Edges: []domain.FlowEdge{execEdge("start-create", "start", "out", "create", "in")}}
	if _, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), create, "start", Packet{}); err != nil {
		t.Fatalf("create Execute() error = %v", err)
	}

	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("name", "data:constant", map[string]any{"value": "Contoso", "type": "text"}),
		cfgNode("values", "data:build_map", map[string]any{"valueType": "any", "entries": []any{
			map[string]any{"id": "customer", "label": "Customer", "key": "customer"},
			map[string]any{"id": "amount", "label": "Amount", "key": "amount", "value": "42"},
		}}),
		cfgNode("fill", "action:word_template_fill", map[string]any{
			"templatePath": template, "outputPath": filled,
		}),
		cfgNode("read", "action:word_read_text", map[string]any{"path": filled}),
	}, Edges: []domain.FlowEdge{
		dataEdge("name-entry", "name", "value", "values", "entry_customer"),
		dataEdge("values-fill", "values", "map", "fill", "values"),
		execEdge("start-fill", "start", "out", "fill", "in"),
		execEdge("fill-read", "fill", "out", "read", "in"),
	}}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var text string
	for _, run := range result.NodeRuns {
		if run.NodeID == "read" {
			output, _ := run.Output.(map[string]any)
			text, _ = output["text"].(string)
		}
	}
	if !strings.Contains(text, "Dear Contoso,") || !strings.Contains(text, "your total is 42.") {
		t.Fatalf("filled text = %q", text)
	}
	if _, err := os.Stat(filled); err != nil {
		t.Fatalf("filled document missing: %v", err)
	}
}

// TestBuildArrayRequiredItemFailsNamesTheItem: an item with neither a wire
// nor a constant is a required pin, and the engine error names its label.
func TestBuildArrayRequiredItemFailsNamesTheItem(t *testing.T) {
	directory := t.TempDir()
	workbook := filepath.Join(directory, "orders.xlsx")

	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("rows", "data:build_array", map[string]any{"elementType": "object", "items": []any{
			map[string]any{"id": "row_1", "label": "First order", "value": `{"Order": "A-101"}`},
			map[string]any{"id": "row_2", "label": "Missing order"},
		}}),
		cfgNode("append", "action:excel_append_rows", map[string]any{
			"path": workbook, "table": "Table1", "createWorkbookIfMissing": true,
		}),
	}, Edges: []domain.FlowEdge{
		dataEdge("rows-append", "rows", "array", "append", "rows"),
		execEdge("start-append", "start", "out", "append", "in"),
	}}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	_, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err == nil || !strings.Contains(err.Error(), "Missing order") {
		t.Fatalf("Execute() error = %v, want the required-item error naming the label", err)
	}
}

// TestTypedArrayRejectsMismatchedWireAtValidation: with a concrete element
// type, a text-typed output feeding a number-typed item pin is a wiring
// error the validator catches before any run starts — the Go-style
// homogeneity guarantee for the whole collection.
func TestTypedArrayRejectsMismatchedWireAtValidation(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("title", "data:constant", map[string]any{"value": "Weekly digest", "type": "text"}),
		cfgNode("rows", "data:build_array", map[string]any{"elementType": "number", "items": []any{
			map[string]any{"id": "total", "label": "Total"},
		}}),
	}, Edges: []domain.FlowEdge{
		dataEdge("title-total", "title", "value", "rows", "item_total"),
	}}
	err := Validate(flow, catalog.New())
	if err == nil || !strings.Contains(err.Error(), "cannot connect string data to float") {
		t.Fatalf("Validate() error = %v, want the element-type mismatch error", err)
	}
}

// TestTypedMapRejectsMismatchedConstantAtValidation: a value type of number
// makes a non-numeric constant a resolve-time error, so the mistake surfaces
// during validation instead of at runtime.
func TestTypedMapRejectsMismatchedConstantAtValidation(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("values", "data:build_map", map[string]any{"valueType": "number", "entries": []any{
			map[string]any{"id": "total", "label": "Total", "key": "total", "value": "abc"},
		}}),
	}}
	err := Validate(flow, catalog.New())
	if err == nil || !strings.Contains(err.Error(), "is not a number") {
		t.Fatalf("Validate() error = %v, want the value-type constant error", err)
	}
}

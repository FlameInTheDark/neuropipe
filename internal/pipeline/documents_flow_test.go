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

func TestWordCreateFillReadFlowsThroughEngine(t *testing.T) {
	directory := t.TempDir()
	template := filepath.Join(directory, "letter.docx")
	filled := filepath.Join(directory, "letter-filled.docx")

	flow := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "create", Type: "action:word_create", Data: map[string]any{"config": map[string]any{
				"path": template, "title": "Offer", "content": "Dear {{customer}},\n\nYour total is {{amount}}.",
			}}},
			{ID: "fill", Type: "action:word_template_fill", Data: map[string]any{"config": map[string]any{
				"templatePath": template, "outputPath": filled, "values": map[string]any{"customer": "Contoso", "amount": "42"},
			}}},
			{ID: "read", Type: "action:word_read_text", Data: map[string]any{"config": map[string]any{"path": filled}}},
		},
		Edges: []domain.FlowEdge{
			execEdge("start-create", "start", "out", "create", "in"),
			execEdge("create-fill", "create", "out", "fill", "in"),
			execEdge("fill-read", "fill", "out", "read", "in"),
		},
	}
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
	if !strings.Contains(text, "Dear Contoso,") || !strings.Contains(text, "Your total is 42.") {
		t.Fatalf("filled text = %q", text)
	}
	// The template keeps its placeholders; only the filled copy is expanded.
	inspect := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "read", Type: "action:word_read_text", Data: map[string]any{"config": map[string]any{"path": template}}},
		},
		Edges: []domain.FlowEdge{execEdge("start-read", "start", "out", "read", "in")},
	}
	templateResult, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), inspect, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() inspect error = %v", err)
	}
	for _, run := range templateResult.NodeRuns {
		if run.NodeID != "read" {
			continue
		}
		output, _ := run.Output.(map[string]any)
		templateText, _ := output["text"].(string)
		if !strings.Contains(templateText, "{{customer}}") {
			t.Fatalf("template was mutated: %q", templateText)
		}
	}
}

func TestExcelAppendReadFlowsThroughEngine(t *testing.T) {
	directory := t.TempDir()
	workbook := filepath.Join(directory, "orders.xlsx")

	flow := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "append", Type: "action:excel_append_rows", Data: map[string]any{"config": map[string]any{
				"path": workbook, "table": "Table1", "createWorkbookIfMissing": true,
				"rows": `[{"Order": "A-101", "Amount": 3}, {"Order": "A-102", "Amount": 5}]`,
			}}},
			{ID: "read", Type: "action:excel_read_rows", Data: map[string]any{"config": map[string]any{
				"path": workbook, "table": "Table1",
			}}},
		},
		Edges: []domain.FlowEdge{
			execEdge("start-append", "start", "out", "append", "in"),
			execEdge("append-read", "append", "out", "read", "in"),
		},
	}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if _, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(workbook); err != nil {
		t.Fatalf("workbook not written: %v", err)
	}

	// A second run reads the persisted rows back with typed values.
	second := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "read", Type: "action:excel_read_rows", Data: map[string]any{"config": map[string]any{
				"path": workbook, "table": "Table1",
			}}},
		},
		Edges: []domain.FlowEdge{execEdge("start-read", "start", "out", "read", "in")},
	}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), second, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() second error = %v", err)
	}
	for _, run := range result.NodeRuns {
		if run.NodeID != "read" {
			continue
		}
		output, _ := run.Output.(map[string]any)
		rows, ok := output["rows"].([]any)
		if !ok || len(rows) != 2 {
			t.Fatalf("rows = %#v", output["rows"])
		}
		first, _ := rows[0].(map[string]any)
		if first["Order"] != "A-101" || first["Amount"] != 3.0 {
			t.Fatalf("first row = %#v", first)
		}
	}
}

func TestDocumentValuePinsFlowThroughEngine(t *testing.T) {
	directory := t.TempDir()
	template := filepath.Join(directory, "offer.docx")

	// Build the template with the Create node.
	create := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("create", "action:word_create", map[string]any{
			"path": template, "title": "Offer", "content": "Dear {{customer}}, your total is {{amount}}.",
		}),
	}, Edges: []domain.FlowEdge{execEdge("start-create", "start", "out", "create", "in")}}
	if err := Validate(create, catalog.New()); err != nil {
		t.Fatalf("create Validate() error = %v", err)
	}
	if _, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), create, "start", Packet{}); err != nil {
		t.Fatalf("create Execute() error = %v", err)
	}

	// One constant node wires its text output straight into the customer pin;
	// the amount pin falls back to its row literal. No Build Object detour.
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("name", "data:constant", map[string]any{"value": "Contoso", "type": "text"}),
		cfgNode("fill", "action:word_template_fill", map[string]any{
			"templatePath": template,
			"outputPath":   filepath.Join(directory, "filled.docx"),
			"valuePins": []any{
				map[string]any{"id": "field_1", "name": "customer", "label": "Customer"},
				map[string]any{"id": "field_2", "name": "amount", "value": 42},
			},
		}),
		cfgNode("read", "action:word_read_text", map[string]any{"path": filepath.Join(directory, "filled.docx")}),
	}, Edges: []domain.FlowEdge{
		dataEdge("name-fill", "name", "value", "fill", "pin_field_1"),
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

	// An edge into a pin the node never configured is a validation error, so
	// typos cannot silently drop data.
	typo := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: flow.Nodes, Edges: []domain.FlowEdge{
		dataEdge("name-fill", "name", "value", "fill", "pin_typo"),
		execEdge("start-fill", "start", "out", "fill", "in"),
	}}
	if err := Validate(typo, catalog.New()); err == nil {
		t.Fatal("unknown pin handle must fail validation")
	}
}

func TestExcelCellPinsFlowThroughEngine(t *testing.T) {
	directory := t.TempDir()
	workbook := filepath.Join(directory, "orders.xlsx")

	// Create the workbook with one table row, then write a cell pin and read
	// two cells back as output pins — all through wired constants.
	seed := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("append", "action:excel_append_rows", map[string]any{
			"path": workbook, "table": "Table1", "createWorkbookIfMissing": true,
			"rows": `[{"Order": "A-101", "Amount": 3}]`,
		}),
	}, Edges: []domain.FlowEdge{execEdge("start-append", "start", "out", "append", "in")}}
	if err := Validate(seed, catalog.New()); err != nil {
		t.Fatalf("seed Validate() error = %v", err)
	}
	if _, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), seed, "start", Packet{}); err != nil {
		t.Fatalf("seed Execute() error = %v", err)
	}

	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("amount", "data:constant", map[string]any{"value": float64(9), "type": "number"}),
		cfgNode("write", "action:excel_write_cell", map[string]any{
			"path": workbook, "sheet": "Sheet1",
			"cellPins": []any{map[string]any{"id": "field_1", "name": "A2", "label": "Amount"}},
		}),
		cfgNode("read", "action:excel_read_cell", map[string]any{
			"path": workbook,
			"cellPins": []any{
				map[string]any{"id": "field_1", "name": "A2", "label": "Amount"},
				map[string]any{"id": "field_2", "name": "B2", "label": "Order"},
			},
		}),
		cfgNode("check", "data:constant", map[string]any{"value": "sentinel", "type": "text"}),
	}, Edges: []domain.FlowEdge{
		dataEdge("amount-write", "amount", "value", "write", "pin_field_1"),
		execEdge("start-write", "start", "out", "write", "in"),
		execEdge("write-read", "write", "out", "read", "in"),
	}}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, run := range result.NodeRuns {
		if run.NodeID != "read" {
			continue
		}
		output, _ := run.Output.(map[string]any)
		// The write pinned 9 into A2 (the Amount cell; the created table sorts
		// its columns alphabetically) and B2 keeps the seeded order id.
		if output["pin_field_1"] != 9.0 {
			t.Fatalf("amount pin = %#v", output["pin_field_1"])
		}
		if output["pin_field_2"] != "A-101" {
			t.Fatalf("order pin = %#v", output["pin_field_2"])
		}
	}
	_ = "check"
}

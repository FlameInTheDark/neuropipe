package pipeline

import (
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestResolveJsonParseObjectOutputContract(t *testing.T) {
	registry := catalog.New()
	resolved := resolveNode(t, registry, "data:json_parse", map[string]any{"type": "object"})
	output := resolved.Outputs[0]
	if output.ID != "value" || output.DataType != domain.DataObject {
		t.Fatalf("object-typed output = %#v", output)
	}
	if output.Type == nil || output.Type.Kind != domain.TypeMap {
		t.Fatalf("object output type = %#v, want the graph-wide map kind", output.Type)
	}
	if output.Type.Key == nil || output.Type.Key.Kind != domain.TypeString || output.Type.Value == nil || output.Type.Value.Kind != domain.TypeAny {
		t.Fatalf("object output map contract = %#v, want map<string, any>", output.Type)
	}
}

func TestResolveJsonParseLegacyConfigStaysUntyped(t *testing.T) {
	registry := catalog.New()
	resolved := resolveNode(t, registry, "data:json_parse", nil)
	output := resolved.Outputs[0]
	if output.DataType != domain.DataAny || output.Type == nil || output.Type.Kind != domain.TypeAny {
		t.Fatalf("legacy output = %#v, want the historical any contract", output)
	}
}

func TestResolveJsonParseRejectsUnsupportedRootType(t *testing.T) {
	registry := catalog.New()
	if err := resolveNodeErr(t, registry, "data:json_parse", map[string]any{"type": "bytes"}); err == nil || !strings.Contains(err.Error(), "not a supported JSON root type") {
		t.Fatalf("resolve error = %v, want the unsupported root type failure", err)
	}
}

func TestJsonParseTypedListFlowsIntoArrayNodes(t *testing.T) {
	nodes := []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("source", "data:constant", map[string]any{"type": "text", "value": `[3, 1, 2]`}),
		cfgNode("parse", "data:json_parse", map[string]any{"type": "list"}),
		cfgNode("sort", "data:array_sort", nil),
		cfgNode("store", "flow:set_variable", map[string]any{"name": "Probe"}),
	}
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("source-parse", "source", "value", "parse", "text"),
		dataEdge("parse-sort", "parse", "value", "sort", "array"),
		storeEdge("sort", "array"),
	}
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: nodes, Edges: edges}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v, want the typed list wire accepted", err)
	}
	outputs, err := runFlow(t, nodes, edges, "sort")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:array_sort", outputs, map[string]any{"array": []any{1.0, 2.0, 3.0}})
}

func TestJsonParseObjectOutputFeedsWordTemplateValues(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
			cfgNode("source", "data:constant", map[string]any{"type": "text", "value": `{"customer":"Contoso"}`}),
			cfgNode("parse", "data:json_parse", map[string]any{"type": "object"}),
			cfgNode("fill", "action:word_template_fill", map[string]any{
				"templatePath": "C:\\Work\\template.docx",
				"outputPath":   "C:\\Work\\filled.docx",
				"values":       map[string]any{},
				"valuePins":    []any{},
			}),
		},
		Edges: []domain.FlowEdge{
			dataEdge("source-parse", "source", "value", "parse", "text"),
			dataEdge("parse-fill", "parse", "value", "fill", "values"),
		},
	}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v, want the object-typed output accepted by the values pin", err)
	}
}

func TestJsonParseObjectOutputRejectedByListPin(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
			cfgNode("source", "data:constant", map[string]any{"type": "text", "value": `{"customer":"Contoso"}`}),
			cfgNode("parse", "data:json_parse", map[string]any{"type": "object"}),
			cfgNode("sort", "data:array_sort", nil),
		},
		Edges: []domain.FlowEdge{
			dataEdge("source-parse", "source", "value", "parse", "text"),
			dataEdge("parse-sort", "parse", "value", "sort", "array"),
		},
	}
	err := Validate(flow, catalog.New())
	if err == nil || !strings.Contains(err.Error(), "cannot connect map data to list") {
		t.Fatalf("Validate() error = %v, want the object output rejected by the list pin", err)
	}
}

func TestJsonParseRootMismatchFailsLoudly(t *testing.T) {
	nodes := []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("source", "data:constant", map[string]any{"type": "text", "value": `[1, 2]`}),
		cfgNode("parse", "data:json_parse", map[string]any{"type": "object"}),
		cfgNode("store", "flow:set_variable", map[string]any{"name": "Probe"}),
	}
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("source-parse", "source", "value", "parse", "text"),
		storeEdge("parse", "value"),
	}
	if _, err := runFlow(t, nodes, edges, "parse"); err == nil || !strings.Contains(err.Error(), "root is list, but Root type is object") {
		t.Fatalf("Execute() error = %v, want the root-kind mismatch failure", err)
	}
}

func TestJsonParseLegacyConfigStillFlows(t *testing.T) {
	nodes := []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("source", "data:constant", map[string]any{"type": "text", "value": `{"a": 1}`}),
		cfgNode("parse", "data:json_parse", nil),
		cfgNode("field", "data:get_field", map[string]any{"outputs": []any{
			map[string]any{"id": "a", "label": "A", "path": "a", "dataType": "number"},
		}}),
		cfgNode("store", "flow:set_variable", map[string]any{"name": "Probe"}),
	}
	edges := []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"),
		dataEdge("source-parse", "source", "value", "parse", "text"),
		dataEdge("parse-field", "parse", "value", "field", "source"),
		storeEdge("field", "a"),
	}
	outputs, err := runFlow(t, nodes, edges, "field")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertOutput(t, "data:get_field", outputs, map[string]any{"a": 1.0})
}

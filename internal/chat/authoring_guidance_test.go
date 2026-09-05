package chat

import (
	"encoding/json"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestNodeContractGuidanceAndExample(t *testing.T) {
	registry := catalog.New()
	contract, err := nodeContractFor(registry, "action:http")
	if err != nil {
		t.Fatalf("nodeContractFor(action:http) failed: %v", err)
	}

	if contract.Type != "action:http" {
		t.Errorf("expected type 'action:http', got %q", contract.Type)
	}
	if contract.BlueprintGuidance == "" {
		t.Error("expected non-empty BlueprintGuidance")
	}
	if contract.ExampleNode.ID == "" {
		t.Error("expected non-empty ExampleNode.ID")
	}
	if contract.ExampleNode.Type != "action:http" {
		t.Errorf("expected ExampleNode.Type == 'action:http', got %q", contract.ExampleNode.Type)
	}
	if contract.ExampleNode.Data == nil || contract.ExampleNode.Data["config"] == nil {
		t.Error("expected ExampleNode.Data.config to be populated")
	}
}

func TestDefinitionFromNormalizesMalformedPayloads(t *testing.T) {
	// Exact malformed payload from user's incident
	rawJSON := `{
		"definition": {
			"edges": {
				"id": "e1",
				"kind": "data",
				"source": "fmt1",
				"sourceHandle": "value",
				"target": "fmt2",
				"targetHandle": "value"
			},
			"nodes": {
				"data": [
					{
						"config": {
							"format": "hi {{.value}}"
						}
					},
					{
						"config": {
							"format": "echo: {{.value}}"
						}
					}
				],
				"id": [
					"fmt1",
					"fmt2"
				],
				"position": [
					{
						"x": "100",
						"y": "100"
					},
					{
						"x": "320",
						"y": "100"
					}
				],
				"type": [
					"data:format_text",
					"data:format_text"
				]
			},
			"schemaVersion": "3",
			"viewport": {
				"x": "0",
				"y": "0",
				"zoom": "1"
			}
		},
		"name": "__test_minimal3__"
	}`

	var body map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &body); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	def, err := definitionFrom(body["definition"])
	if err != nil {
		t.Fatalf("definitionFrom failed to normalize and decode malformed payload: %v", err)
	}

	if def.SchemaVersion != domain.GraphSchemaV3 {
		t.Errorf("expected schemaVersion %d, got %d", domain.GraphSchemaV3, def.SchemaVersion)
	}
	if len(def.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(def.Edges))
	}
	if def.Edges[0].ID != "e1" || def.Edges[0].Source != "fmt1" || def.Edges[0].Target != "fmt2" {
		t.Errorf("unexpected edge content: %+v", def.Edges[0])
	}
	if len(def.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(def.Nodes))
	}
	if def.Nodes[0].ID != "fmt1" || def.Nodes[0].Type != "data:format_text" {
		t.Errorf("unexpected node 0: %+v", def.Nodes[0])
	}
	if def.Nodes[0].Position.X != 100 || def.Nodes[0].Position.Y != 100 {
		t.Errorf("unexpected position for node 0: %+v", def.Nodes[0].Position)
	}
	if def.Nodes[1].ID != "fmt2" || def.Nodes[1].Type != "data:format_text" {
		t.Errorf("unexpected node 1: %+v", def.Nodes[1])
	}
	if def.Nodes[1].Position.X != 320 || def.Nodes[1].Position.Y != 100 {
		t.Errorf("unexpected position for node 1: %+v", def.Nodes[1].Position)
	}
}

func TestDefinitionFromSingleNodeObject(t *testing.T) {
	rawJSON := `{
		"schemaVersion": 3,
		"nodes": {
			"id": "single1",
			"type": "trigger:button",
			"position": {"x": 50, "y": 50},
			"data": {"config": {"label": "Go"}}
		},
		"edges": []
	}`

	var body map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &body); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	def, err := definitionFrom(body)
	if err != nil {
		t.Fatalf("definitionFrom failed for single node object: %v", err)
	}

	if len(def.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(def.Nodes))
	}
	if def.Nodes[0].ID != "single1" {
		t.Errorf("expected node id 'single1', got %q", def.Nodes[0].ID)
	}
}

func TestExecuteToolReturnsErrorForNonExistentTool(t *testing.T) {
	service := &Service{
		authoring: &stubAuthoring{},
	}
	result := service.executeTool(nil, domain.ChatConversation{}, "run-1", domain.ChatToolCall{
		ID:   "call-unknown",
		Name: "made_up_tool",
	})
	if result == "" {
		t.Fatal("executeTool returned empty result")
	}
	if !strings.Contains(result, "Tool failed:") || !strings.Contains(result, "does not exist") {
		t.Fatalf("expected result to indicate tool does not exist, got: %s", result)
	}
}

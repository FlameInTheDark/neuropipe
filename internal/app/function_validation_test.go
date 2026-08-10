package app

import (
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestValidateFunctionRequiresGroundedPublishedToolContract(t *testing.T) {
	function := domain.CustomFunction{
		ID:          "weather",
		Name:        "Get weather",
		Description: "Look up current weather for a city.",
		Kind:        domain.FunctionTool,
		Mode:        domain.NodeImpure,
		Inputs: []domain.FunctionPin{
			{ID: "city", Name: "City", DataType: domain.DataText, Required: true},
		},
		Outputs: []domain.FunctionPin{
			{ID: "forecast", Name: "Forecast", Description: "The current weather summary.", DataType: domain.DataText},
		},
		DraftDefinition: domain.FlowDefinition{
			SchemaVersion: domain.GraphSchemaV3,
			Nodes: []domain.FlowNode{
				{ID: "entry", Type: "function:entry", Data: map[string]any{"config": map[string]any{}}},
				{ID: "return", Type: "function:return", Data: map[string]any{"config": map[string]any{}}},
			},
			Edges: []domain.FlowEdge{{ID: "entry-return", Source: "entry", SourceHandle: "out", Target: "return", TargetHandle: "in", Kind: domain.PinExec}},
		},
	}
	if err := validateFunction(function, catalog.New()); err == nil || !strings.Contains(err.Error(), "model guidance") {
		t.Fatalf("validateFunction() error = %v, want tool input guidance error", err)
	}
	function.Inputs[0].Description = "The city and country, for example Yekaterinburg, RU."
	if err := validateFunction(function, catalog.New()); err != nil {
		t.Fatalf("validateFunction() error = %v", err)
	}
}

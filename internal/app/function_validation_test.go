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

func TestValidateFunctionTriggers(t *testing.T) {
	base := func(nodes ...domain.FlowNode) domain.CustomFunction {
		return domain.CustomFunction{
			ID:          "guard",
			Name:        "Guard",
			Description: "trigger guard fixture",
			Mode:        domain.NodeImpure,
			DraftDefinition: domain.FlowDefinition{
				SchemaVersion: domain.GraphSchemaV3,
				Nodes:         nodes,
			},
		}
	}

	triggers := []struct {
		nodeType string
		fragment string
	}{
		{"trigger:button", "trigger node"},
		{"twitch:event", "trigger node"},
		{"discord:event", "trigger node"},
		{"telegram:event", "trigger node"},
		{"kv:subscribe", "trigger node"},
	}
	for _, trigger := range triggers {
		fn := base(
			domain.FlowNode{ID: "entry", Type: "function:entry", Data: map[string]any{"config": map[string]any{}}},
			domain.FlowNode{ID: "tr", Type: trigger.nodeType, Data: map[string]any{"config": map[string]any{}}},
		)
		err := validateFunctionTriggers(fn, catalog.New())
		if err == nil || !strings.Contains(err.Error(), trigger.fragment) {
			t.Fatalf("validateFunctionTriggers(%s) error = %v, want %q error", trigger.nodeType, err, trigger.fragment)
		}
	}

	// boundary nodes are event-mode but legitimate — must pass untouched
	fn := base(
		domain.FlowNode{ID: "entry", Type: "function:entry", Data: map[string]any{"config": map[string]any{}}},
		domain.FlowNode{ID: "return", Type: "function:return", Data: map[string]any{"config": map[string]any{}}},
		domain.FlowNode{ID: "act", Type: "action:http-request", Data: map[string]any{"config": map[string]any{}}},
	)
	if err := validateFunctionTriggers(fn, catalog.New()); err != nil {
		t.Fatalf("validateFunctionTriggers() error = %v, want nil for boundary + action nodes", err)
	}

	// unknown node types stay draft-saveable (publish reports them instead)
	fn.DraftDefinition.Nodes = []domain.FlowNode{{ID: "x", Type: "plugin:future", Data: map[string]any{"config": map[string]any{}}}}
	if err := validateFunctionTriggers(fn, catalog.New()); err != nil {
		t.Fatalf("validateFunctionTriggers() error = %v, want nil for unknown node type", err)
	}
}

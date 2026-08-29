package drawimage

import (
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/gogpu/gg"
	"image/color"
	"testing"
)

// domainFlowNode wraps a config map into the FlowNode shape resolvers read.
func domainFlowNode(config map[string]any) domain.FlowNode {
	return domain.FlowNode{ID: "node_1", Type: "action:draw_image", Data: map[string]any{"config": config}}
}

// testInvocation builds an executor invocation with definition defaults.
func testInvocation(config map[string]any, inputs map[string]any) nodes.Invocation {
	if config == nil {
		config = map[string]any{}
	}
	def := definition()
	return nodes.Invocation{
		Node:       domainFlowNode(config),
		Definition: def,
		Config:     config,
		Inputs:     inputs,
	}
}

// ggContextForTest renders a solid-color gg context for image fixtures.
func ggContextForTest(t *testing.T, width, height int, fill color.RGBA) *gg.Context {
	t.Helper()
	dc := gg.NewContext(width, height)
	if err := dc.Close(); err != nil {
		t.Fatalf("close context: %v", err)
	}
	dc.ClearWithColor(gg.RGBA{R: float64(fill.R) / 255, G: float64(fill.G) / 255, B: float64(fill.B) / 255, A: float64(fill.A) / 255})
	return dc
}

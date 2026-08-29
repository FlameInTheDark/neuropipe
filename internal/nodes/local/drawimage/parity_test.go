package drawimage

import (
	"context"
	"os"
	"testing"
)

// TestParityArtifacts renders showcase documents to $PARITY_DIR when the
// environment variable is set. This powers the manual visual verification
// harness; it is skipped in normal test runs.
func TestParityArtifacts(t *testing.T) {
	dir := os.Getenv("PARITY_DIR")
	if dir == "" {
		t.Skip("PARITY_DIR not set")
	}

	doc := ParseDocument(map[string]any{
		"version": 1, "width": 800, "height": 450, "background": "#0b0c0d",
		"pins":   []any{map[string]any{"name": "city", "type": "text"}},
		"layers": []any{map[string]any{"id": "l1", "name": "Base", "visible": true, "opacity": 1}},
		"elements": []any{
			map[string]any{
				"id": "bg", "type": "rect", "layerId": "l1", "name": "Card",
				"x": 24, "y": 24, "w": 752, "h": 402, "rotation": 0, "opacity": 1, "visible": true,
				"radius": 24,
				"fill": map[string]any{"type": "linear", "x0": 24, "y0": 24, "x1": 776, "y1": 426,
					"stops": []any{map[string]any{"offset": 0, "color": "#4ea7fc"}, map[string]any{"offset": 1, "color": "#00b8cc"}}},
				"stroke": map[string]any{"color": "#ffffff", "width": 2, "dash": []any{float64(10), float64(6)}, "cap": "round", "join": "round"},
			},
			map[string]any{
				"id": "t1", "type": "text", "layerId": "l1", "name": "Title",
				"x": 64, "y": 56, "w": 500, "h": 70, "rotation": 0, "opacity": 1, "visible": true,
				"content": "Weather for {{city}}", "fontFamily": "inter", "fontSize": 48, "weight": 800,
				"color": "#08090a", "align": "left", "valign": "middle", "lineHeight": 1.2, "wrapWidth": -1,
			},
			map[string]any{
				"id": "t2", "type": "text", "layerId": "l1", "name": "Footer",
				"x": 64, "y": 360, "w": 500, "h": 30, "rotation": 0, "opacity": 0.85, "visible": true,
				"content":    "Draw Image node · {{days.0.temp}} degrees on day 1",
				"fontFamily": "jetbrains-mono", "fontSize": 16, "weight": 400,
				"color": "#08090a", "align": "left", "valign": "middle", "lineHeight": 1.2, "wrapWidth": -1,
			},
			map[string]any{
				"id": "sun", "type": "star", "layerId": "l1", "name": "Sun",
				"x": 600, "y": 80, "w": 120, "h": 120, "rotation": 15, "opacity": 1, "visible": true,
				"points": 8, "innerRatio": 0.55,
				"fill": map[string]any{"type": "solid", "color": "#f0bf00"},
			},
			map[string]any{
				"id": "dot", "type": "ellipse", "layerId": "l1", "name": "Rain dot",
				"x": 620, "y": 240, "w": 80, "h": 80, "rotation": 0, "opacity": 1, "visible": true,
				"visibility": map[string]any{"mode": "condition", "pin": "rain", "op": "isTrue"},
				"fill": map[string]any{"type": "radial", "cx": 660, "cy": 280, "r": 40,
					"stops": []any{map[string]any{"offset": 0, "color": "#e8f4ff"}, map[string]any{"offset": 1, "color": "#4ea7fc"}}},
			},
		},
	})
	values := map[string]any{
		"city": "Berlin",
		"rain": true,
		"days": []any{map[string]any{"temp": 21.5}, map[string]any{"temp": 23.0}},
	}
	encoded, _, err := Render(context.Background(), doc, values, RenderOptions{Format: "png"})
	if err != nil {
		t.Fatalf("weather render: %v", err)
	}
	if err := os.WriteFile(dir+"/weather.png", encoded, 0o644); err != nil {
		t.Fatalf("write weather.png: %v", err)
	}

	doc2 := ParseDocument(map[string]any{
		"version": 1, "width": 800, "height": 120, "background": "#141516",
		"pins":   []any{map[string]any{"name": "days", "type": "array"}},
		"layers": []any{map[string]any{"id": "l1", "visible": true, "opacity": 1}},
		"elements": []any{
			map[string]any{
				"id": "cell", "type": "rect", "layerId": "l1",
				"x": 20, "y": 20, "w": 130, "h": 80, "radius": 12,
				"fill":   map[string]any{"type": "solid", "color": "#1e2022"},
				"repeat": map[string]any{"pin": "days", "offsetX": 152, "offsetY": 0},
			},
			map[string]any{
				"id": "temp", "type": "text", "layerId": "l1",
				"x": 20, "y": 30, "w": 130, "h": 40,
				"content": "{{item.temp}}°C", "fontFamily": "inter", "fontSize": 28, "weight": 700,
				"color": "#f7f8f8", "align": "center", "valign": "middle", "lineHeight": 1.2, "wrapWidth": -1,
				"repeat": map[string]any{"pin": "days", "offsetX": 152, "offsetY": 0},
			},
			map[string]any{
				"id": "day", "type": "text", "layerId": "l1",
				"x": 20, "y": 70, "w": 130, "h": 24,
				"content": "Day {{index}}", "fontFamily": "inter", "fontSize": 14, "weight": 400,
				"color": "#8a8f98", "align": "center", "valign": "middle", "lineHeight": 1.2, "wrapWidth": -1,
				"repeat":     map[string]any{"pin": "days", "offsetX": 152, "offsetY": 0},
				"visibility": map[string]any{"mode": "condition", "pin": "item.temp", "op": "gt", "value": "0"},
			},
		},
	})
	days := []any{}
	for i := 1; i <= 5; i++ {
		days = append(days, map[string]any{"temp": float64(18 + i)})
	}
	encoded2, _, err := Render(context.Background(), doc2, map[string]any{"days": days}, RenderOptions{Format: "png"})
	if err != nil {
		t.Fatalf("repeat render: %v", err)
	}
	if err := os.WriteFile(dir+"/repeat.png", encoded2, 0o644); err != nil {
		t.Fatalf("write repeat.png: %v", err)
	}
}

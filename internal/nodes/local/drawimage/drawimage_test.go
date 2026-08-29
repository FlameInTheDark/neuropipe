package drawimage

import (
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

/* ------------------------------------------------------------------ */
/* model tests                                                         */
/* ------------------------------------------------------------------ */

func TestParseDocumentDefaults(t *testing.T) {
	doc := ParseDocument(nil)
	if doc.Width != 800 || doc.Height != 450 {
		t.Fatalf("default canvas = %dx%d, want 800x450", doc.Width, doc.Height)
	}
	if doc.Background != "#0b0c0d" {
		t.Fatalf("default background = %q", doc.Background)
	}
	if len(doc.Layers) != 1 || doc.Layers[0].ID == "" {
		t.Fatalf("default layers = %+v", doc.Layers)
	}
	if len(doc.Elements) != 0 {
		t.Fatalf("default elements = %+v", doc.Elements)
	}
}

func TestParseDocumentClamps(t *testing.T) {
	doc := ParseDocument(map[string]any{
		"width":  float64(999999),
		"height": float64(-5),
		"pins": []any{
			map[string]any{"name": "bad name!", "type": "text"},
			map[string]any{"name": "ok", "type": "bogus"},
			map[string]any{"name": "num", "type": "number"},
			map[string]any{"name": "in", "type": "text"},
		},
		"layers": []any{
			map[string]any{"id": "l1", "opacity": float64(4)},
		},
		"elements": []any{
			map[string]any{"id": "e1", "type": "rect", "layerId": "missing", "radius": float64(-3), "opacity": float64(2)},
		},
	})
	if doc.Width != MaxCanvasDimension {
		t.Fatalf("width = %d, want %d", doc.Width, MaxCanvasDimension)
	}
	if doc.Height != 1 {
		t.Fatalf("height = %d, want 1", doc.Height)
	}
	if len(doc.Pins) != 2 {
		t.Fatalf("pins = %+v", doc.Pins)
	}
	if doc.Pins[0].Name != "ok" || doc.Pins[0].Type != PinText {
		t.Fatalf("first pin = %+v, want ok/text", doc.Pins[0])
	}
	if doc.Pins[1].Name != "num" || doc.Pins[1].Type != PinNumber {
		t.Fatalf("second pin = %+v, want num/number", doc.Pins[1])
	}
	if doc.Layers[0].Opacity != 1 {
		t.Fatalf("layer opacity = %v, want 1", doc.Layers[0].Opacity)
	}
	if len(doc.Elements) != 1 {
		t.Fatalf("elements = %+v", doc.Elements)
	}
	element := doc.Elements[0]
	if element.LayerID != "l1" {
		t.Fatalf("element layer fallback = %q, want l1", element.LayerID)
	}
	if element.Radius != 0 || element.Opacity != 1 {
		t.Fatalf("element clamps: radius=%v opacity=%v", element.Radius, element.Opacity)
	}
}

func TestParseDocumentRepeatAndVisibility(t *testing.T) {
	doc := ParseDocument(map[string]any{
		"elements": []any{
			map[string]any{
				"id": "e1", "type": "text", "layerId": "l1",
				"visibility": map[string]any{"mode": "condition", "pin": "show", "op": ""},
				"repeat":     map[string]any{"pin": "days", "offsetX": float64(10), "limit": float64(500)},
			},
		},
	})
	element := doc.Elements[0]
	if element.Visibility.Mode != "always" {
		t.Fatalf("invalid condition should fall back to always, got %+v", element.Visibility)
	}
	if element.Repeat == nil || element.Repeat.Limit != MaxRepeatCopies {
		t.Fatalf("repeat = %+v", element.Repeat)
	}
}

func TestDefaultDocumentValueParses(t *testing.T) {
	doc := ParseDocument(DefaultDocumentValue())
	if doc.Width != 800 || doc.Height != 450 {
		t.Fatalf("canvas = %dx%d", doc.Width, doc.Height)
	}
	if len(doc.Pins) != 1 || doc.Pins[0].Name != "title" {
		t.Fatalf("pins = %+v", doc.Pins)
	}
	if len(doc.Elements) != 3 {
		t.Fatalf("elements = %d, want 3", len(doc.Elements))
	}
	if doc.Elements[0].Fill.Type != "linear" || len(doc.Elements[0].Fill.Stops) != 2 {
		t.Fatalf("hero fill = %+v", doc.Elements[0].Fill)
	}
}

/* ------------------------------------------------------------------ */
/* text layout tests                                                   */
/* ------------------------------------------------------------------ */

func TestInterpolate(t *testing.T) {
	ctx := TemplateContext{
		"city":  "Berlin",
		"temp":  21.5,
		"count": 3.0,
		"on":    true,
		"list":  []any{"a", 2.0},
		"obj":   map[string]any{"b": 1.5, "a": "x"},
		"nested": map[string]any{
			"deep": []any{"first", "second"},
		},
	}
	cases := map[string]string{
		"{{city}}":          "Berlin",
		"{{ temp }}":        "21.5",
		"{{count}}":         "3",
		"{{on}}":            "true",
		"{{list}}":          "a, 2",
		"{{obj}}":           `{"a":"x","b":1.5}`,
		"{{nested.deep.1}}": "second",
		"{{missing}}":       "",
		"plain":             "plain",
		"{{city}} {{temp}}": "Berlin 21.5",
	}
	for input, want := range cases {
		if got := Interpolate(input, ctx); got != want {
			t.Errorf("Interpolate(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	cases := map[float64]string{
		21.5:  "21.5",
		3:     "3",
		-0.25: "-0.25",
		100:   "100",
	}
	for input, want := range cases {
		if got := FormatNumber(input); got != want {
			t.Errorf("FormatNumber(%v) = %q, want %q", input, got, want)
		}
	}
}

func TestWrapLines(t *testing.T) {
	// measure: every char is 10px, space is 10px
	measure := func(text string) float64 { return float64(len(text)) * 10 }
	lines := WrapLines("hello world foo\nbar", 110, measure)
	want := []string{"hello world", "foo", "bar"}
	if len(lines) != len(want) {
		t.Fatalf("lines = %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("lines = %v, want %v", lines, want)
		}
	}
	// an unbreakable long word occupies its own line
	lines = WrapLines("tiny supercalifragilistic word", 60, measure)
	if len(lines) != 3 || lines[0] != "tiny" || lines[1] != "supercalifragilistic" || lines[2] != "word" {
		t.Fatalf("long word wrap = %v", lines)
	}
	// limit 0 disables wrapping
	lines = WrapLines("hello world", 0, measure)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Fatalf("no wrap = %v", lines)
	}
}

/* ------------------------------------------------------------------ */
/* condition tests                                                     */
/* ------------------------------------------------------------------ */

func TestEvaluateConditions(t *testing.T) {
	values := map[string]any{
		"show":   true,
		"temp":   21.5,
		"city":   "Berlin",
		"tags":   []any{"rain", "cold"},
		"meta":   map[string]any{"a": 1.0},
		"emptyS": "",
		"item":   "rain",
		"index":  1.0,
	}
	cases := []struct {
		visibility Visibility
		pinType    PinType
		want       bool
	}{
		{Visibility{Mode: "always"}, PinText, true},
		{Visibility{Mode: "condition", Pin: "show", Op: "isTrue"}, PinBoolean, true},
		{Visibility{Mode: "condition", Pin: "show", Op: "isFalse"}, PinBoolean, false},
		{Visibility{Mode: "condition", Pin: "temp", Op: "gt", Value: "20"}, PinNumber, true},
		{Visibility{Mode: "condition", Pin: "temp", Op: "le", Value: "20"}, PinNumber, false},
		{Visibility{Mode: "condition", Pin: "temp", Op: "eq", Value: "21.5"}, PinNumber, true}, // eq string-compares stringified values
		{Visibility{Mode: "condition", Pin: "city", Op: "eq", Value: "Berlin"}, PinText, true},
		{Visibility{Mode: "condition", Pin: "city", Op: "contains", Value: "erl"}, PinText, true},
		{Visibility{Mode: "condition", Pin: "city", Op: "startsWith", Value: "Ber"}, PinText, true},
		{Visibility{Mode: "condition", Pin: "city", Op: "endsWith", Value: "lin"}, PinText, true},
		{Visibility{Mode: "condition", Pin: "emptyS", Op: "isEmpty"}, PinText, true},
		{Visibility{Mode: "condition", Pin: "city", Op: "notEmpty"}, PinText, true},
		{Visibility{Mode: "condition", Pin: "tags", Op: "arrayContains", Value: "rain"}, PinArray, true},
		{Visibility{Mode: "condition", Pin: "tags", Op: "arrayNotContains", Value: "sun"}, PinArray, true},
		{Visibility{Mode: "condition", Pin: "tags", Op: "lenGt", Value: "1"}, PinArray, true},
		{Visibility{Mode: "condition", Pin: "tags", Op: "lenEq", Value: "2"}, PinArray, true},
		{Visibility{Mode: "condition", Pin: "meta", Op: "hasKey", Value: "a"}, PinObject, true},
		{Visibility{Mode: "condition", Pin: "meta", Op: "isEmpty"}, PinObject, false},
		{Visibility{Mode: "condition", Pin: "item", Op: "eq", Value: "rain"}, "", true},
		{Visibility{Mode: "condition", Pin: "item", Op: "eq", Value: "snow"}, "", false},
		{Visibility{Mode: "condition", Pin: "index", Op: "lt", Value: "2"}, "", true},
	}
	for _, testCase := range cases {
		if got := EvaluateCondition(testCase.visibility, values, testCase.pinType); got != testCase.want {
			t.Errorf("EvaluateCondition(%+v) = %v, want %v", testCase.visibility, got, testCase.want)
		}
	}
}

/* ------------------------------------------------------------------ */
/* render tests                                                        */
/* ------------------------------------------------------------------ */

func mustRender(t *testing.T, doc Document, values map[string]any) image.Image {
	t.Helper()
	encoded, warnings, err := Render(context.Background(), doc, values, RenderOptions{Format: "png"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings: %+v", warnings)
	}
	decoded, err := png.Decode(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	return decoded
}

func pixel(t *testing.T, img image.Image, x, y int) color.RGBA {
	t.Helper()
	r, g, b, a := img.At(x, y).RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func expectPixel(t *testing.T, img image.Image, x, y int, want color.RGBA, label string) {
	t.Helper()
	got := pixel(t, img, x, y)
	// compare with small tolerance for anti-aliased rasterizers
	dr := int(got.R) - int(want.R)
	dg := int(got.G) - int(want.G)
	db := int(got.B) - int(want.B)
	da := int(got.A) - int(want.A)
	if dr < 0 {
		dr = -dr
	}
	if dg < 0 {
		dg = -dg
	}
	if db < 0 {
		db = -db
	}
	if da < 0 {
		da = -da
	}
	if dr > 6 || dg > 6 || db > 6 || da > 6 {
		t.Fatalf("%s pixel(%d,%d) = %+v, want %+v", label, x, y, got, want)
	}
}

func TestRenderBackground(t *testing.T) {
	doc := ParseDocument(map[string]any{"width": 64, "height": 32, "background": "#112233"})
	img := mustRender(t, doc, nil)
	if bounds := img.Bounds(); bounds.Dx() != 64 || bounds.Dy() != 32 {
		t.Fatalf("bounds = %v", bounds)
	}
	expectPixel(t, img, 10, 10, color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff}, "background")
}

func TestRenderTransparent(t *testing.T) {
	doc := ParseDocument(map[string]any{"width": 16, "height": 16, "background": "transparent"})
	img := mustRender(t, doc, nil)
	expectPixel(t, img, 5, 5, color.RGBA{A: 0}, "transparent")
}

func TestRenderShapesAndText(t *testing.T) {
	doc := ParseDocument(map[string]any{
		"width": 300, "height": 200, "background": "#000000",
		"pins":   []any{map[string]any{"name": "label", "type": "text"}},
		"layers": []any{map[string]any{"id": "l1", "visible": true, "opacity": 1}},
		"elements": []any{
			map[string]any{
				"id": "r1", "type": "rect", "layerId": "l1",
				"x": 10, "y": 10, "w": 100, "h": 60,
				"fill": map[string]any{"type": "solid", "color": "#ff0000"},
			},
			map[string]any{
				"id": "e1", "type": "ellipse", "layerId": "l1",
				"x": 150, "y": 10, "w": 100, "h": 60,
				"fill": map[string]any{"type": "solid", "color": "#00ff00"},
			},
			map[string]any{
				"id": "t1", "type": "text", "layerId": "l1",
				"x": 10, "y": 100, "w": 200, "h": 40,
				"content":  "Hello {{label}}",
				"fontSize": 24, "color": "#ffffff", "align": "left", "valign": "middle",
			},
		},
	})
	img := mustRender(t, doc, map[string]any{"label": "Neuropipe"})
	expectPixel(t, img, 60, 40, color.RGBA{R: 255, A: 255}, "rect fill")
	expectPixel(t, img, 200, 40, color.RGBA{G: 255, A: 255}, "ellipse fill")
	expectPixel(t, img, 290, 190, color.RGBA{A: 255}, "background")
	// text drawn: some pixel inside the text row must be near-white
	foundInk := false
	for x := 10; x < 210; x++ {
		for y := 100; y < 140; y++ {
			if p := pixel(t, img, x, y); p.R > 200 && p.G > 200 && p.B > 200 {
				foundInk = true
			}
		}
	}
	if !foundInk {
		t.Fatal("no text pixels found in the text element area")
	}
}

func TestRenderVisibilityAndRepeat(t *testing.T) {
	doc := ParseDocument(map[string]any{
		"width": 400, "height": 100, "background": "#000000",
		"pins": []any{
			map[string]any{"name": "show", "type": "boolean"},
			map[string]any{"name": "days", "type": "array"},
		},
		"layers": []any{map[string]any{"id": "l1", "visible": true, "opacity": 1}},
		"elements": []any{
			map[string]any{
				"id": "hidden", "type": "rect", "layerId": "l1",
				"x": 0, "y": 0, "w": 400, "h": 100,
				"fill":       map[string]any{"type": "solid", "color": "#0000ff"},
				"visibility": map[string]any{"mode": "condition", "pin": "show", "op": "isTrue"},
			},
			map[string]any{
				"id": "cells", "type": "rect", "layerId": "l1",
				"x": 0, "y": 40, "w": 80, "h": 20,
				"fill":       map[string]any{"type": "solid", "color": "#ffffff"},
				"repeat":     map[string]any{"pin": "days", "offsetX": 100},
				"visibility": map[string]any{"mode": "condition", "pin": "item", "op": "eq", "value": "on"},
			},
		},
	})
	// hidden (show=false) + two cells, only the second matches item == "on"
	img := mustRender(t, doc, map[string]any{"show": false, "days": []any{"off", "on"}})
	expectPixel(t, img, 200, 10, color.RGBA{A: 255}, "hidden rect stays hidden")
	expectPixel(t, img, 40, 50, color.RGBA{A: 255}, "off cell hidden")
	expectPixel(t, img, 140, 50, color.RGBA{R: 255, G: 255, B: 255, A: 255}, "on cell drawn")
}

func TestRenderLayerOpacity(t *testing.T) {
	doc := ParseDocument(map[string]any{
		"width": 64, "height": 64, "background": "#000000",
		"layers": []any{
			map[string]any{"id": "l1", "visible": true, "opacity": 0.5},
		},
		"elements": []any{
			map[string]any{
				"id": "r", "type": "rect", "layerId": "l1",
				"x": 0, "y": 0, "w": 64, "h": 64,
				"fill": map[string]any{"type": "solid", "color": "#ffffff"},
			},
		},
	})
	img := mustRender(t, doc, nil)
	expectPixel(t, img, 32, 32, color.RGBA{R: 128, G: 128, B: 128, A: 255}, "half opacity over black")
}

func TestRenderRotation(t *testing.T) {
	doc := ParseDocument(map[string]any{
		"width": 100, "height": 100, "background": "#000000",
		"layers": []any{map[string]any{"id": "l1", "visible": true, "opacity": 1}},
		"elements": []any{
			map[string]any{
				"id": "r", "type": "rect", "layerId": "l1",
				"x": 10, "y": 45, "w": 80, "h": 10, "rotation": 90,
				"fill": map[string]any{"type": "solid", "color": "#ffffff"},
			},
		},
	})
	img := mustRender(t, doc, nil)
	// after 90° rotation around (50,50), the bar is vertical through the center
	expectPixel(t, img, 50, 20, color.RGBA{R: 255, G: 255, B: 255, A: 255}, "rotated top")
	expectPixel(t, img, 50, 80, color.RGBA{R: 255, G: 255, B: 255, A: 255}, "rotated bottom")
	expectPixel(t, img, 20, 50, color.RGBA{A: 255}, "rotated side is empty")
}

func TestRenderGradient(t *testing.T) {
	doc := ParseDocument(map[string]any{
		"width": 100, "height": 10, "background": "#000000",
		"layers": []any{map[string]any{"id": "l1", "visible": true, "opacity": 1}},
		"elements": []any{
			map[string]any{
				"id": "g", "type": "rect", "layerId": "l1",
				"x": 0, "y": 0, "w": 100, "h": 10,
				"fill": map[string]any{
					"type": "linear", "x0": 0, "y0": 0, "x1": 100, "y1": 0,
					"stops": []any{
						map[string]any{"offset": 0, "color": "#000000"},
						map[string]any{"offset": 1, "color": "#ffffff"},
					},
				},
			},
		},
	})
	img := mustRender(t, doc, nil)
	dark := pixel(t, img, 2, 5)
	light := pixel(t, img, 97, 5)
	if light.R <= dark.R+60 {
		t.Fatalf("gradient not ascending: left=%+v right=%+v", dark, light)
	}
}

func TestRenderImageElementWithFile(t *testing.T) {
	// build a source PNG on disk
	source := ggContextForTest(t, 20, 20, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "green.png")
	if err := source.SavePNG(sourcePath); err != nil {
		t.Fatalf("save source: %v", err)
	}

	doc := ParseDocument(map[string]any{
		"width": 100, "height": 100, "background": "#000000",
		"layers": []any{map[string]any{"id": "l1", "visible": true, "opacity": 1}},
		"elements": []any{
			map[string]any{
				"id": "img", "type": "image", "layerId": "l1",
				"x": 10, "y": 10, "w": 50, "h": 50,
				"source": map[string]any{"kind": "path", "value": sourcePath},
				"fit":    "fill",
			},
		},
	})
	img := mustRender(t, doc, nil)
	expectPixel(t, img, 35, 35, color.RGBA{G: 255, A: 255}, "image fill")
}

func TestRenderMissingImageSkips(t *testing.T) {
	doc := ParseDocument(map[string]any{
		"width": 32, "height": 32, "background": "#000000",
		"layers": []any{map[string]any{"id": "l1", "visible": true, "opacity": 1}},
		"elements": []any{
			map[string]any{
				"id": "img", "type": "image", "layerId": "l1",
				"x": 0, "y": 0, "w": 32, "h": 32,
				"source":    map[string]any{"kind": "path", "value": filepath.Join(t.TempDir(), "missing.png")},
				"onMissing": "skip",
			},
		},
	})
	encoded, warnings, err := Render(context.Background(), doc, nil, RenderOptions{Format: "png"})
	if err != nil {
		t.Fatalf("render should not fail: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "missing.png") {
		t.Fatalf("warnings = %+v", warnings)
	}
	if len(encoded) == 0 {
		t.Fatal("render produced no bytes")
	}

	// onMissing=error must fail the render
	doc.Elements[0].OnMissing = "error"
	if _, _, err := Render(context.Background(), doc, nil, RenderOptions{Format: "png"}); err == nil {
		t.Fatal("onMissing=error should fail the render")
	}
}

func TestRenderPreviewJSON(t *testing.T) {
	documentJSON := `{"width":16,"height":16,"background":"#101010"}`
	base64PNG, err := RenderPreviewJSON(context.Background(), documentJSON, `{}`)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(base64PNG)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	if _, err := png.Decode(strings.NewReader(string(raw))); err != nil {
		t.Fatalf("decode preview png: %v", err)
	}
	if _, err := RenderPreviewJSON(context.Background(), `not json`, ""); err == nil {
		t.Fatal("invalid JSON should error")
	}
}

func TestLoadImageSourceDataURL(t *testing.T) {
	source := ggContextForTest(t, 8, 8, color.RGBA{R: 255, A: 255})
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "red.png")
	if err := source.SavePNG(sourcePath); err != nil {
		t.Fatalf("save source: %v", err)
	}
	dataURL, err := LoadImageSourceDataURL(context.Background(), "path", sourcePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("data URL = %q", dataURL[:40])
	}
	if _, err := LoadImageSourceDataURL(context.Background(), "path", filepath.Join(dir, "nope.png")); err == nil {
		t.Fatal("missing file should error")
	}
}

func TestDetectImageMIME(t *testing.T) {
	if got := DetectImageMIME([]byte("\x89PNG\r\n\x1a\n----")); got != "image/png" {
		t.Fatalf("png mime = %q", got)
	}
	if got := DetectImageMIME([]byte{0xff, 0xd8, 0xff, 0x00}); got != "image/jpeg" {
		t.Fatalf("jpeg mime = %q", got)
	}
	if got := DetectImageMIME([]byte("nothing")); got != "application/octet-stream" {
		t.Fatalf("unknown mime = %q", got)
	}
}

/* ------------------------------------------------------------------ */
/* resolver + executor tests                                           */
/* ------------------------------------------------------------------ */

func TestResolveDerivesDynamicPins(t *testing.T) {
	node := domainFlowNode(map[string]any{
		"document": map[string]any{
			"pins": []any{
				map[string]any{"name": "city", "type": "text"},
				map[string]any{"name": "temp", "type": "number"},
				map[string]any{"name": "show", "type": "boolean"},
				map[string]any{"name": "days", "type": "array"},
				map[string]any{"name": "meta", "type": "object"},
			},
		},
	})
	def, err := resolve(node)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(def.Inputs) != 1+1+5 {
		t.Fatalf("inputs = %d, want 7", len(def.Inputs))
	}
	byID := map[string]domain.NodePort{}
	for _, port := range def.Inputs {
		byID[port.ID] = port
	}
	if byID["in"].Kind != domain.PinExec {
		t.Fatal("first input must be exec")
	}
	if byID["city"].DataType != domain.DataText {
		t.Fatalf("city pin = %+v", byID["city"])
	}
	if byID["temp"].DataType != domain.DataNumber {
		t.Fatalf("temp pin = %+v", byID["temp"])
	}
	if byID["show"].DataType != domain.DataBoolean {
		t.Fatalf("show pin = %+v", byID["show"])
	}
	if byID["days"].DataType != domain.DataList {
		t.Fatalf("days pin = %+v", byID["days"])
	}
	if byID["meta"].DataType != domain.DataObject {
		t.Fatalf("meta pin = %+v", byID["meta"])
	}
	if _, ok := byID["outputPath"]; !ok {
		t.Fatal("outputPath pin missing")
	}
}

func TestResolveFallsBackOnInvalidDocument(t *testing.T) {
	def, err := resolve(domainFlowNode(nil))
	if err != nil {
		t.Fatalf("resolve fallback error: %v", err)
	}
	if len(def.Inputs) != 2 {
		t.Fatalf("fallback inputs = %d, want 2", len(def.Inputs))
	}
}

func TestExecute(t *testing.T) {
	invocation := testInvocation(map[string]any{
		"document": map[string]any{
			"width": 64, "height": 48, "background": "#222222",
			"pins":   []any{map[string]any{"name": "title", "type": "text"}},
			"layers": []any{map[string]any{"id": "l1", "visible": true, "opacity": 1}},
			"elements": []any{
				map[string]any{
					"id": "t", "type": "text", "layerId": "l1",
					"x": 4, "y": 4, "w": 56, "h": 20, "content": "{{title}}",
					"fontSize": 12, "color": "#ffffff",
				},
			},
		},
		"format": "png",
	}, map[string]any{"title": "Hello"})
	result, err := execute(context.Background(), invocation, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %v", result.Ports)
	}
	imageBytes, ok := result.Outputs["image"].([]byte)
	if !ok || len(imageBytes) == 0 {
		t.Fatalf("image output = %T", result.Outputs["image"])
	}
	base64Text, ok := result.Outputs["base64"].(string)
	if !ok || base64Text == "" {
		t.Fatalf("base64 output = %T", result.Outputs["base64"])
	}
	if decoded, err := base64.StdEncoding.DecodeString(base64Text); err != nil || len(decoded) != len(imageBytes) {
		t.Fatalf("base64 mismatch: %v", err)
	}
	record, ok := result.Outputs["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %T", result.Outputs["result"])
	}
	if record["width"] != float64(64) || record["height"] != float64(48) {
		t.Fatalf("result record = %+v", record)
	}
	if record["path"] != "" {
		t.Fatalf("path should be empty, got %v", record["path"])
	}
}

func TestExecuteWritesFile(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "sub", "out.png")
	invocation := testInvocation(map[string]any{
		"document":   map[string]any{"width": 8, "height": 8, "background": "#00ff00"},
		"outputPath": outputPath,
	}, map[string]any{"outputPath": outputPath})
	result, err := execute(context.Background(), invocation, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	record := result.Outputs["result"].(map[string]any)
	if record["path"] != filepath.Clean(outputPath) {
		t.Fatalf("path = %v", record["path"])
	}
	if data, err := os.ReadFile(outputPath); err != nil || len(data) == 0 {
		t.Fatalf("written file: %v", err)
	}
}

func TestExecuteJPEG(t *testing.T) {
	invocation := testInvocation(map[string]any{
		"document": map[string]any{"width": 8, "height": 8, "background": "#336699"},
		"format":   "jpeg",
		"quality":  80,
	}, nil)
	result, err := execute(context.Background(), invocation, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	record := result.Outputs["result"].(map[string]any)
	if record["format"] != "jpeg" {
		t.Fatalf("format = %v", record["format"])
	}
	imageBytes := result.Outputs["image"].([]byte)
	if imageBytes[0] != 0xff || imageBytes[1] != 0xd8 {
		t.Fatalf("jpeg magic = % x", imageBytes[:2])
	}
}

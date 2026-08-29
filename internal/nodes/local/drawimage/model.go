package drawimage

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

/* ------------------------------------------------------------------ */
/* document model                                                      */
/* ------------------------------------------------------------------ */

// MaxCanvasDimension caps the document resolution.
const MaxCanvasDimension = 8192

// MaxRepeatCopies caps data-driven repetition.
const MaxRepeatCopies = 100

// PinType enumerates the declared input pin wire types.
type PinType string

const (
	PinText    PinType = "text"
	PinNumber  PinType = "number"
	PinBoolean PinType = "boolean"
	PinObject  PinType = "object"
	PinArray   PinType = "array"
)

// ElementType enumerates drawable element kinds.
type ElementType string

const (
	ElementRect    ElementType = "rect"
	ElementEllipse ElementType = "ellipse"
	ElementLine    ElementType = "line"
	ElementStar    ElementType = "star"
	ElementText    ElementType = "text"
	ElementImage   ElementType = "image"
)

// Document is the parsed, clamped draw image design.
type Document struct {
	Version    int
	Width      int
	Height     int
	Background string // hex color or "transparent"
	Pins       []DocPin
	Layers     []Layer
	Elements   []Element
}

// DocPin declares one dynamic input pin plus editor sample/default values.
type DocPin struct {
	Name    string
	Type    PinType
	Sample  string
	Default string
}

// Layer groups elements; layers render bottom-to-top in slice order.
type Layer struct {
	ID      string
	Name    string
	Visible bool
	Opacity float64
	Locked  bool
}

// Element is one drawable object with common transform and style fields.
type Element struct {
	ID         string
	Type       ElementType
	LayerID    string
	Name       string
	X, Y, W, H float64
	Rotation   float64 // degrees, clockwise on screen
	Opacity    float64 // 0..1
	Visible    bool
	Visibility Visibility
	Repeat     *Repeat

	// rect
	Radius float64
	Fill   Paint
	Stroke *Stroke

	// line
	Points []Point

	// star
	StarPoints int
	InnerRatio float64 // 0.05..0.95 of outer radius

	// text
	Content    string
	FontFamily string // "inter" | "jetbrains-mono"
	FontSize   float64
	Weight     int
	Italic     bool
	Color      string
	Align      string // left | center | right
	VAlign     string // top | middle | bottom
	LineHeight float64
	WrapWidth  float64 // 0 = no wrap

	// image
	Source    ImageSource
	Fit       string // fill | contain | cover
	OnMissing string // skip | error
}

// Point is a polyline vertex in canvas coordinates.
type Point struct{ X, Y float64 }

// Paint is a solid color or gradient fill.
type Paint struct {
	Type  string // solid | linear | radial
	Color string
	// linear gradient endpoints in canvas coordinates
	X0, Y0, X1, Y1 float64
	// radial gradient center/radius in canvas coordinates
	CX, CY, R float64
	Stops     []GradientStop
}

// GradientStop is one gradient color stop.
type GradientStop struct {
	Offset float64 // 0..1
	Color  string
}

// Stroke describes an outline.
type Stroke struct {
	Color string
	Width float64
	Dash  []float64
	Cap   string // butt | round | square
	Join  string // miter | round | bevel
}

// Visibility is the element render condition.
type Visibility struct {
	Mode  string // always | condition
	Pin   string
	Op    string
	Value string
}

// Repeat drives data-driven repetition over an array pin.
type Repeat struct {
	Pin     string
	OffsetX float64
	OffsetY float64
	Limit   int // 0 = auto
}

// ImageSource points at an image by URL, disk path, or pin value.
type ImageSource struct {
	Kind  string // url | path | pin
	Value string
}

/* ------------------------------------------------------------------ */
/* parsing helpers                                                     */
/* ------------------------------------------------------------------ */

func mapOf(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return nil
}

func str(value any, fallback string) string {
	if s, ok := value.(string); ok {
		return s
	}
	return fallback
}

func num(value any, fallback float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%g", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func integer(value any, fallback int) int {
	f := num(value, float64(fallback))
	if f != math.Trunc(f) {
		return fallback
	}
	return int(f)
}

func boolean(value any, fallback bool) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func oneOf(value, fallback string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

var pinNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// reservedPinNames collide with fixed node ports or repeat pseudo-pins.
var reservedPinNames = map[string]bool{
	"in": true, "out": true, "image": true, "base64": true, "result": true,
	"outputPath": true, "item": true, "index": true,
}

// ValidPinName reports whether name is a usable pin identifier.
func ValidPinName(name string) bool {
	return pinNamePattern.MatchString(name) && !reservedPinNames[name]
}

/* ------------------------------------------------------------------ */
/* parse                                                               */
/* ------------------------------------------------------------------ */

// ParseDocument coerces an untyped config value into a clamped document.
// Unknown or malformed fragments fall back to defaults; parsing never fails.
func ParseDocument(value any) Document {
	doc := Document{
		Version:    1,
		Width:      800,
		Height:     450,
		Background: "#0b0c0d",
	}
	root := mapOf(value)
	if root == nil {
		doc.Layers = []Layer{defaultLayer("layer_1")}
		doc.Elements = nil
		return doc
	}
	doc.Version = clampInt(integer(root["version"], 1), 1, 99)
	doc.Width = clampInt(integer(root["width"], doc.Width), 1, MaxCanvasDimension)
	doc.Height = clampInt(integer(root["height"], doc.Height), 1, MaxCanvasDimension)
	doc.Background = parseBackground(str(root["background"], doc.Background))

	if pinsRaw, ok := root["pins"].([]any); ok {
		for _, raw := range pinsRaw {
			entry := mapOf(raw)
			if entry == nil {
				continue
			}
			name := strings.TrimSpace(str(entry["name"], ""))
			if !ValidPinName(name) {
				continue
			}
			pinType := PinType(oneOf(str(entry["type"], string(PinText)), string(PinText),
				string(PinText), string(PinNumber), string(PinBoolean), string(PinObject), string(PinArray)))
			doc.Pins = append(doc.Pins, DocPin{
				Name:    name,
				Type:    pinType,
				Sample:  str(entry["sample"], ""),
				Default: str(entry["default"], ""),
			})
		}
		// drop duplicate names, keeping the first declaration
		seen := map[string]bool{}
		pins := doc.Pins[:0]
		for _, pin := range doc.Pins {
			if seen[pin.Name] {
				continue
			}
			seen[pin.Name] = true
			pins = append(pins, pin)
		}
		doc.Pins = pins
	}

	layerIDs := map[string]bool{}
	if layersRaw, ok := root["layers"].([]any); ok && len(layersRaw) > 0 {
		for _, raw := range layersRaw {
			entry := mapOf(raw)
			if entry == nil {
				continue
			}
			id := str(entry["id"], "")
			if id == "" || layerIDs[id] {
				continue
			}
			layerIDs[id] = true
			doc.Layers = append(doc.Layers, Layer{
				ID:      id,
				Name:    str(entry["name"], id),
				Visible: boolean(entry["visible"], true),
				Opacity: clamp(num(entry["opacity"], 1), 0, 1),
				Locked:  boolean(entry["locked"], false),
			})
		}
	}
	if len(doc.Layers) == 0 {
		doc.Layers = []Layer{defaultLayer("layer_1")}
		layerIDs[doc.Layers[0].ID] = true
	}

	if elementsRaw, ok := root["elements"].([]any); ok {
		for _, raw := range elementsRaw {
			element, ok := parseElement(raw, layerIDs, doc.Layers[0].ID)
			if ok {
				doc.Elements = append(doc.Elements, element)
			}
		}
	}
	return doc
}

func defaultLayer(id string) Layer {
	return Layer{ID: id, Name: "Layer 1", Visible: true, Opacity: 1, Locked: false}
}

func parseBackground(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" || trimmed == "transparent" || trimmed == "none" {
		return "transparent"
	}
	if !strings.HasPrefix(trimmed, "#") {
		return "#0b0c0d"
	}
	hex := strings.TrimPrefix(trimmed, "#")
	switch len(hex) {
	case 3, 4, 6, 8:
		if isHex(hex) {
			return "#" + hex
		}
	}
	return "#0b0c0d"
}

func isHex(value string) bool {
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func parseElement(value any, layerIDs map[string]bool, firstLayerID string) (Element, bool) {
	entry := mapOf(value)
	if entry == nil {
		return Element{}, false
	}
	elementType := ElementType(oneOf(str(entry["type"], ""), string(ElementRect),
		string(ElementRect), string(ElementEllipse), string(ElementLine), string(ElementStar), string(ElementText), string(ElementImage)))
	id := str(entry["id"], "")
	if id == "" {
		return Element{}, false
	}
	layerID := str(entry["layerId"], "")
	if !layerIDs[layerID] {
		// elements without a valid layer ride on the first layer
		layerID = firstLayerID
	}
	element := Element{
		ID:         id,
		Type:       elementType,
		LayerID:    layerID,
		Name:       str(entry["name"], defaultElementName(elementType)),
		X:          num(entry["x"], 0),
		Y:          num(entry["y"], 0),
		W:          clamp(num(entry["w"], 100), -MaxCanvasDimension*2, MaxCanvasDimension*2),
		H:          clamp(num(entry["h"], 100), -MaxCanvasDimension*2, MaxCanvasDimension*2),
		Rotation:   clamp(num(entry["rotation"], 0), -360, 360),
		Opacity:    clamp(num(entry["opacity"], 1), 0, 1),
		Visible:    boolean(entry["visible"], true),
		Visibility: parseVisibility(entry["visibility"]),
		Repeat:     parseRepeat(entry["repeat"]),

		Radius:     clamp(num(entry["radius"], 0), 0, MaxCanvasDimension),
		Fill:       parsePaint(entry["fill"]),
		Stroke:     parseStroke(entry["stroke"]),
		Points:     parsePoints(entry["points"]),
		StarPoints: clampInt(integer(entry["points"], 5), 3, 24),
		InnerRatio: clamp(num(entry["innerRatio"], 0.5), 0.05, 0.95),

		Content:    str(entry["content"], ""),
		FontFamily: oneOf(str(entry["fontFamily"], FontInter), FontInter, FontInter, FontMono),
		FontSize:   clamp(num(entry["fontSize"], 24), 1, 512),
		Weight:     clampInt(integer(entry["weight"], 400), 100, 900),
		Italic:     boolean(entry["italic"], false),
		Color:      parseColorValue(str(entry["color"], "#f7f8f8"), "#f7f8f8"),
		Align:      oneOf(str(entry["align"], "left"), "left", "left", "center", "right"),
		VAlign:     oneOf(str(entry["valign"], "top"), "top", "top", "middle", "bottom"),
		LineHeight: clamp(num(entry["lineHeight"], 1.2), 0.5, 3),
		// WrapWidth: -1 wraps to the element width, 0 disables wrapping.
		WrapWidth: clamp(num(entry["wrapWidth"], 0), -1, MaxCanvasDimension),

		Source:    parseImageSource(entry["source"]),
		Fit:       oneOf(str(entry["fit"], "cover"), "cover", "fill", "contain", "cover"),
		OnMissing: oneOf(str(entry["onMissing"], "skip"), "skip", "skip", "error"),
	}
	return element, true
}

func defaultElementName(elementType ElementType) string {
	switch elementType {
	case ElementRect:
		return "Rectangle"
	case ElementEllipse:
		return "Ellipse"
	case ElementLine:
		return "Line"
	case ElementStar:
		return "Star"
	case ElementText:
		return "Text"
	case ElementImage:
		return "Image"
	}
	return "Element"
}

func parseVisibility(value any) Visibility {
	entry := mapOf(value)
	if entry == nil {
		return Visibility{Mode: "always"}
	}
	mode := oneOf(str(entry["mode"], "always"), "always", "always", "condition")
	visibility := Visibility{Mode: mode}
	if mode == "condition" {
		visibility.Pin = strings.TrimSpace(str(entry["pin"], ""))
		visibility.Op = str(entry["op"], "")
		visibility.Value = str(entry["value"], "")
		if visibility.Pin == "" || visibility.Op == "" {
			return Visibility{Mode: "always"}
		}
	}
	return visibility
}

func parseRepeat(value any) *Repeat {
	entry := mapOf(value)
	if entry == nil {
		return nil
	}
	pin := strings.TrimSpace(str(entry["pin"], ""))
	if pin == "" {
		return nil
	}
	return &Repeat{
		Pin:     pin,
		OffsetX: clamp(num(entry["offsetX"], 0), -MaxCanvasDimension*2, MaxCanvasDimension*2),
		OffsetY: clamp(num(entry["offsetY"], 0), -MaxCanvasDimension*2, MaxCanvasDimension*2),
		Limit:   clampInt(integer(entry["limit"], 0), 0, MaxRepeatCopies),
	}
}

func parsePaint(value any) Paint {
	entry := mapOf(value)
	if entry == nil {
		return Paint{Type: "solid", Color: "#141516"}
	}
	paintType := oneOf(str(entry["type"], "solid"), "solid", "solid", "linear", "radial")
	paint := Paint{Type: paintType}
	switch paintType {
	case "linear":
		paint.X0 = num(entry["x0"], 0)
		paint.Y0 = num(entry["y0"], 0)
		paint.X1 = num(entry["x1"], 100)
		paint.Y1 = num(entry["y1"], 0)
		paint.Stops = parseStops(entry["stops"])
		if len(paint.Stops) == 0 {
			return Paint{Type: "solid", Color: "#141516"}
		}
	case "radial":
		paint.CX = num(entry["cx"], 50)
		paint.CY = num(entry["cy"], 50)
		paint.R = clamp(num(entry["r"], 50), 0.01, MaxCanvasDimension)
		paint.Stops = parseStops(entry["stops"])
		if len(paint.Stops) == 0 {
			return Paint{Type: "solid", Color: "#141516"}
		}
	default:
		paint.Color = parseColorValue(str(entry["color"], "#141516"), "#141516")
	}
	return paint
}

func parseStops(value any) []GradientStop {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	var stops []GradientStop
	for _, item := range raw {
		entry := mapOf(item)
		if entry == nil {
			continue
		}
		stops = append(stops, GradientStop{
			Offset: clamp(num(entry["offset"], 0), 0, 1),
			Color:  parseColorValue(str(entry["color"], "#ffffff"), "#ffffff"),
		})
	}
	if len(stops) == 0 {
		return nil
	}
	return stops
}

func parseStroke(value any) *Stroke {
	entry := mapOf(value)
	if entry == nil {
		return nil
	}
	width := clamp(num(entry["width"], 1), 0, 200)
	if width <= 0 {
		return nil
	}
	return &Stroke{
		Color: parseColorValue(str(entry["color"], "#232326"), "#232326"),
		Width: width,
		Dash:  parseDash(entry["dash"]),
		Cap:   oneOf(str(entry["cap"], "butt"), "butt", "butt", "round", "square"),
		Join:  oneOf(str(entry["join"], "miter"), "miter", "miter", "round", "bevel"),
	}
}

func parseDash(value any) []float64 {
	raw, ok := value.([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	var dash []float64
	for _, item := range raw {
		length := clamp(num(item, 0), 0, 2000)
		if length > 0 {
			dash = append(dash, length)
		}
	}
	return dash
}

func parsePoints(value any) []Point {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	var points []Point
	for _, item := range raw {
		entry := mapOf(item)
		if entry == nil {
			continue
		}
		points = append(points, Point{X: num(entry["x"], 0), Y: num(entry["y"], 0)})
	}
	return points
}

func parseImageSource(value any) ImageSource {
	entry := mapOf(value)
	if entry == nil {
		return ImageSource{Kind: "url", Value: ""}
	}
	kind := oneOf(str(entry["kind"], "url"), "url", "url", "path", "pin")
	return ImageSource{Kind: kind, Value: strings.TrimSpace(str(entry["value"], ""))}
}

// parseColorValue validates a hex color, returning fallback on garbage.
func parseColorValue(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "#") {
		return fallback
	}
	hex := strings.TrimPrefix(trimmed, "#")
	switch len(hex) {
	case 3, 4, 6, 8:
		lower := strings.ToLower(hex)
		if isHex(lower) {
			return "#" + lower
		}
	}
	return fallback
}

// PinByName returns the declared pin and whether it exists.
func (d Document) PinByName(name string) (DocPin, bool) {
	for _, pin := range d.Pins {
		if pin.Name == name {
			return pin, true
		}
	}
	return DocPin{}, false
}

// LayerByID returns the layer and whether it exists.
func (d Document) LayerByID(id string) (Layer, bool) {
	for _, layer := range d.Layers {
		if layer.ID == id {
			return layer, true
		}
	}
	return Layer{}, false
}

// Package drawimage registers the Draw Image Blueprint node, which composes a
// raster image at runtime from a visual document (canvas, layers, shapes,
// text, and image elements) designed in the pipeline editor. The same
// document drives the editor preview (HTML5 canvas) and this package's gg
// renderer so both stay visually identical.
package drawimage

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// New creates the Draw Image module implementation.
func New() Node {
	definition := definition()
	return Node{Metadata: definition, Resolver: resolve, Executor: execute}
}

// Register contributes the Draw Image module to the node registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func definition() domain.NodeDefinition {
	imageType := domain.TypeSpec{Kind: domain.TypeBytes}
	resultType := resultType()
	defaults := map[string]any{
		"document":   DefaultDocumentValue(),
		"outputPath": "",
		"format":     "png",
		"quality":    90,
	}
	return domain.NodeDefinition{
		Type:        "action:draw_image",
		Category:    "Image",
		Label:       "Draw Image",
		Description: "Compose a pixel-perfect image from layers, shapes, text, and pictures, then emit it as bytes, base64, and an optional file.",
		Icon:        "image",
		Color:       "#38bdf8",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "outputPath", Label: "Output path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: ptr(typespec.String()), Color: "#e879f9", MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "image", Label: "Image", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataBytes, Type: &imageType, Color: "#fbbf24", MaxConnections: 1},
			{ID: "base64", Label: "Base64", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: ptr(typespec.String()), Color: "#e879f9", MaxConnections: 1},
			{
				ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput,
				DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1,
				Fields: []domain.DataField{
					{Path: "path", Label: "Path", DataType: domain.DataText, Description: "Written file path, empty when no output path was set."},
					{Path: "width", Label: "Width", DataType: domain.DataNumber, Description: "Rendered image width in pixels."},
					{Path: "height", Label: "Height", DataType: domain.DataNumber, Description: "Rendered image height in pixels."},
					{Path: "sizeBytes", Label: "Size bytes", DataType: domain.DataNumber, Description: "Encoded image size in bytes."},
					{Path: "format", Label: "Format", DataType: domain.DataText, Description: "Encoding format: png or jpeg."},
					{Path: "warnings", Label: "Warnings", DataType: domain.DataList, Description: "Non-fatal render notices (e.g. skipped missing images)."},
				},
			},
		},
		Fields: []domain.ConfigField{
			{Name: "document", Label: "Image document", Kind: "image-editor", Required: true},
			{Name: "outputPath", Label: "Output path", Kind: "string", Placeholder: "C:\\Images\\weather.png", Required: false},
			{Name: "format", Label: "Format", Kind: "select", Required: true, Options: []domain.Option{
				{Value: "png", Label: "PNG"},
				{Value: "jpeg", Label: "JPEG"},
			}},
			{Name: "quality", Label: "JPEG quality", Kind: "number", Placeholder: "90", Required: false},
		},
		Capabilities:      []domain.Capability{domain.CapabilityFileWrite, domain.CapabilityNetwork},
		DefaultConfig:     defaults,
		Source:            "builtin",
		PortContractOwned: true,
	}
}

// DefaultDocumentValue is the starter document installed on new nodes: a
// dark canvas with a gradient hero bar, a title bound to the {{title}} pin,
// and a subtitle, mirroring the editor's "new document" state.
func DefaultDocumentValue() map[string]any {
	return map[string]any{
		"version":    1,
		"width":      800,
		"height":     450,
		"background": "#0b0c0d",
		"pins":       []any{map[string]any{"name": "title", "type": "text", "sample": "Weather Report", "default": "Weather Report"}},
		"layers":     []any{map[string]any{"id": "layer_1", "name": "Layer 1", "visible": true, "opacity": 1, "locked": false}},
		"elements": []any{
			map[string]any{
				"id": "el_1", "type": "rect", "layerId": "layer_1", "name": "Hero",
				"x": 0, "y": 0, "w": 800, "h": 160, "rotation": 0, "opacity": 1, "visible": true,
				"visibility": map[string]any{"mode": "always"},
				"radius":     0,
				"fill":       map[string]any{"type": "linear", "x0": 0, "y0": 0, "x1": 800, "y1": 160, "stops": []any{map[string]any{"offset": 0, "color": "#4ea7fc"}, map[string]any{"offset": 1, "color": "#00b8cc"}}},
				"stroke":     nil,
			},
			map[string]any{
				"id": "el_2", "type": "text", "layerId": "layer_1", "name": "Title",
				"x": 40, "y": 40, "w": 720, "h": 80, "rotation": 0, "opacity": 1, "visible": true,
				"visibility": map[string]any{"mode": "always"},
				"content":    "{{title}}",
				"fontFamily": "inter", "fontSize": 44, "weight": 700, "italic": false,
				"color": "#f7f8f8", "align": "left", "valign": "middle",
				"lineHeight": 1.2, "wrapWidth": 720,
			},
			map[string]any{
				"id": "el_3", "type": "text", "layerId": "layer_1", "name": "Subtitle",
				"x": 40, "y": 120, "w": 720, "h": 30, "rotation": 0, "opacity": 0.8, "visible": true,
				"visibility": map[string]any{"mode": "always"},
				"content":    "Drawn with the Draw Image node",
				"fontFamily": "inter", "fontSize": 18, "weight": 400, "italic": false,
				"color": "#08090a", "align": "left", "valign": "middle",
				"lineHeight": 1.2, "wrapWidth": 0,
			},
		},
	}
}

func resultType() domain.TypeSpec {
	return domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "path", Name: "path", Type: typespec.String()},
		{ID: "width", Name: "width", Type: typespec.Float()},
		{ID: "height", Name: "height", Type: typespec.Float()},
		{ID: "sizeBytes", Name: "sizeBytes", Type: typespec.Float()},
		{ID: "format", Name: "format", Type: typespec.String()},
		{ID: "warnings", Name: "warnings", Type: domain.TypeSpec{Kind: domain.TypeList, Element: ptr(typespec.String())}},
	}}
}

func ptr[T any](value T) *T {
	return &value
}

/* ------------------------------------------------------------------ */
/* dynamic pins                                                        */
/* ------------------------------------------------------------------ */

// pinPortSpec maps a declared document pin onto a node port.
func pinPortSpec(pin DocPin) domain.NodePort {
	var dataType domain.DataType
	var typeSpec domain.TypeSpec
	switch pin.Type {
	case PinNumber:
		dataType, typeSpec = domain.DataNumber, typespec.Float()
	case PinBoolean:
		dataType, typeSpec = domain.DataBoolean, typespec.Bool()
	case PinObject:
		dataType = domain.DataObject
		typeSpec = domain.TypeSpec{Kind: domain.TypeMap, Key: ptr(typespec.String()), Value: ptr(typespec.Any())}
	case PinArray:
		dataType = domain.DataList
		typeSpec = domain.TypeSpec{Kind: domain.TypeList, Element: ptr(typespec.Any())}
	default:
		dataType, typeSpec = domain.DataText, typespec.String()
	}
	port := domain.NodePort{
		ID: pin.Name, Label: pin.Name, Kind: domain.PinData, Direction: domain.PinInput,
		DataType: dataType, Type: &typeSpec, Color: pinColor(dataType), MaxConnections: 1,
	}
	if pin.Default != "" && pin.Type == PinText {
		port.Default = pin.Default
	}
	return port
}

func pinColor(dataType domain.DataType) string {
	switch dataType {
	case domain.DataText:
		return "#e879f9"
	case domain.DataNumber:
		return "#86efac"
	case domain.DataBoolean:
		return "#f87171"
	case domain.DataList:
		return "#facc15"
	case domain.DataObject:
		return "#60a5fa"
	default:
		return "#a1a1aa"
	}
}

// resolve adapts the input pin contract to the document's declared pins so
// the editor highlights compatible connections per pin type.
func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	def := definition()
	doc := ParseDocument(configValue(node, "document"))
	resolved := def
	resolved.Inputs = append([]domain.NodePort{def.Inputs[0]}, pinPorts(doc, def)...)
	return resolved, nil
}

func pinPorts(doc Document, def domain.NodeDefinition) []domain.NodePort {
	ports := make([]domain.NodePort, 0, len(doc.Pins)+1)
	ports = append(ports, def.Inputs[1]) // outputPath
	for _, pin := range doc.Pins {
		ports = append(ports, pinPortSpec(pin))
	}
	return ports
}

func configValue(node domain.FlowNode, key string) any {
	config := map[string]any{}
	config, _ = node.Data["config"].(map[string]any)
	return config[key]
}

/* ------------------------------------------------------------------ */
/* execution                                                           */
/* ------------------------------------------------------------------ */

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("draw image cancelled: %w", err)
	}
	documentValue := invocation.Config["document"]
	if documentValue == nil {
		if defaults := invocation.Definition.DefaultConfig; defaults != nil {
			documentValue = defaults["document"]
		}
	}
	doc := ParseDocument(documentValue)

	format := stringOrConfig(invocation, "format", "png")
	if format != "jpeg" {
		format = "png"
	}
	quality := intOrConfig(invocation, "quality", 90)
	if quality < 1 || quality > 100 {
		quality = 90
	}

	values := map[string]any{}
	for _, pin := range doc.Pins {
		if value, ok := invocation.Inputs[pin.Name]; ok {
			values[pin.Name] = value
			continue
		}
		if pin.Default != "" {
			values[pin.Name] = typedDefault(pin)
		}
	}

	encoded, warnings, err := Render(ctx, doc, values, RenderOptions{Format: format, Quality: quality})
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("draw image: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("draw image cancelled: %w", err)
	}

	outputPath := strings.TrimSpace(stringInput(invocation, "outputPath"))
	writtenPath := ""
	if outputPath != "" {
		cleanPath := filepath.Clean(outputPath)
		if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("create image directory: %w", err)
		}
		if err := os.WriteFile(cleanPath, encoded, 0o600); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("write image: %w", err)
		}
		writtenPath = cleanPath
	}

	warningTexts := make([]any, 0, len(warnings))
	for _, warning := range warnings {
		warningTexts = append(warningTexts, fmt.Sprintf("%s: %s", warning.Element, warning.Message))
	}

	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"image":  encoded,
			"base64": base64.StdEncoding.EncodeToString(encoded),
			"result": map[string]any{
				"path":      writtenPath,
				"width":     float64(doc.Width),
				"height":    float64(doc.Height),
				"sizeBytes": float64(len(encoded)),
				"format":    format,
				"warnings":  warningTexts,
			},
		},
		Ports: []string{"out"},
	}, nil
}

func typedDefault(pin DocPin) any {
	switch pin.Type {
	case PinNumber:
		return pin.Default
	case PinBoolean:
		return pin.Default == "true"
	default:
		return pin.Default
	}
}

func stringInput(invocation nodes.Invocation, key string) string {
	if value, ok := invocation.Inputs[key].(string); ok {
		return value
	}
	if value, ok := invocation.Config[key].(string); ok {
		return value
	}
	return ""
}

func stringOrConfig(invocation nodes.Invocation, key, fallback string) string {
	if value, ok := invocation.Config[key].(string); ok && value != "" {
		return value
	}
	return fallback
}

func intOrConfig(invocation nodes.Invocation, key string, fallback int) int {
	switch value := invocation.Config[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

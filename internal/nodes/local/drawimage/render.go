package drawimage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"math"

	"github.com/gogpu/gg"
	"github.com/gogpu/gg/text"
)

/* ------------------------------------------------------------------ */
/* renderer                                                            */
/* ------------------------------------------------------------------ */

// RenderOptions tweaks output encoding.
type RenderOptions struct {
	Format  string // "png" | "jpeg"
	Quality int    // jpeg quality 1..100
}

// RenderWarning is a non-fatal render notice (e.g. skipped missing image).
type RenderWarning struct {
	Element string
	Message string
}

// Render draws the document into an encoded image. values carries the
// resolved pin values keyed by pin name.
func Render(ctx context.Context, doc Document, values map[string]any, options RenderOptions) ([]byte, []RenderWarning, error) {
	dc := gg.NewContext(doc.Width, doc.Height)
	defer dc.Close()

	if doc.Background == "transparent" {
		dc.Clear()
	} else {
		dc.ClearWithColor(hexColor(doc.Background))
	}

	loader := newImageLoader(ctx)
	var warnings []RenderWarning

	for _, layer := range doc.Layers {
		if !layer.Visible {
			continue
		}
		if layer.Opacity >= 1 {
			for _, element := range doc.ElementsForLayer(layer.ID) {
				var err error
				warnings, err = drawElement(dc, loader, doc, element, values, warnings)
				if err != nil {
					return nil, warnings, err
				}
			}
			continue
		}
		// group composite for translucent layers
		dc.PushLayer(gg.BlendNormal, layer.Opacity)
		for _, element := range doc.ElementsForLayer(layer.ID) {
			var err error
			warnings, err = drawElement(dc, loader, doc, element, values, warnings)
			if err != nil {
				dc.PopLayer()
				return nil, warnings, err
			}
		}
		dc.PopLayer()
	}

	var buffer bytes.Buffer
	if options.Format == "jpeg" {
		if err := dc.EncodeJPEG(&buffer, options.Quality); err != nil {
			return nil, warnings, fmt.Errorf("encode jpeg: %w", err)
		}
	} else {
		if err := dc.EncodePNG(&buffer); err != nil {
			return nil, warnings, fmt.Errorf("encode png: %w", err)
		}
	}
	return buffer.Bytes(), warnings, nil
}

// ElementsForLayer returns this layer's elements in document order.
func (d Document) ElementsForLayer(layerID string) []Element {
	var result []Element
	for _, element := range d.Elements {
		if element.LayerID == layerID {
			result = append(result, element)
		}
	}
	return result
}

/* ------------------------------------------------------------------ */
/* element dispatch                                                    */
/* ------------------------------------------------------------------ */

func drawElement(dc *gg.Context, loader *imageLoader, doc Document, element Element, values map[string]any, warnings []RenderWarning) ([]RenderWarning, error) {
	if !element.Visible {
		return warnings, nil
	}
	if element.Repeat != nil {
		// repeated elements evaluate their condition per copy so pseudo-pins
		// (item/index) resolve against each array entry
		return drawRepeated(dc, loader, doc, element, values, warnings)
	}
	if !EvaluateCondition(element.Visibility, values, doc.pinTypeOf(element.Visibility.Pin)) {
		return warnings, nil
	}
	if err := drawOnce(dc, loader, element, TemplateContext(values)); err != nil {
		if element.Type == ElementImage && element.OnMissing == "error" {
			return warnings, fmt.Errorf("image %q: %w", element.Name, err)
		}
		return append(warnings, RenderWarning{Element: element.Name, Message: err.Error()}), nil
	}
	return warnings, nil
}

func drawRepeated(dc *gg.Context, loader *imageLoader, doc Document, element Element, values map[string]any, warnings []RenderWarning) ([]RenderWarning, error) {
	items, ok := values[element.Repeat.Pin].([]any)
	if !ok {
		// non-array or missing pin: render nothing (an explicit condition can
		// gate the element when the array is empty)
		return warnings, nil
	}
	count := len(items)
	if count > MaxRepeatCopies {
		count = MaxRepeatCopies
	}
	if element.Repeat.Limit > 0 && element.Repeat.Limit < count {
		count = element.Repeat.Limit
	}
	for index := 0; index < count; index++ {
		ctx := TemplateContext(values)
		ctx["item"] = items[index]
		ctx["index"] = float64(index)
		clone := element
		clone.X += float64(index) * element.Repeat.OffsetX
		clone.Y += float64(index) * element.Repeat.OffsetY
		if !EvaluateCondition(clone.Visibility, ctx, "") {
			continue
		}
		if err := drawOnce(dc, loader, clone, ctx); err != nil {
			if clone.Type == ElementImage && clone.OnMissing == "error" {
				return warnings, fmt.Errorf("image %q: %w", clone.Name, err)
			}
			warnings = append(warnings, RenderWarning{Element: clone.Name, Message: err.Error()})
		}
	}
	return warnings, nil
}

func (d Document) pinTypeOf(name string) PinType {
	if isPseudoPin(name) {
		return ""
	}
	if pin, ok := d.PinByName(name); ok {
		return pin.Type
	}
	return ""
}

/* ------------------------------------------------------------------ */
/* single element rendering                                            */
/* ------------------------------------------------------------------ */

func drawOnce(dc *gg.Context, loader *imageLoader, element Element, ctx TemplateContext) error {
	opacity := element.Opacity
	rotation := element.Rotation

	switch element.Type {
	case ElementText:
		return drawTextElement(dc, element, ctx, opacity)
	case ElementImage:
		return drawImageElement(dc, loader, element, ctx, opacity, rotation)
	case ElementLine:
		return withRotation(dc, element, rotation, func() error {
			return strokeShape(dc, element, opacity, buildLinePath(element))
		})
	default:
		return withRotation(dc, element, rotation, func() error {
			return fillStrokeShape(dc, element, opacity, func() {
				switch element.Type {
				case ElementRect:
					dc.DrawRoundedRectangle(element.X, element.Y, element.W, element.H, clampRadius(element))
				case ElementEllipse:
					dc.DrawEllipse(element.X+element.W/2, element.Y+element.H/2, element.W/2, element.H/2)
				case ElementStar:
					dc.DrawPath(buildStarPath(element))
				}
			})
		})
	}
}

// withRotation wraps draw in a save/rotate/restore block when rotated.
func withRotation(dc *gg.Context, element Element, rotation float64, draw func() error) error {
	if rotation == 0 {
		return draw()
	}
	cx, cy := element.X+element.W/2, element.Y+element.H/2
	dc.Push()
	dc.RotateAbout(rotation*degree, cx, cy)
	err := draw()
	dc.Pop()
	return err
}

const degree = math.Pi / 180

func clampRadius(element Element) float64 {
	radius := element.Radius
	half := math.Min(math.Abs(element.W), math.Abs(element.H)) / 2
	if radius > half {
		return half
	}
	if radius < 0 {
		return 0
	}
	return radius
}

func buildLinePath(element Element) *gg.Path {
	builder := gg.BuildPath()
	first := true
	for _, point := range element.Points {
		if first {
			builder.MoveTo(point.X, point.Y)
			first = false
		} else {
			builder.LineTo(point.X, point.Y)
		}
	}
	return builder.Build()
}

// buildStarPath computes a regular star polygon inscribed in the element
// bbox with the first outer point pointing up. Mirrored exactly in TS.
func buildStarPath(element Element) *gg.Path {
	cx := element.X + element.W/2
	cy := element.Y + element.H/2
	outer := math.Min(math.Abs(element.W), math.Abs(element.H)) / 2
	inner := outer * element.InnerRatio
	count := element.StarPoints
	if count < 3 {
		count = 3
	}
	builder := gg.BuildPath()
	for i := 0; i < count*2; i++ {
		angle := -math.Pi/2 + float64(i)*math.Pi/float64(count)
		radius := outer
		if i%2 == 1 {
			radius = inner
		}
		x := cx + radius*math.Cos(angle)
		y := cy + radius*math.Sin(angle)
		if i == 0 {
			builder.MoveTo(x, y)
		} else {
			builder.LineTo(x, y)
		}
	}
	builder.Close()
	return builder.Build()
}

/* ------------------------------------------------------------------ */
/* paint application                                                   */
/* ------------------------------------------------------------------ */

func hexColor(value string) gg.RGBA {
	return gg.Hex(value)
}

// applyFillBrush configures the fill brush, multiplying element opacity
// into every stop's alpha.
func applyFillBrush(dc *gg.Context, paint Paint, opacity float64) {
	switch paint.Type {
	case "linear":
		gradient := gg.NewLinearGradientBrush(paint.X0, paint.Y0, paint.X1, paint.Y1)
		for _, stop := range paint.Stops {
			gradient.AddColorStop(stop.Offset, withAlpha(hexColor(stop.Color), opacity))
		}
		dc.SetFillBrush(gradient)
	case "radial":
		gradient := gg.NewRadialGradientBrush(paint.CX, paint.CY, 0, paint.R)
		for _, stop := range paint.Stops {
			gradient.AddColorStop(stop.Offset, withAlpha(hexColor(stop.Color), opacity))
		}
		dc.SetFillBrush(gradient)
	default:
		dc.SetFillBrush(gg.Solid(withAlpha(hexColor(paint.Color), opacity)))
	}
}

func withAlpha(base gg.RGBA, opacity float64) gg.RGBA {
	base.A *= opacity
	return base
}

func applyStrokeStyle(dc *gg.Context, stroke *Stroke, opacity float64) {
	dc.SetStrokeBrush(gg.Solid(withAlpha(hexColor(stroke.Color), opacity)))
	dc.SetLineWidth(stroke.Width)
	switch stroke.Cap {
	case "round":
		dc.SetLineCap(gg.LineCapRound)
	case "square":
		dc.SetLineCap(gg.LineCapSquare)
	default:
		dc.SetLineCap(gg.LineCapButt)
	}
	switch stroke.Join {
	case "round":
		dc.SetLineJoin(gg.LineJoinRound)
	case "bevel":
		dc.SetLineJoin(gg.LineJoinBevel)
	default:
		dc.SetLineJoin(gg.LineJoinMiter)
	}
	if len(stroke.Dash) > 0 {
		dc.SetDash(stroke.Dash...)
	} else {
		dc.ClearDash()
	}
}

func fillStrokeShape(dc *gg.Context, element Element, opacity float64, buildPath func()) error {
	applyFillBrush(dc, element.Fill, opacity)
	buildPath()
	if err := dc.FillPreserve(); err != nil {
		return fmt.Errorf("fill: %w", err)
	}
	if element.Stroke != nil && element.Stroke.Width > 0 {
		applyStrokeStyle(dc, element.Stroke, opacity)
		if err := dc.Stroke(); err != nil {
			return fmt.Errorf("stroke: %w", err)
		}
	}
	dc.ClearPath()
	return nil
}

func strokeShape(dc *gg.Context, element Element, opacity float64, path *gg.Path) error {
	if element.Stroke == nil || element.Stroke.Width <= 0 {
		return nil
	}
	applyStrokeStyle(dc, element.Stroke, opacity)
	if err := dc.StrokePath(path); err != nil {
		return fmt.Errorf("stroke: %w", err)
	}
	return nil
}

/* ------------------------------------------------------------------ */
/* text element                                                        */
/* ------------------------------------------------------------------ */

func drawTextElement(dc *gg.Context, element Element, ctx TemplateContext, opacity float64) error {
	face, err := Face(element.FontFamily, element.Weight, element.Italic, element.FontSize)
	if err != nil {
		return err
	}
	dc.SetFont(face)
	dc.SetFillBrush(gg.Solid(withAlpha(hexColor(element.Color), opacity)))

	content := Interpolate(element.Content, ctx)
	wrapLimit := element.WrapWidth
	if wrapLimit == -1 {
		wrapLimit = element.W
	}
	lines := WrapLines(content, wrapLimit, func(line string) float64 {
		return text.MeasureText(line, face)
	})

	lineAdvance := element.FontSize * element.LineHeight
	blockHeight := float64(len(lines)) * lineAdvance

	// horizontal anchor inside bbox
	anchorX := element.X
	switch element.Align {
	case "center":
		anchorX = element.X + element.W/2
	case "right":
		anchorX = element.X + element.W
	}
	// vertical block placement inside bbox
	blockTop := element.Y
	switch element.VAlign {
	case "middle":
		blockTop = element.Y + (element.H-blockHeight)/2
	case "bottom":
		blockTop = element.Y + element.H - blockHeight
	}

	return withRotation(dc, element, element.Rotation, func() error {
		for i, line := range lines {
			if line == "" {
				continue
			}
			ax := 0.0
			switch element.Align {
			case "center":
				ax = 0.5
			case "right":
				ax = 1
			}
			// anchor ay=0.5 centers the line box (ascent+descent) like the
			// editor's canvas twin
			y := blockTop + float64(i)*lineAdvance + lineAdvance/2
			dc.DrawStringAnchored(line, anchorX, y, ax, 0.5)
		}
		return nil
	})
}

/* ------------------------------------------------------------------ */
/* image element                                                       */
/* ------------------------------------------------------------------ */

func drawImageElement(dc *gg.Context, loader *imageLoader, element Element, ctx TemplateContext, opacity, rotation float64) error {
	buf, err := loader.Load(element.Source, ctx)
	if err != nil {
		return err
	}
	srcW := float64(buf.Width())
	srcH := float64(buf.Height())
	if srcW <= 0 || srcH <= 0 {
		return fmt.Errorf("image has no pixels")
	}

	// object-fit math — mirrored exactly in TS
	dstW, dstH := element.W, element.H
	srcRect := image.Rectangle{Max: image.Point{X: int(srcW), Y: int(srcH)}}
	switch element.Fit {
	case "contain":
		scale := math.Min(dstW/srcW, dstH/srcH)
		dstW, dstH = srcW*scale, srcH*scale
	case "cover":
		scale := math.Max(dstW/srcW, dstH/srcH)
		cropW, cropH := dstW/scale, dstH/scale
		srcX := (srcW - cropW) / 2
		srcY := (srcH - cropH) / 2
		srcRect = image.Rect(int(srcX), int(srcY), int(srcX+cropW), int(srcY+cropH))
	}

	// center contain-fit inside bbox; other fits anchor at the bbox origin
	dstX, dstY := element.X, element.Y
	if element.Fit == "contain" {
		dstX = element.X + (element.W-dstW)/2
		dstY = element.Y + (element.H-dstH)/2
	}

	return withRotation(dc, element, rotation, func() error {
		options := gg.DrawImageOptions{
			X:             dstX,
			Y:             dstY,
			DstWidth:      dstW,
			DstHeight:     dstH,
			SrcRect:       &srcRect,
			Opacity:       opacity,
			Interpolation: gg.InterpBilinear,
		}
		if element.Radius > 0 {
			dc.Push()
			dc.ClipRoundRect(element.X, element.Y, element.W, element.H, clampRadius(element))
			dc.DrawImageEx(buf, options)
			dc.Pop()
			return nil
		}
		dc.DrawImageEx(buf, options)
		return nil
	})
}

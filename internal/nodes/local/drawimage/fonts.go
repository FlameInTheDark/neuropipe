package drawimage

import (
	"embed"
	"fmt"
	"sync"

	"github.com/gogpu/gg/text"
)

// Font family identifiers stored in the document.
const (
	FontInter = "inter"
	FontMono  = "jetbrains-mono"
)

//go:embed fonts/*.ttf fonts/LICENSE-*.txt
var fontFS embed.FS

// fontAsset maps a family/italic pair to its embedded TTF file.
func fontAsset(family string, italic bool) (string, bool) {
	switch family {
	case FontMono:
		if italic {
			return "fonts/JetBrainsMono-VariableItalic.ttf", true
		}
		return "fonts/JetBrainsMono-Variable.ttf", true
	default:
		if italic {
			return "fonts/InterVariable-Italic.ttf", true
		}
		return "fonts/InterVariable.ttf", true
	}
}

type fontSourceKey struct {
	family string
	italic bool
}

type faceKey struct {
	family string
	italic bool
	weight int
	size   float64
}

var (
	fontSourcesMu sync.Mutex
	fontSources   = map[fontSourceKey]*text.FontSource{}

	facesMu sync.Mutex
	faces   = map[faceKey]text.Face{}
)

// loadFontSource lazily parses an embedded TTF exactly once per process.
func loadFontSource(family string, italic bool) (*text.FontSource, error) {
	key := fontSourceKey{family: family, italic: italic}
	fontSourcesMu.Lock()
	defer fontSourcesMu.Unlock()
	if source, ok := fontSources[key]; ok {
		return source, nil
	}
	asset, ok := fontAsset(family, italic)
	if !ok {
		return nil, fmt.Errorf("unknown font family %q", family)
	}
	data, err := fontFS.ReadFile(asset)
	if err != nil {
		return nil, fmt.Errorf("read embedded font: %w", err)
	}
	source, err := text.NewFontSource(data)
	if err != nil {
		return nil, fmt.Errorf("parse font %s: %w", asset, err)
	}
	fontSources[key] = source
	return source, nil
}

// Face returns a cached shaped face for family/weight/italic/size.
func Face(family string, weight int, italic bool, size float64) (text.Face, error) {
	if weight < 100 {
		weight = 400
	}
	if size <= 0 {
		size = 16
	}
	key := faceKey{family: family, italic: italic, weight: weight, size: size}
	facesMu.Lock()
	if face, ok := faces[key]; ok {
		facesMu.Unlock()
		return face, nil
	}
	facesMu.Unlock()

	source, err := loadFontSource(family, italic)
	if err != nil {
		return nil, err
	}
	var options []text.FaceOption
	if source.IsVariable() {
		options = append(options, text.WithVariations(text.NewFontVariation("wght", float32(weight))))
	}
	face := source.Face(size, options...)

	facesMu.Lock()
	faces[key] = face
	facesMu.Unlock()
	return face, nil
}

// FontFamilyLabel returns the human label for a family identifier.
func FontFamilyLabel(family string) string {
	if family == FontMono {
		return "JetBrains Mono"
	}
	return "Inter"
}

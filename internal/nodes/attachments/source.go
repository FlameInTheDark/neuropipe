package attachments

import (
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// Source modes shared by the send-message nodes' image/file source dropdown.
// The empty mode is Auto: every source participates and the first one that is
// set wins, which is exactly how graphs saved before the dropdown behaved.
const (
	SourceURL    = "url"
	SourceFile   = "file"
	SourceBase64 = "base64"
	SourceBytes  = "bytes"
)

// SourceOptions lists the dropdown options for a source selector. Auto keeps
// graphs fully functional because it is the value an unconfigured node lacks.
func SourceOptions() []domain.Option {
	return []domain.Option{
		{Value: "", Label: "Auto — use whatever is set"},
		{Value: SourceURL, Label: "URL"},
		{Value: SourceFile, Label: "Local file"},
		{Value: SourceBase64, Label: "Base64"},
		{Value: SourceBytes, Label: "Bytes from another node"},
	}
}

// SourceMode normalises a selector value. Unknown values fall back to Auto so
// a typo can never hide every input.
func SourceMode(value any) string {
	text, _ := value.(string)
	switch text {
	case SourceURL, SourceFile, SourceBase64, SourceBytes:
		return text
	default:
		return ""
	}
}

// SourceIncludes reports whether the named source participates under the
// selected mode; Auto includes everything.
func SourceIncludes(mode, source string) bool {
	if mode == "" {
		return true
	}
	return mode == source
}

// NameIncludes reports whether the file-name input participates under the
// selected mode. It only matters for the sources that carry raw data without
// a derivable name: Base64, Bytes, and Auto (which may use either).
func NameIncludes(mode string) bool {
	switch mode {
	case SourceBase64, SourceBytes:
		return true
	default:
		return mode == ""
	}
}

// SourceLabel renders one mode the way the dropdown shows it, for
// required-input error messages.
func SourceLabel(mode string) string {
	switch mode {
	case SourceURL:
		return "URL"
	case SourceFile:
		return "Local file"
	case SourceBase64:
		return "Base64"
	case SourceBytes:
		return "Bytes from another node"
	default:
		return "Auto"
	}
}

// DataValue filters a raw data pin value down to what the loader accepts:
// nil and empty/blank text are treated as "no data" so a wired-but-empty pin
// never turns into a hard loader error, mirroring the pre-selector behaviour.
func DataValue(value any) any {
	if value == nil {
		return nil
	}
	if text, ok := value.(string); ok && IsBlank(text) {
		return nil
	}
	return value
}

// IsBlank reports whether a string carries nothing but whitespace.
func IsBlank(text string) bool {
	return strings.TrimSpace(text) == ""
}

package drawimage

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

/* ------------------------------------------------------------------ */
/* template interpolation                                              */
/* ------------------------------------------------------------------ */

var placeholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.]+)\s*}}`)

// TemplateContext resolves placeholder names to values. Pseudo-pins "item"
// and "index" are provided by the repetition renderer.
type TemplateContext map[string]any

// Interpolate expands {{name}} placeholders in content using ctx.
func Interpolate(content string, ctx TemplateContext) string {
	if !strings.Contains(content, "{{") {
		return content
	}
	return placeholderPattern.ReplaceAllStringFunc(content, func(match string) string {
		groups := placeholderPattern.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}
		return StringifyValue(resolvePath(ctx, groups[1]))
	})
}

// resolvePath walks dotted paths: "a.b.0" looks up ctx["a"], then key "b",
// then index 0. Missing segments resolve to nil.
func resolvePath(ctx TemplateContext, path string) any {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil
	}
	current, ok := ctx[parts[0]]
	if !ok {
		return nil
	}
	for _, part := range parts[1:] {
		if current == nil {
			return nil
		}
		switch typed := current.(type) {
		case map[string]any:
			next, exists := typed[part]
			if !exists {
				return nil
			}
			current = next
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return nil
			}
			current = typed[index]
		default:
			return nil
		}
	}
	return current
}

// StringifyValue renders a pin value for text interpolation. The rules are
// mirrored by the editor's TypeScript twin:
//   - string: as-is
//   - number: shortest round-trip decimal (21.5, not 21.500000)
//   - bool:   "true" / "false"
//   - []any:  items stringified and joined with ", "
//   - map:    compact JSON with sorted keys
//   - nil:    ""
func StringifyValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return FormatNumber(typed)
	case int:
		return FormatNumber(float64(typed))
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, StringifyValue(item))
		}
		return strings.Join(parts, ", ")
	case []byte:
		return string(typed)
	case map[string]any:
		return compactSortedJSON(typed)
	default:
		// Fallbacks for exotic wire values (int64, json.Number, ...).
		if marshalable, err := json.Marshal(typed); err == nil {
			var generic any
			if json.Unmarshal(marshalable, &generic) == nil {
				return StringifyValue(generic)
			}
		}
		return fmt.Sprintf("%v", typed)
	}
}

// FormatNumber renders a float64 the way the browser's String(number) does:
// shortest representation that round-trips, no exponent for typical ranges.
func FormatNumber(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "0"
	}
	if value == math.Trunc(value) && math.Abs(value) < 1e21 {
		return strconv.FormatFloat(value, 'f', 0, 64)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func compactSortedJSON(value map[string]any) string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	builder.WriteByte('{')
	for index, key := range keys {
		if index > 0 {
			builder.WriteByte(',')
		}
		keyJSON, _ := json.Marshal(key)
		builder.Write(keyJSON)
		builder.WriteByte(':')
		builder.WriteString(stringifyJSONScalar(value[key]))
	}
	builder.WriteByte('}')
	return builder.String()
}

func stringifyJSONScalar(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		return compactSortedJSON(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, stringifyJSONScalar(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case nil:
		return "null"
	default:
		// JSON scalars keep their type: numbers stay numbers, strings stay
		// quoted. Exotic Go values round-trip through encoding/json first.
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "null"
		}
		return string(encoded)
	}
}

/* ------------------------------------------------------------------ */
/* word wrapping                                                       */
/* ------------------------------------------------------------------ */

// Measurer measures a single line of text.
type Measurer func(text string) float64

// WrapLines splits content into rendered lines: explicit newlines first,
// then greedy word wrapping against limit when limit > 0. The algorithm is
// identical to the editor's TypeScript twin:
//   - split on "\n"
//   - split each paragraph on spaces (sequences of spaces collapse)
//   - append words while (line empty or line+space+word fits limit)
//   - a single word longer than the limit occupies its own line
func WrapLines(content string, limit float64, measure Measurer) []string {
	paragraphs := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var lines []string
	for _, paragraph := range paragraphs {
		if limit <= 0 {
			lines = append(lines, paragraph)
			continue
		}
		words := splitWords(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		current := ""
		for _, word := range words {
			candidate := word
			if current != "" {
				candidate = current + " " + word
			}
			if current == "" || measure(candidate) <= limit {
				current = candidate
				continue
			}
			lines = append(lines, current)
			current = word
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

// splitWords splits on spaces, collapsing runs of whitespace, matching the
// browser's string.split(/\s+/) minus empty edges.
func splitWords(text string) []string {
	fields := strings.Fields(text)
	return fields
}

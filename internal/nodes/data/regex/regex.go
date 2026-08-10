// Package regex contains shared RE2 contracts and result builders for the
// focused Regex data-node modules. It is not itself a registered node.
package regex

import (
	"fmt"
	"regexp"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Compile builds a Go RE2 expression without altering the supplied pattern.
func Compile(pattern string) (*regexp.Regexp, error) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile RE2 pattern: %w", err)
	}
	return compiled, nil
}

// Text reads one exact text value. Node input validation performs the same
// check during graph execution; this guard also keeps direct module use safe.
func Text(inputs map[string]any, key string) (string, error) {
	value, ok := inputs[key].(string)
	if !ok {
		return "", fmt.Errorf("%s must be text", key)
	}
	return value, nil
}

// TextPin creates a strict Text input or output pin.
func TextPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	typeSpec := typespec.String()
	return domain.NodePort{
		ID:             id,
		Label:          label,
		Kind:           domain.PinData,
		Direction:      direction,
		DataType:       domain.DataText,
		Type:           &typeSpec,
		Color:          "#e879f9",
		Required:       required,
		MaxConnections: 1,
	}
}

// BoolPin creates a strict Boolean output pin.
func BoolPin(id, label string) domain.NodePort {
	typeSpec := typespec.Bool()
	return domain.NodePort{
		ID:             id,
		Label:          label,
		Kind:           domain.PinData,
		Direction:      domain.PinOutput,
		DataType:       domain.DataBoolean,
		Type:           &typeSpec,
		Color:          "#f87171",
		MaxConnections: 1,
	}
}

// IntPin creates a strict integer output pin. The presentation data type stays
// Number so existing number-color and editor affordances continue to apply.
func IntPin(id, label string) domain.NodePort {
	typeSpec := typespec.Int()
	return domain.NodePort{
		ID:             id,
		Label:          label,
		Kind:           domain.PinData,
		Direction:      domain.PinOutput,
		DataType:       domain.DataNumber,
		Type:           &typeSpec,
		Color:          "#86efac",
		MaxConnections: 1,
	}
}

// StringListPin creates an exact list-of-Text output pin.
func StringListPin(id, label string) domain.NodePort {
	element := typespec.String()
	typeSpec := domain.TypeSpec{Kind: domain.TypeList, Element: &element}
	return domain.NodePort{
		ID:             id,
		Label:          label,
		Kind:           domain.PinData,
		Direction:      domain.PinOutput,
		DataType:       domain.DataList,
		Type:           &typeSpec,
		Color:          "#facc15",
		MaxConnections: 1,
	}
}

// MatchListPin creates an exact list of named RegexMatch records.
func MatchListPin(id, label string) domain.NodePort {
	typeSpec := MatchListType()
	return domain.NodePort{
		ID:             id,
		Label:          label,
		Kind:           domain.PinData,
		Direction:      domain.PinOutput,
		DataType:       domain.DataList,
		Type:           &typeSpec,
		Color:          "#facc15",
		MaxConnections: 1,
	}
}

// MatchListType is the stable list[RegexMatch] public wire contract.
func MatchListType() domain.TypeSpec {
	match := MatchType()
	return domain.TypeSpec{Kind: domain.TypeList, Element: &match}
}

// MatchType is the named structure returned for every complete match.
func MatchType() domain.TypeSpec {
	captures := CaptureType()
	captureList := domain.TypeSpec{Kind: domain.TypeList, Element: &captures}
	return domain.TypeSpec{
		Kind: domain.TypeRecord,
		Name: "RegexMatch",
		Fields: []domain.TypeFieldSpec{
			{ID: "text", Name: "text", Type: typespec.String()},
			{ID: "startByte", Name: "startByte", Type: typespec.Int()},
			{ID: "endByte", Name: "endByte", Type: typespec.Int()},
			{ID: "captures", Name: "captures", Type: captureList},
		},
	}
}

// CaptureType is the named structure returned for every capturing group.
func CaptureType() domain.TypeSpec {
	return domain.TypeSpec{
		Kind: domain.TypeRecord,
		Name: "RegexCapture",
		Fields: []domain.TypeFieldSpec{
			{ID: "index", Name: "index", Type: typespec.Int()},
			{ID: "name", Name: "name", Type: typespec.String()},
			{ID: "matched", Name: "matched", Type: typespec.Bool()},
			{ID: "text", Name: "text", Type: typespec.String()},
			{ID: "startByte", Name: "startByte", Type: typespec.Int()},
			{ID: "endByte", Name: "endByte", Type: typespec.Int()},
		},
	}
}

// RegexMatch is the native Go representation of the named RegexMatch wire
// record. Its JSON tags preserve the field names used by graph contracts.
type RegexMatch struct {
	Text      string         `json:"text"`
	StartByte int            `json:"startByte"`
	EndByte   int            `json:"endByte"`
	Captures  []RegexCapture `json:"captures"`
}

// RegexCapture is the native Go representation of a RegexMatch capture.
type RegexCapture struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Matched   bool   `json:"matched"`
	Text      string `json:"text"`
	StartByte int    `json:"startByte"`
	EndByte   int    `json:"endByte"`
}

// Matches converts RE2 match indices to contract-complete records. Offsets are
// UTF-8 byte offsets, matching Go's regexp package semantics.
func Matches(expression *regexp.Regexp, text string) []RegexMatch {
	indices := expression.FindAllStringSubmatchIndex(text, -1)
	matches := make([]RegexMatch, 0, len(indices))
	names := expression.SubexpNames()
	for _, indicesForMatch := range indices {
		captures := make([]RegexCapture, 0, len(indicesForMatch)/2-1)
		for index := 1; index < len(indicesForMatch)/2; index++ {
			start := indicesForMatch[index*2]
			end := indicesForMatch[index*2+1]
			matched := start >= 0
			captureText := ""
			if matched {
				captureText = text[start:end]
			}
			captures = append(captures, RegexCapture{Index: index, Name: names[index], Matched: matched, Text: captureText, StartByte: start, EndByte: end})
		}
		start := indicesForMatch[0]
		end := indicesForMatch[1]
		matches = append(matches, RegexMatch{Text: text[start:end], StartByte: start, EndByte: end, Captures: captures})
	}
	return matches
}

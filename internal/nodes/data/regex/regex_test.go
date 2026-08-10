package regex

import (
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

func TestMatchesPreserveStructuredUnicodeCaptures(t *testing.T) {
	expression, err := Compile(`(?P<letter>é)(?P<suffix>x)?`)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	matches := Matches(expression, "éx é")
	if err := typespec.ValidateValue(matches, MatchListType()); err != nil {
		t.Fatalf("ValidateValue() error = %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("match count = %d, want 2", len(matches))
	}
	if got, want := matches[0], (RegexMatch{
		Text: "éx", StartByte: 0, EndByte: 3,
		Captures: []RegexCapture{
			{Index: 1, Name: "letter", Matched: true, Text: "é", StartByte: 0, EndByte: 2},
			{Index: 2, Name: "suffix", Matched: true, Text: "x", StartByte: 2, EndByte: 3},
		},
	}); !reflect.DeepEqual(got, want) {
		t.Fatalf("first match = %#v, want %#v", got, want)
	}
	if got, want := matches[1].Captures[1], (RegexCapture{Index: 2, Name: "suffix", Matched: false, Text: "", StartByte: -1, EndByte: -1}); got != want {
		t.Fatalf("optional capture = %#v, want %#v", got, want)
	}
}

func TestCompileRejectsUnsupportedRE2Syntax(t *testing.T) {
	_, err := Compile(`word(?=\d)`)
	if err == nil || !strings.Contains(err.Error(), "compile RE2 pattern") {
		t.Fatalf("Compile() error = %v, want unsupported pattern error", err)
	}
}

func TestTextRejectsNonStringValues(t *testing.T) {
	_, err := Text(map[string]any{"pattern": 12}, "pattern")
	if err == nil || !strings.Contains(err.Error(), "pattern must be text") {
		t.Fatalf("Text() error = %v, want exact text error", err)
	}
}

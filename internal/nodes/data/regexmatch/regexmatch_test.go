package regexmatch

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regex"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

func TestRegisterReportsStrictMatchContract(t *testing.T) {
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	node, exists := registry.Get("data:regex_match")
	if !exists {
		t.Fatal("data:regex_match was not registered")
	}
	definition := node.Definition()
	if got := definition.Inputs[0]; !got.Required || got.Type == nil || got.Type.Kind != "string" {
		t.Fatalf("text input = %#v, want required Text", got)
	}
	if got := definition.Outputs[2]; got.Type == nil || !typespec.Assignable(*got.Type, regex.MatchListType()) || !typespec.Assignable(regex.MatchListType(), *got.Type) {
		t.Fatalf("matches output type = %#v, want list[RegexMatch]", got.Type)
	}
	if fields := definition.Outputs[2].Fields; len(fields) == 0 {
		t.Fatal("matches output must document its RegexMatch fields for the editor")
	}
}

func TestEvaluateExtractsRepeatedAndZeroWidthMatches(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		pattern string
		count   int
	}{
		{name: "repeated named captures", text: "a-1 b-2", pattern: `(?P<letter>\w)-(?P<digit>\d)`, count: 2},
		{name: "zero width", text: "text", pattern: `^`, count: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputs, err := evaluate(context.Background(), nodes.Invocation{Inputs: map[string]any{"text": test.text, "pattern": test.pattern}}, nil)
			if err != nil {
				t.Fatalf("evaluate() error = %v", err)
			}
			if got := outputs["count"]; got != test.count {
				t.Fatalf("count = %#v, want %d", got, test.count)
			}
			matches, ok := outputs["matches"].([]regex.RegexMatch)
			if !ok {
				t.Fatalf("matches = %T, want []regex.RegexMatch", outputs["matches"])
			}
			if err := typespec.ValidateValue(matches, regex.MatchListType()); err != nil {
				t.Fatalf("matches contract error = %v", err)
			}
		})
	}
}

func TestEvaluateReturnsNormalNoMatchAndRejectsInvalidPattern(t *testing.T) {
	outputs, err := evaluate(context.Background(), nodes.Invocation{Inputs: map[string]any{"text": "plain", "pattern": `\d+`}}, nil)
	if err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if outputs["matched"] != false || outputs["count"] != 0 || len(outputs["matches"].([]regex.RegexMatch)) != 0 {
		t.Fatalf("no-match outputs = %#v", outputs)
	}
	_, err = evaluate(context.Background(), nodes.Invocation{Inputs: map[string]any{"text": "plain", "pattern": `(?=p)`}}, nil)
	if err == nil || !strings.Contains(err.Error(), "compile RE2 pattern") {
		t.Fatalf("invalid pattern error = %v", err)
	}
}

func TestEvaluateRequiresTextInputs(t *testing.T) {
	_, err := evaluate(context.Background(), nodes.Invocation{Inputs: map[string]any{"pattern": `\d+`}}, nil)
	if err == nil || !strings.Contains(err.Error(), "text must be text") {
		t.Fatalf("missing text error = %v, want the text-input requirement", err)
	}
	_, err = evaluate(context.Background(), nodes.Invocation{Inputs: map[string]any{"text": "plain", "pattern": 42}}, nil)
	if err == nil || !strings.Contains(err.Error(), "pattern must be text") {
		t.Fatalf("non-string pattern error = %v, want the pattern-input requirement", err)
	}
}

func TestEvaluateReportsCaptureDetails(t *testing.T) {
	outputs, err := evaluate(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"text": "ab cd", "pattern": `(?P<letters>[a-z]+)(?P<digits>\d+)?`},
	}, nil)
	if err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if outputs["matched"] != true || outputs["count"] != 2 {
		t.Fatalf("outputs = %#v, want two matches", outputs)
	}
	want := []regex.RegexMatch{
		{
			Text: "ab", StartByte: 0, EndByte: 2,
			Captures: []regex.RegexCapture{
				{Index: 1, Name: "letters", Matched: true, Text: "ab", StartByte: 0, EndByte: 2},
				{Index: 2, Name: "digits", Matched: false, Text: "", StartByte: -1, EndByte: -1},
			},
		},
		{
			Text: "cd", StartByte: 3, EndByte: 5,
			Captures: []regex.RegexCapture{
				{Index: 1, Name: "letters", Matched: true, Text: "cd", StartByte: 3, EndByte: 5},
				{Index: 2, Name: "digits", Matched: false, Text: "", StartByte: -1, EndByte: -1},
			},
		},
	}
	if !reflect.DeepEqual(outputs["matches"], want) {
		t.Fatalf("matches = %#v, want %#v", outputs["matches"], want)
	}
}

func TestEvaluateCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := evaluate(ctx, nodes.Invocation{Inputs: map[string]any{"text": "plain", "pattern": "p"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "regex match cancelled") {
		t.Fatalf("evaluate(cancelled) error = %v, want the cancellation failure", err)
	}
}

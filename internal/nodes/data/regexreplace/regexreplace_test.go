package regexreplace

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestRegisterReportsStrictTextAndIntegerContracts(t *testing.T) {
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	node, exists := registry.Get("data:regex_replace")
	if !exists {
		t.Fatal("data:regex_replace was not registered")
	}
	definition := node.Definition()
	if got := definition.Inputs[2]; !got.Required || got.Type == nil || got.Type.Kind != "string" {
		t.Fatalf("replacement input = %#v, want required Text", got)
	}
	if got := definition.Outputs[1]; got.Type == nil || got.Type.Kind != "int" {
		t.Fatalf("replacements output = %#v, want Int", got)
	}
}

func TestEvaluateExpandsNamedAndNumericCaptures(t *testing.T) {
	outputs, err := evaluate(context.Background(), nodes.Invocation{Inputs: map[string]any{
		"text":        "Ada Lovelace",
		"pattern":     `(?P<first>\w+) (?P<last>\w+)`,
		"replacement": `${last}, $1`,
	}}, nil)
	if err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if got, want := outputs["text"], "Lovelace, Ada"; got != want {
		t.Fatalf("text = %#v, want %#v", got, want)
	}
	if outputs["replacements"] != 1 || outputs["changed"] != true {
		t.Fatalf("replacement metadata = %#v", outputs)
	}
}

func TestEvaluateReturnsOriginalTextForNoMatchAndRejectsInvalidPattern(t *testing.T) {
	outputs, err := evaluate(context.Background(), nodes.Invocation{Inputs: map[string]any{"text": "plain", "pattern": `\d+`, "replacement": "number"}}, nil)
	if err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if outputs["text"] != "plain" || outputs["replacements"] != 0 || outputs["changed"] != false {
		t.Fatalf("no-match outputs = %#v", outputs)
	}
	_, err = evaluate(context.Background(), nodes.Invocation{Inputs: map[string]any{"text": "plain", "pattern": `(?=p)`, "replacement": ""}}, nil)
	if err == nil || !strings.Contains(err.Error(), "compile RE2 pattern") {
		t.Fatalf("invalid pattern error = %v", err)
	}
}

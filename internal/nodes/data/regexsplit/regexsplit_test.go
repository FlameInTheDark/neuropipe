package regexsplit

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestRegisterReportsStrictStringListContract(t *testing.T) {
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	node, exists := registry.Get("data:regex_split")
	if !exists {
		t.Fatal("data:regex_split was not registered")
	}
	definition := node.Definition()
	if got := definition.Outputs[0]; got.Type == nil || got.Type.Kind != "list" || got.Type.Element == nil || got.Type.Element.Kind != "string" {
		t.Fatalf("parts output = %#v, want list[string]", got)
	}
}

func TestEvaluatePreservesEmptyLeadingAndTrailingParts(t *testing.T) {
	outputs, err := evaluate(context.Background(), nodes.Invocation{Inputs: map[string]any{"text": ",one,,two,", "pattern": ","}}, nil)
	if err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if got, want := outputs["parts"], []string{"", "one", "", "two", ""}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parts = %#v, want %#v", got, want)
	}
	if outputs["splits"] != 4 || outputs["matched"] != true {
		t.Fatalf("split metadata = %#v", outputs)
	}
}

func TestEvaluateReturnsOnePartForNoMatchAndRejectsInvalidPattern(t *testing.T) {
	outputs, err := evaluate(context.Background(), nodes.Invocation{Inputs: map[string]any{"text": "plain", "pattern": ","}}, nil)
	if err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if got, want := outputs["parts"], []string{"plain"}; !reflect.DeepEqual(got, want) || outputs["splits"] != 0 || outputs["matched"] != false {
		t.Fatalf("no-match outputs = %#v", outputs)
	}
	_, err = evaluate(context.Background(), nodes.Invocation{Inputs: map[string]any{"text": "plain", "pattern": `(?=p)`}}, nil)
	if err == nil || !strings.Contains(err.Error(), "compile RE2 pattern") {
		t.Fatalf("invalid pattern error = %v", err)
	}
}

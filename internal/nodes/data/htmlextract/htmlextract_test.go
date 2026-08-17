package htmlextract

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

const sample = `<!DOCTYPE html><html><head><title>Shop</title></head><body><h1 class="title">Widgets</h1><ul><li><a href="/a">Alpha</a></li><li><a href="/b">Beta</a></li></ul></body></html>`

func invocation(config map[string]any, html string) nodes.Invocation {
	return nodes.Invocation{Config: config, Inputs: map[string]any{"html": html}, Definition: definition()}
}

func domainFlowNode(config map[string]any) domain.FlowNode {
	return domain.FlowNode{Data: map[string]any{"config": config}}
}

func TestRegisterReportsExtractionContract(t *testing.T) {
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	node, exists := registry.Get("data:html_extract")
	if !exists {
		t.Fatal("data:html_extract was not registered")
	}
	resolved, err := node.Resolve(domainFlowNode(map[string]any{"extractions": []any{
		map[string]any{"id": "title", "label": "Title", "selector": "h1.title", "mode": "text", "attribute": "", "returnAll": false},
		map[string]any{"id": "links", "label": "Links", "selector": "ul li a", "mode": "attribute", "attribute": "href", "returnAll": true},
	}}))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(resolved.Outputs) != 2 {
		t.Fatalf("resolved outputs = %d, want 2", len(resolved.Outputs))
	}
	if got := resolved.Outputs[0]; got.DataType != domain.DataText || got.Type == nil || got.Type.Kind != domain.TypeString {
		t.Fatalf("title output = %#v, want Text", got)
	}
	if got := resolved.Outputs[1]; got.DataType != domain.DataList || got.Type == nil || got.Type.Kind != domain.TypeList {
		t.Fatalf("links output = %#v, want List", got)
	}
}

func TestEvaluateExtractsTextHtmlAndAttributes(t *testing.T) {
	config := map[string]any{"extractions": []any{
		map[string]any{"id": "title", "label": "Title", "selector": "h1.title", "mode": "text"},
		map[string]any{"id": "heading", "label": "Heading", "selector": "h1", "mode": "html"},
		map[string]any{"id": "firstHref", "label": "First href", "selector": "a", "mode": "attribute", "attribute": "href"},
		map[string]any{"id": "hrefs", "label": "All hrefs", "selector": "a", "mode": "attribute", "attribute": "href", "returnAll": true},
		map[string]any{"id": "names", "label": "Names", "selector": "li a", "mode": "text", "returnAll": true},
	}}
	outputs, err := Evaluate(context.Background(), invocation(config, sample), nil)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if got := outputs["title"]; got != "Widgets" {
		t.Fatalf("title = %#v, want Widgets", got)
	}
	heading, _ := outputs["heading"].(string)
	if !strings.Contains(heading, `<h1 class="title">Widgets</h1>`) {
		t.Fatalf("heading = %#v, want rendered h1 markup", outputs["heading"])
	}
	if got := outputs["firstHref"]; got != "/a" {
		t.Fatalf("firstHref = %#v, want /a", got)
	}
	if got := outputs["hrefs"]; len(got.([]any)) != 2 || got.([]any)[1] != "/b" {
		t.Fatalf("hrefs = %#v, want [/a /b]", got)
	}
	if got := outputs["names"]; len(got.([]any)) != 2 || got.([]any)[0] != "Alpha" {
		t.Fatalf("names = %#v, want [Alpha Beta]", got)
	}
	element := domain.TypeSpec{Kind: domain.TypeString}
	if err := typespec.ValidateValue(outputs["hrefs"], domain.TypeSpec{Kind: domain.TypeList, Element: &element}); err != nil {
		t.Fatalf("hrefs contract error = %v", err)
	}
}

func TestEvaluateReturnsEmptyForMissingMatches(t *testing.T) {
	config := map[string]any{"extractions": []any{
		map[string]any{"id": "missing", "label": "Missing", "selector": ".nope", "mode": "text"},
		map[string]any{"id": "missingList", "label": "Missing list", "selector": ".nope", "mode": "text", "returnAll": true},
	}}
	outputs, err := Evaluate(context.Background(), invocation(config, sample), nil)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if got := outputs["missing"]; got != "" {
		t.Fatalf("missing = %#v, want empty text", got)
	}
	if got := outputs["missingList"]; len(got.([]any)) != 0 {
		t.Fatalf("missingList = %#v, want empty list", got)
	}
}

func TestEvaluateRejectsInvalidSelectorAndConfig(t *testing.T) {
	badSelector := map[string]any{"extractions": []any{map[string]any{"id": "a", "selector": "h1 >>", "mode": "text"}}}
	if _, err := Evaluate(context.Background(), invocation(badSelector, sample), nil); err == nil || !strings.Contains(err.Error(), "invalid CSS selector") {
		t.Fatalf("invalid selector error = %v", err)
	}
	missingAttribute := map[string]any{"extractions": []any{map[string]any{"id": "a", "selector": "a", "mode": "attribute"}}}
	if _, err := Evaluate(context.Background(), invocation(missingAttribute, sample), nil); err == nil || !strings.Contains(err.Error(), "attribute") {
		t.Fatalf("missing attribute error = %v", err)
	}
	duplicate := map[string]any{"extractions": []any{map[string]any{"id": "a", "selector": "a"}, map[string]any{"id": "a", "selector": "b"}}}
	if _, err := Evaluate(context.Background(), invocation(duplicate, sample), nil); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate error = %v", err)
	}
	if _, err := Evaluate(context.Background(), invocation(map[string]any{"extractions": []any{}}, sample), nil); err == nil {
		t.Fatal("empty extractions must error")
	}
}

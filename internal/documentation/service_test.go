package docs

import (
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
)

func TestEmbeddedDocumentationCoversCoreCatalog(t *testing.T) {
	service, err := New(nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	definitions := catalog.New().All()
	types := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Source == "builtin" {
			types = append(types, definition.Type)
		}
	}
	if err := service.ValidateNodeCoverage(types); err != nil {
		t.Fatal(err)
	}
}

func TestEmbeddedDocumentationSearchAndNodeLookup(t *testing.T) {
	service, err := New(nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	results, err := service.Search("en", "timing-safe")
	if err != nil || len(results) == 0 {
		t.Fatalf("Search() = %#v, %v; want a local match", results, err)
	}
	reference, err := service.ForNode("action:terminal")
	if err != nil {
		t.Fatalf("ForNode() error = %v", err)
	}
	if reference.DocumentID != "node:action:terminal" || reference.Anchor != "" {
		t.Fatalf("ForNode() = %#v", reference)
	}
	document, err := service.Get("en", reference.DocumentID)
	if err != nil || !strings.Contains(document.Markdown, "Run Terminal Command") {
		t.Fatalf("Get() = %#v, %v", document, err)
	}
}

func TestGetFieldDocumentationDescribesItsDynamicOutputs(t *testing.T) {
	service, err := New(nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	document, err := service.Get("en", "node:data:get_field")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !strings.Contains(document.Markdown, "**Value** (`value`) — data · any") || !strings.Contains(document.Markdown, "Each row in **Outputs** creates one typed data pin") {
		t.Fatalf("Get Field documentation does not describe dynamic output pins:\n%s", document.Markdown)
	}
}

func TestSwitchDocumentationDescribesTypedOrderedControlFlow(t *testing.T) {
	t.Parallel()
	service, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	document, err := service.Get("en", "node:flow:switch")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"**Value** (`selection`) — data · any", "first matching case", "never converts"} {
		if !strings.Contains(document.Markdown, value) {
			t.Fatalf("Switch documentation is missing %q", value)
		}
	}
	localizedMarkers := map[string]string{
		"de": "ersten passenden Fall",
		"fr": "premier cas correspondant",
		"ru": "первый совпавший",
	}
	for language, marker := range localizedMarkers {
		localized, err := service.Get(language, "node:flow:switch")
		if err != nil {
			t.Fatalf("Get(%q) error = %v", language, err)
		}
		if !strings.Contains(localized.Markdown, marker) {
			t.Fatalf("%s Switch documentation did not use its embedded translation", language)
		}
	}
}

func TestLocalizedDocumentationUsesEmbeddedTranslationAndEnglishFallback(t *testing.T) {
	t.Parallel()
	service, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	translated, err := service.Get("ru", "getting-started/overview")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(translated.Markdown, "Обзор Neuropipe") {
		t.Fatalf("Russian overview was not selected: %q", translated.Markdown)
	}
	if translated.Title != "Обзор Neuropipe" {
		t.Fatalf("translated title = %q, want the localized Markdown title", translated.Title)
	}
	if !strings.Contains(translated.Summary, "локальное рабочее пространство") {
		t.Fatalf("translated summary = %q, want localized Markdown prose", translated.Summary)
	}
	fallback, err := service.Get("de", "concepts/chat")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fallback.Markdown, "# Chat") {
		t.Fatalf("missing translations must fall back to English, got %q", fallback.Markdown)
	}
}

func TestWebhookDocumentationDescribesTheRuntimeContractInEveryLocale(t *testing.T) {
	t.Parallel()
	service, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	english, err := service.Get("en", "concepts/api-webhooks")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"POST request", "X-Neuropipe-Signature", "202 Accepted", "exact raw request body"} {
		if !strings.Contains(english.Markdown, value) {
			t.Fatalf("English webhook documentation is missing %q", value)
		}
	}
	localizedMarkers := map[string]string{
		"de": "Der Local-Webhook-Event",
		"fr": "L'événement Local Webhook",
		"ru": "Событие Local Webhook",
	}
	for language, marker := range localizedMarkers {
		document, err := service.Get(language, "node:trigger:webhook")
		if err != nil {
			t.Fatalf("Get(%q) error = %v", language, err)
		}
		if !strings.Contains(document.Markdown, marker) || !strings.Contains(document.Markdown, "X-Neuropipe-Signature") {
			t.Fatalf("%s Local Webhook documentation fell back or omitted signature guidance", language)
		}
	}
}

func TestPluginGuidesDescribeTheSupportedBoundaryAndQuickStart(t *testing.T) {
	t.Parallel()
	service, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	system, err := service.Get("en", "extensions/plugin-system")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"Current v1 boundary", "does **not yet** launch a plugin sidecar", "HashiCorp go-plugin", "1 MiB"} {
		if !strings.Contains(system.Markdown, value) {
			t.Fatalf("Plugin system documentation is missing %q", value)
		}
	}
	quickStart, err := service.Get("en", "extensions/first-plugin")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"Rediscover plugins", "No Library node", "convert-temperature", "classify-temperature", "does **not** launch a sidecar"} {
		if !strings.Contains(quickStart.Markdown, value) {
			t.Fatalf("Plugin quick-start documentation is missing %q", value)
		}
	}
	translations := map[string]string{
		"de": "Plugin-System",
		"fr": "Système de plug-ins",
		"ru": "Система плагинов",
	}
	for language, marker := range translations {
		document, err := service.Get(language, "extensions/plugin-system")
		if err != nil {
			t.Fatalf("Get(%q) error = %v", language, err)
		}
		if !strings.Contains(document.Markdown, marker) {
			t.Fatalf("%s plugin documentation did not use its embedded translation", language)
		}
	}
}

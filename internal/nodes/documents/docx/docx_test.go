package docx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

// invoke runs a node executor with an empty runtime. Input keys inside the
// reserved "pin_" namespace are marked as connected: in the engine only wires
// can deliver those keys, so the harness mirrors that provenance.
func invoke(t *testing.T, register func(nodes.Registrar) error, inputs map[string]any, config map[string]any) (nodes.ExecutionResult, error) {
	t.Helper()
	var module nodes.Node
	registrar := registrarFunc(func(node nodes.Node) error {
		module = node
		return nil
	})
	if err := register(registrar); err != nil {
		t.Fatalf("register: %v", err)
	}
	connected := map[string]bool{}
	for key := range inputs {
		if strings.HasPrefix(key, "pin_") {
			connected[key] = true
		}
	}
	return module.Execute(context.Background(), nodes.Invocation{
		Node:            domain.FlowNode{Type: module.Definition().Type, Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: connected,
	}, nil)
}

// registrarFunc adapts a closure to the nodes.Registrar interface.
type registrarFunc func(nodes.Node) error

func (f registrarFunc) Register(node nodes.Node) error { return f(node) }

func documentPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

func TestCreateReadRoundTrip(t *testing.T) {
	path := documentPath(t, "created.docx")
	result, err := invoke(t, RegisterCreate,
		map[string]any{"path": path, "title": "Weekly report", "content": "First line.\n\nThird line."},
		nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.Outputs["path"] != path || result.Outputs["paragraphs"] != 4.0 {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("document not written: %v", err)
	}

	read, err := invoke(t, RegisterReadText, map[string]any{"path": path}, nil)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text, _ := read.Outputs["text"].(string)
	if !strings.Contains(text, "Weekly report") || !strings.Contains(text, "First line.") {
		t.Fatalf("text = %q", text)
	}
	if !strings.Contains(text, "First line.\n\nThird line.") {
		t.Fatalf("empty middle paragraph lost: %q", text)
	}
	if read.Outputs["paragraphs"] != 4.0 {
		t.Fatalf("paragraphs = %#v", read.Outputs["paragraphs"])
	}
}

func TestCreateRejectsEmptyContentAndBadExtension(t *testing.T) {
	if _, err := invoke(t, RegisterCreate, map[string]any{"path": documentPath(t, "a.docx"), "content": ""}, nil); err == nil {
		t.Fatal("empty content must fail")
	}
	if _, err := invoke(t, RegisterCreate, map[string]any{"path": documentPath(t, "a.txt"), "content": "x"}, nil); err == nil {
		t.Fatal("non .docx path must fail")
	}
}

func TestReadTextParsesTablesAndBreaks(t *testing.T) {
	document := newDocument()
	document.setBody([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:r><w:t>Intro</w:t></w:r></w:p>
<w:tbl><w:tr><w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc></w:tr>
<w:tr><w:tc><w:p><w:r><w:t>C</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>D</w:t></w:r></w:p></w:tc></w:tr></w:tbl>
<w:p><w:r><w:t>line one</w:t></w:r><w:br/><w:r><w:t>line two</w:t></w:r></w:p>
</w:body></w:document>`))
	path := documentPath(t, "table.docx")
	if err := writeDocument(path, document); err != nil {
		t.Fatalf("write: %v", err)
	}
	read, err := invoke(t, RegisterReadText, map[string]any{"path": path}, nil)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text, _ := read.Outputs["text"].(string)
	if !strings.Contains(text, "Intro") {
		t.Fatalf("missing intro: %q", text)
	}
	if !strings.Contains(text, "A\tB\nC\tD") {
		t.Fatalf("table not flattened: %q", text)
	}
	if !strings.Contains(text, "line one\nline two") {
		t.Fatalf("break not preserved: %q", text)
	}
}

func TestTemplateFillMergesSplitRuns(t *testing.T) {
	document := newDocument()
	// Word frequently splits placeholders across runs; the engine merges each
	// paragraph before substituting.
	document.setBody([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">Hello {{na</w:t></w:r><w:r><w:t>me}}!</w:t></w:r></w:p>
</w:body></w:document>`))
	path := documentPath(t, "template.docx")
	if err := writeDocument(path, document); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := invoke(t, RegisterTemplateFill,
		map[string]any{"templatePath": path, "values": map[string]any{"name": "Ada"}},
		nil)
	if err != nil {
		t.Fatalf("fill: %v", err)
	}
	filled, _ := result.Outputs["path"].(string)
	if filled != strings.TrimSuffix(path, ".docx")+"-filled.docx" {
		t.Fatalf("path = %q", filled)
	}
	if result.Outputs["replacements"] != 1.0 {
		t.Fatalf("replacements = %#v", result.Outputs["replacements"])
	}

	// The template itself stays untouched and the filled document carries the
	// substituted text plus the original paragraph properties.
	filledDoc, err := readDocument(filled)
	if err != nil {
		t.Fatalf("open filled: %v", err)
	}
	text, _, err := filledDoc.text()
	if err != nil {
		t.Fatalf("read filled: %v", err)
	}
	if text != "Hello Ada!" {
		t.Fatalf("text = %q", text)
	}
	if !strings.Contains(string(filledDoc.body()), `<w:jc w:val="center"/>`) {
		t.Fatalf("paragraph properties lost: %s", filledDoc.body())
	}
	if !strings.Contains(string(filledDoc.body()), "<w:b/>") {
		t.Fatalf("run formatting lost: %s", filledDoc.body())
	}
	source, err := readDocument(path)
	if err != nil {
		t.Fatalf("reopen template: %v", err)
	}
	sourceText, _, _ := source.text()
	if sourceText != "Hello {{name}}!" {
		t.Fatalf("template mutated: %q", sourceText)
	}
}

func TestTemplateFillKeepsUnknownPlaceholdersAndValidates(t *testing.T) {
	document := newDocument()
	document.setBody([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>{{known}} {{unknown}}</w:t></w:r></w:p></w:body></w:document>`))
	path := documentPath(t, "keep.docx")
	if err := writeDocument(path, document); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := invoke(t, RegisterTemplateFill,
		map[string]any{"templatePath": path, "values": map[string]any{"known": "x"}, "outputPath": documentPath(t, "out.docx")},
		nil)
	if err != nil {
		t.Fatalf("fill: %v", err)
	}
	filled, _ := readDocument(result.Outputs["path"].(string))
	text, _, _ := filled.text()
	if text != "x {{unknown}}" {
		t.Fatalf("text = %q", text)
	}

	if _, err := invoke(t, RegisterTemplateFill, map[string]any{"templatePath": path, "values": map[string]any{}}, nil); err == nil {
		t.Fatal("empty values must fail")
	}
	if _, err := invoke(t, RegisterTemplateFill, map[string]any{"templatePath": path}, nil); err == nil {
		t.Fatal("missing values must fail")
	}
}

func TestTemplateFillValuePins(t *testing.T) {
	document := newDocument()
	document.setBody([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Dear {{customer}}, your total is {{amount}}.</w:t></w:r></w:p></w:body></w:document>`))
	path := documentPath(t, "pins.docx")
	if err := writeDocument(path, document); err != nil {
		t.Fatalf("write: %v", err)
	}

	// The resolver turns configured rows into input pins under stable IDs.
	config := map[string]any{
		"templatePath": path,
		"outputPath":   documentPath(t, "pins-out.docx"),
		"valuePins": []any{
			map[string]any{"id": "field_1", "name": "customer", "label": "Customer"},
			map[string]any{"id": "field_2", "name": "amount", "value": 42},
		},
	}
	var module nodes.Node
	registrar := registrarFunc(func(node nodes.Node) error { module = node; return nil })
	if err := RegisterTemplateFill(registrar); err != nil {
		t.Fatalf("register: %v", err)
	}
	resolved, err := module.Resolve(domain.FlowNode{Type: "action:word_template_fill", Data: map[string]any{"config": config}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	var pin bool
	for _, input := range resolved.Inputs {
		if input.ID == "pin_field_1" && input.Label == "Customer" && input.Kind == domain.PinData {
			pin = true
		}
	}
	if !pin {
		t.Fatalf("resolved inputs miss the value pin: %#v", resolved.Inputs)
	}

	// A wired text value (the LLM output case) plus a literal fallback fill
	// the whole template without any Values object.
	result, err := invoke(t, RegisterTemplateFill, map[string]any{
		"templatePath": path,
		"outputPath":   config["outputPath"],
		"pin_field_1":  "Contoso",
	}, config)
	if err != nil {
		t.Fatalf("fill via pins: %v", err)
	}
	// The replacement counter tracks rewritten paragraphs: both placeholders
	// live in one paragraph, so one rewrite is expected.
	if result.Outputs["replacements"] != 1.0 {
		t.Fatalf("replacements = %#v", result.Outputs["replacements"])
	}
	filled, err := readDocument(result.Outputs["path"].(string))
	if err != nil {
		t.Fatalf("open filled: %v", err)
	}
	text, _, err := filled.text()
	if err != nil {
		t.Fatalf("read filled: %v", err)
	}
	if text != "Dear Contoso, your total is 42." {
		t.Fatalf("text = %q", text)
	}

	// Pin values override the Values object for the same placeholder while
	// object-only placeholders keep flowing.
	merged, err := invoke(t, RegisterTemplateFill, map[string]any{
		"templatePath": path,
		"outputPath":   config["outputPath"],
		"pin_field_1":  "Litware",
		"values":       map[string]any{"customer": "ignored", "amount": 9},
	}, config)
	if err != nil {
		t.Fatalf("fill merged: %v", err)
	}
	mergedDoc, err := readDocument(merged.Outputs["path"].(string))
	if err != nil {
		t.Fatalf("open merged: %v", err)
	}
	mergedText, _, _ := mergedDoc.text()
	if mergedText != "Dear Litware, your total is 9." {
		t.Fatalf("merged text = %q", mergedText)
	}

	// Literal-only rows still fill their placeholders without any wire; the
	// unwired, literal-less customer row leaves its placeholder visible.
	literalOnly, err := invoke(t, RegisterTemplateFill, map[string]any{
		"templatePath": path, "outputPath": config["outputPath"],
	}, config)
	if err != nil {
		t.Fatalf("literal-only fill: %v", err)
	}
	literalDoc, err := readDocument(literalOnly.Outputs["path"].(string))
	if err != nil {
		t.Fatalf("open literal-only: %v", err)
	}
	literalText, _, _ := literalDoc.text()
	if literalText != "Dear {{customer}}, your total is 42." {
		t.Fatalf("literal-only text = %q", literalText)
	}

	// Nothing at all — no object, no wires, no literals — fails like the
	// old empty Values object.
	bare := map[string]any{
		"templatePath": path, "outputPath": config["outputPath"],
		"valuePins": []any{map[string]any{"id": "field_1", "name": "customer"}},
	}
	if _, err := invoke(t, RegisterTemplateFill, map[string]any{
		"templatePath": path, "outputPath": config["outputPath"],
	}, bare); err == nil {
		t.Fatal("no values, no wired pins, and no literals must fail")
	}
}

func TestAppendPreservesSectionProperties(t *testing.T) {
	path := documentPath(t, "append.docx")
	if _, err := invoke(t, RegisterCreate, map[string]any{"path": path, "content": "First."}, nil); err != nil {
		t.Fatalf("create: %v", err)
	}
	result, err := invoke(t, RegisterAppendText, map[string]any{"path": path, "text": "Second.\n\nThird."}, nil)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if result.Outputs["paragraphs"] != 3.0 {
		t.Fatalf("paragraphs = %#v", result.Outputs["paragraphs"])
	}
	read, err := invoke(t, RegisterReadText, map[string]any{"path": path}, nil)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text, _ := read.Outputs["text"].(string)
	if !strings.HasPrefix(text, "First.") || !strings.HasSuffix(text, "Second.\n\nThird.") {
		t.Fatalf("text = %q", text)
	}
	if _, err := invoke(t, RegisterAppendText, map[string]any{"path": path, "text": "  "}, nil); err == nil {
		t.Fatal("empty text must fail")
	}
}

func TestReplaceTextRewritesAndCounts(t *testing.T) {
	path := documentPath(t, "replace.docx")
	if _, err := invoke(t, RegisterCreate, map[string]any{"path": path, "content": "draft one\ndraft two\nfinal"}, nil); err != nil {
		t.Fatalf("create: %v", err)
	}
	result, err := invoke(t, RegisterReplaceText,
		map[string]any{"path": path, "find": "draft", "replacement": "final"}, nil)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if result.Outputs["changed"] != 2.0 {
		t.Fatalf("changed = %#v", result.Outputs["changed"])
	}
	read, err := invoke(t, RegisterReadText, map[string]any{"path": path}, nil)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if text, _ := read.Outputs["text"].(string); text != "final one\nfinal two\nfinal" {
		t.Fatalf("text = %q", text)
	}
	if _, err := invoke(t, RegisterReplaceText, map[string]any{"path": path, "find": "", "replacement": "x"}, nil); err == nil {
		t.Fatal("empty find must fail")
	}
}

func TestFillPlaceholdersWritesMultilineValues(t *testing.T) {
	document := newDocument()
	document.setBody([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>{{body}}</w:t></w:r></w:p></w:body></w:document>`))
	path := documentPath(t, "multi.docx")
	if err := writeDocument(path, document); err != nil {
		t.Fatalf("write: %v", err)
	}
	multi, err := invoke(t, RegisterTemplateFill,
		map[string]any{"templatePath": path, "values": map[string]any{"body": "one\ntwo"}, "outputPath": documentPath(t, "multi-out.docx")},
		nil)
	if err != nil {
		t.Fatalf("fill: %v", err)
	}
	filled, err := readDocument(multi.Outputs["path"].(string))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !strings.Contains(string(filled.body()), "<w:br/>") {
		t.Fatalf("newline was not written as a break: %s", filled.body())
	}
}

func TestReadRejectsNonDocxArchive(t *testing.T) {
	path := documentPath(t, "fake.docx")
	if err := os.WriteFile(path, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := invoke(t, RegisterReadText, map[string]any{"path": path}, nil); err == nil {
		t.Fatal("non-zip docx must fail")
	}
}

func TestEngineHelpers(t *testing.T) {
	if got := stringifyValue(42.5); got != "42.5" {
		t.Fatalf("stringifyValue(42.5) = %q", got)
	}
	if got := stringifyValue(42.0); got != "42" {
		t.Fatalf("stringifyValue(42.0) = %q", got)
	}
	if got := stringifyValue(true); got != "true" {
		t.Fatalf("stringifyValue(true) = %q", got)
	}
	if got := expandPlaceholders("no placeholders", map[string]string{"a": "b"}); got != "no placeholders" {
		t.Fatalf("expandPlaceholders = %q", got)
	}
	if got := expandPlaceholders("a {{x}} b", nil); got != "a {{x}} b" {
		t.Fatalf("expandPlaceholders unknown = %q", got)
	}
}

package docx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/documents/dynpins"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops"
)

const wordColor = "#2b579a"

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// register contributes one complete node implementation to the registry.
func register(registrar nodes.Registrar, definition domain.NodeDefinition, executor func(context.Context, nodes.Invocation, nodes.Runtime) (nodes.ExecutionResult, error)) error {
	return registrar.Register(Node{Metadata: definition, Executor: executor})
}

// registerResolved additionally contributes a dynamic port resolver so the
// node's configuration-driven value pins appear in the editor, validator, and
// engine under stable IDs.
func registerResolved(registrar nodes.Registrar, definition domain.NodeDefinition, resolver func(domain.FlowNode) (domain.NodeDefinition, error), executor func(context.Context, nodes.Invocation, nodes.Runtime) (nodes.ExecutionResult, error)) error {
	return registrar.Register(Node{Metadata: definition, Resolver: resolver, Executor: executor})
}

// definition assembles the common NodeDefinition skeleton for Word nodes.
func definition(nodeType, label, description, icon string, inputs []domain.NodePort, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any, capabilities ...domain.Capability) domain.NodeDefinition {
	if defaults == nil {
		defaults = map[string]any{}
	}
	return domain.NodeDefinition{
		Type: nodeType, Category: "Documents", Label: label, Description: description,
		Icon: icon, Color: wordColor, Mode: domain.NodeImpure, PortContractOwned: true,
		Capabilities:  capabilities,
		Inputs:        inputs,
		Outputs:       outputs,
		Fields:        fields,
		DefaultConfig: defaults,
		Source:        "builtin",
	}
}

/* ---------------- Pin builders ---------------- */

func execPin(id, label string, direction domain.PinDirection) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: direction, Color: "#fafafa", MaxConnections: 1}
}

func textPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	pinType := domain.TypeSpec{Kind: domain.TypeString}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataText, Type: &pinType, Color: "#e879f9", Required: required, MaxConnections: 1}
}

func numberPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	pinType := domain.TypeSpec{Kind: domain.TypeFloat}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataNumber, Type: &pinType, Color: "#86efac", Required: required, MaxConnections: 1}
}

func objectPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	keyType := domain.TypeSpec{Kind: domain.TypeString}
	valueType := domain.TypeSpec{Kind: domain.TypeAny}
	pinType := domain.TypeSpec{Kind: domain.TypeMap, Key: &keyType, Value: &valueType}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataObject, Type: &pinType, Color: "#60a5fa", Required: required, MaxConnections: 1}
}

func thenPin() domain.NodePort { return execPin("out", "Then", domain.PinOutput) }

/* ---------------- Invocation helpers ---------------- */

// docxPath validates and cleans a .docx file path input.
func docxPath(invocation nodes.Invocation, name string) (string, error) {
	raw, _ := invocation.Inputs[name].(string)
	path, err := fileops.CleanPath(raw)
	if err != nil {
		return "", fmt.Errorf("%s is required", name)
	}
	if !strings.EqualFold(filepath.Ext(path), ".docx") {
		return "", fmt.Errorf("%s must point to a .docx document", name)
	}
	return path, nil
}

// textValue reads a trimmed string from a connected pin or the inspector.
func textValue(invocation nodes.Invocation, name string) string {
	raw, _ := invocation.Inputs[name].(string)
	return strings.TrimSpace(raw)
}

// readDocument opens a .docx file from disk.
func readDocument(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read document: %w", err)
	}
	return readDocx(data)
}

// writeDocument persists a document to disk, creating parent directories.
func writeDocument(path string, document *Document) error {
	data, err := document.writeDocx()
	if err != nil {
		return err
	}
	if err := fileops.EnsureParentDir(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write document: %w", err)
	}
	return nil
}

// valuesInput reads the placeholder values object from a connected pin or
// the inspector's key/value editor. Values are stringified for substitution.
// An absent or empty object is not an error: dynamic value pins may carry the
// whole payload, so the caller combines both sources before validating.
func valuesInput(invocation nodes.Invocation) (map[string]string, error) {
	raw, exists := invocation.Inputs["values"]
	if !exists || raw == nil {
		return map[string]string{}, nil
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("values must be an object of placeholder values")
	}
	values := make(map[string]string, len(object))
	for key, value := range object {
		values[strings.TrimSpace(key)] = stringifyValue(value)
	}
	return values, nil
}

// stringifyValue renders a wired or configured value as template text.
func stringifyValue(value any) string {
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
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return fmt.Sprint(typed)
	}
}

/* ------------------------------------------------------------------ */
/* Read Word Text                                                     */
/* ------------------------------------------------------------------ */

// RegisterReadText contributes the Read Word Text node.
func RegisterReadText(registrar nodes.Registrar) error {
	definition := definition("action:word_read_text", "Read Word Text",
		"Extract the visible text of a Word document: paragraphs, line breaks, and tables.",
		"file-text",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
		},
		[]domain.NodePort{
			thenPin(),
			textPin("text", "Text", domain.PinOutput, false),
			numberPin("paragraphs", "Paragraphs", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\report.docx", Required: true},
		},
		map[string]any{},
		domain.CapabilityFileRead,
	)
	return register(registrar, definition, executeReadText)
}

func executeReadText(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read word text cancelled: %w", err)
	}
	path, err := docxPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	document, err := readDocument(path)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	text, paragraphs, err := document.text()
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"text": text, "paragraphs": float64(paragraphs)}, Ports: []string{"out"}}, nil
}

/* ------------------------------------------------------------------ */
/* Create Word Document — bold title plus one paragraph per line    */
/* ------------------------------------------------------------------ */

// RegisterCreate contributes the Create Word Document node.
func RegisterCreate(registrar nodes.Registrar) error {
	definition := definition("action:word_create", "Create Word Document",
		"Create a new Word document from a bold title and one paragraph per text line.",
		"file-output",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
			textPin("title", "Title", domain.PinInput, false),
			textPin("content", "Content", domain.PinInput, true),
		},
		[]domain.NodePort{
			thenPin(),
			textPin("path", "Path", domain.PinOutput, false),
			numberPin("paragraphs", "Paragraphs", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\report.docx", Required: true},
			{Name: "title", Label: "Title", Kind: "string", Placeholder: "Weekly report"},
			{Name: "content", Label: "Content", Kind: "textarea", Placeholder: "One paragraph per line.", Required: true},
		},
		map[string]any{"title": "", "content": ""},
		domain.CapabilityFileWrite,
	)
	return register(registrar, definition, executeCreate)
}

func executeCreate(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create word document cancelled: %w", err)
	}
	path, err := docxPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	title := textValue(invocation, "title")
	content, _ := invocation.Inputs["content"].(string)
	if strings.TrimSpace(content) == "" && title == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("content is required")
	}
	document := newDocument()
	document.setBody(buildBody(title, content))
	paragraphs := len(splitLines(content))
	if title != "" {
		paragraphs++
	}
	if err := writeDocument(path, document); err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"path": path, "paragraphs": float64(paragraphs)}, Ports: []string{"out"}}, nil
}

// buildBody renders the document body: an optional bold 16pt title followed
// by one paragraph per content line.
func buildBody(title, content string) []byte {
	var body strings.Builder
	if title != "" {
		body.WriteString(`<w:p><w:r><w:rPr><w:b/><w:sz w:val="32"/></w:rPr>`)
		body.WriteString(`<w:t xml:space="preserve">`)
		body.WriteString(xmlEscape(title))
		body.WriteString(`</w:t></w:r></w:p>`)
	}
	for _, line := range splitLines(content) {
		body.WriteString(paragraphXMLFor(line))
	}
	return []byte(documentOpen + body.String() + documentClose)
}

const documentOpen = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`

const documentClose = `</w:body></w:document>`

/* ------------------------------------------------------------------ */
/* Populate Word Template — fill {{placeholders}} from values/pins   */
/* ------------------------------------------------------------------ */

// RegisterTemplateFill contributes the Populate Word Template node.
func RegisterTemplateFill(registrar nodes.Registrar) error {
	return registerResolved(registrar, templateFillDefinition(), resolveTemplateFill, executeTemplateFill)
}

// templateFillDefinition is the static contract of the template fill node.
// The Values object pin is optional because dynamic value pins can carry the
// whole payload instead.
func templateFillDefinition() domain.NodeDefinition {
	return definition("action:word_template_fill", "Populate Word Template",
		"Fill {{placeholder}} fields of a Word template with an object of values and save the result as a new document.",
		"file-input",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("templatePath", "Template", domain.PinInput, true),
			textPin("outputPath", "Output path", domain.PinInput, false),
			objectPin("values", "Values", domain.PinInput, false),
		},
		[]domain.NodePort{
			thenPin(),
			textPin("path", "Path", domain.PinOutput, false),
			numberPin("replacements", "Replacements", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "templatePath", Label: "Template", Kind: "string", Placeholder: "C:\\Work\\template.docx", Required: true},
			{Name: "outputPath", Label: "Output path", Kind: "string", Placeholder: "C:\\Work\\filled.docx (empty writes beside the template)"},
			{Name: "values", Label: "Values", Kind: "kv-hash-fields", Placeholder: `customer = Contoso`},
			{Name: "valuePins", Label: "Value pins", Kind: "pin-bindings", Placeholder: `customer`},
		},
		map[string]any{"templatePath": "", "outputPath": "", "values": map[string]any{}, "valuePins": []any{}},
		domain.CapabilityFileRead, domain.CapabilityFileWrite,
	)
}

// resolveTemplateFill expands the configured value pins into data input
// pins. Each pin feeds one {{placeholder}}: a wired value wins, the row's
// literal is the fallback, and pin values override the Values object for the
// same placeholder.
func resolveTemplateFill(node domain.FlowNode) (domain.NodeDefinition, error) {
	result := templateFillDefinition()
	rows, err := dynpins.Configured(configOf(node), "valuePins")
	if err != nil {
		return result, err
	}
	result.Inputs = append(result.Inputs, dynpins.InputPins(rows, "#a1a1aa")...)
	return result, nil
}

// templateValues combines the Values object and the dynamic value pins into
// one substitution map. A wired pin overrides the object entry for the same
// placeholder; a row literal only fills placeholders the object leaves open.
func templateValues(invocation nodes.Invocation) (map[string]string, error) {
	values, err := valuesInput(invocation)
	if err != nil {
		return nil, err
	}
	rows, err := dynpins.Configured(invocation.Config, "valuePins")
	if err != nil {
		return nil, err
	}
	for name, value := range dynpins.WiredValues(invocation, rows) {
		values[name] = stringifyValue(value)
	}
	for name, value := range dynpins.FallbackLiterals(rows) {
		if _, known := values[name]; !known {
			values[name] = stringifyValue(value)
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("at least one placeholder value or value pin is required")
	}
	return values, nil
}

// configOf reads the persisted node configuration map.
func configOf(node domain.FlowNode) map[string]any {
	if config, ok := node.Data["config"].(map[string]any); ok {
		return config
	}
	return map[string]any{}
}

func executeTemplateFill(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("populate word template cancelled: %w", err)
	}
	templatePath, err := docxPath(invocation, "templatePath")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	values, err := templateValues(invocation)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	document, err := readDocument(templatePath)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	replacements, err := document.fillPlaceholders(values)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}

	outputPath := textValue(invocation, "outputPath")
	if outputPath == "" {
		extension := filepath.Ext(templatePath)
		outputPath = strings.TrimSuffix(templatePath, extension) + "-filled" + extension
	} else {
		outputPath, err = fileops.CleanPath(outputPath)
		if err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("output path is required")
		}
		if !strings.EqualFold(filepath.Ext(outputPath), ".docx") {
			return nodes.ExecutionResult{}, fmt.Errorf("output path must point to a .docx document")
		}
	}
	if err := writeDocument(outputPath, document); err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"path": outputPath, "replacements": float64(replacements)}, Ports: []string{"out"}}, nil
}

/* ------------------------------------------------------------------ */
/* Append to Word Document                                            */
/* ------------------------------------------------------------------ */

// RegisterAppendText contributes the Append to Word Document node.
func RegisterAppendText(registrar nodes.Registrar) error {
	definition := definition("action:word_append_text", "Append to Word Document",
		"Append one paragraph per text line to an existing Word document.",
		"file-down",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
			textPin("text", "Text", domain.PinInput, true),
		},
		[]domain.NodePort{
			thenPin(),
			textPin("path", "Path", domain.PinOutput, false),
			numberPin("paragraphs", "Paragraphs", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\report.docx", Required: true},
			{Name: "text", Label: "Text", Kind: "textarea", Placeholder: "One paragraph per line.", Required: true},
		},
		map[string]any{"text": ""},
		domain.CapabilityFileRead, domain.CapabilityFileWrite,
	)
	return register(registrar, definition, executeAppendText)
}

func executeAppendText(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("append word text cancelled: %w", err)
	}
	path, err := docxPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	text, _ := invocation.Inputs["text"].(string)
	if strings.TrimSpace(text) == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("text is required")
	}
	document, err := readDocument(path)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	lines := splitLines(text)
	document.appendParagraphs(lines)
	if err := writeDocument(path, document); err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"path": path, "paragraphs": float64(len(lines))}, Ports: []string{"out"}}, nil
}

/* ------------------------------------------------------------------ */
/* Replace Word Text                                                  */
/* ------------------------------------------------------------------ */

// RegisterReplaceText contributes the Replace Word Text node.
func RegisterReplaceText(registrar nodes.Registrar) error {
	definition := definition("action:word_replace_text", "Replace Word Text",
		"Replace every occurrence of a text fragment in a Word document and save it in place.",
		"repeat",
		[]domain.NodePort{
			execPin("in", "Exec", domain.PinInput),
			textPin("path", "Path", domain.PinInput, true),
			textPin("find", "Find", domain.PinInput, true),
			textPin("replacement", "Replacement", domain.PinInput, true),
		},
		[]domain.NodePort{
			thenPin(),
			textPin("path", "Path", domain.PinOutput, false),
			numberPin("changed", "Changed", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\report.docx", Required: true},
			{Name: "find", Label: "Find", Kind: "string", Placeholder: "draft", Required: true},
			{Name: "replacement", Label: "Replacement", Kind: "string", Placeholder: "final", Required: true},
		},
		map[string]any{"find": "", "replacement": ""},
		domain.CapabilityFileRead, domain.CapabilityFileWrite,
	)
	return register(registrar, definition, executeReplaceText)
}

func executeReplaceText(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("replace word text cancelled: %w", err)
	}
	path, err := docxPath(invocation, "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	find := textValue(invocation, "find")
	if find == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("text to find is required")
	}
	replacement, _ := invocation.Inputs["replacement"].(string)
	document, err := readDocument(path)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	changed, err := document.replaceText(find, replacement)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	if err := writeDocument(path, document); err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"path": path, "changed": float64(changed)}, Ports: []string{"out"}}, nil
}

// splitLines normalizes line endings and returns the non-empty line list.
func splitLines(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	trimmed := strings.TrimRight(normalized, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

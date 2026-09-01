// Package docx implements a deliberately small WordprocessingML engine for
// the Word Blueprint nodes. It reads, creates, appends to, fills, and edits
// .docx files without external processes: a .docx is a ZIP archive whose
// word/document.xml holds the body as <w:p> paragraphs.
//
// Two fidelity levels are used. Reading walks the XML with encoding/xml and
// preserves only text (paragraph breaks, line breaks, tabs, and tables).
// Mutation (append, template fill, replace) performs paragraph-level surgery:
// the affected <w:p> keeps its <w:pPr> properties and its first run's
// formatting, and its text is rewritten as a single run. Unknown structures
// in untouched paragraphs are preserved byte-for-byte.
package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// minimalDocument is a complete, Word-openable .docx with one empty
// paragraph. [Content_Types].xml, _rels/.rels, and word/document.xml are the
// minimum required parts.
const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`

const relsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`

const documentSkeleton = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p/></w:body></w:document>`

// partName identifies every text-bearing main document part that template
// fill and text replacement visit.
func isDocumentPart(name string) bool {
	if name == "word/document.xml" {
		return true
	}
	if strings.HasPrefix(name, "word/header") && strings.HasSuffix(name, ".xml") {
		return true
	}
	return strings.HasPrefix(name, "word/footer") && strings.HasSuffix(name, ".xml")
}

// Document is an opened .docx archive with its raw parts in memory.
type Document struct {
	parts map[string][]byte
	order []string
}

// readDocx loads every part of a .docx archive into memory.
func readDocx(data []byte) (*Document, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open docx archive: %w", err)
	}
	document := &Document{parts: make(map[string][]byte, len(reader.File))}
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		payload, err := readZipEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name, err)
		}
		if _, duplicate := document.parts[entry.Name]; duplicate {
			return nil, fmt.Errorf("docx contains duplicate part %s", entry.Name)
		}
		document.parts[entry.Name] = payload
		document.order = append(document.order, entry.Name)
	}
	if _, exists := document.parts["word/document.xml"]; !exists {
		return nil, fmt.Errorf("the file is not a Word document: word/document.xml is missing")
	}
	return document, nil
}

// readZipEntry decompresses one archive member.
func readZipEntry(entry *zip.File) ([]byte, error) {
	file, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// writeDocx serializes the document back into a .docx archive, keeping the
// original part order and compression.
func (d *Document) writeDocx() ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	seen := make(map[string]struct{}, len(d.order))
	for _, name := range d.order {
		payload, exists := d.parts[name]
		if !exists {
			continue
		}
		entry, err := writer.Create(name)
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
		if _, err := entry.Write(payload); err != nil {
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
		seen[name] = struct{}{}
	}
	for name, payload := range d.parts {
		if _, exists := seen[name]; exists {
			continue
		}
		entry, err := writer.Create(name)
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
		if _, err := entry.Write(payload); err != nil {
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close docx archive: %w", err)
	}
	return buffer.Bytes(), nil
}

// newDocument builds a valid empty .docx in memory.
func newDocument() *Document {
	return &Document{
		parts: map[string][]byte{
			"[Content_Types].xml": []byte(contentTypesXML),
			"_rels/.rels":         []byte(relsXML),
			"word/document.xml":   []byte(documentSkeleton),
		},
		order: []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml"},
	}
}

// body returns the raw XML of the main document part.
func (d *Document) body() []byte { return d.parts["word/document.xml"] }

// setBody replaces the main document part.
func (d *Document) setBody(data []byte) { d.parts["word/document.xml"] = data }

/* ------------------------------------------------------------------ */
/* Reading                                                            */
/* ------------------------------------------------------------------ */

// text extracts the document's visible text. Paragraphs are separated by
// newlines, line breaks inside runs become newlines, tabs are preserved, and
// tables are flattened into tab-separated rows.
func (d *Document) text() (string, int, error) {
	extracted, paragraphs, err := extractText(d.body())
	if err != nil {
		return "", 0, fmt.Errorf("parse document.xml: %w", err)
	}
	return extracted, paragraphs, nil
}

// extractText walks one WordprocessingML part with a streaming decoder so
// run text, breaks, tabs, and table cells keep their exact order. Cell and
// row separators are deferred: they are only flushed when more text follows,
// which keeps flattened tables free of trailing tabs and spaces.
func extractText(data []byte) (string, int, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var builder strings.Builder
	paragraphs := 0
	depthInCell := 0
	pending := ""
	flush := func() {
		if pending != "" {
			builder.WriteString(pending)
			pending = ""
		}
	}
	write := func(text string) {
		flush()
		builder.WriteString(text)
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", 0, err
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "p":
				if depthInCell == 0 {
					paragraphs++
				}
			case "tc":
				depthInCell++
			case "br":
				write("\n")
			case "tab":
				write("\t")
			case "t":
				var text textXML
				if err := decoder.DecodeElement(&text, &element); err != nil {
					return "", 0, err
				}
				write(text.Text)
			}
		case xml.EndElement:
			switch element.Name.Local {
			case "p":
				if depthInCell == 0 {
					flush()
					builder.WriteString("\n")
				} else {
					// A paragraph break inside a cell separates paragraphs with a
					// space instead of a newline, keeping rows single-line.
					if pending != "\t" {
						pending = " "
					}
				}
			case "tc":
				depthInCell--
				pending = "\t"
			case "tr":
				pending = ""
				builder.WriteString("\n")
			}
		}
	}
	return strings.TrimRight(builder.String(), "\n"), paragraphs, nil
}

// textXML is one <w:t> element.
type textXML struct {
	Text string `xml:",chardata"`
}

// paragraphXML is the decoding target for paragraph-level surgery: only the
// concatenated run text is needed.
type paragraphXML struct {
	TextRuns []runXML `xml:"r"`
}

// runXML is one text run: <w:r><w:t>text</w:t></w:r>.
type runXML struct {
	Texts []textXML `xml:"t"`
}

// paragraph span is one <w:p>...</w:p> byte range inside a part.
type paragraphSpan struct {
	start int // index of '<' of <w:p
	end   int // index after '>' of </w:p>
}

// findParagraphs locates top-level <w:p> elements by scanning balanced
// structure. Nested paragraphs inside tables are skipped: a <w:p> opened
// while another is open belongs to a cell, and closing tags are matched by
// nesting depth.
func findParagraphs(data []byte) []paragraphSpan {
	var spans []paragraphSpan
	text := string(data)
	depth := 0
	start := -1
	for index := 0; index < len(text); index++ {
		if text[index] != '<' {
			continue
		}
		switch {
		case strings.HasPrefix(text[index:], "<w:p ") || strings.HasPrefix(text[index:], "<w:p>"):
			if depth == 0 {
				start = index
			}
			depth++
		case strings.HasPrefix(text[index:], "<w:p/>"):
			if depth == 0 {
				spans = append(spans, paragraphSpan{start: index, end: index + len("<w:p/>")})
			}
		case strings.HasPrefix(text[index:], "</w:p>"):
			depth--
			if depth == 0 && start >= 0 {
				spans = append(spans, paragraphSpan{start: start, end: index + len("</w:p>")})
				start = -1
			}
			if depth < 0 {
				depth = 0
			}
		}
	}
	return spans
}

// paragraphText returns the concatenated <w:t> text of one paragraph span.
func paragraphText(data []byte, span paragraphSpan) string {
	var decoded paragraphXML
	segment := data[span.start:span.end]
	if err := xml.Unmarshal(segment, &decoded); err != nil {
		return ""
	}
	var builder strings.Builder
	for _, run := range decoded.TextRuns {
		for _, text := range run.Texts {
			builder.WriteString(text.Text)
		}
	}
	return builder.String()
}

// replaceParagraphs rewrites the paragraphs whose text changes under apply.
// Rewritten paragraphs keep their <w:pPr> block and the first run's <w:rPr>
// formatting, and values containing newlines become <w:br/> runs.
func replaceParagraphs(data []byte, apply func(text string) (string, bool)) ([]byte, int) {
	spans := findParagraphs(data)
	if len(spans) == 0 {
		return data, 0
	}
	replacements := 0
	var output bytes.Buffer
	cursor := 0
	for _, span := range spans {
		output.Write(data[cursor:span.start])
		original := paragraphText(data, span)
		if replacement, changed := apply(original); changed && replacement != original {
			output.WriteString(rebuildParagraph(data[span.start:span.end], replacement))
			replacements++
		} else {
			output.Write(data[span.start:span.end])
		}
		cursor = span.end
	}
	output.Write(data[cursor:])
	return output.Bytes(), replacements
}

// rebuildParagraph builds a new single-run paragraph carrying over the
// original paragraph properties and first run properties.
func rebuildParagraph(segment []byte, text string) string {
	pPr := extractBlock(segment, "w:pPr")
	rPr := extractBlock(segment, "w:rPr")
	var builder strings.Builder
	builder.WriteString("<w:p>")
	if pPr != "" {
		builder.WriteString(pPr)
	}
	// Multi-line replacement text becomes one run with <w:br/> between lines.
	lines := strings.Split(text, "\n")
	builder.WriteString("<w:r>")
	if rPr != "" {
		builder.WriteString(rPr)
	}
	for index, line := range lines {
		if index > 0 {
			builder.WriteString("<w:br/>")
		}
		if line != "" {
			builder.WriteString(`<w:t xml:space="preserve">`)
			builder.WriteString(xmlEscape(line))
			builder.WriteString("</w:t>")
		}
	}
	builder.WriteString("</w:r>")
	builder.WriteString("</w:p>")
	return builder.String()
}

// extractBlock returns the raw XML of the first direct child block with the
// given tag inside the segment, or "" when absent.
func extractBlock(segment []byte, tag string) string {
	text := string(segment)
	open := "<" + tag
	for index := 0; index < len(text); index++ {
		if !strings.HasPrefix(text[index:], open) {
			continue
		}
		after := text[index+len(open):]
		if after != "" && after[0] != '>' && after[0] != ' ' && after[0] != '/' {
			continue
		}
		// Self-closing block.
		if strings.HasPrefix(after, "/>") {
			return text[index : index+len(open)+2]
		}
		closing := "</" + tag + ">"
		end := strings.Index(text[index:], closing)
		if end < 0 {
			return ""
		}
		return text[index : index+end+len(closing)]
	}
	return ""
}

// xmlEscape encodes text for XML content.
func xmlEscape(text string) string {
	var builder strings.Builder
	if err := xml.EscapeText(&builder, []byte(text)); err != nil {
		return text
	}
	return builder.String()
}

/* ------------------------------------------------------------------ */
/* Document-level mutations                                           */
/* ------------------------------------------------------------------ */

// appendParagraphs appends plain text lines as new paragraphs before the
// body's final <w:sectPr>, or before </w:body> when no section properties
// exist.
func (d *Document) appendParagraphs(lines []string) {
	if len(lines) == 0 {
		return
	}
	body := string(d.body())
	insert := &strings.Builder{}
	for _, line := range lines {
		insert.WriteString(paragraphXMLFor(line))
	}
	section := strings.LastIndex(body, "<w:sectPr")
	closing := strings.LastIndex(body, "</w:body>")
	if section >= 0 && (closing < 0 || section < closing) {
		body = body[:section] + insert.String() + body[section:]
	} else if closing >= 0 {
		body = body[:closing] + insert.String() + body[closing:]
	} else {
		body += insert.String()
	}
	d.setBody([]byte(body))
}

// paragraphXMLFor renders one paragraph of plain text. Empty lines become
// empty paragraphs so the caller's line structure is preserved.
func paragraphXMLFor(line string) string {
	if line == "" {
		return "<w:p/>"
	}
	var builder strings.Builder
	builder.WriteString("<w:r><w:t xml:space=\"preserve\">")
	builder.WriteString(xmlEscape(line))
	builder.WriteString("</w:t></w:r>")
	return "<w:p>" + builder.String() + "</w:p>"
}

// fillPlaceholders replaces {{name}} placeholders in every document part.
// It returns the number of replacements performed.
func (d *Document) fillPlaceholders(values map[string]string) (int, error) {
	if len(values) == 0 {
		return 0, fmt.Errorf("at least one placeholder value is required")
	}
	replacements := 0
	for name := range d.parts {
		if !isDocumentPart(name) {
			continue
		}
		updated, count := replaceParagraphs(d.parts[name], func(text string) (string, bool) {
			return expandPlaceholders(text, values), true
		})
		d.parts[name] = updated
		replacements += count
	}
	return replacements, nil
}

// expandPlaceholders substitutes {{name}} occurrences in one text value.
func expandPlaceholders(text string, values map[string]string) string {
	if !strings.Contains(text, "{{") {
		return text
	}
	var builder strings.Builder
	cursor := 0
	for {
		open := strings.Index(text[cursor:], "{{")
		if open < 0 {
			builder.WriteString(text[cursor:])
			return builder.String()
		}
		position := cursor + open
		close := strings.Index(text[position:], "}}")
		if close < 0 {
			builder.WriteString(text[cursor:])
			return builder.String()
		}
		name := strings.TrimSpace(text[position+2 : position+close])
		value, known := values[name]
		if !known {
			// Unknown placeholders are preserved so missing data is visible
			// in the generated document instead of silently emptied.
			builder.WriteString(text[cursor : position+close+2])
			cursor = position + close + 2
			continue
		}
		builder.WriteString(text[cursor:position])
		builder.WriteString(value)
		cursor = position + close + 2
	}
}

// replaceText substitutes every occurrence of a literal find string. It
// returns the number of changed paragraphs.
func (d *Document) replaceText(find, replacement string) (int, error) {
	if find == "" {
		return 0, fmt.Errorf("text to find is required")
	}
	changed := 0
	for name := range d.parts {
		if !isDocumentPart(name) {
			continue
		}
		updated, count := replaceParagraphs(d.parts[name], func(text string) (string, bool) {
			return strings.ReplaceAll(text, find, replacement), true
		})
		d.parts[name] = updated
		changed += count
	}
	return changed, nil
}

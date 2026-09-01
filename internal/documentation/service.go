// Package docs provides the local, embedded Markdown reference used
// by the Documentation tab and the editor inspector.
package docs

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/localization"
)

//go:embed content
var embeddedContent embed.FS

// PluginDocuments is deliberately small so the core documentation service is
// independent of plugin process management.
type PluginDocuments interface {
	Documentation() []domain.DocumentationDocument
}

// Service combines built-in, read-only Markdown with validated documentation
// made available by enabled plugins.
type Service struct {
	plugins        PluginDocuments
	core           map[string]domain.DocumentationDocument
	localized      map[string]map[string]string
	corePaths      map[string]string
	nodeReferences map[string]domain.DocumentationReference
}

type indexManifest struct {
	Documents      []indexDocument                          `json:"documents"`
	NodeReferences map[string]domain.DocumentationReference `json:"nodeReferences"`
}

type indexDocument struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	Category  []string `json:"category"`
	Path      string   `json:"path"`
	NodeTypes []string `json:"nodeTypes"`
}

// New loads and validates all embedded core documents. A malformed embedded
// index is treated as an app construction failure rather than a UI error.
func New(plugins PluginDocuments) (*Service, error) {
	content, err := fs.Sub(embeddedContent, "content")
	if err != nil {
		return nil, fmt.Errorf("open embedded documentation: %w", err)
	}
	data, err := fs.ReadFile(content, "index.json")
	if err != nil {
		return nil, fmt.Errorf("read documentation index: %w", err)
	}
	var index indexManifest
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parse documentation index: %w", err)
	}
	service := &Service{plugins: plugins, core: make(map[string]domain.DocumentationDocument, len(index.Documents)), localized: make(map[string]map[string]string), corePaths: make(map[string]string), nodeReferences: index.NodeReferences}
	for _, entry := range index.Documents {
		if err := validateCoreEntry(entry); err != nil {
			return nil, err
		}
		if _, exists := service.core[entry.ID]; exists {
			return nil, fmt.Errorf("duplicate documentation id %q", entry.ID)
		}
		markdown, err := fs.ReadFile(content, entry.Path)
		if err != nil {
			return nil, fmt.Errorf("read documentation %q: %w", entry.ID, err)
		}
		service.core[entry.ID] = domain.DocumentationDocument{
			DocumentationEntry: domain.DocumentationEntry{
				ID:        entry.ID,
				Title:     entry.Title,
				Summary:   entry.Summary,
				Category:  append([]string(nil), entry.Category...),
				NodeTypes: append([]string(nil), entry.NodeTypes...),
				Source:    "core",
			},
			Markdown: string(markdown),
		}
		service.corePaths[entry.ID] = entry.Path
	}
	for nodeType, reference := range service.nodeReferences {
		if strings.TrimSpace(nodeType) == "" || strings.TrimSpace(reference.DocumentID) == "" {
			return nil, fmt.Errorf("documentation node reference is incomplete")
		}
		if _, exists := service.core[reference.DocumentID]; !exists {
			return nil, fmt.Errorf("node %q references unknown documentation %q", nodeType, reference.DocumentID)
		}
	}
	if err := service.loadBuiltinNodeDocuments(content); err != nil {
		return nil, err
	}
	if err := service.loadLocalizedCoreDocuments(content); err != nil {
		return nil, err
	}
	return service, nil
}

// loadBuiltinNodeDocuments makes node documentation one file per current
// discoverable Blueprint node. The catalog contributes exact pin/configuration
// metadata, while every Markdown file carries its node-specific purpose and
// practical example.
func (s *Service) loadBuiltinNodeDocuments(content fs.FS) error {
	for _, definition := range catalog.New().All() {
		if definition.Source != "builtin" {
			continue
		}
		id := "node:" + definition.Type
		if _, exists := s.core[id]; exists {
			return fmt.Errorf("duplicate node documentation id %q", id)
		}
		path := "nodes/" + strings.NewReplacer(":", "-", "_", "-").Replace(definition.Type) + ".md"
		markdown, err := fs.ReadFile(content, path)
		if err != nil {
			return fmt.Errorf("read documentation for node %q: %w", definition.Type, err)
		}
		s.core[id] = domain.DocumentationDocument{
			DocumentationEntry: domain.DocumentationEntry{ID: id, Title: definition.Label, Summary: definition.Description, Category: []string{"Node reference", definition.Category}, NodeTypes: []string{definition.Type}, Source: "core"},
			Markdown:           enrichNodeMarkdown(string(markdown), definition),
		}
		s.corePaths[id] = path
		s.nodeReferences[definition.Type] = domain.DocumentationReference{DocumentID: id}
	}
	return nil
}

func (s *Service) loadLocalizedCoreDocuments(content fs.FS) error {
	for _, language := range localization.SupportedLanguages {
		if language == localization.English {
			continue
		}
		translations := make(map[string]string)
		for id, path := range s.corePaths {
			markdown, err := fs.ReadFile(content, "locales/"+language+"/"+path)
			if err == nil {
				translations[id] = string(markdown)
				continue
			}
			if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("read %s translation for %q: %w", language, id, err)
			}
		}
		s.localized[language] = translations
	}
	return nil
}

func enrichNodeMarkdown(markdown string, definition domain.NodeDefinition) string {
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(markdown))
	builder.WriteString("\n\n## Pins\n")
	writePins := func(label string, pins []domain.NodePort) {
		builder.WriteString("\n### " + label + "\n")
		if len(pins) == 0 {
			builder.WriteString("This node has no " + strings.ToLower(label) + ".\n")
			return
		}
		for _, pin := range pins {
			kind := string(pin.Kind)
			if pin.Kind == domain.PinData {
				kind += " · " + string(pin.DataType)
			}
			required := ""
			if pin.Required {
				required = " · required"
			}
			builder.WriteString("- **" + pin.Label + "** (`" + pin.ID + "`) — " + kind + required + ".\n")
		}
	}
	inputs := definition.Inputs
	if definition.Type == "data:build_object" {
		inputs = []domain.NodePort{{ID: "<configured-field>", Label: "Configured field", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataAny}}
	}
	if definition.Type == "data:build_array" {
		inputs = []domain.NodePort{{ID: "<configured-item>", Label: "Configured item", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataAny}}
	}
	if definition.Type == "data:build_map" {
		inputs = []domain.NodePort{{ID: "<configured-entry>", Label: "Configured entry", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataAny}}
	}
	writePins("Inputs", inputs)
	outputs := definition.Outputs
	// Get Field and Break Object create their data outputs from inspector
	// mappings. The catalog intentionally has no fixed output pin, so the
	// reference needs to describe the stable default plus the dynamic contract.
	if definition.Type == "data:get_field" || definition.Type == "data:break_object" {
		outputs = []domain.NodePort{{ID: "value", Label: "Value", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataAny}}
	}
	writePins("Outputs", outputs)
	builder.WriteString("\n## Configuration\n")
	if len(definition.Fields) == 0 {
		builder.WriteString("No inspector fields are required. Connect typed data pins for dynamic values.\n")
	} else {
		for _, field := range definition.Fields {
			required := "optional"
			if field.Required {
				required = "required"
			}
			builder.WriteString("- **" + field.Label + "** (`" + field.Name + "`, " + field.Kind + ", " + required + ")")
			if field.Placeholder != "" {
				builder.WriteString(" — example: `" + field.Placeholder + "`")
			}
			builder.WriteString(".\n")
		}
	}
	builder.WriteString("\n## Produced values\n")
	if definition.Type == "data:get_field" || definition.Type == "data:break_object" {
		builder.WriteString("Each row in **Outputs** creates one typed data pin. Its stable ID keeps connected wires intact when you rename its label; configure the path and expected data type for every output.\n")
	} else if definition.Type == "data:build_object" {
		builder.WriteString("Each row in **Fields** creates one typed input pin. Its stable ID keeps connected wires intact when you rename its label or remap its object key; dotted keys construct nested objects.\n")
	} else if definition.Type == "data:build_array" {
		builder.WriteString("Each row in **Items** creates one input pin typed by **Element type**; its stable ID keeps connected wires intact when you rename or reorder it, and a constant fills the element without a wire. Every pin, constant, and the Array output share that one type — choose **any** to allow mixed elements.\n")
	} else if definition.Type == "data:build_map" {
		builder.WriteString("Each row in **Entries** creates one input pin typed by **Value type**; its stable ID keeps connected wires intact when you rename or reorder it, keys are used verbatim, and a constant fills the entry without a wire. Every pin, constant, and the Map output share that one type — choose **any** to allow mixed values.\n")
	} else if len(definition.Outputs) == 0 {
		builder.WriteString("This node produces no downstream value.\n")
	} else {
		builder.WriteString("Outputs are scoped to this active execution path and are never reused by a later run.\n")
	}
	builder.WriteString("\n## Capabilities and approval\n")
	if len(definition.Capabilities) == 0 {
		builder.WriteString("This node has no additional capability grant. Downstream actions may still require approval.\n")
	} else {
		values := make([]string, 0, len(definition.Capabilities))
		for _, capability := range definition.Capabilities {
			values = append(values, "`"+string(capability)+"`")
		}
		builder.WriteString("This node requests " + strings.Join(values, ", ") + ". Manual runs may prompt; unattended schedules and webhooks require trust for the exact published revision.\n")
	}
	builder.WriteString("\n## Failure notes\n")
	switch definition.Mode {
	case domain.NodePure:
		builder.WriteString("A pure value is evaluated only when requested. A bad input or conversion stops the requesting execution path; it does not run an action.\n")
	case domain.NodeEvent:
		builder.WriteString("The trigger must be fully configured and published before it can start a live run. Trigger delivery failures are recorded without running downstream nodes.\n")
	case domain.NodeVisual:
		builder.WriteString("Canvas-only nodes do not execute and cannot fail a run.\n")
	default:
		builder.WriteString("Required inputs are resolved immediately before the exec pulse. Any action or provider error stops the run and appears in the execution log with redacted details.\n")
	}
	return builder.String()
}

func validateCoreEntry(entry indexDocument) error {
	if strings.TrimSpace(entry.ID) == "" || strings.TrimSpace(entry.Title) == "" || len(entry.Category) == 0 || strings.TrimSpace(entry.Path) == "" {
		return fmt.Errorf("documentation entry needs id, title, category, and path")
	}
	if strings.HasPrefix(entry.Path, "/") || strings.Contains(entry.Path, "..") || !strings.HasSuffix(strings.ToLower(entry.Path), ".md") {
		return fmt.Errorf("documentation entry %q has an invalid Markdown path", entry.ID)
	}
	return nil
}

// List returns tree metadata only, sorted deterministically for rendering.
func (s *Service) List(language string) ([]domain.DocumentationEntry, error) {
	documents, err := s.documents(language)
	if err != nil {
		return nil, err
	}
	entries := make([]domain.DocumentationEntry, 0, len(documents))
	for _, document := range documents {
		entries = append(entries, document.DocumentationEntry)
	}
	sort.Slice(entries, func(i, j int) bool {
		left, right := strings.Join(entries[i].Category, "\x00"), strings.Join(entries[j].Category, "\x00")
		if left == right {
			return entries[i].Title < entries[j].Title
		}
		return left < right
	})
	return entries, nil
}

// Get returns a complete Markdown document by stable ID.
func (s *Service) Get(language, id string) (domain.DocumentationDocument, error) {
	documents, err := s.documents(language)
	if err != nil {
		return domain.DocumentationDocument{}, err
	}
	document, exists := documents[strings.TrimSpace(id)]
	if !exists {
		return domain.DocumentationDocument{}, fmt.Errorf("documentation %q was not found", id)
	}
	return document, nil
}

// Search performs a local, case-insensitive full-text search over document
// metadata and Markdown. It is bounded by the embedded/plugin file limits.
func (s *Service) Search(language, query string) ([]domain.DocumentationSearchResult, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []domain.DocumentationSearchResult{}, nil
	}
	documents, err := s.documents(language)
	if err != nil {
		return nil, err
	}
	results := make([]domain.DocumentationSearchResult, 0)
	for _, document := range documents {
		haystack := strings.ToLower(document.Title + "\n" + document.Summary + "\n" + document.Markdown)
		if !strings.Contains(haystack, query) {
			continue
		}
		results = append(results, domain.DocumentationSearchResult{Document: document.DocumentationEntry, Excerpt: excerpt(document.Markdown, query)})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Document.Title < results[j].Document.Title
	})
	return results, nil
}

// ForNode resolves an inspector node type to its documentation page. Plugin
// documents can associate a node type, but never override a built-in page.
func (s *Service) ForNode(nodeType string) (domain.DocumentationReference, error) {
	nodeType = strings.TrimSpace(nodeType)
	if reference, exists := s.nodeReferences[nodeType]; exists {
		return reference, nil
	}
	documents, err := s.documents(localization.English)
	if err != nil {
		return domain.DocumentationReference{}, err
	}
	for _, document := range documents {
		if document.Source != "plugin" {
			continue
		}
		for _, associatedType := range document.NodeTypes {
			if associatedType == nodeType {
				return domain.DocumentationReference{DocumentID: document.ID}, nil
			}
		}
	}
	return domain.DocumentationReference{}, fmt.Errorf("documentation is unavailable for node %q", nodeType)
}

// ValidateNodeCoverage checks that every discoverable core Blueprint node has
// a reference page. It is intentionally exported for catalog-level tests.
func (s *Service) ValidateNodeCoverage(nodeTypes []string) error {
	for _, nodeType := range nodeTypes {
		if _, exists := s.nodeReferences[nodeType]; !exists {
			return fmt.Errorf("missing documentation for node %q", nodeType)
		}
	}
	return nil
}

func (s *Service) documents(language string) (map[string]domain.DocumentationDocument, error) {
	language = localization.Normalize(language)
	documents := make(map[string]domain.DocumentationDocument, len(s.core))
	for id, document := range s.core {
		if markdown, exists := s.localized[language][id]; exists {
			document.Markdown = markdown
			if title := markdownTitle(markdown); title != "" {
				document.Title = title
			}
			if summary := markdownSummary(markdown); summary != "" {
				document.Summary = summary
			}
		}
		documents[id] = document
	}
	if s.plugins == nil {
		return documents, nil
	}
	for _, document := range s.plugins.Documentation() {
		if _, exists := documents[document.ID]; exists {
			return nil, fmt.Errorf("duplicate documentation id %q", document.ID)
		}
		documents[document.ID] = document
	}
	return documents, nil
}

func markdownTitle(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

// markdownSummary supplies a localized navigation summary without duplicating
// metadata in every translated document. It uses the first prose paragraph
// after the title, keeping the tree compact and predictable.
func markdownSummary(markdown string) string {
	for _, raw := range strings.Split(markdown, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "```") {
			continue
		}
		return truncateRunes(line, 220)
	}
	return ""
}

func excerpt(markdown, query string) string {
	plain := strings.Join(strings.Fields(markdown), " ")
	if plain == "" {
		return ""
	}
	lower := strings.ToLower(plain)
	index := strings.Index(lower, query)
	if index < 0 {
		return truncateRunes(plain, 180)
	}
	start := index - 64
	if start < 0 {
		start = 0
	}
	end := index + len(query) + 116
	if end > len(plain) {
		end = len(plain)
	}
	value := strings.TrimSpace(plain[start:end])
	if start > 0 {
		value = "…" + value
	}
	if end < len(plain) {
		value += "…"
	}
	return value
}

func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "…"
}

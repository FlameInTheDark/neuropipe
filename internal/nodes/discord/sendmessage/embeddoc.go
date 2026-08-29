package sendmessage

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

/* ------------------------------------------------------------------ */
/* embed document model                                                */
/* ------------------------------------------------------------------ */

// Discord's embed limits, validated before any request so rejections carry
// Neuropipe's precise reasons instead of Discord's Invalid Form Body.
const (
	MaxEmbedsPerMessage   = 10
	MaxEmbedTitle         = 256
	MaxEmbedDescription   = 4096
	MaxEmbedAuthorName    = 256
	MaxEmbedFooterText    = 2048
	MaxEmbedFields        = 25
	MaxEmbedFieldName     = 256
	MaxEmbedFieldValue    = 1024
	MaxEmbedTotalRunes    = 6000
	maxEmbedDocumentPins  = 32
	embedTemplateMaxParts = 64
)

// EmbedDocument is the editor-authored embed payload stored in node config.
// Text fields may reference declared pins with {{name}} templates; the
// execution path resolves them from input pin values, the editor preview from
// sample values.
type EmbedDocument struct {
	Version int
	Pins    []EmbedPin
	Embeds  []EmbedSpec
}

// EmbedPin declares one template variable and its dynamic input pin.
type EmbedPin struct {
	Name    string
	Type    string // text | number | boolean
	Sample  string
	Default string
}

// Pin type identifiers shared by the document, the resolver, and the editor.
const (
	PinText    = "text"
	PinNumber  = "number"
	PinBoolean = "boolean"
)

// EmbedSpec is one embed card in the document. Color is a #RRGGBB string in
// the document and becomes Discord's integer color at build time.
type EmbedSpec struct {
	ID          string
	Title       string
	Description string
	URL         string
	Color       string
	Timestamp   string
	Author      EmbedAuthorSpec
	Footer      EmbedFooterSpec
	Image       EmbedMediaSpec
	Thumbnail   EmbedMediaSpec
	Fields      []EmbedFieldSpec
}

type EmbedAuthorSpec struct {
	Name    string
	URL     string
	IconURL string
}

type EmbedFooterSpec struct {
	Text    string
	IconURL string
}

type EmbedMediaSpec struct {
	URL string
}

type EmbedFieldSpec struct {
	ID     string
	Name   string
	Value  string
	Inline bool
}

// EmptyEmbedDocument is the default config value: no pins, no embeds. New
// nodes behave exactly like the pre-embed Send Message node until the user
// deliberately designs embeds in the editor.
func EmptyEmbedDocument() map[string]any {
	return map[string]any{"version": 1, "pins": []any{}, "embeds": []any{}}
}

/* ------------------------------------------------------------------ */
/* parsing                                                             */
/* ------------------------------------------------------------------ */

// ParseEmbedDocument defensively normalizes any persisted config value into
// an EmbedDocument. Malformed entries are dropped, never fatal: a damaged
// document degrades to fewer embeds instead of a broken pipeline.
func ParseEmbedDocument(value any) EmbedDocument {
	document := EmbedDocument{Version: 1}
	root, ok := value.(map[string]any)
	if !ok {
		return document
	}
	if version, ok := root["version"].(float64); ok && int(version) >= 1 {
		document.Version = int(version)
	}
	if pins, ok := root["pins"].([]any); ok {
		for _, entry := range pins {
			pin, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			name := strings.TrimSpace(stringValue(pin["name"]))
			if name == "" || len(document.Pins) >= maxEmbedDocumentPins {
				continue
			}
			pinType := stringValue(pin["type"])
			if pinType != PinNumber && pinType != PinBoolean {
				pinType = PinText
			}
			document.Pins = append(document.Pins, EmbedPin{
				Name: name, Type: pinType,
				Sample: stringValue(pin["sample"]), Default: stringValue(pin["default"]),
			})
		}
	}
	if embeds, ok := root["embeds"].([]any); ok {
		for _, entry := range embeds {
			spec, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			document.Embeds = append(document.Embeds, parseEmbedSpec(spec))
		}
	}
	return document
}

func parseEmbedSpec(spec map[string]any) EmbedSpec {
	parsed := EmbedSpec{
		ID:          strings.TrimSpace(stringValue(spec["id"])),
		Title:       stringValue(spec["title"]),
		Description: stringValue(spec["description"]),
		URL:         strings.TrimSpace(stringValue(spec["url"])),
		Color:       strings.TrimSpace(stringValue(spec["color"])),
		Timestamp:   strings.TrimSpace(stringValue(spec["timestamp"])),
	}
	author := safeMap(spec["author"])
	parsed.Author = EmbedAuthorSpec{
		Name:    stringValue(author["name"]),
		URL:     strings.TrimSpace(stringValue(author["url"])),
		IconURL: strings.TrimSpace(stringValue(author["iconUrl"])),
	}
	footer := safeMap(spec["footer"])
	parsed.Footer = EmbedFooterSpec{
		Text:    stringValue(footer["text"]),
		IconURL: strings.TrimSpace(stringValue(footer["iconUrl"])),
	}
	parsed.Image = EmbedMediaSpec{URL: strings.TrimSpace(stringValue(safeMap(spec["image"])["url"]))}
	parsed.Thumbnail = EmbedMediaSpec{URL: strings.TrimSpace(stringValue(safeMap(spec["thumbnail"])["url"]))}
	if fields, ok := spec["fields"].([]any); ok {
		for _, entry := range fields {
			field, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			parsed.Fields = append(parsed.Fields, EmbedFieldSpec{
				ID:     strings.TrimSpace(stringValue(field["id"])),
				Name:   stringValue(field["name"]),
				Value:  stringValue(field["value"]),
				Inline: field["inline"] == true,
			})
		}
	}
	return parsed
}

// stringValue coerces common JSON scalar shapes to string without panicking
// on nil maps.
func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func safeMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

/* ------------------------------------------------------------------ */
/* interpolation                                                       */
/* ------------------------------------------------------------------ */

// Interpolate replaces {{name}} references with resolved pin values. Unknown
// names resolve to the empty string so a partially wired pipeline still
// renders instead of leaking template syntax to Discord.
func Interpolate(text string, values map[string]any) string {
	if !strings.Contains(text, "{{") {
		return text
	}
	var builder strings.Builder
	rest := text
	replacements := 0
	for {
		open := strings.Index(rest, "{{")
		if open < 0 || replacements >= embedTemplateMaxParts {
			builder.WriteString(rest)
			return builder.String()
		}
		closeIdx := strings.Index(rest[open:], "}}")
		if closeIdx < 0 {
			builder.WriteString(rest)
			return builder.String()
		}
		builder.WriteString(rest[:open])
		name := strings.TrimSpace(rest[open+2 : open+closeIdx])
		builder.WriteString(FormatValue(values[name]))
		rest = rest[open+closeIdx+2:]
		replacements++
	}
}

// FormatValue renders one pin value for text interpolation: scalars print
// naturally (numbers never in scientific notation), composites as compact
// JSON, missing values as empty string.
func FormatValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		return typed.String()
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case []byte:
		return string(typed)
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(encoded)
	}
}

// PinValues resolves the document's pins from invocation inputs, falling back
// to declared defaults. Samples are preview-only and never used at execution.
func PinValues(document EmbedDocument, inputs map[string]any) map[string]any {
	values := make(map[string]any, len(document.Pins))
	for _, pin := range document.Pins {
		if value, ok := inputs[pin.Name]; ok && value != nil {
			values[pin.Name] = value
			continue
		}
		if pin.Default != "" {
			values[pin.Name] = typedEmbedDefault(pin)
		}
	}
	return values
}

func typedEmbedDefault(pin EmbedPin) any {
	switch pin.Type {
	case PinNumber:
		if number, err := strconv.ParseFloat(pin.Default, 64); err == nil {
			return number
		}
		return pin.Default
	case PinBoolean:
		return pin.Default == "true"
	default:
		return pin.Default
	}
}

/* ------------------------------------------------------------------ */
/* building Discord embeds                                             */
/* ------------------------------------------------------------------ */

// BuildEmbeds converts the document into Discord embeds with templates
// resolved and limits enforced. A non-empty reason means the request must be
// soft-rejected with it; the returned embeds are then meaningless.
func (document EmbedDocument) BuildEmbeds(values map[string]any) ([]*domain.DiscordEmbed, string) {
	if len(document.Embeds) == 0 {
		return nil, ""
	}
	if len(document.Embeds) > MaxEmbedsPerMessage {
		return nil, fmt.Sprintf("message has %d embeds, over Discord's limit of %d", len(document.Embeds), MaxEmbedsPerMessage)
	}
	embeds := make([]*domain.DiscordEmbed, 0, len(document.Embeds))
	total := 0
	for index, spec := range document.Embeds {
		embed, runes, reason := spec.build(values)
		if reason != "" {
			return nil, fmt.Sprintf("embed %d: %s", index+1, reason)
		}
		total += runes
		embeds = append(embeds, embed)
	}
	if total > MaxEmbedTotalRunes {
		return nil, fmt.Sprintf("embeds total %d characters, over Discord's combined limit of %d", total, MaxEmbedTotalRunes)
	}
	return embeds, ""
}

func (spec EmbedSpec) build(values map[string]any) (*domain.DiscordEmbed, int, string) {
	embed := &domain.DiscordEmbed{
		Title:       Interpolate(spec.Title, values),
		Description: Interpolate(spec.Description, values),
		URL:         strings.TrimSpace(Interpolate(spec.URL, values)),
		Timestamp:   spec.Timestamp,
	}
	total := utf8.RuneCountInString(embed.Title) + utf8.RuneCountInString(embed.Description)
	if utf8.RuneCountInString(embed.Title) > MaxEmbedTitle {
		return nil, 0, fmt.Sprintf("title is %d characters, over the limit of %d", utf8.RuneCountInString(embed.Title), MaxEmbedTitle)
	}
	if utf8.RuneCountInString(embed.Description) > MaxEmbedDescription {
		return nil, 0, fmt.Sprintf("description is %d characters, over the limit of %d", utf8.RuneCountInString(embed.Description), MaxEmbedDescription)
	}
	if embed.URL != "" && !looksLikeURL(embed.URL) {
		return nil, 0, fmt.Sprintf("url %q is not an absolute http(s) URL", embed.URL)
	}
	if spec.Timestamp != "" {
		if _, err := time.Parse(time.RFC3339, spec.Timestamp); err != nil {
			return nil, 0, fmt.Sprintf("timestamp %q is not an ISO 8601 date-time", spec.Timestamp)
		}
	}
	if color, reason := parseEmbedColor(spec.Color); reason == "" {
		embed.Color = color
	} else {
		return nil, 0, reason
	}
	if name := Interpolate(spec.Author.Name, values); name != "" {
		if utf8.RuneCountInString(name) > MaxEmbedAuthorName {
			return nil, 0, fmt.Sprintf("author name is %d characters, over the limit of %d", utf8.RuneCountInString(name), MaxEmbedAuthorName)
		}
		total += utf8.RuneCountInString(name)
		embed.Author = &domain.DiscordEmbedAuthor{
			Name:    name,
			URL:     strings.TrimSpace(Interpolate(spec.Author.URL, values)),
			IconURL: strings.TrimSpace(Interpolate(spec.Author.IconURL, values)),
		}
		if embed.Author.URL != "" && !looksLikeURL(embed.Author.URL) {
			return nil, 0, fmt.Sprintf("author url %q is not an absolute http(s) URL", embed.Author.URL)
		}
		if embed.Author.IconURL != "" && !looksLikeURL(embed.Author.IconURL) {
			return nil, 0, fmt.Sprintf("author icon url %q is not an absolute http(s) URL", embed.Author.IconURL)
		}
	}
	if text := Interpolate(spec.Footer.Text, values); text != "" {
		if utf8.RuneCountInString(text) > MaxEmbedFooterText {
			return nil, 0, fmt.Sprintf("footer is %d characters, over the limit of %d", utf8.RuneCountInString(text), MaxEmbedFooterText)
		}
		total += utf8.RuneCountInString(text)
		embed.Footer = &domain.DiscordEmbedFooter{
			Text:    text,
			IconURL: strings.TrimSpace(Interpolate(spec.Footer.IconURL, values)),
		}
		if embed.Footer.IconURL != "" && !looksLikeURL(embed.Footer.IconURL) {
			return nil, 0, fmt.Sprintf("footer icon url %q is not an absolute http(s) URL", embed.Footer.IconURL)
		}
	}
	if url := strings.TrimSpace(Interpolate(spec.Image.URL, values)); url != "" {
		if !looksLikeURL(url) {
			return nil, 0, fmt.Sprintf("image url %q is not an absolute http(s) URL", url)
		}
		embed.Image = &domain.DiscordEmbedMedia{URL: url}
	}
	if url := strings.TrimSpace(Interpolate(spec.Thumbnail.URL, values)); url != "" {
		if !looksLikeURL(url) {
			return nil, 0, fmt.Sprintf("thumbnail url %q is not an absolute http(s) URL", url)
		}
		embed.Thumbnail = &domain.DiscordEmbedMedia{URL: url}
	}
	if len(spec.Fields) > MaxEmbedFields {
		return nil, 0, fmt.Sprintf("embed has %d fields, over the limit of %d", len(spec.Fields), MaxEmbedFields)
	}
	for _, field := range spec.Fields {
		name := Interpolate(field.Name, values)
		value := Interpolate(field.Value, values)
		if name == "" || value == "" {
			continue // an empty field would be rejected by Discord; drop it
		}
		if utf8.RuneCountInString(name) > MaxEmbedFieldName {
			return nil, 0, fmt.Sprintf("field %q name is %d characters, over the limit of %d", name, utf8.RuneCountInString(name), MaxEmbedFieldName)
		}
		if utf8.RuneCountInString(value) > MaxEmbedFieldValue {
			return nil, 0, fmt.Sprintf("field %q value is %d characters, over the limit of %d", name, utf8.RuneCountInString(value), MaxEmbedFieldValue)
		}
		total += utf8.RuneCountInString(name) + utf8.RuneCountInString(value)
		embed.Fields = append(embed.Fields, &domain.DiscordEmbedField{Name: name, Value: value, Inline: field.Inline})
	}
	if embedIsEmpty(embed) {
		return nil, 0, "embed is empty — set a title, description, author, footer, fields, or an image"
	}
	return embed, total, ""
}

func embedIsEmpty(embed *domain.DiscordEmbed) bool {
	return embed.Title == "" && embed.Description == "" && embed.URL == "" &&
		embed.Author == nil && embed.Footer == nil && embed.Image == nil &&
		embed.Thumbnail == nil && len(embed.Fields) == 0
}

// parseEmbedColor converts "#5865F2" or "5865F2" into Discord's integer
// color. An empty color is Discord's default bar; raw JSON pins carry integer
// colors already, so only the editor's hex form is parsed here.
func parseEmbedColor(raw string) (int, string) {
	if raw == "" {
		return 0, ""
	}
	hex := strings.TrimPrefix(strings.TrimSpace(raw), "#")
	if len(hex) == 6 {
		if value, err := strconv.ParseUint(hex, 16, 32); err == nil {
			return int(value), ""
		}
	}
	return 0, fmt.Sprintf("color %q is not a #RRGGBB hex value", raw)
}

func looksLikeURL(raw string) bool {
	return strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")
}

/* ------------------------------------------------------------------ */
/* raw embedsJson pin                                                  */
/* ------------------------------------------------------------------ */

// ParseEmbedsJSON accepts one embed object or an array of embeds in Discord's
// canonical snake_case JSON, the shape users paste from Discord's
// documentation, Discohook, or embed-generator exports.
func ParseEmbedsJSON(raw string) ([]*domain.DiscordEmbed, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var single domain.DiscordEmbed
	if trimmed[0] == '{' {
		if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
			return nil, fmt.Errorf("embeds JSON is not a valid embed object: %w", err)
		}
		return []*domain.DiscordEmbed{&single}, nil
	}
	var list []*domain.DiscordEmbed
	if err := json.Unmarshal([]byte(trimmed), &list); err != nil {
		return nil, fmt.Errorf("embeds JSON is not a valid embed array: %w", err)
	}
	for index, embed := range list {
		if embed == nil {
			return nil, fmt.Errorf("embeds JSON entry %d is null", index)
		}
	}
	if len(list) > MaxEmbedsPerMessage {
		return nil, fmt.Errorf("embeds JSON has %d embeds, over Discord's limit of %d", len(list), MaxEmbedsPerMessage)
	}
	return list, nil
}

// ValidateEmbeds enforces the same limits on raw JSON embeds that the editor
// document path enforces, so both sources reject identically broken payloads.
func ValidateEmbeds(embeds []*domain.DiscordEmbed) string {
	if len(embeds) == 0 {
		return ""
	}
	total := 0
	for index, embed := range embeds {
		total += utf8.RuneCountInString(embed.Title) + utf8.RuneCountInString(embed.Description)
		if utf8.RuneCountInString(embed.Title) > MaxEmbedTitle {
			return fmt.Sprintf("embed %d: title is %d characters, over the limit of %d", index+1, utf8.RuneCountInString(embed.Title), MaxEmbedTitle)
		}
		if utf8.RuneCountInString(embed.Description) > MaxEmbedDescription {
			return fmt.Sprintf("embed %d: description is %d characters, over the limit of %d", index+1, utf8.RuneCountInString(embed.Description), MaxEmbedDescription)
		}
		if embed.Author != nil {
			if utf8.RuneCountInString(embed.Author.Name) > MaxEmbedAuthorName {
				return fmt.Sprintf("embed %d: author name is %d characters, over the limit of %d", index+1, utf8.RuneCountInString(embed.Author.Name), MaxEmbedAuthorName)
			}
			total += utf8.RuneCountInString(embed.Author.Name)
		}
		if embed.Footer != nil {
			if utf8.RuneCountInString(embed.Footer.Text) > MaxEmbedFooterText {
				return fmt.Sprintf("embed %d: footer is %d characters, over the limit of %d", index+1, utf8.RuneCountInString(embed.Footer.Text), MaxEmbedFooterText)
			}
			total += utf8.RuneCountInString(embed.Footer.Text)
		}
		if len(embed.Fields) > MaxEmbedFields {
			return fmt.Sprintf("embed %d has %d fields, over the limit of %d", index+1, len(embed.Fields), MaxEmbedFields)
		}
		for _, field := range embed.Fields {
			if field == nil {
				continue
			}
			if utf8.RuneCountInString(field.Name) > MaxEmbedFieldName {
				return fmt.Sprintf("embed %d: field %q name is %d characters, over the limit of %d", index+1, field.Name, utf8.RuneCountInString(field.Name), MaxEmbedFieldName)
			}
			if utf8.RuneCountInString(field.Value) > MaxEmbedFieldValue {
				return fmt.Sprintf("embed %d: field %q value is %d characters, over the limit of %d", index+1, field.Name, utf8.RuneCountInString(field.Value), MaxEmbedFieldValue)
			}
			total += utf8.RuneCountInString(field.Name) + utf8.RuneCountInString(field.Value)
		}
	}
	if total > MaxEmbedTotalRunes {
		return fmt.Sprintf("embeds total %d characters, over Discord's combined limit of %d", total, MaxEmbedTotalRunes)
	}
	return ""
}

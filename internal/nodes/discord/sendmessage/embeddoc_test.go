package sendmessage

import (
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestParseEmbedDocumentDefensive(t *testing.T) {
	document := ParseEmbedDocument(nil)
	if len(document.Embeds) != 0 || len(document.Pins) != 0 || document.Version != 1 {
		t.Fatalf("nil document = %#v", document)
	}

	document = ParseEmbedDocument(map[string]any{
		"version": float64(1),
		"pins": []any{
			map[string]any{"name": "city", "type": "text", "sample": "Berlin"},
			map[string]any{"name": "", "type": "text"},
			map[string]any{"name": "temp", "type": "number", "default": "20"},
			map[string]any{"name": "hot", "type": "weird"},
			"not a map",
		},
		"embeds": []any{
			map[string]any{
				"id": "e1", "title": "Hello", "description": "World", "color": "#5865F2",
				"author":    map[string]any{"name": "Neuropipe", "iconUrl": "https://example.com/a.png"},
				"footer":    map[string]any{"text": "Bye"},
				"image":     map[string]any{"url": "https://example.com/i.png"},
				"thumbnail": nil,
				"fields": []any{
					map[string]any{"id": "f1", "name": "A", "value": "1", "inline": true},
					map[string]any{"id": "f2", "name": "", "value": "kept"},
					"junk",
				},
			},
			"junk embed",
		},
	})
	if len(document.Pins) != 3 {
		t.Fatalf("pins = %#v", document.Pins)
	}
	if document.Pins[2].Type != PinText {
		t.Fatalf("unknown pin type not normalized to text: %#v", document.Pins[2])
	}
	if len(document.Embeds) != 1 {
		t.Fatalf("embeds = %#v", document.Embeds)
	}
	embed := document.Embeds[0]
	if embed.Title != "Hello" || embed.Author.Name != "Neuropipe" || embed.Footer.Text != "Bye" {
		t.Fatalf("embed = %#v", embed)
	}
	if embed.Image.URL != "https://example.com/i.png" || embed.Thumbnail.URL != "" {
		t.Fatalf("media = %#v", embed)
	}
	if len(embed.Fields) != 2 || embed.Fields[0].Inline != true || embed.Fields[1].Inline != false {
		t.Fatalf("fields = %#v", embed.Fields)
	}
}

func TestInterpolate(t *testing.T) {
	values := map[string]any{
		"city":   "Berlin",
		"temp":   21.5,
		"count":  int64(3),
		"hot":    true,
		"list":   []any{"a", 1.0},
		"object": map[string]any{"k": "v"},
	}
	cases := map[string]string{
		"{{city}}":                      "Berlin",
		"Weather in {{city}}!":          "Weather in Berlin!",
		"{{temp}}°C {{count}}x":         "21.5°C 3x",
		"{{hot}}":                       "true",
		"{{list}}":                      `["a",1]`,
		"{{object}}":                    `{"k":"v"}`,
		"{{missing}}":                   "",
		"no templates":                  "no templates",
		"{{ city }}":                    "Berlin",
		"{{unclosed":                    "{{unclosed",
		"a {{x}} b {{y}} c {{missing}}": "a  b  c ",
	}
	for input, expected := range cases {
		if got := Interpolate(input, values); got != expected {
			t.Errorf("Interpolate(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestBuildEmbedsHappyPath(t *testing.T) {
	document := ParseEmbedDocument(map[string]any{
		"pins": []any{map[string]any{"name": "city", "type": "text"}},
		"embeds": []any{map[string]any{
			"title": "Weather in {{city}}", "description": "Sunny", "color": "#5865F2",
			"url":       "https://example.com/report",
			"timestamp": "2026-08-29T12:00:00Z",
			"author":    map[string]any{"name": "Bot"},
			"footer":    map[string]any{"text": "generated"},
			"fields": []any{
				map[string]any{"name": "Temp", "value": "21", "inline": true},
				map[string]any{"name": "", "value": "dropped"}, // empty name drops the field
			},
		}},
	})
	embeds, reason := document.BuildEmbeds(map[string]any{"city": "Berlin"})
	if reason != "" {
		t.Fatalf("reason = %q", reason)
	}
	if len(embeds) != 1 {
		t.Fatalf("embeds = %#v", embeds)
	}
	embed := embeds[0]
	if embed.Title != "Weather in Berlin" || embed.Color != 0x5865F2 {
		t.Fatalf("embed = %#v", embed)
	}
	if embed.Author == nil || embed.Author.Name != "Bot" || embed.Footer == nil || embed.Footer.Text != "generated" {
		t.Fatalf("author/footer = %#v %#v", embed.Author, embed.Footer)
	}
	if len(embed.Fields) != 1 || embed.Fields[0].Name != "Temp" || !embed.Fields[0].Inline {
		t.Fatalf("fields = %#v", embed.Fields)
	}
}

func TestBuildEmbedsValidations(t *testing.T) {
	base := func(title, description string, fields int) map[string]any {
		fieldList := make([]any, fields)
		for index := range fieldList {
			fieldList[index] = map[string]any{"name": "F", "value": "V"}
		}
		return map[string]any{
			"embeds": []any{map[string]any{"title": title, "description": description, "fields": fieldList}},
		}
	}
	cases := []struct {
		document map[string]any
		want     string
	}{
		{base(strings.Repeat("t", 257), "", 0), "title is 257 characters"},
		{base("", strings.Repeat("d", 4097), 0), "description is 4097 characters"},
		{base("T", "", 26), "over the limit of 25"},
		{map[string]any{"embeds": []any{map[string]any{"color": "notacolor"}}}, "is not a #RRGGBB hex value"},
		{map[string]any{"embeds": []any{map[string]any{"url": "ftp://example.com"}}}, "not an absolute http(s) URL"},
		{map[string]any{"embeds": []any{map[string]any{"timestamp": "yesterday"}}}, "not an ISO 8601 date-time"},
		{map[string]any{"embeds": []any{map[string]any{}}}, "embed is empty"},
	}
	for _, testCase := range cases {
		_, reason := ParseEmbedDocument(testCase.document).BuildEmbeds(map[string]any{})
		if reason == "" || !strings.Contains(reason, testCase.want) {
			t.Errorf("document %v: reason = %q, want %q", testCase.document, reason, testCase.want)
		}
	}

	// over the combined limit: 3 embeds x 2001-char descriptions
	big := map[string]any{"embeds": []any{}}
	for index := 0; index < 3; index++ {
		big["embeds"] = append(big["embeds"].([]any), map[string]any{"description": strings.Repeat("x", 2001)})
	}
	if _, reason := ParseEmbedDocument(big).BuildEmbeds(map[string]any{}); !strings.Contains(reason, "combined limit") {
		t.Errorf("combined limit reason = %q", reason)
	}

	// more than 10 embeds
	many := map[string]any{"embeds": []any{}}
	for index := 0; index < 11; index++ {
		many["embeds"] = append(many["embeds"].([]any), map[string]any{"title": "T"})
	}
	if _, reason := ParseEmbedDocument(many).BuildEmbeds(map[string]any{}); !strings.Contains(reason, "over Discord's limit of 10") {
		t.Errorf("embed count reason = %q", reason)
	}
}

func TestParseEmbedsJSON(t *testing.T) {
	single, err := ParseEmbedsJSON(`{"title":"Hello","description":"World","color":3092790,"fields":[{"name":"A","value":"B","inline":true}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(single) != 1 || single[0].Title != "Hello" || single[0].Color != 3092790 {
		t.Fatalf("single = %#v", single)
	}
	if len(single[0].Fields) != 1 || single[0].Fields[0].Name != "A" {
		t.Fatalf("fields = %#v", single[0].Fields)
	}

	list, err := ParseEmbedsJSON(`[{"title":"One"},{"title":"Two"}]`)
	if err != nil || len(list) != 2 || list[1].Title != "Two" {
		t.Fatalf("list = %#v, %v", list, err)
	}

	if _, err := ParseEmbedsJSON(""); err != nil || list == nil {
		t.Fatalf("empty = %#v, %v", list, err)
	}
	if _, err := ParseEmbedsJSON(`{"title":`); err == nil {
		t.Fatal("broken object accepted")
	}
	if _, err := ParseEmbedsJSON(`[{"title":"A"}, null]`); err == nil {
		t.Fatal("null entry accepted")
	}
	if _, err := ParseEmbedsJSON(`[{}]`); err != nil {
		t.Fatalf("empty embed object rejected: %v", err) // Discord rejects it; ValidateEmbeds decides separately
	}
}

func TestValidateEmbeds(t *testing.T) {
	if reason := ValidateEmbeds(nil); reason != "" {
		t.Fatalf("nil reason = %q", reason)
	}
	oversize := []*domain.DiscordEmbed{{Description: strings.Repeat("x", 4097)}}
	if reason := ValidateEmbeds(oversize); !strings.Contains(reason, "description is 4097") {
		t.Fatalf("oversize reason = %q", reason)
	}
	overTotal := make([]*domain.DiscordEmbed, 0, 3)
	for index := 0; index < 3; index++ {
		overTotal = append(overTotal, &domain.DiscordEmbed{Description: strings.Repeat("x", 2001)})
	}
	if reason := ValidateEmbeds(overTotal); !strings.Contains(reason, "combined limit") {
		t.Fatalf("total reason = %q", reason)
	}
}

func TestPinValuesFallbacks(t *testing.T) {
	document := ParseEmbedDocument(map[string]any{
		"pins": []any{
			map[string]any{"name": "city", "type": "text", "default": "Nowhere"},
			map[string]any{"name": "temp", "type": "number", "default": "20"},
			map[string]any{"name": "hot", "type": "boolean", "default": "true"},
		},
	})
	values := PinValues(document, map[string]any{"city": "Berlin"})
	if !reflect.DeepEqual(values, map[string]any{"city": "Berlin", "temp": float64(20), "hot": true}) {
		t.Fatalf("values = %#v", values)
	}
}

func TestParseEmbedColor(t *testing.T) {
	cases := []struct {
		raw    string
		want   int
		reason string
	}{
		{"", 0, ""},
		{"#5865F2", 0x5865F2, ""},
		{"5865F2", 0x5865F2, ""},
		{"#FFFFFF", 0xFFFFFF, ""},
		{"#12345", 0, "is not a #RRGGBB hex value"},
		{"3092790", 0, "is not a #RRGGBB hex value"},
		{"green", 0, "is not a #RRGGBB hex value"},
	}
	for _, testCase := range cases {
		color, reason := parseEmbedColor(testCase.raw)
		if color != testCase.want || (reason == "") != (testCase.reason == "") {
			t.Errorf("parseEmbedColor(%q) = %d, %q", testCase.raw, color, reason)
		}
	}
}

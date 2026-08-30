package jsonquery

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

// geonamesSource mirrors the geonames.org API payload shape the node is
// most often pointed at.
func geonamesSource() map[string]any {
	return map[string]any{
		"geonames": []any{
			map[string]any{"name": "Saint Petersburg", "lat": 59.9, "lng": 30.25},
			map[string]any{"name": "Moscow", "lat": 55.75, "lng": 37.62},
		},
		"totalResultsCount": 2.0,
	}
}

func queryValue(t *testing.T, path string, source any) any {
	t.Helper()
	result, err := Evaluate(context.Background(), nodes.Invocation{
		Config: map[string]any{"path": path},
		Inputs: map[string]any{"source": source},
	}, nil)
	if err != nil {
		t.Fatalf("Evaluate(%q) error = %v", path, err)
	}
	return result["value"]
}

func TestQueryJSONPathSelectors(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		source any
		want   any
	}{
		{"user's geonames case", "$.geonames[0].lng", geonamesSource(), 30.25},
		{"second element", "$.geonames[1].name", geonamesSource(), "Moscow"},
		{"negative index", "$.geonames[-1].name", geonamesSource(), "Moscow"},
		{"negative index first", "$.geonames[-2].lng", geonamesSource(), 30.25},
		{"root only", "$", geonamesSource(), geonamesSource()},
		{"root with dot", "$.geonames", geonamesSource(), geonamesSource()["geonames"]},
		{"bracket name", "$['geonames'][0]['lng']", geonamesSource(), 30.25},
		{"implicit root", "geonames[0].lng", geonamesSource(), 30.25},
		{"out of range", "$.geonames[5]", geonamesSource(), nil},
		{"unknown key", "$.geonames[0].missing", geonamesSource(), nil},
		{"index into object", "$.geonames[0.lat]", geonamesSource(), nil},
		{"list at root", "$[1]", []any{"a", "b"}, "b"},
		{"scalar at root", "$", 42.0, 42.0},
		{"wildcard over list", "$.geonames[*].lng", geonamesSource(), []any{30.25, 37.62}},
		{"dot wildcard over list", "$.geonames.*.name", geonamesSource(), []any{"Saint Petersburg", "Moscow"}},
		{"wildcard over object sorted", "$.geonames[0].*", geonamesSource(), []any{59.9, 30.25, "Saint Petersburg"}},
		{"slice", "$.geonames[0:2].name", geonamesSource(), []any{"Saint Petersburg", "Moscow"}},
		{"slice open end", "$.geonames[1:].name", geonamesSource(), "Moscow"},
		{"slice with step", "$.geonames[::2].name", geonamesSource(), "Saint Petersburg"},
		{"negative step slice", "$.geonames[::-1].name", geonamesSource(), []any{"Moscow", "Saint Petersburg"}},
		{"empty slice", "$.geonames[5:9].name", geonamesSource(), nil},
		{"union of indexes", "$.geonames[1,0].name", geonamesSource(), []any{"Moscow", "Saint Petersburg"}},
		{"union of names", "$['totalResultsCount','missing']", geonamesSource(), 2.0},
		{"union keeps duplicates", "$.geonames[0,0].lng", geonamesSource(), []any{30.25, 30.25}},
		{"recursive descent", "$..lng", geonamesSource(), []any{30.25, 37.62}},
		{"recursive with index", "$..geonames[0].lng", geonamesSource(), 30.25},
		{"recursive misses nothing", "$..name", geonamesSource(), []any{"Saint Petersburg", "Moscow"}},
		{"quoted key with dot", "$['www.geonames.org']", map[string]any{"www.geonames.org": "ok"}, "ok"},
		{"quoted key with escape", `$['it\'s']`, map[string]any{"it's": "ok"}, "ok"},
		{"double quoted key", `$["totalResultsCount"]`, geonamesSource(), 2.0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := queryValue(t, test.path, test.source)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Query(%q) = %#v, want %#v", test.path, got, test.want)
			}
		})
	}
}

func TestQueryJSONPathFilters(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		source any
		want   any
	}{
		{"numeric greater than", "$.geonames[?(@.lng > 35)].name", geonamesSource(), "Moscow"},
		{"numeric gte", "$.geonames[?(@.lng >= 37.62)].name", geonamesSource(), "Moscow"},
		{"less than", "$.geonames[?(@.lat < 56)].name", geonamesSource(), "Moscow"},
		{"string equality", "$.geonames[?(@.name == 'Moscow')].lng", geonamesSource(), 37.62},
		{"double quoted equality", `$.geonames[?(@.name == "Moscow")].lng`, geonamesSource(), 37.62},
		{"string inequality", "$.geonames[?(@.name != 'Moscow')].lng", geonamesSource(), 30.25},
		{"string ordering", "$.geonames[?(@.name < 'Saint Petersburg')].name", geonamesSource(), "Moscow"},
		{"and chain", "$.geonames[?(@.lng > 35 && @.lat < 60)].name", geonamesSource(), "Moscow"},
		{"or chain", "$.geonames[?(@.name == 'Moscow' || @.lat > 60)].name", geonamesSource(), "Moscow"},
		{"grouped logic", "$.geonames[?((@.lng > 35 && @.lat < 60) || @.name == 'Nobody')].name", geonamesSource(), "Moscow"},
		{"truthiness of key", "$.geonames[?(@.lat)].name", geonamesSource(), []any{"Saint Petersburg", "Moscow"}},
		{"negation", "$.geonames[?(!(@.lng > 35))].name", geonamesSource(), "Saint Petersburg"},
		{"missing key equality", "$.geonames[?(@.missing == 'x')].name", geonamesSource(), nil},
		{"missing key inequality", "$.geonames[?(@.missing != 'x')].name", geonamesSource(), []any{"Saint Petersburg", "Moscow"}},
		{"bare candidate comparison", "$.scores[?(@ > 2)]", map[string]any{"scores": []any{1.0, 3.0, 4.0}}, []any{3.0, 4.0}},
		{"root reference in filter", "$.geonames[?(@.name == $.wanted)].lng", map[string]any{"wanted": "Moscow", "geonames": geonamesSource()["geonames"]}, 37.62},
		{"nested candidate path", "$.geonames[?(@.meta.active == true)].name", map[string]any{"geonames": []any{
			map[string]any{"name": "Saint Petersburg", "meta": map[string]any{"active": false}},
			map[string]any{"name": "Moscow", "meta": map[string]any{"active": true}},
		}}, "Moscow"},
		{"filter over object values", "$.cities[?(@ > 1)]", map[string]any{"cities": map[string]any{"a": 1.0, "b": 2.0, "c": 3.0}}, []any{2.0, 3.0}},
		{"boolean literal comparison", "$.flags[?(@ == false)]", map[string]any{"flags": []any{true, false}}, false},
		{"null literal comparison", "$.list[?(@.v == null)].n", map[string]any{"list": []any{
			map[string]any{"n": "a", "v": "text"},
			map[string]any{"n": "b", "v": nil},
		}}, "b"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := queryValue(t, test.path, test.source)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Query(%q) = %#v, want %#v", test.path, got, test.want)
			}
		})
	}
}

func TestQueryLegacyDottedPathsKeepWorking(t *testing.T) {
	source := geonamesSource()
	if got := queryValue(t, "geonames.0.lng", source); got != 30.25 {
		t.Fatalf("legacy dotted pick = %#v, want 30.25", got)
	}
	if got := queryValue(t, "totalResultsCount", source); got != 2.0 {
		t.Fatalf("legacy flat key pick = %#v, want 2", got)
	}
	if got := queryValue(t, "geonames.5.lng", source); got != nil {
		t.Fatalf("legacy out-of-range pick = %#v, want nil", got)
	}
}

func TestQueryEmptyPathReturnsSource(t *testing.T) {
	source := geonamesSource()
	got := queryValue(t, "   ", source)
	if !reflect.DeepEqual(got, source) {
		t.Fatalf("empty path = %#v, want the source itself", got)
	}
}

func TestQueryInvalidPathsReturnNil(t *testing.T) {
	source := geonamesSource()
	for _, path := range []string{
		"$.geonames[",
		"$.geonames[?]",
		"$.geonames[0",
		"$.",
		"$.geonames[0].",
		"$[abc]",
		"$.geonames[1:2:0]",
		"$.geonames[?(@.lng >)]",
		"$..",
	} {
		if got := queryValue(t, path, source); got != nil {
			t.Fatalf("Query(%q) = %#v, want nil for an unparseable path", path, got)
		}
	}
}

type userRecord struct {
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Nested struct {
		City string `json:"city"`
	} `json:"nested"`
}

func TestQueryWalksStructs(t *testing.T) {
	var record userRecord
	record.Name = "neuro"
	record.Score = 9
	record.Nested.City = "Moscow"
	wrapped := map[string]any{"user": record, "users": []any{record}}

	if got := queryValue(t, "$.user.name", wrapped); got != "neuro" {
		t.Fatalf("struct json tag pick = %#v, want neuro", got)
	}
	if got := queryValue(t, "$.user.Score", wrapped); got != 9 {
		t.Fatalf("struct field pick = %#v, want 9", got)
	}
	if got := queryValue(t, "$.user.nested.city", wrapped); got != "Moscow" {
		t.Fatalf("nested struct pick = %#v, want Moscow", got)
	}
	if got := queryValue(t, "$.users[0].score", wrapped); got != 9 {
		t.Fatalf("struct in list pick = %#v, want 9", got)
	}
	if got := queryValue(t, "$.user.*", wrapped); !reflect.DeepEqual(got, []any{"neuro", 9, record.Nested}) {
		t.Fatalf("struct wildcard = %#v", got)
	}
}

func TestQueryResultShapeFollowsMatchCount(t *testing.T) {
	source := geonamesSource()
	// One match unwraps to the value itself...
	if got := queryValue(t, "$.geonames[?(@.lng > 37)].lng", source); !reflect.DeepEqual(got, 37.62) {
		t.Fatalf("single filter match = %#v, want the bare value 37.62", got)
	}
	// ...several matches come back as a list...
	if got := queryValue(t, "$.geonames[?(@.lng > 0)].lng", source); !reflect.DeepEqual(got, []any{30.25, 37.62}) {
		t.Fatalf("multi filter match = %#v, want a list", got)
	}
	// ...and no match is null.
	if got := queryValue(t, "$.geonames[?(@.lng > 999)]", source); got != nil {
		t.Fatalf("no filter match = %#v, want nil", got)
	}
}

func TestQueryMapIterationIsDeterministic(t *testing.T) {
	// A map big enough that random iteration order would surface in a
	// wildcard pick if the keys were not sorted.
	source := map[string]any{}
	keys := []string{"zulu", "alpha", "mike", "bravo", "charlie", "delta"}
	for index, key := range keys {
		source[key] = float64(index)
	}
	got := queryValue(t, "$.*", source)
	want := []any{1.0, 3.0, 4.0, 5.0, 2.0, 0.0} // alpha, bravo, charlie, delta, mike, zulu
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wildcard over map = %#v, want sorted-by-key values %#v", got, want)
	}
}

func TestQuerySliceNegativeStepOnShortList(t *testing.T) {
	source := map[string]any{"steps": []any{"a", "b", "c", "d"}}
	tests := []struct {
		path string
		want any
	}{
		{"$.steps[::-1]", []any{"d", "c", "b", "a"}},
		{"$.steps[::-2]", []any{"d", "b"}},
		{"$.steps[3:1:-1]", []any{"d", "c"}},
		{"$.steps[-1::-2]", []any{"d", "b"}},
		{"$.steps[1:3]", []any{"b", "c"}},
		{"$.steps[-3:-1]", []any{"b", "c"}},
	}
	for _, test := range tests {
		if got := queryValue(t, test.path, source); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("Query(%q) = %#v, want %#v", test.path, got, test.want)
		}
	}
}

func TestQueryJSONNumberComparisons(t *testing.T) {
	// Numbers may arrive as json.Number, float64, or int depending on where
	// the source value was produced; filters must compare across all of them.
	source := map[string]any{"scores": []any{1.0, 3, json.Number("5")}}
	got := queryValue(t, "$.scores[?(@ > 2)]", source)
	if !reflect.DeepEqual(got, []any{3, json.Number("5")}) {
		t.Fatalf("mixed number filter = %#v", got)
	}
	if got := queryValue(t, "$.scores[?(@ == 5)]", source); !reflect.DeepEqual(got, json.Number("5")) {
		t.Fatalf("json.Number equality = %#v", got)
	}
}

// TestInspectorCopiedPaths is the contract between the execution log's data
// inspector and this node: every JSONPath string the frontend copy-path
// button can emit (frontend/src/lib/jsonPath.ts, mirrored by
// scripts/test-json-path.mts) must resolve through Query exactly as the
// inspector's tree structure implies. If either side changes its path
// dialect, this test breaks first.
func TestInspectorCopiedPaths(t *testing.T) {
	firstItem := map[string]any{"customer": map[string]any{"name": "Ada"}}
	secondItem := map[string]any{
		"customer":    map[string]any{"name": "Bob"},
		"total price": 9.5,
	}
	source := map[string]any{
		"headers":      map[string]any{"authorization": "Bearer token"},
		"items":        []any{firstItem, secondItem},
		"matrix":       []any{[]any{1.0, 2.0}, []any{3.0, 4.0}, []any{5.0, 6.0}, []any{7.0, 8.0}},
		"$schema":      map[string]any{"my_key": 1.0},
		"field2":       map[string]any{"a1b2": true},
		"weird key":    "x",
		"a.b":          map[string]any{"c": "y"},
		"123abc":       3.0,
		"x-request-id": "abc",
		"":             "empty",
		"0":            "zero key",
	}
	tests := []struct {
		name string
		path string
		want any
	}{
		{"root row", "$", source},
		{"dotted object keys", "$.headers.authorization", "Bearer token"},
		{"array index", "$.items[0]", firstItem},
		{"mixed depth", "$.items[1].customer.name", "Bob"},
		{"array of arrays", "$.matrix[3][1]", 8.0},
		{"dollar-prefixed key", "$.$schema.my_key", 1.0},
		{"key with underscore/digits", "$.field2.a1b2", true},
		{"key with space", `$["weird key"]`, "x"},
		{"key containing a dot", `$["a.b"].c`, "y"},
		{"key with leading digit", `$["123abc"]`, 3.0},
		{"key with dash", `$["x-request-id"]`, "abc"},
		{"quoted key after index", `$["items"][1]["total price"]`, 9.5},
		{"empty-string key", `$[""]`, "empty"},
		{"numeric-string key is a name, not an index", `$["0"]`, "zero key"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := queryValue(t, test.path, source); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Query(%q) = %#v, want %#v", test.path, got, test.want)
			}
		})
	}
	// A leading index only ever appears when the root itself is a list
	// (the inspector's tree root), which $[0] addresses directly.
	if got := queryValue(t, "$[0]", []any{"a", "b"}); got != "a" {
		t.Fatalf("Query(%q) = %#v, want %q", "$[0]", got, "a")
	}
}

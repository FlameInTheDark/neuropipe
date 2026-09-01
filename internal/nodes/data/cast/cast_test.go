package cast

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

func castValue(t *testing.T, target string, value any) (any, error) {
	t.Helper()
	result, err := Evaluate(context.Background(), nodes.Invocation{
		Config: map[string]any{"target": target},
		Inputs: map[string]any{"value": value},
	}, nil)
	if err != nil {
		return nil, err
	}
	return result["value"], nil
}

func TestCastObjectPassesObjectsThrough(t *testing.T) {
	object := map[string]any{"name": "neuro", "count": 2.0}
	result, err := castValue(t, "object", object)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !reflect.DeepEqual(result, object) {
		t.Fatalf("object cast = %#v, want the same object", result)
	}
}

func TestCastObjectParsesJSONText(t *testing.T) {
	result, err := castValue(t, "object", `{"name":"neuro","tags":["a"],"nested":{"ok":true}}`)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	want := map[string]any{"name": "neuro", "tags": []any{"a"}, "nested": map[string]any{"ok": true}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("object cast = %#v, want %#v", result, want)
	}
}

func TestCastObjectRejectsNonObjects(t *testing.T) {
	for _, value := range []any{3.0, true, []any{1.0, 2.0}, `[1,2]`, "bare text", nil} {
		if _, err := castValue(t, "object", value); err == nil || !strings.Contains(err.Error(), "cannot cast") {
			t.Fatalf("object cast of %#v error = %v, want cannot cast failure", value, err)
		}
	}
}

func TestCastListPassesListsThrough(t *testing.T) {
	list := []any{"a", 1.0, true}
	result, err := castValue(t, "list", list)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !reflect.DeepEqual(result, list) {
		t.Fatalf("list cast = %#v, want the same list", result)
	}
}

func TestCastListParsesJSONText(t *testing.T) {
	result, err := castValue(t, "list", `[1, "two", {"three": 3}]`)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	want := []any{1.0, "two", map[string]any{"three": 3.0}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("list cast = %#v, want %#v", result, want)
	}
}

func TestCastListRejectsNonLists(t *testing.T) {
	for _, value := range []any{3.0, map[string]any{"a": 1.0}, `{"a":1}`, "bare text", []byte("nope")} {
		if _, err := castValue(t, "list", value); err == nil || !strings.Contains(err.Error(), "cannot cast") {
			t.Fatalf("list cast of %#v error = %v, want cannot cast failure", value, err)
		}
	}
}

func TestCastBytesEncodesText(t *testing.T) {
	result, err := castValue(t, "bytes", "hello")
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	data, ok := result.([]byte)
	if !ok || string(data) != "hello" {
		t.Fatalf("bytes cast = %#v, want []byte(hello)", result)
	}
}

func TestCastBytesPassesBytesThrough(t *testing.T) {
	result, err := castValue(t, "bytes", []byte("raw"))
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !reflect.DeepEqual(result, []byte("raw")) {
		t.Fatalf("bytes cast = %#v, want the same bytes", result)
	}
}

func TestCastBytesRejectsOtherTypes(t *testing.T) {
	for _, value := range []any{3.0, true, map[string]any{}, []any{}} {
		if _, err := castValue(t, "bytes", value); err == nil || !strings.Contains(err.Error(), "cannot cast") {
			t.Fatalf("bytes cast of %#v error = %v, want cannot cast failure", value, err)
		}
	}
}

func TestCastTextSerializesObjectsAsJSON(t *testing.T) {
	result, err := castValue(t, "text", map[string]any{"b": 2.0, "a": "x"})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	encoded, ok := result.(string)
	if !ok {
		t.Fatalf("text cast = %#v, want string", result)
	}
	if encoded != `{"a":"x","b":2}` {
		t.Fatalf("text cast = %q, want compact JSON (keys sorted)", encoded)
	}
}

func TestCastTextSerializesListsAsJSON(t *testing.T) {
	result, err := castValue(t, "text", []any{"a", 1.0})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if got, want := result.(string), `["a",1]`; got != want {
		t.Fatalf("text cast = %q, want %q", got, want)
	}
}

func TestCastTextDecodesBytes(t *testing.T) {
	result, err := castValue(t, "text", []byte("hello"))
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if got, want := result.(string), "hello"; got != want {
		t.Fatalf("text cast = %q, want %q", got, want)
	}
}

func TestCastNumberParsesBytesAndText(t *testing.T) {
	for _, value := range []any{[]byte("42"), "3.5", 7} {
		result, err := castValue(t, "number", value)
		if err != nil {
			t.Fatalf("number cast of %#v error = %v", value, err)
		}
		if number, ok := result.(float64); !ok || number <= 0 {
			t.Fatalf("number cast of %#v = %#v, want a positive float", value, result)
		}
	}
}

func TestCastBooleanParsesText(t *testing.T) {
	result, err := castValue(t, "boolean", "true")
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if result != true {
		t.Fatalf("boolean cast = %#v, want true", result)
	}
}

func TestCastDefaultsToTextWithoutTarget(t *testing.T) {
	result, err := Evaluate(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"value": 12.0},
	}, nil)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if got, want := result["value"], "12"; got != want {
		t.Fatalf("default cast = %#v, want %q", got, want)
	}
}

func TestCastRejectsUnknownTarget(t *testing.T) {
	if _, err := castValue(t, "xml", "x"); err == nil || !strings.Contains(err.Error(), "unknown cast target") {
		t.Fatalf("unknown target error = %v, want unknown cast target", err)
	}
}

type stubRegistrar struct{ node nodes.Node }

func (stub *stubRegistrar) Register(node nodes.Node) error { stub.node = node; return nil }

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	var stub stubRegistrar
	if err := Register(&stub); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	return stub.node
}

func TestRegisterOffersEveryConcreteTarget(t *testing.T) {
	module := registeredModule(t)
	field := module.Definition().Fields[0]
	if field.Name != "target" {
		t.Fatalf("first field = %q, want target", field.Name)
	}
	values := make([]string, 0, len(field.Options))
	for _, option := range field.Options {
		values = append(values, option.Value)
	}
	want := []string{"text", "number", "boolean", "object", "list", "bytes"}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("target options = %v, want %v", values, want)
	}
}

func TestRegisterResolverTypesEveryTarget(t *testing.T) {
	module := registeredModule(t)
	kinds := map[string]domain.TypeKind{
		"text":    domain.TypeString,
		"number":  domain.TypeFloat,
		"boolean": domain.TypeBool,
		"object":  domain.TypeMap,
		"list":    domain.TypeList,
		"bytes":   domain.TypeBytes,
	}
	tests := []struct {
		target   string
		dataType domain.DataType
		color    string
	}{
		{"text", domain.DataText, "#e879f9"},
		{"number", domain.DataNumber, "#86efac"},
		{"boolean", domain.DataBoolean, "#f87171"},
		{"object", domain.DataObject, "#60a5fa"},
		{"list", domain.DataList, "#facc15"},
		{"bytes", domain.DataBytes, "#fbbf24"},
	}
	for _, test := range tests {
		resolved, err := module.Resolve(domain.FlowNode{Type: "data:cast", Data: map[string]any{"config": map[string]any{"target": test.target}}})
		if err != nil {
			t.Fatalf("Resolve(%s) error = %v", test.target, err)
		}
		output := resolved.Outputs[0]
		if output.DataType != test.dataType {
			t.Fatalf("target %s output data type = %q, want %q", test.target, output.DataType, test.dataType)
		}
		if output.Type == nil || output.Type.Kind != kinds[test.target] {
			t.Fatalf("target %s output type = %#v, want kind %s", test.target, output.Type, kinds[test.target])
		}
		if output.Color != test.color {
			t.Fatalf("target %s output color = %q, want %q", test.target, output.Color, test.color)
		}
	}
}

func TestRegisterResolverKeepsAnyWithoutTarget(t *testing.T) {
	module := registeredModule(t)
	resolved, err := module.Resolve(domain.FlowNode{Type: "data:cast", Data: map[string]any{}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if output := resolved.Outputs[0]; output.DataType != domain.DataAny {
		t.Fatalf("output data type = %q, want any for unconfigured nodes", output.DataType)
	}
}

func TestObjectOutputConnectsToFirstPartyObjectPins(t *testing.T) {
	spec := OutputSpec(domain.DataObject)
	if spec.Kind != domain.TypeMap || spec.Key == nil || spec.Key.Kind != domain.TypeString || spec.Value == nil || spec.Value.Kind != domain.TypeAny {
		t.Fatalf("object output spec = %#v, want map<string, any>", spec)
	}
	// The KV Hash Set fields pin, SQL rows elements, and storage entries all
	// declare exactly this contract; the cast output must connect to them.
	key := domain.TypeSpec{Kind: domain.TypeString}
	value := domain.TypeSpec{Kind: domain.TypeAny}
	consumer := domain.TypeSpec{Kind: domain.TypeMap, Key: &key, Value: &value}
	if !typespec.Assignable(spec, consumer) {
		t.Fatal("object cast output is not assignable to a map<string, any> pin")
	}
	listSpec := OutputSpec(domain.DataList)
	listConsumer := typespec.FromDataType(domain.DataList)
	if !typespec.Assignable(listSpec, listConsumer) {
		t.Fatal("list cast output is not assignable to a list<any> pin")
	}
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:cast" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinInput {
		t.Fatalf("value input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "value" || got.DataType != domain.DataAny || got.Direction != domain.PinOutput {
		t.Fatalf("value output = %#v", got)
	}
	if !reflect.DeepEqual(definition.DefaultConfig, map[string]any{"target": "text"}) {
		t.Fatalf("default config = %#v, want target text", definition.DefaultConfig)
	}
}

func TestCastNumberConversionsExact(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  float64
	}{
		{"text float", "3.5", 3.5},
		{"padded text", "  42  ", 42.0},
		{"int passthrough", 7, 7.0},
		{"int64 passthrough", int64(-3), -3.0},
		{"float32 passthrough", float32(2.5), 2.5},
		{"json number text", json.Number("2.5"), 2.5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := castValue(t, "number", test.value)
			if err != nil {
				t.Fatalf("number cast of %#v error = %v", test.value, err)
			}
			if !reflect.DeepEqual(result, test.want) {
				t.Fatalf("number cast of %#v = %#v, want %#v", test.value, result, test.want)
			}
		})
	}
}

func TestCastNumberRejectsNonNumbers(t *testing.T) {
	for _, value := range []any{"nope", true, nil, map[string]any{"a": 1.0}, []any{}} {
		if _, err := castValue(t, "number", value); err == nil || !strings.Contains(err.Error(), "cannot cast") {
			t.Fatalf("number cast of %#v error = %v, want cannot cast failure", value, err)
		}
	}
}

func TestCastBooleanConversions(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{"bool passthrough false", false, false},
		{"uppercase text", "TRUE", true},
		{"one text", "1", true},
		{"zero text", "0", false},
		{"int one renders parseable text", 1, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := castValue(t, "boolean", test.value)
			if err != nil {
				t.Fatalf("boolean cast of %#v error = %v", test.value, err)
			}
			if result != test.want {
				t.Fatalf("boolean cast of %#v = %#v, want %v", test.value, result, test.want)
			}
		})
	}
}

func TestCastBooleanRejectsUnparseableText(t *testing.T) {
	for _, value := range []any{"yes", 2, nil} {
		if _, err := castValue(t, "boolean", value); err == nil || !strings.Contains(err.Error(), "cannot cast") {
			t.Fatalf("boolean cast of %#v error = %v, want cannot cast failure", value, err)
		}
	}
}

func TestCastTextRendersScalarsWithSprint(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"float", 2.5, "2.5"},
		{"whole float drops decimals", 12.0, "12"},
		{"int", 7, "7"},
		{"true", true, "true"},
		{"false", false, "false"},
		{"nil", nil, "<nil>"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := castValue(t, "text", test.value)
			if err != nil {
				t.Fatalf("text cast of %#v error = %v", test.value, err)
			}
			if result != test.want {
				t.Fatalf("text cast of %#v = %#v, want %q", test.value, result, test.want)
			}
		})
	}
}

func TestEvaluateTrimsTarget(t *testing.T) {
	result, err := Evaluate(context.Background(), nodes.Invocation{
		Config: map[string]any{"target": "  number  "},
		Inputs: map[string]any{"value": "5"},
	}, nil)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !reflect.DeepEqual(result["value"], 5.0) {
		t.Fatalf("trimmed target result = %#v, want 5", result["value"])
	}
}

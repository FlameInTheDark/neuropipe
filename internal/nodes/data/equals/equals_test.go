package equals

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func invocation(left, right any) nodes.Invocation {
	return nodes.Invocation{Inputs: map[string]any{"left": left, "right": right}}
}

func TestEqualsComparesNumericKindsByValue(t *testing.T) {
	// An HTTP status arrives as a Go int while a Constant survives a JSON
	// round trip as float64. Both hold the number 200, so Equals must agree.
	tests := []struct {
		name        string
		left, right any
		want        bool
	}{
		{name: "int vs float64", left: 200, right: 200.0, want: true},
		{name: "float64 vs int64", left: 404.0, right: int64(404), want: true},
		{name: "uint vs int", left: uint(7), right: 7, want: true},
		{name: "json.Number vs float", left: json.Number("200"), right: 200.0, want: true},
		{name: "different numbers", left: 200, right: 404.0, want: false},
		{name: "text is not a number", left: "200", right: 200, want: false},
		{name: "equal strings", left: "hello", right: "hello", want: true},
		{name: "different strings", left: "hello", right: "world", want: false},
		{name: "equal booleans", left: true, right: true, want: true},
		{name: "bool vs number", left: true, right: 1, want: false},
		{name: "number vs bool zero", left: 0, right: false, want: false},
		{name: "equal lists", left: []any{1.0, "two"}, right: []any{1.0, "two"}, want: true},
		{name: "distinct lists", left: []any{1.0}, right: []any{1}, want: false},
		{name: "nil vs nil", left: nil, right: nil, want: true},
		{name: "nil vs zero", left: nil, right: 0, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputs, err := Evaluate(context.Background(), invocation(test.left, test.right), nil)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if got := outputs["value"].(bool); got != test.want {
				t.Fatalf("Evaluate(%#v, %#v) = %v, want %v", test.left, test.right, got, test.want)
			}
		})
	}
}

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	module, ok := registry.Get("data:equals")
	if !ok {
		t.Fatal("data:equals was not registered")
	}
	return module
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:equals" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "left" || got.DataType != domain.DataAny || got.Direction != domain.PinInput {
		t.Fatalf("left input = %#v", got)
	}
	if got := definition.Inputs[1]; got.ID != "right" || got.DataType != domain.DataAny || got.Direction != domain.PinInput {
		t.Fatalf("right input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "value" || got.DataType != domain.DataBoolean || got.Direction != domain.PinOutput {
		t.Fatalf("value output = %#v", got)
	}
	if !reflect.DeepEqual(definition.DefaultConfig, map[string]any{}) {
		t.Fatalf("default config = %#v, want empty", definition.DefaultConfig)
	}
}

func TestEqualsComparesStructuralValues(t *testing.T) {
	tests := []struct {
		name        string
		left, right any
		want        bool
	}{
		{"equal objects", map[string]any{"a": 1.0, "b": "x"}, map[string]any{"b": "x", "a": 1.0}, true},
		{"different objects", map[string]any{"a": 1.0}, map[string]any{"a": 2.0}, false},
		{"object versus list", map[string]any{"a": 1.0}, []any{map[string]any{"a": 1.0}}, false},
		{"nested structures equal", map[string]any{"list": []any{1.0, map[string]any{"ok": true}}}, map[string]any{"list": []any{1.0, map[string]any{"ok": true}}}, true},
		{"nested structures differ", map[string]any{"list": []any{1.0}}, map[string]any{"list": []any{1}}, false},
		{"equal bytes", []byte("ab"), []byte("ab"), true},
		{"different bytes", []byte("ab"), []byte("ac"), false},
		{"bytes versus text", []byte("ab"), "ab", false},
		{"float32 equals float64", float32(2.5), 2.5, true},
		{"int8 equals float64", int8(5), 5.0, true},
		{"uint64 equals float64", uint64(9), 9.0, true},
		{"NaN never equals itself", math.NaN(), math.NaN(), false},
		{"unparseable json numbers compare structurally", json.Number("abc"), json.Number("abc"), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputs, err := Evaluate(context.Background(), invocation(test.left, test.right), nil)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if got := outputs["value"].(bool); got != test.want {
				t.Fatalf("Evaluate(%#v, %#v) = %v, want %v", test.left, test.right, got, test.want)
			}
		})
	}
}

package equals

import (
	"context"
	"encoding/json"
	"testing"

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

package base64decode

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestResolveUsesSelectedInputAndOutputTypes(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		config     map[string]any
		inputKind  domain.TypeKind
		outputKind domain.TypeKind
		wantError  bool
	}{
		{name: "defaults", config: map[string]any{}, inputKind: domain.TypeString, outputKind: domain.TypeBytes},
		{name: "bytes to text", config: map[string]any{"inputType": "bytes", "outputType": "text"}, inputKind: domain.TypeBytes, outputKind: domain.TypeString},
		{name: "invalid output type", config: map[string]any{"outputType": "list"}, wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resolved, err := New().Resolve(domain.FlowNode{Data: map[string]any{"config": testCase.config}})
			if (err != nil) != testCase.wantError {
				t.Fatalf("Resolve() error = %v, want error %t", err, testCase.wantError)
			}
			if testCase.wantError {
				return
			}
			if resolved.Inputs[0].Type == nil || resolved.Inputs[0].Type.Kind != testCase.inputKind || resolved.Outputs[0].Type == nil || resolved.Outputs[0].Type.Kind != testCase.outputKind {
				t.Fatalf("resolved contract = %#v", resolved)
			}
		})
	}
}

func TestEvaluateDecodesOnlyTheSelectedRepresentation(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		config    map[string]any
		value     any
		want      any
		wantError bool
	}{
		{name: "default text to bytes", value: "aGVsbG8=", want: []byte("hello")},
		{name: "bytes to text", config: map[string]any{"inputType": "bytes", "outputType": "text"}, value: []byte("aGVsbG8="), want: "hello"},
		{name: "binary text output", config: map[string]any{"outputType": "text"}, value: "/wAB", wantError: true},
		{name: "malformed", value: "not-base64", wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			output, err := evaluate(context.Background(), nodes.Invocation{Definition: New().Definition(), Config: testCase.config, Inputs: map[string]any{"value": testCase.value}}, nil)
			if (err != nil) != testCase.wantError {
				t.Fatalf("evaluate() error = %v, want error %t", err, testCase.wantError)
			}
			if testCase.wantError {
				return
			}
			if got := output["result"]; !sameValue(got, testCase.want) {
				t.Fatalf("result = %#v, want %#v", got, testCase.want)
			}
		})
	}
}

func sameValue(left, right any) bool {
	switch expected := right.(type) {
	case []byte:
		actual, ok := left.([]byte)
		return ok && string(actual) == string(expected)
	default:
		return left == right
	}
}

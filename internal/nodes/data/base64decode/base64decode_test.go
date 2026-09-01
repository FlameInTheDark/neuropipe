package base64decode

import (
	"context"
	"reflect"
	"strings"
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

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	module, ok := registry.Get("data:base64_decode")
	if !ok {
		t.Fatal("data:base64_decode was not registered")
	}
	return module
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:base64_decode" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "value" || got.DataType != domain.DataText || got.Direction != domain.PinInput || !got.Required {
		t.Fatalf("value input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "result" || got.DataType != domain.DataBytes || got.Direction != domain.PinOutput {
		t.Fatalf("result output = %#v", got)
	}
	for index, name := range []string{"inputType", "outputType"} {
		if field := definition.Fields[index]; field.Name != name || field.Kind != "wire-representation" || !field.Required {
			t.Fatalf("field %d = %#v, want the %s wire-representation field", index, field, name)
		}
	}
	if want := map[string]any{"inputType": "text", "outputType": "bytes"}; !reflect.DeepEqual(definition.DefaultConfig, want) {
		t.Fatalf("default config = %#v, want %#v", definition.DefaultConfig, want)
	}
}

func TestExecuteDecodesExactValues(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name   string
		config map[string]any
		value  any
		want   any
	}{
		{"default text to bytes", nil, "aGVsbG8=", []byte("hello")},
		{"unicode text to text", map[string]any{"outputType": "text"}, "0J/RgNC40LLQtdGC", "Привет"},
		{"empty payload", nil, "", []byte{}},
		{"bytes input to bytes output", map[string]any{"inputType": "bytes", "outputType": "bytes"}, []byte("AQI="), []byte{1, 2}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), nodes.Invocation{
				Node:            domain.FlowNode{Type: "data:base64_decode", Data: map[string]any{"config": test.config}},
				Definition:      module.Definition(),
				SchemaVersion:   domain.GraphSchemaV3,
				Config:          test.config,
				Inputs:          map[string]any{"value": test.value},
				ConnectedInputs: map[string]bool{},
			}, nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got := result.Outputs["result"]; !reflect.DeepEqual(got, test.want) {
				t.Fatalf("result = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestExecuteRejectsWrongRepresentations(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name    string
		config  map[string]any
		value   any
		message string
	}{
		{"text value with bytes input", map[string]any{"inputType": "bytes"}, "aGVsbG8=", "value must be bytes"},
		{"bytes value with text input", map[string]any{"inputType": "text"}, []byte("aGVsbG8="), "value must be text"},
		{"missing value with text input", map[string]any{"inputType": "text"}, nil, "value must be text"},
		{"non-string inputType", map[string]any{"inputType": 5.0}, "aGVsbG8=", "inputType must be text"},
		{"unsupported inputType", map[string]any{"inputType": "json"}, "aGVsbG8=", `inputType "json" is unsupported`},
		{"unsupported outputType", map[string]any{"outputType": "list"}, "aGVsbG8=", `outputType "list" is unsupported`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), nodes.Invocation{
				Node:            domain.FlowNode{Type: "data:base64_decode", Data: map[string]any{"config": test.config}},
				Definition:      module.Definition(),
				SchemaVersion:   domain.GraphSchemaV3,
				Config:          test.config,
				Inputs:          map[string]any{"value": test.value},
				ConnectedInputs: map[string]bool{},
			}, nil)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Execute() error = %v, want it to contain %q", err, test.message)
			}
		})
	}
}

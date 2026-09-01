package base64encode

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
		{name: "defaults", config: map[string]any{}, inputKind: domain.TypeBytes, outputKind: domain.TypeString},
		{name: "text to bytes", config: map[string]any{"inputType": "text", "outputType": "bytes"}, inputKind: domain.TypeString, outputKind: domain.TypeBytes},
		{name: "invalid input type", config: map[string]any{"inputType": "number"}, wantError: true},
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

func TestEvaluateEncodesOnlyTheSelectedRepresentation(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		config    map[string]any
		value     any
		want      any
		wantError bool
	}{
		{name: "default bytes to text", value: []byte("hello"), want: "aGVsbG8="},
		{name: "text to bytes", config: map[string]any{"inputType": "text", "outputType": "bytes"}, value: "hello", want: []byte("aGVsbG8=")},
		{name: "wrong input type", value: "hello", wantError: true},
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
	module, ok := registry.Get("data:base64_encode")
	if !ok {
		t.Fatal("data:base64_encode was not registered")
	}
	return module
}

func TestRegisterMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "data:base64_encode" || definition.Mode != domain.NodePure || definition.Category != "Data" {
		t.Fatalf("definition header = %#v", definition)
	}
	if got := definition.Inputs[0]; got.ID != "value" || got.DataType != domain.DataBytes || got.Direction != domain.PinInput || !got.Required {
		t.Fatalf("value input = %#v", got)
	}
	if got := definition.Outputs[0]; got.ID != "result" || got.DataType != domain.DataText || got.Direction != domain.PinOutput {
		t.Fatalf("result output = %#v", got)
	}
	for index, name := range []string{"inputType", "outputType"} {
		if field := definition.Fields[index]; field.Name != name || field.Kind != "wire-representation" || !field.Required {
			t.Fatalf("field %d = %#v, want the %s wire-representation field", index, field, name)
		}
	}
	if want := map[string]any{"inputType": "bytes", "outputType": "text"}; !reflect.DeepEqual(definition.DefaultConfig, want) {
		t.Fatalf("default config = %#v, want %#v", definition.DefaultConfig, want)
	}
}

func TestExecuteEncodesExactValues(t *testing.T) {
	module := registeredModule(t)
	tests := []struct {
		name   string
		config map[string]any
		value  any
		want   any
	}{
		{"default bytes to text", nil, []byte("hello"), "aGVsbG8="},
		{"text input to text output", map[string]any{"inputType": "text", "outputType": "text"}, "hello", "aGVsbG8="},
		{"unicode text to text output", map[string]any{"inputType": "text", "outputType": "text"}, "Привет", "0J/RgNC40LLQtdGC"},
		{"empty bytes", nil, []byte{}, ""},
		{"bytes input to bytes output", map[string]any{"inputType": "bytes", "outputType": "bytes"}, []byte{1, 2}, []byte("AQI=")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), nodes.Invocation{
				Node:            domain.FlowNode{Type: "data:base64_encode", Data: map[string]any{"config": test.config}},
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
		{"text value with bytes input", nil, "hello", "value must be bytes"},
		{"missing value with bytes input", nil, nil, "value must be bytes"},
		{"invalid UTF-8 text", map[string]any{"inputType": "text"}, "\xff\xfe", "text value must be valid UTF-8"},
		{"non-string inputType", map[string]any{"inputType": 5.0}, []byte("hello"), "inputType must be text"},
		{"unsupported inputType", map[string]any{"inputType": "json"}, []byte("hello"), `inputType "json" is unsupported`},
		{"unsupported outputType", map[string]any{"outputType": "list"}, []byte("hello"), `outputType "list" is unsupported`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.Execute(context.Background(), nodes.Invocation{
				Node:            domain.FlowNode{Type: "data:base64_encode", Data: map[string]any{"config": test.config}},
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

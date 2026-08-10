package readfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestExecuteReturnsOnlyTheSelectedRepresentation(t *testing.T) {
	directory := t.TempDir()
	for _, testCase := range []struct {
		name      string
		content   []byte
		config    map[string]any
		want      any
		wantError bool
	}{
		{name: "default bytes", content: []byte{0xff, 0x00, 0x01}, want: []byte{0xff, 0x00, 0x01}},
		{name: "UTF-8 text", content: []byte("hello"), config: map[string]any{"outputType": "text"}, want: "hello"},
		{name: "binary text output", content: []byte{0xff, 0x00, 0x01}, config: map[string]any{"outputType": "text"}, wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(directory, testCase.name+".bin")
			if err := os.WriteFile(path, testCase.content, 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			result, err := New().Execute(context.Background(), nodes.Invocation{Definition: New().Definition(), Config: testCase.config, Inputs: map[string]any{"path": path}}, nil)
			if (err != nil) != testCase.wantError {
				t.Fatalf("Execute() error = %v, want error %t", err, testCase.wantError)
			}
			if testCase.wantError {
				return
			}
			if got := result.Outputs["result"]; !sameValue(got, testCase.want) {
				t.Fatalf("result = %#v, want %#v", got, testCase.want)
			}
		})
	}
}

func TestResolveDeclaresSelectedOutputType(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		config    map[string]any
		wantKind  domain.TypeKind
		wantError bool
	}{
		{name: "defaults", config: map[string]any{}, wantKind: domain.TypeBytes},
		{name: "text", config: map[string]any{"outputType": "text"}, wantKind: domain.TypeString},
		{name: "invalid", config: map[string]any{"outputType": "number"}, wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			definition, err := New().Resolve(domain.FlowNode{Data: map[string]any{"config": testCase.config}})
			if (err != nil) != testCase.wantError {
				t.Fatalf("Resolve() error = %v, want error %t", err, testCase.wantError)
			}
			if testCase.wantError {
				return
			}
			if definition.Outputs[1].Type == nil || definition.Outputs[1].Type.Kind != testCase.wantKind {
				t.Fatalf("Result output = %#v", definition.Outputs[1])
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

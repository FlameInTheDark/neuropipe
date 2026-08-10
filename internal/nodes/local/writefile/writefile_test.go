package writefile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestExecuteWritesOnlyTheSelectedContentRepresentation(t *testing.T) {
	directory := t.TempDir()
	for _, testCase := range []struct {
		name      string
		config    map[string]any
		content   any
		want      []byte
		wantError bool
	}{
		{name: "default text", content: "hello", want: []byte("hello")},
		{name: "bytes", config: map[string]any{"contentType": "bytes"}, content: []byte{0xff, 0x00, 0x01}, want: []byte{0xff, 0x00, 0x01}},
		{name: "wrong input type", config: map[string]any{"contentType": "bytes"}, content: "hello", wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(directory, testCase.name+".bin")
			result, err := New().Execute(context.Background(), nodes.Invocation{Definition: New().Definition(), Config: testCase.config, Inputs: map[string]any{"path": path, "content": testCase.content}}, nil)
			if (err != nil) != testCase.wantError {
				t.Fatalf("Execute() error = %v, want error %t", err, testCase.wantError)
			}
			if testCase.wantError {
				return
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != string(testCase.want) {
				t.Fatalf("written data = %#v, read error = %v, want %#v", data, err, testCase.want)
			}
			output, ok := result.Outputs["result"].(map[string]any)
			if !ok || output["path"] != path || output["written"] != true {
				t.Fatalf("result = %#v", result.Outputs["result"])
			}
		})
	}
}

func TestResolveDeclaresSelectedContentInput(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		config    map[string]any
		wantKind  domain.TypeKind
		wantError bool
	}{
		{name: "defaults", config: map[string]any{}, wantKind: domain.TypeString},
		{name: "bytes", config: map[string]any{"contentType": "bytes"}, wantKind: domain.TypeBytes},
		{name: "invalid", config: map[string]any{"contentType": "number"}, wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			definition, err := New().Resolve(domain.FlowNode{Data: map[string]any{"config": testCase.config}})
			if (err != nil) != testCase.wantError {
				t.Fatalf("Resolve() error = %v, want error %t", err, testCase.wantError)
			}
			if testCase.wantError {
				return
			}
			if definition.Inputs[2].Type == nil || definition.Inputs[2].Type.Kind != testCase.wantKind {
				t.Fatalf("Content input = %#v", definition.Inputs[2])
			}
		})
	}
}

package pathops

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestGetPathPartSplitsCorrectly(t *testing.T) {
	module := NewGetPathPart()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": "/Work/file.txt"},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out, ok := result.Outputs["result"].(map[string]any)
	if !ok {
		t.Fatalf("result not a record: %#v", result.Outputs["result"])
	}
	if got, want := out["ext"], ".txt"; got != want {
		t.Fatalf("ext = %q, want %q", got, want)
	}
	if got, want := out["name"], "file"; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
}

func TestBuildPathJoinsListItems(t *testing.T) {
	module := NewBuildPath()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"parts": []any{"C:\\Work", "subfolder", "file.txt"}},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["result"]; got == "" {
		t.Fatalf("expected non-empty joined path, got %q", got)
	}
}

func TestCleanPathNormalizes(t *testing.T) {
	module := NewCleanPath()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": "C:\\Work\\..\\Work\\.\\file.txt"},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["result"]; got == "" {
		t.Fatalf("expected non-empty cleaned path")
	}
}

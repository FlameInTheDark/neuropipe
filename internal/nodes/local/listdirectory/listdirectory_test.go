package listdirectory

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

func TestNewDeclaresTypedDirectoryEntries(t *testing.T) {
	definition := New().Definition()
	if definition.Type != "action:list_directory" || definition.Mode != domain.NodeImpure {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Capabilities) != 1 || definition.Capabilities[0] != domain.CapabilityFileRead {
		t.Fatalf("capabilities = %#v, want file-read", definition.Capabilities)
	}
	var result domain.NodePort
	for _, pin := range definition.Outputs {
		if pin.ID == "result" {
			result = pin
		}
	}
	if result.Label != "Files" || result.Type == nil || result.Type.Kind != domain.TypeList || result.Type.Element == nil || result.Type.Element.Kind != domain.TypeRecord {
		t.Fatalf("Files output = %#v", result)
	}
	if err := typespec.ValidateSpec(*result.Type); err != nil {
		t.Fatalf("Files output type = %v", err)
	}
}

func TestExecuteListsDirectoryEntries(t *testing.T) {
	directory := t.TempDir()
	filePath := filepath.Join(directory, "alpha.txt")
	if err := os.WriteFile(filePath, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	updatedAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(filePath, updatedAt, updatedAt); err != nil {
		t.Fatalf("set fixture time: %v", err)
	}
	if err := os.Mkdir(filepath.Join(directory, "bravo"), 0o700); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}

	result, err := New().Execute(context.Background(), nodes.Invocation{Inputs: map[string]any{"path": directory}}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := result.Ports, []string{"out"}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("Ports = %#v, want %#v", got, want)
	}
	files, ok := result.Outputs["result"].([]any)
	if !ok {
		t.Fatalf("Files output type = %T, want []any", result.Outputs["result"])
	}
	definition := New().Definition()
	if err := typespec.ValidateValue(files, *definition.Outputs[1].Type); err != nil {
		t.Fatalf("Files output violates its contract: %v", err)
	}
	byName := make(map[string]map[string]any, len(files))
	for _, file := range files {
		item, ok := file.(map[string]any)
		if !ok {
			t.Fatalf("file entry type = %T, want map[string]any", file)
		}
		byName[item["name"].(string)] = item
	}
	file := byName["alpha.txt"]
	if file["path"] != filePath || file["size"] != int64(3) || file["type"] != "file" || file["updatedAt"] != updatedAt.Format(time.RFC3339Nano) {
		t.Fatalf("file record = %#v", file)
	}
	if runtime.GOOS == "windows" {
		if _, ok := file["createdAt"].(string); !ok {
			t.Fatalf("Windows file record does not expose creation time: %#v", file)
		}
	}
	folder := byName["bravo"]
	if folder["type"] != "directory" {
		t.Fatalf("folder record type = %v, want directory", folder["type"])
	}
}

func TestExecuteRejectsInvalidOrCancelledCalls(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, testCase := range []struct {
		name      string
		ctx       context.Context
		path      string
		wantError bool
	}{
		{name: "missing path", ctx: context.Background(), wantError: true},
		{name: "cancelled", ctx: cancelled, path: t.TempDir(), wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := New().Execute(testCase.ctx, nodes.Invocation{Inputs: map[string]any{"path": testCase.path}}, nil)
			if (err != nil) != testCase.wantError {
				t.Fatalf("Execute() error = %v, want error %t", err, testCase.wantError)
			}
		})
	}
}

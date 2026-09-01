package zipfiles

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("register: %v", err)
	}
	module, ok := registry.Get("action:zip_files")
	if !ok {
		t.Fatal("action:zip_files was not registered")
	}
	return module
}

func invocation(definition domain.NodeDefinition, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:zip_files", Data: map[string]any{"config": map[string]any{}}},
		Definition:      definition,
		SchemaVersion:   3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func assertPinIDs(t *testing.T, ports []domain.NodePort, want []string) {
	t.Helper()
	got := make([]string, 0, len(ports))
	for _, port := range ports {
		got = append(got, port.ID)
	}
	if len(got) != len(want) {
		t.Fatalf("pin ids = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("pin ids = %v, want %v", got, want)
		}
	}
}

func writeSourceFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// zipEntries reads every entry of an archive so tests can assert names,
// directory entries, and uncompressed contents.
func zipEntries(t *testing.T, archivePath string) map[string]string {
	t.Helper()
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open zip %q: %v", archivePath, err)
	}
	defer reader.Close()
	entries := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			entries[file.Name] = ""
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open entry %q: %v", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		closeErr := rc.Close()
		if err != nil {
			t.Fatalf("read entry %q: %v", file.Name, err)
		}
		if closeErr != nil {
			t.Fatalf("close entry %q: %v", file.Name, closeErr)
		}
		entries[file.Name] = string(data)
	}
	return entries
}

func TestRegistrationMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "action:zip_files" || definition.Mode != domain.NodeImpure || definition.Category != "Archive" {
		t.Fatalf("definition = %#v", definition)
	}
	assertPinIDs(t, definition.Inputs, []string{"in", "paths", "targetDirectory", "archiveName"})
	assertPinIDs(t, definition.Outputs, []string{"out", "result"})
}

func TestZipsFilesAndReportsResult(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "src")
	target := filepath.Join(directory, "out")
	writeSourceFile(t, filepath.Join(source, "a.txt"), "alpha")
	writeSourceFile(t, filepath.Join(source, "b.txt"), "beta")

	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"paths":           filepath.Join(source, "a.txt") + ";" + filepath.Join(source, "b.txt"),
		"targetDirectory": target,
		"archiveName":     "bundle",
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	// The .zip suffix is appended when missing.
	archivePath := filepath.Join(target, "bundle.zip")
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["archivePath"] != archivePath || output["entryCount"] != int64(2) || output["bytesWritten"] != int64(len("alpha")+len("beta")) {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	entries := zipEntries(t, archivePath)
	if entries["a.txt"] != "alpha" || entries["b.txt"] != "beta" || len(entries) != 2 {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestZipsDirectoryRecursively(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "tree")
	target := filepath.Join(directory, "out")
	writeSourceFile(t, filepath.Join(source, "top.txt"), "top")
	writeSourceFile(t, filepath.Join(source, "sub", "nested.txt"), "nested")

	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"paths":           source,
		"targetDirectory": target,
		"archiveName":     "tree.zip",
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["entryCount"] != int64(2) || output["bytesWritten"] != int64(len("top")+len("nested")) {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	// Directory entries use forward slashes for cross-platform archives and
	// directory entries themselves are not counted as files.
	entries := zipEntries(t, filepath.Join(target, "tree.zip"))
	if entries["top.txt"] != "top" || entries["sub/nested.txt"] != "nested" || entries["sub/"] != "" {
		t.Fatalf("entries = %#v", entries)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %#v", entries)
	}
}

// Two sources with the same base name must not collide inside the archive:
// the second entry is renamed with an " (1)" suffix.
func TestDuplicateEntryNamesAreRenumbered(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "one")
	second := filepath.Join(directory, "two")
	target := filepath.Join(directory, "out")
	writeSourceFile(t, filepath.Join(first, "a.txt"), "first")
	writeSourceFile(t, filepath.Join(second, "a.txt"), "second")

	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"paths":           filepath.Join(first, "a.txt") + ";" + filepath.Join(second, "a.txt"),
		"targetDirectory": target,
		"archiveName":     "dup",
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if output, ok := result.Outputs["result"].(map[string]any); !ok || output["entryCount"] != int64(2) {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	entries := zipEntries(t, filepath.Join(target, "dup.zip"))
	if entries["a.txt"] != "first" || entries["a (1).txt"] != "second" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestValidationErrors(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "src")
	writeSourceFile(t, filepath.Join(source, "a.txt"), "alpha")
	target := filepath.Join(directory, "out")

	module := registeredModule(t)
	definition := module.Definition()
	cases := []struct {
		name   string
		inputs map[string]any
		want   string
	}{
		{"missing paths", map[string]any{"targetDirectory": target, "archiveName": "a"}, "zip files: at least one source path is required"},
		{"paths of empty entries", map[string]any{"paths": " ; ; ", "targetDirectory": target, "archiveName": "a"}, "zip files: at least one source path is required"},
		{"missing target directory", map[string]any{"paths": source, "archiveName": "a"}, "zip files target directory: path is required"},
		{"missing archive name", map[string]any{"paths": source, "targetDirectory": target}, "zip files archive name: path is required"},
		{"archive name with separator", map[string]any{"paths": source, "targetDirectory": target, "archiveName": filepath.Join("nested", "bundle")}, "archive name must not contain path separators"},
		{"archive is its own source", map[string]any{"paths": filepath.Join(target, "self.zip"), "targetDirectory": target, "archiveName": "self"}, "is one of its own sources"},
		{"missing source file", map[string]any{"paths": filepath.Join(directory, "definitely-missing.txt"), "targetDirectory": target, "archiveName": "a"}, "stat"},
	}
	for _, testCase := range cases {
		_, err := module.Execute(context.Background(), invocation(definition, testCase.inputs), nil)
		if err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("%s: err = %v, want %q", testCase.name, err, testCase.want)
		}
	}
}

func TestCancelledContextFails(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "src")
	target := filepath.Join(directory, "out")
	writeSourceFile(t, filepath.Join(source, "a.txt"), "alpha")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	module := registeredModule(t)
	_, err := module.Execute(ctx, invocation(module.Definition(), map[string]any{
		"paths": filepath.Join(source, "a.txt"), "targetDirectory": target, "archiveName": "a",
	}), nil)
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("err = %v", err)
	}
}

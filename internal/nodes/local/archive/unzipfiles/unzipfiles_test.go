package unzipfiles

import (
	"archive/zip"
	"context"
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
	module, ok := registry.Get("action:unzip_files")
	if !ok {
		t.Fatal("action:unzip_files was not registered")
	}
	return module
}

func invocation(definition domain.NodeDefinition, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:unzip_files", Data: map[string]any{"config": config}},
		Definition:      definition,
		SchemaVersion:   3,
		Config:          config,
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

type zipEntry struct {
	name    string
	content string
}

// writeZip builds an archive with the given entries in order. Entry names are
// written verbatim so hostile paths (zip-slip) can be crafted deliberately.
func writeZip(t *testing.T, archivePath string, entries []zipEntry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o700); err != nil {
		t.Fatal(err)
	}
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	writer := zip.NewWriter(archive)
	for _, entry := range entries {
		file, err := writer.Create(entry.name)
		if err != nil {
			t.Fatalf("create entry %q: %v", entry.name, err)
		}
		if _, err := file.Write([]byte(entry.content)); err != nil {
			t.Fatalf("write entry %q: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return string(data)
}

func TestRegistrationMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "action:unzip_files" || definition.Mode != domain.NodeImpure || definition.Category != "Archive" {
		t.Fatalf("definition = %#v", definition)
	}
	assertPinIDs(t, definition.Inputs, []string{"in", "archivePath", "targetDirectory", "overwrite"})
	assertPinIDs(t, definition.Outputs, []string{"out", "result"})
}

func TestExtractsArchiveWithNestedEntries(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "bundle.zip")
	target := filepath.Join(directory, "out")
	writeZip(t, archivePath, []zipEntry{
		{name: "a.txt", content: "alpha"},
		{name: "sub/b.txt", content: "beta"},
	})

	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, map[string]any{
		"archivePath": archivePath, "targetDirectory": target,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	files, ok := output["extractedFiles"].([]string)
	if !ok || len(files) != 2 || files[0] != "a.txt" || files[1] != filepath.FromSlash("sub/b.txt") {
		t.Fatalf("extractedFiles = %#v", output["extractedFiles"])
	}
	if output["entryCount"] != int64(2) {
		t.Fatalf("entryCount = %#v", output["entryCount"])
	}
	if readFile(t, filepath.Join(target, "a.txt")) != "alpha" || readFile(t, filepath.Join(target, "sub", "b.txt")) != "beta" {
		t.Fatal("extracted contents mismatch")
	}
}

func TestOverwriteFalseSkipsExistingFiles(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "bundle.zip")
	target := filepath.Join(directory, "out")
	writeZip(t, archivePath, []zipEntry{
		{name: "exists.txt", content: "new"},
		{name: "fresh.txt", content: "new"},
	})
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "exists.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, map[string]any{
		"archivePath": archivePath, "targetDirectory": target, "overwrite": false,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	files, ok := output["extractedFiles"].([]string)
	if !ok || len(files) != 1 || files[0] != "fresh.txt" {
		t.Fatalf("extractedFiles = %#v", output["extractedFiles"])
	}
	if output["entryCount"] != int64(1) {
		t.Fatalf("entryCount = %#v", output["entryCount"])
	}
	if readFile(t, filepath.Join(target, "exists.txt")) != "old" {
		t.Fatal("overwrite=false must leave the existing file untouched")
	}
	if readFile(t, filepath.Join(target, "fresh.txt")) != "new" {
		t.Fatal("overwrite=false must still extract missing files")
	}
}

// Without an explicit overwrite pin the node defaults to overwriting.
func TestOverwriteDefaultsToTrue(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "bundle.zip")
	target := filepath.Join(directory, "out")
	writeZip(t, archivePath, []zipEntry{{name: "a.txt", content: "new"}})
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "a.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, map[string]any{
		"archivePath": archivePath, "targetDirectory": target,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["entryCount"] != int64(1) {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	if readFile(t, filepath.Join(target, "a.txt")) != "new" {
		t.Fatal("default overwrite must replace the existing file")
	}
}

// A wired overwrite pin takes precedence over the inspector config.
func TestOverwritePinOverridesConfig(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "bundle.zip")
	target := filepath.Join(directory, "out")
	writeZip(t, archivePath, []zipEntry{{name: "a.txt", content: "new"}})
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "a.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{"overwrite": true}, map[string]any{
		"archivePath": archivePath, "targetDirectory": target, "overwrite": false,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, filepath.Join(target, "a.txt")) != "old" {
		t.Fatal("pinned overwrite=false must beat a config overwrite=true")
	}
}

func TestZipSlipEntriesAreRefused(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "out")
	module := registeredModule(t)
	definition := module.Definition()
	for _, entry := range []string{"../evil.txt", "/absolute/evil.txt", "sub/../../evil.txt"} {
		archivePath := filepath.Join(directory, "hostile.zip")
		writeZip(t, archivePath, []zipEntry{{name: entry, content: "malicious"}})
		_, err := module.Execute(context.Background(), invocation(definition, map[string]any{}, map[string]any{
			"archivePath": archivePath, "targetDirectory": target,
		}), nil)
		if err == nil || !strings.Contains(err.Error(), "refuse unsafe entry path") {
			t.Fatalf("entry %q: err = %v", entry, err)
		}
		if _, err := os.Stat(filepath.Join(directory, "evil.txt")); !os.IsNotExist(err) {
			t.Fatalf("entry %q escaped the target directory", entry)
		}
	}
}

func TestValidationErrors(t *testing.T) {
	directory := t.TempDir()
	module := registeredModule(t)
	definition := module.Definition()
	cases := []struct {
		name   string
		inputs map[string]any
		want   string
	}{
		{"missing archive path", map[string]any{"targetDirectory": filepath.Join(directory, "out")}, "unzip files archive: path is required"},
		{"missing target directory", map[string]any{"archivePath": filepath.Join(directory, "missing.zip")}, "unzip files target: path is required"},
		{"missing archive file", map[string]any{"archivePath": filepath.Join(directory, "definitely-missing.zip"), "targetDirectory": filepath.Join(directory, "out")}, "open archive"},
	}
	for _, testCase := range cases {
		_, err := module.Execute(context.Background(), invocation(definition, map[string]any{}, testCase.inputs), nil)
		if err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("%s: err = %v, want %q", testCase.name, err, testCase.want)
		}
	}
}

func TestEmptyArchiveExtractsNothing(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "empty.zip")
	target := filepath.Join(directory, "out")
	writeZip(t, archivePath, nil)

	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, map[string]any{
		"archivePath": archivePath, "targetDirectory": target,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	files, ok := output["extractedFiles"].([]string)
	if !ok || len(files) != 0 {
		t.Fatalf("extractedFiles = %#v, want an empty list", output["extractedFiles"])
	}
	if output["entryCount"] != int64(0) {
		t.Fatalf("entryCount = %#v", output["entryCount"])
	}
}

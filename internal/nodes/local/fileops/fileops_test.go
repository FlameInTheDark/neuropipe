package fileops_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/base64tofile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/copyfile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/copyfolder"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/deletefile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/deletefolder"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/fileexists"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/filetobase64"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/folderexists"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/movefile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/renamefile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/renamefolder"
)

func TestFileExistsReportsTrueForRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(path, []byte("hi"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	module := fileexists.New()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": path},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Outputs["result"].(bool) {
		t.Fatalf("expected file_exists to return true for an existing regular file")
	}
}

func TestFileExistsReportsFalseForDirectory(t *testing.T) {
	dir := t.TempDir()
	module := fileexists.New()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": dir},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["result"].(bool) {
		t.Fatalf("expected file_exists to return false for a directory")
	}
}

func TestFolderExistsReportsTrueForDirectory(t *testing.T) {
	dir := t.TempDir()
	module := folderexists.New()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": dir},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Outputs["result"].(bool) {
		t.Fatalf("expected folder_exists to return true for an existing directory")
	}
}

func TestCopyFileWritesIdenticalContent(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "src.txt")
	target := filepath.Join(dir, "out", "dst.txt")
	if err := os.WriteFile(source, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	module := copyfile.New()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{
			"sourcePath": source,
			"targetPath": target,
			"overwrite":  true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["result"] == nil {
		t.Fatal("expected result object")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("target content = %q, want %q", string(data), "payload")
	}
}

func TestCopyFolderRecursivelyCopiesAllFiles(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "src")
	target := filepath.Join(dir, "dst")
	nested := filepath.Join(source, "sub", "deep")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "top.txt"), []byte("top"), 0o600); err != nil {
		t.Fatalf("write top: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "deep.txt"), []byte("deep"), 0o600); err != nil {
		t.Fatalf("write deep: %v", err)
	}
	module := copyfolder.New()
	if _, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{
			"sourcePath": source,
			"targetPath": target,
			"overwrite":  true,
		},
	}, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "sub", "deep", "deep.txt")); err != nil {
		t.Fatalf("expected nested file in target, got %v", err)
	}
}

func TestMoveFileAtomicallyMovesFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "src.txt")
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(source, []byte("data"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	module := movefile.New()
	if _, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"sourcePath": source, "targetPath": target},
	}, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source should be gone after move, got err=%v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "data" {
		t.Fatalf("target content wrong; err=%v data=%q", err, string(data))
	}
}

func TestDeleteFileSilentlySkipsMissingFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.txt")
	module := deletefile.New()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": missing, "failIfMissing": false},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["result"].(map[string]any)["deleted"].(bool) {
		t.Fatalf("expected deleted=false when file was missing")
	}
}

func TestDeleteFileFailsWhenMissingAndFailFlagSet(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.txt")
	module := deletefile.New()
	if _, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": missing, "failIfMissing": true},
	}, nil); err == nil {
		t.Fatalf("expected error when failIfMissing=true and file is missing")
	}
}

func TestDeleteFolderRemovesTree(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "tree")
	nested := filepath.Join(target, "sub")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "x.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	module := deletefolder.New()
	if _, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": target, "failIfMissing": true},
	}, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected folder to be gone after delete, err=%v", err)
	}
}

func TestRenameFileRejectsPathSeparators(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(source, []byte("x"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	module := renamefile.New()
	if _, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": source, "newName": "sub/escape.txt"},
	}, nil); err == nil {
		t.Fatalf("expected error when newName contains a separator")
	}
}

func TestRenameFolderMovesWithinSameParent(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old")
	newPath := filepath.Join(dir, "new")
	if err := os.MkdirAll(filepath.Join(oldPath, "sub"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldPath, "sub", "x.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	module := renamefolder.New()
	if _, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": oldPath, "newName": "new"},
	}, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new path should exist after rename, err=%v", err)
	}
}

func TestFileToBase64ProducesStandardBase64(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	module := filetobase64.New()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": path},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["result"] == "" {
		t.Fatalf("expected non-empty Base64 result")
	}
}

func TestBase64ToFileWritesDecodedBytes(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.bin")
	module := base64tofile.New()
	if _, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{
			"base64": "cGF5bG9hZA==", // "payload"
			"path":   target,
		},
	}, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("decoded = %q, want %q", string(data), "payload")
	}
}

// Ensure shared helpers compile and basic operations work as expected.
func TestSplitSemicolonListSkipsEmpty(t *testing.T) {
	got := fileops.SplitSemicolonList("a;;b;")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("SplitSemicolonList = %#v", got)
	}
}

func TestValidateNameRejectsSeparators(t *testing.T) {
	if err := fileops.ValidateName("sub/file.txt"); err == nil {
		t.Fatalf("expected separator to be rejected")
	}
	if err := fileops.ValidateName(".."); err == nil {
		t.Fatalf("expected '..' to be rejected")
	}
	if err := fileops.ValidateName("ok.txt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

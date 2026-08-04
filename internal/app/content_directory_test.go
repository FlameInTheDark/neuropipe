package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeContentDirectory(t *testing.T) {
	t.Parallel()
	dataRoot := t.TempDir()

	contentDirectory, err := normalizeContentDirectory("", dataRoot)
	if err != nil {
		t.Fatalf("normalize default content directory: %v", err)
	}
	want := filepath.Join(dataRoot, "content")
	if contentDirectory != want {
		t.Fatalf("content directory = %q, want %q", contentDirectory, want)
	}
	if info, err := os.Stat(contentDirectory); err != nil || !info.IsDir() {
		t.Fatalf("default content directory was not created: info=%v err=%v", info, err)
	}
}

func TestNormalizeContentDirectoryRejectsInvalidLocations(t *testing.T) {
	t.Parallel()
	dataRoot := t.TempDir()
	file := filepath.Join(dataRoot, "not-a-directory")
	if err := os.WriteFile(file, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, value := range []string{"relative-content", file} {
		if _, err := normalizeContentDirectory(value, dataRoot); err == nil {
			t.Errorf("normalizeContentDirectory(%q) succeeded, want error", value)
		}
	}
}

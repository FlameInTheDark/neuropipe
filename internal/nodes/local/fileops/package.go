// Package fileops provides shared helpers for filesystem Blueprint nodes.
package fileops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrEmptyPath signals the caller passed an empty path to a file/folder node.
var ErrEmptyPath = errors.New("path is required")

// CleanPath trims surrounding whitespace, normalizes separators, and rejects
// empty inputs. It never touches the filesystem.
func CleanPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", ErrEmptyPath
	}
	return filepath.Clean(path), nil
}

// CleanInput extracts a string from a connected or configured value and
// returns its cleaned form. The error wraps CleanPath's so nodes can surface
// a single "path is required" message.
func CleanInput(value any) (string, error) {
	raw, _ := value.(string)
	return CleanPath(raw)
}

// EnsureParentDir creates the parent directory of path with conservative
// permissions if it does not yet exist.
func EnsureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	return nil
}

// SplitSemicolonList parses a ;-separated list of paths. Empty entries are
// skipped so "a;;b" is treated as ["a","b"].
func SplitSemicolonList(raw string) []string {
	var result []string
	for _, part := range strings.Split(raw, ";") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, filepath.Clean(trimmed))
		}
	}
	return result
}

// PathExists reports whether a path exists on the filesystem. Errors other
// than os.ErrNotExist are surfaced to the caller.
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// IsFile reports whether path is a regular file (or a symlink to one).
func IsFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return info.Mode().IsRegular(), nil
}

// IsDir reports whether path is a directory.
func IsDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

// IsPathInTree reports whether candidate is inside root. Both must be cleaned.
// Used to detect zip-slip and similar attacks on extraction.
func IsPathInTree(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ValidateName checks that name is a single path segment (no separators, no
// parent traversal). It rejects names such as ".." or names containing a
// separator, which would otherwise let a Rename node cross directories.
func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name is required")
	}
	if strings.ContainsAny(name, `/\`) {
		return errors.New("name must not contain path separators")
	}
	if name == "." || name == ".." {
		return errors.New("name must not be . or ..")
	}
	return nil
}

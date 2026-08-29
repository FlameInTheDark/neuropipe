package storages

import (
	"errors"
	"path"
	"strings"
)

// ErrInvalidPath reports a remote path that could escape its storage root or
// is otherwise malformed.
var ErrInvalidPath = errors.New("path must not contain \"..\" segments")

// CleanRemotePath normalizes a storage path: forward-slash separated, no
// leading or trailing slash, no empty, "." or ".." segments. The empty string
// is the storage root. Windows-style backslashes are treated as separators
// so pasted local paths still resolve sensibly.
func CleanRemotePath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" || trimmed == "." {
		return "", nil
	}
	parts := make([]string, 0, 8)
	for _, segment := range strings.Split(trimmed, "/") {
		switch segment {
		case "", ".":
			continue
		case "..":
			return "", ErrInvalidPath
		default:
			parts = append(parts, segment)
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, "/"), nil
}

// remotePrefix returns the listing prefix for a directory path: "" for the
// root, otherwise the path with a trailing slash.
func remotePrefix(dir string) string {
	if dir == "" {
		return ""
	}
	return dir + "/"
}

// entryPath joins a directory path and one entry name.
func entryPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// baseName returns the last segment of a cleaned remote path.
func baseName(p string) string {
	return path.Base(p)
}

// Package zipfiles registers the Zip Files Blueprint node.
package zipfiles

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node {
	return Node{Metadata: definition(), Executor: execute}
}

func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	pathType := typespec.String()
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "archivePath", Name: "archivePath", Type: typespec.String()},
		{ID: "entryCount", Name: "entryCount", Type: typespec.Int()},
		{ID: "bytesWritten", Name: "bytesWritten", Type: typespec.Int()},
	}}
	return domain.NodeDefinition{
		Type:        "action:zip_files",
		Category:    "Archive",
		Label:       "Zip Files",
		Description: "Compress one or more files or folders into a local .zip archive.",
		Icon:        "file-archive",
		Color:       "#fbbf24",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "paths", Label: "Paths", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "targetDirectory", Label: "Target directory", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "archiveName", Label: "Archive name", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "paths", Label: "Paths (;-separated)", Kind: "textarea", Placeholder: "C:\\Work\\file1.txt;C:\\Work\\folder", Required: true},
			{Name: "targetDirectory", Label: "Target directory", Kind: "string", Placeholder: "C:\\Work\\out", Required: true},
			{Name: "archiveName", Label: "Archive name", Kind: "string", Placeholder: "archive", Required: true},
		},
		Capabilities:      []domain.Capability{domain.CapabilityFileRead, domain.CapabilityFileWrite},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	rawPaths, _ := invocation.Inputs["paths"].(string)
	paths := fileops.SplitSemicolonList(rawPaths)
	if len(paths) == 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("zip files: at least one source path is required")
	}
	targetDir, err := fileops.CleanInput(invocation.Inputs["targetDirectory"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("zip files target directory: %w", err)
	}
	archiveName, err := fileops.CleanInput(invocation.Inputs["archiveName"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("zip files archive name: %w", err)
	}
	// Append .zip if missing; reject path separators in archive name.
	if !strings.HasSuffix(strings.ToLower(archiveName), ".zip") {
		archiveName += ".zip"
	}
	if strings.ContainsAny(archiveName, `/\`) {
		return nodes.ExecutionResult{}, fmt.Errorf("archive name must not contain path separators")
	}
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create target directory: %w", err)
	}
	archivePath := filepath.Join(targetDir, archiveName)
	// Refuse to write a zip into one of its own sources (would loop forever).
	archiveAbs, err := filepath.Abs(archivePath)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("resolve archive path: %w", err)
	}
	for _, source := range paths {
		if sourceAbs, err := filepath.Abs(source); err == nil && sourceAbs == archiveAbs {
			return nodes.ExecutionResult{}, fmt.Errorf("archive path %q is one of its own sources", archivePath)
		}
	}

	zipFile, err := os.OpenFile(archivePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create archive: %w", err)
	}
	defer zipFile.Close()
	zipWriter := zip.NewWriter(zipFile)

	entryCount := 0
	bytesWritten := int64(0)
	used := make(map[string]struct{})
	for _, source := range paths {
		if err := ctx.Err(); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("zip files cancelled: %w", err)
		}
		info, err := os.Stat(source)
		if err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("stat %q: %w", source, err)
		}
		if info.IsDir() {
			count, written, err := addDirectory(ctx, zipWriter, source, used)
			if err != nil {
				return nodes.ExecutionResult{}, err
			}
			entryCount += count
			bytesWritten += written
			continue
		}
		name := filepath.Base(source)
		written, err := addFile(ctx, zipWriter, source, name, used)
		if err != nil {
			return nodes.ExecutionResult{}, err
		}
		entryCount++
		bytesWritten += written
	}
	if err := zipWriter.Close(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("close zip writer: %w", err)
	}
	if err := zipFile.Sync(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("sync archive: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"result": map[string]any{
				"archivePath":  archivePath,
				"entryCount":   int64(entryCount),
				"bytesWritten": bytesWritten,
			},
		},
		Ports: []string{"out"},
	}, nil
}

func addDirectory(ctx context.Context, zw *zip.Writer, root string, used map[string]struct{}) (int, int64, error) {
	var entryCount int
	var bytesWritten int64
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Use forward slashes for cross-platform zip compatibility.
		rel = filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := rel + "/"
			if uniqueName, ok := uniqueEntry(name, used); ok {
				if _, err := zw.Create(uniqueName); err != nil {
					return fmt.Errorf("create directory entry %q: %w", name, err)
				}
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Skip symlinks to avoid following them during archiving.
			return nil
		}
		written, err := addFile(ctx, zw, path, rel, used)
		if err != nil {
			return err
		}
		entryCount++
		bytesWritten += written
		return nil
	})
	if walkErr != nil {
		return 0, 0, fmt.Errorf("walk directory %q: %w", root, walkErr)
	}
	return entryCount, bytesWritten, nil
}

func addFile(ctx context.Context, zw *zip.Writer, source, name string, used map[string]struct{}) (int64, error) {
	entryName := uniqueNameOrSuffix(name, used)
	writer, err := zw.Create(entryName)
	if err != nil {
		return 0, fmt.Errorf("create zip entry %q: %w", name, err)
	}
	file, err := os.Open(source)
	if err != nil {
		return 0, fmt.Errorf("open source %q: %w", source, err)
	}
	defer file.Close()
	copied, err := io.Copy(writer, file)
	if err != nil {
		return 0, fmt.Errorf("write zip entry %q: %w", name, err)
	}
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("zip files cancelled: %w", err)
	}
	return copied, nil
}

func uniqueEntry(name string, used map[string]struct{}) (string, bool) {
	if _, exists := used[name]; !exists {
		used[name] = struct{}{}
		return name, true
	}
	return uniqueNameOrSuffix(name, used), true
}

func uniqueNameOrSuffix(name string, used map[string]struct{}) string {
	if _, exists := used[name]; !exists {
		used[name] = struct{}{}
		return name
	}
	base, ext := splitExt(name)
	for index := 1; ; index++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, index, ext)
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}

func splitExt(name string) (string, string) {
	// Treat only the last dot as extension separator, matching OS conventions.
	idx := strings.LastIndex(name, ".")
	if idx <= 0 {
		return name, ""
	}
	return name[:idx], name[idx:]
}

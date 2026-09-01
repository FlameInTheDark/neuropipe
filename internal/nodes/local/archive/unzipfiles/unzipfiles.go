// Package unzipfiles registers the Unzip Files Blueprint node.
package unzipfiles

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
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
	boolType := typespec.Bool()
	strList := domain.TypeSpec{Kind: domain.TypeList, Element: &domain.TypeSpec{Kind: domain.TypeString}}
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "extractedFiles", Name: "extractedFiles", Type: strList},
		{ID: "entryCount", Name: "entryCount", Type: typespec.Int()},
	}}
	return domain.NodeDefinition{
		Type:        "action:unzip_files",
		Category:    "Archive",
		Label:       "Unzip Files",
		Description: "Extract a .zip archive into a target directory. Refuses zip-slip paths.",
		Icon:        "file-archive",
		Color:       "#fbbf24",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "archivePath", Label: "Archive", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "targetDirectory", Label: "Target directory", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "overwrite", Label: "Overwrite", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataBoolean, Type: &boolType, Color: "#f87171", MaxConnections: 1, Default: true},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "archivePath", Label: "Archive", Kind: "string", Placeholder: "C:\\Work\\archive.zip", Required: true},
			{Name: "targetDirectory", Label: "Target directory", Kind: "string", Placeholder: "C:\\Work\\out", Required: true},
			{Name: "overwrite", Label: "Overwrite", Kind: "boolean", Required: false},
		},
		Capabilities:      []domain.Capability{domain.CapabilityFileRead, domain.CapabilityFileWrite},
		DefaultConfig:     map[string]any{"overwrite": true},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	archivePath, err := fileops.CleanInput(invocation.Inputs["archivePath"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("unzip files archive: %w", err)
	}
	targetDir, err := fileops.CleanInput(invocation.Inputs["targetDirectory"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("unzip files target: %w", err)
	}
	overwrite := boolValue(invocation.Inputs["overwrite"], invocation.Config["overwrite"], true)

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("open archive: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create target directory: %w", err)
	}
	targetAbs, err := filepath.Abs(targetDir)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("resolve target directory: %w", err)
	}

	var extracted []string
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("unzip files cancelled: %w", err)
		}
		name := filepath.FromSlash(file.Name)
		// Reject absolute paths and parent traversal (zip-slip). The check is
		// deliberately platform independent: entry names are inspected with
		// both separators treated as separators and Windows drive-letter
		// prefixes refused, so hostile archives get the same verdict on every
		// OS. filepath.IsAbs alone misses rooted names such as
		// "/absolute/evil.txt" on Windows, where a path without a volume is
		// not considered absolute.
		if isUnsafeEntryPath(file.Name) {
			return nodes.ExecutionResult{}, fmt.Errorf("refuse unsafe entry path: %q", file.Name)
		}
		destPath := filepath.Join(targetDir, name)
		destAbs, err := filepath.Abs(destPath)
		if err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("resolve entry path: %w", err)
		}
		if !fileops.IsPathInTree(targetAbs, destAbs) {
			return nodes.ExecutionResult{}, fmt.Errorf("refuse entry outside target directory: %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0o700); err != nil {
				return nodes.ExecutionResult{}, fmt.Errorf("create directory %q: %w", destPath, err)
			}
			continue
		}
		if !overwrite {
			if exists, err := fileops.PathExists(destPath); err != nil {
				return nodes.ExecutionResult{}, err
			} else if exists {
				continue
			}
		}
		if err := extractFile(file, destPath); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("extract %q: %w", file.Name, err)
		}
		extracted = append(extracted, name)
	}
	if extracted == nil {
		extracted = []string{}
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"result": map[string]any{
				"extractedFiles": extracted,
				"entryCount":     int64(len(extracted)),
			},
		},
		Ports: []string{"out"},
	}, nil
}

func extractFile(file *zip.File, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o700); err != nil {
		return err
	}
	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer dst.Close()
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Sync()
}

// isUnsafeEntryPath reports whether a zip entry name tries to escape the
// target directory: rooted names (a leading "/" or "\"), Windows
// drive-letter prefixes, or any ".." component. Both separators are treated
// as separators so the verdict never depends on the host OS.
func isUnsafeEntryPath(name string) bool {
	slash := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(slash, "/") {
		return true
	}
	for index, part := range strings.Split(slash, "/") {
		if part == ".." {
			return true
		}
		if index == 0 && hasVolumePrefix(part) {
			return true
		}
	}
	return false
}

// hasVolumePrefix reports whether part starts with a Windows drive-letter
// prefix such as "C:" or "D:", which would redirect the entry to another
// drive when extracted on Windows.
func hasVolumePrefix(part string) bool {
	return len(part) >= 2 && part[1] == ':' && isDriveLetter(part[0])
}

func isDriveLetter(char byte) bool {
	return 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z'
}

func boolValue(value any, configValue any, fallback bool) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	if b, ok := configValue.(bool); ok {
		return b
	}
	return fallback
}

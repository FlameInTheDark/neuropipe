// Package copyfolder registers the Copy Folder Blueprint node.
package copyfolder

import (
	"context"
	"errors"
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
	boolType := typespec.Bool()
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "targetPath", Name: "targetPath", Type: typespec.String()},
		{ID: "filesCopied", Name: "filesCopied", Type: typespec.Int()},
		{ID: "bytesCopied", Name: "bytesCopied", Type: typespec.Int()},
		{ID: "warnings", Name: "warnings", Type: domain.TypeSpec{Kind: domain.TypeList, Element: &domain.TypeSpec{Kind: domain.TypeString}}},
	}}
	return domain.NodeDefinition{
		Type:        "action:copy_folder",
		Category:    "Folders",
		Label:       "Copy Folder",
		Description: "Recursively copy a folder tree to a target path, skipping symlinks with a warning.",
		Icon:        "folder-copy",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "sourcePath", Label: "Source", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "targetPath", Label: "Target", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "overwrite", Label: "Overwrite", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataBoolean, Type: &boolType, Color: "#f87171", MaxConnections: 1, Default: true},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "sourcePath", Label: "Source", Kind: "string", Placeholder: "C:\\Work\\src", Required: true},
			{Name: "targetPath", Label: "Target", Kind: "string", Placeholder: "C:\\Work\\dst", Required: true},
			{Name: "overwrite", Label: "Overwrite", Kind: "boolean", Required: false},
		},
		Capabilities:      []domain.Capability{domain.CapabilityFileRead, domain.CapabilityFileWrite},
		DefaultConfig:     map[string]any{"overwrite": true},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	source, err := fileops.CleanInput(invocation.Inputs["sourcePath"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("copy folder source: %w", err)
	}
	target, err := fileops.CleanInput(invocation.Inputs["targetPath"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("copy folder target: %w", err)
	}
	overwrite := boolValue(invocation.Inputs["overwrite"], invocation.Config["overwrite"], true)

	info, err := os.Stat(source)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("copy folder source: %w", err)
	}
	if !info.IsDir() {
		return nodes.ExecutionResult{}, fmt.Errorf("source is not a directory: %s", source)
	}
	if !fileops.IsPathInTree(source, target) && source == target {
		return nodes.ExecutionResult{}, fmt.Errorf("source and target must not be the same")
	}

	if err := os.MkdirAll(target, 0o700); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create target directory: %w", err)
	}

	var warnings []string
	var filesCopied, bytesCopied int64
	walkErr := filepath.WalkDir(source, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == source {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)

		info, err := d.Info()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skip %q: %v", rel, err))
			return nil
		}
		mode := info.Mode()
		if mode&os.ModeSymlink != 0 {
			warnings = append(warnings, fmt.Sprintf("skip symlink %q", rel))
			return nil
		}
		if mode.IsDir() {
			if err := os.MkdirAll(dest, 0o700); err != nil {
				return fmt.Errorf("create directory %q: %w", dest, err)
			}
			return nil
		}
		if !overwrite {
			if exists, err := fileops.PathExists(dest); err != nil {
				return err
			} else if exists {
				warnings = append(warnings, fmt.Sprintf("skip existing %q (overwrite disabled)", rel))
				return nil
			}
		}
		copied, err := copyFile(path, dest)
		if err != nil {
			return fmt.Errorf("copy %q: %w", rel, err)
		}
		filesCopied++
		bytesCopied += copied
		return nil
	})
	if walkErr != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("copy folder: %w", walkErr)
	}
	if warnings == nil {
		warnings = []string{}
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"result": map[string]any{
				"targetPath":  target,
				"filesCopied": filesCopied,
				"bytesCopied": bytesCopied,
				"warnings":    warnings,
			},
		},
		Ports: []string{"out"},
	}, nil
}

func copyFile(source, target string) (int64, error) {
	src, err := os.Open(source)
	if err != nil {
		return 0, err
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return 0, err
	}
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, err
	}
	defer dst.Close()
	copied, err := io.Copy(dst, src)
	if err != nil {
		return 0, err
	}
	if err := dst.Sync(); err != nil {
		return 0, err
	}
	return copied, nil
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

// strings imported so future versions can join warnings for display; today
// the result exposes them as a list.
var _ = strings.TrimSpace
var _ = errors.Is

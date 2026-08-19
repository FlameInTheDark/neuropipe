// Package movefolder registers the Move Folder Blueprint node.
package movefolder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

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
		{ID: "targetPath", Name: "targetPath", Type: typespec.String()},
		{ID: "filesMoved", Name: "filesMoved", Type: typespec.Int()},
	}}
	return domain.NodeDefinition{
		Type:        "action:move_folder",
		Category:    "Folders",
		Label:       "Move Folder",
		Description: "Move a folder tree to a new path. Falls back to copy+delete for cross-device moves.",
		Icon:        "folder-input",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "sourcePath", Label: "Source", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "targetPath", Label: "Target", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "sourcePath", Label: "Source", Kind: "string", Placeholder: "C:\\Work\\src", Required: true},
			{Name: "targetPath", Label: "Target", Kind: "string", Placeholder: "C:\\Work\\dst", Required: true},
		},
		Capabilities:      []domain.Capability{domain.CapabilityFileRead, domain.CapabilityFileWrite},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	source, err := fileops.CleanInput(invocation.Inputs["sourcePath"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("move folder source: %w", err)
	}
	target, err := fileops.CleanInput(invocation.Inputs["targetPath"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("move folder target: %w", err)
	}
	if err := fileops.EnsureParentDir(target); err != nil {
		return nodes.ExecutionResult{}, err
	}
	// Try atomic rename first.
	if err := os.Rename(source, target); err == nil {
		if err := ctx.Err(); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("move folder cancelled: %w", err)
		}
		return nodes.ExecutionResult{
			Outputs: map[string]any{"result": map[string]any{"targetPath": target, "filesMoved": int64(0)}},
			Ports:   []string{"out"},
		}, nil
	} else if !isCrossDevice(err) {
		return nodes.ExecutionResult{}, fmt.Errorf("move folder: %w", err)
	}
	// Cross-device fallback: walk-copy then remove.
	if err := os.MkdirAll(target, 0o700); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create target directory: %w", err)
	}
	var filesMoved int64
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
			return err
		}
		mode := info.Mode()
		if mode&os.ModeSymlink != 0 {
			// Skip symlinks during cross-device move (preserves the simple,
			// explicit semantics; the user can convert them to real files
			// with Copy Folder if needed).
			return nil
		}
		if mode.IsDir() {
			return os.MkdirAll(dest, 0o700)
		}
		if err := copyFile(path, dest); err != nil {
			return err
		}
		filesMoved++
		return nil
	})
	if walkErr != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("move folder copy phase: %w", walkErr)
	}
	if err := os.RemoveAll(source); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("remove source after copy: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": map[string]any{"targetPath": target, "filesMoved": filesMoved}},
		Ports:   []string{"out"},
	}, nil
}

func copyFile(source, target string) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		_ = os.Remove(target)
		return err
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		_ = os.Remove(target)
		return err
	}
	return dst.Close()
}

func isCrossDevice(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return linkErr.Err != nil && linkErr.Err.Error() == "invalid cross-device link"
	}
	return false
}

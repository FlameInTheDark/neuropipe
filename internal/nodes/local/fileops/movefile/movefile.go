// Package movefile registers the Move File Blueprint node.
package movefile

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	}}
	return domain.NodeDefinition{
		Type:        "action:move_file",
		Category:    "Files",
		Label:       "Move File",
		Description: "Move a file to a new path. Falls back to copy+delete for cross-device moves.",
		Icon:        "file-output",
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
			{Name: "sourcePath", Label: "Source", Kind: "string", Placeholder: "C:\\Work\\source.txt", Required: true},
			{Name: "targetPath", Label: "Target", Kind: "string", Placeholder: "C:\\Work\\moved.txt", Required: true},
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
		return nodes.ExecutionResult{}, fmt.Errorf("move file source: %w", err)
	}
	target, err := fileops.CleanInput(invocation.Inputs["targetPath"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("move file target: %w", err)
	}
	if err := fileops.EnsureParentDir(target); err != nil {
		return nodes.ExecutionResult{}, err
	}
	if err := os.Rename(source, target); err == nil {
		if err := ctx.Err(); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("move file cancelled: %w", err)
		}
		return nodes.ExecutionResult{
			Outputs: map[string]any{"result": map[string]any{"targetPath": target}},
			Ports:   []string{"out"},
		}, nil
	} else if !isCrossDevice(err) {
		return nodes.ExecutionResult{}, fmt.Errorf("move file: %w", err)
	}
	// Cross-device fallback: copy then delete.
	src, err := os.Open(source)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("open source: %w", err)
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create target: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		_ = os.Remove(target)
		return nodes.ExecutionResult{}, fmt.Errorf("copy bytes: %w", err)
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		_ = os.Remove(target)
		return nodes.ExecutionResult{}, fmt.Errorf("sync target: %w", err)
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(target)
		return nodes.ExecutionResult{}, fmt.Errorf("close target: %w", err)
	}
	if err := os.Remove(source); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("remove source after copy: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("move file cancelled: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": map[string]any{"targetPath": target}},
		Ports:   []string{"out"},
	}, nil
}

// isCrossDevice reports whether err is the cross-device link error on this
// platform. os.ErrCrossDevicePath covers modern Go; some older builds return
// a plain *os.LinkError with errno 18 (EXDEV on Linux).
func isCrossDevice(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		// Errno text contains "invalid cross-device link" on most platforms.
		return linkErr.Err != nil && (linkErr.Err.Error() == "invalid cross-device link")
	}
	return false
}

var _ = filepath.Clean

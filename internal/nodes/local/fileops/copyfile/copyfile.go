// Package copyfile registers the Copy File Blueprint node.
package copyfile

import (
	"context"
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
	boolType := typespec.Bool()
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "targetPath", Name: "targetPath", Type: typespec.String()},
		{ID: "bytesCopied", Name: "bytesCopied", Type: typespec.Int()},
	}}
	return domain.NodeDefinition{
		Type:        "action:copy_file",
		Category:    "Files",
		Label:       "Copy File",
		Description: "Copy a file from a source path to a target path, creating the parent directory if missing.",
		Icon:        "copy",
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
			{Name: "sourcePath", Label: "Source", Kind: "string", Placeholder: "C:\\Work\\source.txt", Required: true},
			{Name: "targetPath", Label: "Target", Kind: "string", Placeholder: "C:\\Work\\target.txt", Required: true},
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
		return nodes.ExecutionResult{}, fmt.Errorf("copy file source: %w", err)
	}
	target, err := fileops.CleanInput(invocation.Inputs["targetPath"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("copy file target: %w", err)
	}
	overwrite := boolValue(invocation.Inputs["overwrite"], invocation.Config["overwrite"], true)

	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("copy file cancelled: %w", err)
	}
	sourceFile, err := os.Open(source)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("open source: %w", err)
	}
	defer sourceFile.Close()

	if !overwrite {
		if exists, err := fileops.PathExists(target); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("check target: %w", err)
		} else if exists {
			return nodes.ExecutionResult{}, fmt.Errorf("target exists and overwrite is disabled")
		}
	}
	if err := fileops.EnsureParentDir(target); err != nil {
		return nodes.ExecutionResult{}, err
	}
	targetFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create target: %w", err)
	}
	defer targetFile.Close()
	copied, err := io.Copy(targetFile, sourceFile)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("copy bytes: %w", err)
	}
	if err := targetFile.Sync(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("sync target: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("copy file cancelled: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"result": map[string]any{
				"targetPath":  target,
				"bytesCopied": int64(copied),
			},
		},
		Ports: []string{"out"},
	}, nil
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

// filepath imported so future versions can normalize target paths through
// filepath.Rel; today EnsureParentDir already cleans.
var _ = filepath.Clean

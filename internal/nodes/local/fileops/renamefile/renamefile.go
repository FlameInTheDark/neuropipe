// Package renamefile registers the Rename File Blueprint node.
package renamefile

import (
	"context"
	"fmt"
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
		{ID: "newPath", Name: "newPath", Type: typespec.String()},
	}}
	return domain.NodeDefinition{
		Type:        "action:rename_file",
		Category:    "Files",
		Label:       "Rename File",
		Description: "Rename a file within the same directory. The new name must be a single path segment.",
		Icon:        "pencil-line",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "path", Label: "Path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "newName", Label: "New name", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\old.txt", Required: true},
			{Name: "newName", Label: "New name", Kind: "string", Placeholder: "new.txt", Required: true},
		},
		Capabilities:      []domain.Capability{domain.CapabilityFileRead, domain.CapabilityFileWrite},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	path, err := fileops.CleanInput(invocation.Inputs["path"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("rename file: %w", err)
	}
	newName, err := fileops.CleanInput(invocation.Inputs["newName"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("rename file: %w", err)
	}
	if err := fileops.ValidateName(newName); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("rename file new name: %w", err)
	}
	dir := filepath.Dir(path)
	newPath := filepath.Join(dir, newName)
	if exists, err := fileops.PathExists(newPath); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("check target: %w", err)
	} else if exists {
		return nodes.ExecutionResult{}, fmt.Errorf("a file with name %q already exists in the same directory", newName)
	}
	if err := os.Rename(path, newPath); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("rename file: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": map[string]any{"newPath": newPath}},
		Ports:   []string{"out"},
	}, nil
}

var _ = strings.TrimSpace

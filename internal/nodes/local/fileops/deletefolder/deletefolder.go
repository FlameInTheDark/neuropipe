// Package deletefolder registers the Delete Folder Blueprint node.
package deletefolder

import (
	"context"
	"errors"
	"fmt"
	"os"

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
		{ID: "deleted", Name: "deleted", Type: typespec.Bool()},
	}}
	return domain.NodeDefinition{
		Type:        "action:delete_folder",
		Category:    "Folders",
		Label:       "Delete Folder",
		Description: "Recursively delete a folder tree. Optionally succeed when the folder is already missing.",
		Icon:        "folder-x",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "path", Label: "Path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "failIfMissing", Label: "Fail if missing", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataBoolean, Type: &boolType, Color: "#f87171", MaxConnections: 1, Default: false},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\stale", Required: true},
			{Name: "failIfMissing", Label: "Fail if missing", Kind: "boolean", Required: false},
		},
		Capabilities:      []domain.Capability{domain.CapabilityFileWrite},
		DefaultConfig:     map[string]any{"failIfMissing": false},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	path, err := fileops.CleanInput(invocation.Inputs["path"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("delete folder: %w", err)
	}
	failIfMissing := boolValue(invocation.Inputs["failIfMissing"], invocation.Config["failIfMissing"], false)
	if err := os.RemoveAll(path); err != nil {
		if errors.Is(err, os.ErrNotExist) && !failIfMissing {
			return nodes.ExecutionResult{
				Outputs: map[string]any{"result": map[string]any{"deleted": false}},
				Ports:   []string{"out"},
			}, nil
		}
		return nodes.ExecutionResult{}, fmt.Errorf("delete folder: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": map[string]any{"deleted": true}},
		Ports:   []string{"out"},
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

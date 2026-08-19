// Package folderexists registers the Folder Exists Blueprint node.
package folderexists

import (
	"context"
	"fmt"

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
		{ID: "exists", Name: "exists", Type: typespec.Bool()},
	}}
	return domain.NodeDefinition{
		Type:        "action:folder_exists",
		Category:    "Folders",
		Label:       "Folder Exists",
		Description: "Check whether a path exists on the filesystem and is a directory.",
		Icon:        "folder-question",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "path", Label: "Path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataBoolean, Type: &boolType, Color: "#f87171", MaxConnections: 1},
			{ID: "summary", Label: "Summary", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work", Required: true},
		},
		Capabilities:      []domain.Capability{domain.CapabilityFileRead},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	path, err := fileops.CleanInput(invocation.Inputs["path"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("folder exists: %w", err)
	}
	exists, err := fileops.IsDir(path)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("folder exists: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"result":  exists,
			"summary": map[string]any{"exists": exists},
		},
		Ports: []string{"out"},
	}, nil
}

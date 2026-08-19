// Package filetobase64 registers the File To Base64 Blueprint node.
package filetobase64

import (
	"context"
	"encoding/base64"
	"fmt"
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
	textType := typespec.String()
	return domain.NodeDefinition{
		Type:        "action:file_to_base64",
		Category:    "Files",
		Label:       "File To Base64",
		Description: "Read a local file and encode its contents as a Base64 string.",
		Icon:        "file-code-2",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "path", Label: "Path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\image.png", Required: true},
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
		return nodes.ExecutionResult{}, fmt.Errorf("file to base64: %w", err)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("file to base64: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": base64.StdEncoding.EncodeToString(data)},
		Ports:   []string{"out"},
	}, nil
}

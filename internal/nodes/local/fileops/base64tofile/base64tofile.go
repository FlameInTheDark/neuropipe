// Package base64tofile registers the Base64 To File Blueprint node.
package base64tofile

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
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
		{ID: "path", Name: "path", Type: typespec.String()},
		{ID: "bytesWritten", Name: "bytesWritten", Type: typespec.Int()},
	}}
	return domain.NodeDefinition{
		Type:        "action:base64_to_file",
		Category:    "Files",
		Label:       "Base64 To File",
		Description: "Decode a Base64 string and write the resulting bytes to a local file.",
		Icon:        "file-input",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "base64", Label: "Base64", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "path", Label: "Path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "base64", Label: "Base64", Kind: "textarea", Placeholder: "Paste Base64 here", Required: true},
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\out.bin", Required: true},
		},
		Capabilities:      []domain.Capability{domain.CapabilityFileWrite},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	encoded, _ := invocation.Inputs["base64"].(string)
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("base64 input is required")
	}
	path, err := fileops.CleanInput(invocation.Inputs["path"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("base64 to file: %w", err)
	}
	data, err := decodeBase64(encoded)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("base64 to file decode: %w", err)
	}
	if err := fileops.EnsureParentDir(path); err != nil {
		return nodes.ExecutionResult{}, err
	}
	if err := writeFile(path, data); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("base64 to file write: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": map[string]any{"path": path, "bytesWritten": int64(len(data))}},
		Ports:   []string{"out"},
	}, nil
}

// decodeBase64 tries standard, URL-safe, and raw variants so callers do not
// need to know which encoder produced the input.
func decodeBase64(encoded string) ([]byte, error) {
	if data, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return data, nil
	}
	if data, err := base64.URLEncoding.DecodeString(encoded); err == nil {
		return data, nil
	}
	if data, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
		return data, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("input is not valid Base64 in any supported variant")
	}
	return data, nil
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

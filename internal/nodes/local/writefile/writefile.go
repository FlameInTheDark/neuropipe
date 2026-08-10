// Package writefile registers the strict local file writer Blueprint node.
package writefile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/textbytes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// New creates the Write File module implementation.
func New() Node {
	definition := definition()
	return Node{Metadata: definition, Resolver: resolve, Executor: execute}
}

// Register contributes the complete Write File module to the node registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("write file cancelled: %w", err)
	}
	path, ok := invocation.Inputs["path"].(string)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("file path must be text")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("file path is required")
	}
	defaults := invocation.Definition.DefaultConfig
	if defaults == nil {
		defaults = definition().DefaultConfig
	}
	contentType, err := textbytes.Resolve(invocation.Config, defaults, "contentType")
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("write file configuration: %w", err)
	}
	content, err := textbytes.InputBytes(invocation.Inputs["content"], contentType)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("write file content: %w", err)
	}
	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create file directory: %w", err)
	}
	if err := os.WriteFile(cleanPath, content, 0o600); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("write file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("write file cancelled: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"result": map[string]any{"path": cleanPath, "written": true}},
		Ports:   []string{"out"},
	}, nil
}

func definition() domain.NodeDefinition {
	pathType := typespec.String()
	resultType := resultType()
	defaults := map[string]any{"contentType": string(textbytes.Text), "content": ""}
	return domain.NodeDefinition{
		Type:        "action:file_write",
		Category:    "Local",
		Label:       "Write File",
		Description: "Write explicitly selected UTF-8 text or raw bytes to an approved local path.",
		Icon:        "file-up",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "path", Label: "Path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			contentPin(textbytes.Text),
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{
				ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput,
				DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1,
				Fields: []domain.DataField{
					{Path: "path", Label: "Path", DataType: domain.DataText, Description: "Written file path."},
					{Path: "written", Label: "Written", DataType: domain.DataBoolean, Description: "Whether the write completed."},
				},
			},
		},
		Fields: []domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\output.bin", Required: true},
			{Name: "contentType", Label: "Content type", Kind: "wire-representation", Required: true, Options: textbytes.Options()},
			{Name: "content", Label: "Content", Kind: "textarea", Placeholder: "", Required: true},
		},
		Capabilities:  []domain.Capability{domain.CapabilityFileWrite},
		DefaultConfig: defaults,
		Source:        "builtin",
	}
}

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := definition()
	contentType, err := textbytes.Resolve(config(node), definition.DefaultConfig, "contentType")
	if err != nil {
		return definition, err
	}
	definition.Inputs[2] = contentPin(contentType)
	return definition, nil
}

func config(node domain.FlowNode) map[string]any {
	if value, ok := node.Data["config"].(map[string]any); ok {
		return value
	}
	return node.Data
}

func resultType() domain.TypeSpec {
	return domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "path", Name: "path", Type: typespec.String()},
		{ID: "written", Name: "written", Type: typespec.Bool()},
	}}
}

func contentPin(representation textbytes.Representation) domain.NodePort {
	pin := textbytes.Pin("content", "Content", domain.PinInput, representation, true)
	if representation == textbytes.Text {
		pin.Default = ""
	}
	return pin
}

// Package readfile registers the byte-preserving local file reader node.
package readfile

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

// New creates the Read File module implementation.
func New() Node {
	definition := definition()
	return Node{
		Metadata: definition,
		Resolver: resolve,
		Executor: execute,
	}
}

// Register contributes the complete Read File module to the node registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read file cancelled: %w", err)
	}
	path, _ := invocation.Inputs["path"].(string)
	path = strings.TrimSpace(path)
	if path == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("file path is required")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read file cancelled: %w", err)
	}
	defaults := invocation.Definition.DefaultConfig
	if defaults == nil {
		defaults = definition().DefaultConfig
	}
	outputType, err := textbytes.Resolve(invocation.Config, defaults, "outputType")
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read file configuration: %w", err)
	}
	result, err := textbytes.OutputValue(data, outputType)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("read file output: %w", err)
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": result}, Ports: []string{"out"}}, nil
}

func definition() domain.NodeDefinition {
	pathType := typespec.String()
	defaults := map[string]any{"outputType": string(textbytes.Bytes)}
	return domain.NodeDefinition{
		Type:        "action:file_read",
		Category:    "Files",
		Label:       "Read File",
		Description: "Read an approved local file as explicitly selected bytes or UTF-8 text.",
		Icon:        "file-down",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "path", Label: "Path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			textbytes.Pin("result", "Result", domain.PinOutput, textbytes.Bytes, false),
		},
		Fields: []domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\input.bin", Required: true},
			{Name: "outputType", Label: "Output", Kind: "wire-representation", Required: true, Options: textbytes.Options()},
		},
		Capabilities:  []domain.Capability{domain.CapabilityFileRead},
		DefaultConfig: defaults,
		Source:        "builtin",
	}
}

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := definition()
	outputType, err := textbytes.Resolve(config(node), definition.DefaultConfig, "outputType")
	if err != nil {
		return definition, err
	}
	definition.Outputs[1] = textbytes.Pin("result", "Result", domain.PinOutput, outputType, false)
	return definition, nil
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

// Package base64decode registers the explicit Base64 decoder.
package base64decode

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/textbytes"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// New creates the Base64 Decode module implementation.
func New() Node {
	definition := definition()
	return Node{
		Metadata: definition,
		Resolver: resolve,
		Executor: nodes.Outputs(evaluate),
	}
}

// Register contributes the complete Base64 Decode module to the node registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	defaults := invocation.Definition.DefaultConfig
	if defaults == nil {
		defaults = definition().DefaultConfig
	}
	inputType, err := textbytes.Resolve(invocation.Config, defaults, "inputType")
	if err != nil {
		return nil, err
	}
	outputType, err := textbytes.Resolve(invocation.Config, defaults, "outputType")
	if err != nil {
		return nil, err
	}
	encoded, err := textbytes.InputBytes(invocation.Inputs["value"], inputType)
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode Base64: %w", err)
	}
	result, err := textbytes.OutputValue(data, outputType)
	if err != nil {
		return nil, err
	}
	return map[string]any{"result": result}, nil
}

func definition() domain.NodeDefinition {
	defaults := map[string]any{"inputType": string(textbytes.Text), "outputType": string(textbytes.Bytes)}
	return domain.NodeDefinition{
		Type:        "data:base64_decode",
		Category:    "Data",
		Label:       "Base64 Decode",
		Description: "Explicitly decode a selected Base64 text or bytes representation into selected text or bytes.",
		Icon:        "binary",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs:      []domain.NodePort{textbytes.Pin("value", "Value", domain.PinInput, textbytes.Text, true)},
		Outputs:     []domain.NodePort{textbytes.Pin("result", "Result", domain.PinOutput, textbytes.Bytes, false)},
		Fields: []domain.ConfigField{
			{Name: "inputType", Label: "Input", Kind: "wire-representation", Required: true, Options: textbytes.Options()},
			{Name: "outputType", Label: "Output", Kind: "wire-representation", Required: true, Options: textbytes.Options()},
		},
		DefaultConfig: defaults,
		Source:        "builtin",
	}
}

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := definition()
	inputType, err := textbytes.Resolve(config(node), definition.DefaultConfig, "inputType")
	if err != nil {
		return definition, err
	}
	outputType, err := textbytes.Resolve(config(node), definition.DefaultConfig, "outputType")
	if err != nil {
		return definition, err
	}
	definition.Inputs = []domain.NodePort{textbytes.Pin("value", "Value", domain.PinInput, inputType, true)}
	definition.Outputs = []domain.NodePort{textbytes.Pin("result", "Result", domain.PinOutput, outputType, false)}
	return definition, nil
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

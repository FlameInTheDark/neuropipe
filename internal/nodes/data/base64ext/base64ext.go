// Package base64ext registers two pure Blueprint nodes that bridge raw bytes
// and Base64-encoded text:
//   - data:bytes_to_base64 — encode connected bytes as Base64 text
//   - data:base64_to_bytes — decode Base64 text to raw bytes
//
// These nodes complement the existing data:base64_encode / data:base64_decode
// pair by skipping the wire-representation picker: bytes_to_base64 always
// consumes bytes and produces text; base64_to_bytes always consumes text and
// produces bytes. This keeps their contract unambiguous when wiring them
// against bytes-only pin types.
package base64ext

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// ---------- Bytes To Base64 ----------

type bytesToBase64Node = nodes.Implementation

var _ nodes.Node = bytesToBase64Node{}

func NewBytesToBase64() bytesToBase64Node {
	return bytesToBase64Node{Metadata: bytesToBase64Definition(), Executor: executeBytesToBase64}
}

func RegisterBytesToBase64(registrar nodes.Registrar) error {
	return registrar.Register(NewBytesToBase64())
}

func bytesToBase64Definition() domain.NodeDefinition {
	bytesType := domain.TypeSpec{Kind: domain.TypeBytes}
	textType := typespec.String()
	return domain.NodeDefinition{
		Type:        "data:bytes_to_base64",
		Category:    "Data",
		Label:       "Bytes To Base64",
		Description: "Encode connected raw bytes as a Base64 string.",
		Icon:        "binary",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs: []domain.NodePort{
			{ID: "value", Label: "Value", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataBytes, Type: &bytesType, Color: "#fbbf24", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
		},
		Fields:            []domain.ConfigField{},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func executeBytesToBase64(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	data, err := asBytes(invocation.Inputs["value"])
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": base64.StdEncoding.EncodeToString(data)}}, nil
}

// ---------- Base64 To Bytes ----------

type base64ToBytesNode = nodes.Implementation

var _ nodes.Node = base64ToBytesNode{}

func NewBase64ToBytes() base64ToBytesNode {
	return base64ToBytesNode{Metadata: base64ToBytesDefinition(), Executor: executeBase64ToBytes}
}

func RegisterBase64ToBytes(registrar nodes.Registrar) error {
	return registrar.Register(NewBase64ToBytes())
}

func base64ToBytesDefinition() domain.NodeDefinition {
	textType := typespec.String()
	bytesType := domain.TypeSpec{Kind: domain.TypeBytes}
	return domain.NodeDefinition{
		Type:        "data:base64_to_bytes",
		Category:    "Data",
		Label:       "Base64 To Bytes",
		Description: "Decode a Base64 string to raw bytes, auto-detecting standard and URL-safe variants.",
		Icon:        "binary",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs: []domain.NodePort{
			{ID: "value", Label: "Value", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataBytes, Type: &bytesType, Color: "#fbbf24", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "value", Label: "Value", Kind: "textarea", Placeholder: "Paste Base64 here", Required: true},
		},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func executeBase64ToBytes(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	raw, _ := invocation.Inputs["value"].(string)
	encoded := strings.TrimSpace(raw)
	if encoded == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("base64 input is required")
	}
	data, err := decodeBase64(encoded)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("decode base64: %w", err)
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": data}}, nil
}

// ---------- helpers ----------

func asBytes(value any) ([]byte, error) {
	switch typed := value.(type) {
	case nil:
		return nil, fmt.Errorf("value is required")
	case []byte:
		return typed, nil
	case string:
		return []byte(typed), nil
	default:
		return nil, fmt.Errorf("value must be bytes or text, got %T", value)
	}
}

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

// Package uuid registers Neuropipe's UUID generation and analysis nodes.
//
// Four nodes live in this package, all in the Data category:
//   - data:uuid_generate — produce a UUID in a selected version (v1, v3, v4, v5, v7)
//   - data:uuid_parse    — parse a UUID and surface version/variant/bytes
//   - data:uuid_validate — boolean validator with the input forwarded as result
//   - data:uuid_extract  — extract every UUID-like substring from free text
package uuid

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// ---------- Generate ----------

type generateNode = nodes.Implementation

var _ nodes.Node = generateNode{}

func NewGenerate() generateNode {
	return generateNode{Metadata: generateDefinition(), Executor: executeGenerate}
}

func RegisterGenerate(registrar nodes.Registrar) error { return registrar.Register(NewGenerate()) }

func generateDefinition() domain.NodeDefinition {
	textType := typespec.String()
	return domain.NodeDefinition{
		Type:        "data:uuid_generate",
		Category:    "Data",
		Label:       "Generate UUID",
		Description: "Generate a UUID in a selected version (v1, v3, v4, v5, or v7).",
		Icon:        "fingerprint",
		Color:       "#86efac",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "namespace", Label: "Namespace", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
			{ID: "name", Label: "Name", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "value", Label: "Value", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "version", Label: "Version", Kind: "select", Required: true, Options: []domain.Option{
				{Value: "v4", Label: "v4 (random)"},
				{Value: "v1", Label: "v1 (time + node)"},
				{Value: "v7", Label: "v7 (time-ordered random)"},
				{Value: "v5", Label: "v5 (SHA-1 named)"},
				{Value: "v3", Label: "v3 (MD5 named)"},
			}},
			{Name: "namespace", Label: "Namespace (v3/v5)", Kind: "string", Placeholder: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", Required: false},
			{Name: "name", Label: "Name (v3/v5)", Kind: "string", Placeholder: "example.com", Required: false},
		},
		Capabilities:      []domain.Capability{},
		DefaultConfig:     map[string]any{"version": "v4"},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func executeGenerate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	version, _ := invocation.Config["version"].(string)
	if version == "" {
		version = "v4"
	}
	var value uuid.UUID
	var err error
	switch version {
	case "v4":
		value, err = uuid.NewRandom()
	case "v1":
		value, err = uuid.NewUUID()
	case "v7":
		value, err = uuid.NewV7()
	case "v5", "v3":
		namespaceRaw, _ := invocation.Inputs["namespace"].(string)
		name, _ := invocation.Inputs["name"].(string)
		if strings.TrimSpace(namespaceRaw) == "" {
			namespaceRaw, _ = invocation.Config["namespace"].(string)
		}
		if strings.TrimSpace(name) == "" {
			name, _ = invocation.Config["name"].(string)
		}
		if strings.TrimSpace(namespaceRaw) == "" {
			return nodes.ExecutionResult{}, fmt.Errorf("UUID %s requires a namespace UUID", version)
		}
		if strings.TrimSpace(name) == "" {
			return nodes.ExecutionResult{}, fmt.Errorf("UUID %s requires a name", version)
		}
		ns, parseErr := uuid.Parse(namespaceRaw)
		if parseErr != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("UUID namespace parse: %w", parseErr)
		}
		if version == "v5" {
			value = uuid.NewSHA1(ns, []byte(name))
		} else {
			value = uuid.NewMD5(ns, []byte(name))
		}
	default:
		return nodes.ExecutionResult{}, fmt.Errorf("unsupported UUID version %q", version)
	}
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("generate UUID %s: %w", version, err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"value": value.String(), "result": value.String()},
		Ports:   []string{"out"},
	}, nil
}

// ---------- Parse ----------

type parseNode = nodes.Implementation

var _ nodes.Node = parseNode{}

func NewParse() parseNode {
	return parseNode{Metadata: parseDefinition(), Executor: executeParse}
}

func RegisterParse(registrar nodes.Registrar) error { return registrar.Register(NewParse()) }

func parseDefinition() domain.NodeDefinition {
	textType := typespec.String()
	bytesType := domain.TypeSpec{Kind: domain.TypeBytes}
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "isValid", Name: "isValid", Type: typespec.Bool()},
		{ID: "version", Name: "version", Type: typespec.Int()},
		{ID: "variant", Name: "variant", Type: typespec.String()},
		{ID: "bytes", Name: "bytes", Type: bytesType},
		{ID: "urn", Name: "urn", Type: typespec.String()},
	}}
	return domain.NodeDefinition{
		Type:        "data:uuid_parse",
		Category:    "Data",
		Label:       "Parse UUID",
		Description: "Parse a UUID string and surface its version, variant, raw bytes, and URN form.",
		Icon:        "fingerprint",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs: []domain.NodePort{
			{ID: "value", Label: "Value", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1, Fields: []domain.DataField{
				{Path: "isValid", Label: "Valid", DataType: domain.DataBoolean},
				{Path: "version", Label: "Version", DataType: domain.DataNumber},
				{Path: "variant", Label: "Variant", DataType: domain.DataText},
				{Path: "bytes", Label: "Bytes", DataType: domain.DataBytes},
				{Path: "urn", Label: "URN", DataType: domain.DataText},
			}},
		},
		Fields: []domain.ConfigField{
			{Name: "value", Label: "Value", Kind: "string", Placeholder: "550e8400-e29b-41d4-a716-446655440000", Required: true},
		},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func executeParse(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	raw, _ := invocation.Inputs["value"].(string)
	value, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		// Don't fail the node — surface isValid=false so pipelines can branch.
		return nodes.ExecutionResult{Outputs: map[string]any{
			"result": map[string]any{
				"isValid": false,
				"version": int64(0),
				"variant": "",
				"bytes":   []byte{},
				"urn":     "",
			},
		}}, nil
	}
	return nodes.ExecutionResult{Outputs: map[string]any{
		"result": map[string]any{
			"isValid": true,
			"version": int64(value.Version()),
			"variant": variantName(value),
			"bytes":   value[:],
			"urn":     value.URN(),
		},
	}}, nil
}

func variantName(value uuid.UUID) string {
	switch value.Variant() {
	case uuid.RFC4122:
		return "rfc4122"
	case uuid.Microsoft:
		return "microsoft"
	case uuid.Future:
		return "future"
	case uuid.Reserved:
		return "reserved"
	default:
		return "unknown"
	}
}

// ---------- Validate ----------

type validateNode = nodes.Implementation

var _ nodes.Node = validateNode{}

func NewValidate() validateNode {
	return validateNode{Metadata: validateDefinition(), Executor: executeValidate}
}

func RegisterValidate(registrar nodes.Registrar) error { return registrar.Register(NewValidate()) }

func validateDefinition() domain.NodeDefinition {
	textType := typespec.String()
	boolType := typespec.Bool()
	return domain.NodeDefinition{
		Type:        "data:uuid_validate",
		Category:    "Data",
		Label:       "Validate UUID",
		Description: "Return whether a string is a valid UUID. The input is forwarded as the result for convenience.",
		Icon:        "fingerprint",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs: []domain.NodePort{
			{ID: "value", Label: "Value", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "value", Label: "Value", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataBoolean, Type: &boolType, Color: "#f87171", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "value", Label: "Value", Kind: "string", Placeholder: "550e8400-e29b-41d4-a716-446655440000", Required: true},
		},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func executeValidate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	raw, _ := invocation.Inputs["value"].(string)
	_, err := uuid.Parse(strings.TrimSpace(raw))
	return nodes.ExecutionResult{Outputs: map[string]any{
		"value":  raw,
		"result": err == nil,
	}}, nil
}

// ---------- Extract ----------

type extractNode = nodes.Implementation

var _ nodes.Node = extractNode{}

var uuidPattern = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)

func NewExtract() extractNode {
	return extractNode{Metadata: extractDefinition(), Executor: executeExtract}
}

func RegisterExtract(registrar nodes.Registrar) error { return registrar.Register(NewExtract()) }

func extractDefinition() domain.NodeDefinition {
	textType := typespec.String()
	listType := domain.TypeSpec{Kind: domain.TypeList, Element: &textType}
	return domain.NodeDefinition{
		Type:        "data:uuid_extract",
		Category:    "Data",
		Label:       "Extract UUIDs",
		Description: "Extract every UUID-like substring from the connected text and return them as a list.",
		Icon:        "fingerprint",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs: []domain.NodePort{
			{ID: "value", Label: "Value", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataList, Type: &listType, Color: "#facc15", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "value", Label: "Value", Kind: "textarea", Placeholder: "Paste text containing UUIDs…", Required: true},
		},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func executeExtract(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	raw, _ := invocation.Inputs["value"].(string)
	matches := uuidPattern.FindAllString(raw, -1)
	if matches == nil {
		matches = []string{}
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": matches}}, nil
}

// Package randomnumber registers a random-number source Blueprint node.
//
// The node supports both floating-point and integer outputs, an optional range
// (from / to), and accepts either inspector fields or matching data pins for
// the range bounds. Data pins always take priority over inspector fields, so
// upstream nodes can dynamically control the range while the inspector keeps a
// static fallback. A new random value is produced for each Blueprint execution
// that resolves the node, then cached for the remainder of that execution.
package randomnumber

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// New creates the Random Number module implementation.
func New() Node {
	definition := definition()
	return Node{Metadata: definition, Resolver: resolve, Executor: execute}
}

// Register contributes the complete Random Number module to the registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func definition() domain.NodeDefinition {
	floatType := typespec.Float()
	return domain.NodeDefinition{
		Type:        "data:random_number",
		Category:    "Data",
		Label:       "Random Number",
		Description: "Generate a random number as a float or integer, optionally within a configured range.",
		Icon:        "dice",
		Color:       "#86efac",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "from", Label: "From", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataNumber, Type: &floatType, Color: "#86efac", MaxConnections: 1, Default: 0.0},
			{ID: "to", Label: "To", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataNumber, Type: &floatType, Color: "#86efac", MaxConnections: 1, Default: 1.0},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "value", Label: "Value", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataNumber, Type: &floatType, Color: "#86efac", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			datanodes.Select("type", "Type", "float", "integer"),
			{Name: "useRange", Label: "Use range", Kind: "boolean", Required: false},
			{Name: "from", Label: "From", Kind: "number", Placeholder: "0", Required: false},
			{Name: "to", Label: "To", Kind: "number", Placeholder: "1", Required: false},
		},
		Capabilities:      []domain.Capability{},
		DefaultConfig:     map[string]any{"type": "float", "useRange": false, "from": 0.0, "to": 1.0},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

// resolve adapts the input AND output pin contracts to the configured number
// type so the editor highlights integer-only connections when the user selects
// `integer`, while keeping the on-wire value as a JSON number. Both the
// range input pins (from/to) and the value output pin follow the selected
// type, so int sources can drive int range pins and int outputs can feed int
// targets without triggering the strict type checker.
func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := definition()
	config := config(node)
	target, _ := config["type"].(string)
	if target == "integer" {
		definition.Inputs = append([]domain.NodePort(nil), definition.Inputs...)
		definition.Outputs = append([]domain.NodePort(nil), definition.Outputs...)
		intType := typespec.Int()
		for index := range definition.Inputs {
			if definition.Inputs[index].ID == "from" || definition.Inputs[index].ID == "to" {
				definition.Inputs[index].Type = &intType
			}
		}
		for index := range definition.Outputs {
			if definition.Outputs[index].ID == "value" {
				definition.Outputs[index].Type = &intType
			}
		}
	}
	return definition, nil
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("random number cancelled: %w", err)
	}
	target, _ := invocation.Config["type"].(string)
	if target == "" {
		target = "float"
	}
	useRange, _ := invocation.Config["useRange"].(bool)

	var from, to float64
	if useRange {
		from = numberValue(invocation, "from", 0)
		to = numberValue(invocation, "to", 1)
		if to < from {
			return nodes.ExecutionResult{}, fmt.Errorf("random range From must be less than or equal to To")
		}
	}

	value, err := generate(target, useRange, from, to)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"value": value, "result": map[string]any{"value": value}},
		Ports:   []string{"out"},
	}, nil
}

// numberValue resolves a numeric input by prioritising connected data pins
// over the inspector field, matching the contract documented for this node.
func numberValue(invocation nodes.Invocation, name string, fallback float64) float64 {
	if value, ok := asFloat(invocation.Inputs[name]); ok {
		return value
	}
	if value, ok := asFloat(invocation.Config[name]); ok {
		return value
	}
	return fallback
}

func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			var parsed float64
			if _, err := fmt.Sscanf(trimmed, "%g", &parsed); err == nil {
				return parsed, true
			}
		}
		return 0, false
	default:
		return 0, false
	}
}

func generate(target string, useRange bool, from, to float64) (any, error) {
	if useRange {
		return ranged(target, from, to)
	}
	switch target {
	case "integer":
		n, err := rand.Int(rand.Reader, big.NewInt(1<<62))
		if err != nil {
			return nil, fmt.Errorf("generate random integer: %w", err)
		}
		return n.Int64(), nil
	default:
		n, err := rand.Int(rand.Reader, big.NewInt(1<<53))
		if err != nil {
			return nil, fmt.Errorf("generate random float: %w", err)
		}
		// Scale to [0, 1) using 53 bits of precision (matching float64 mantissa).
		return float64(n.Int64()) / float64(1<<53), nil
	}
}

func ranged(target string, from, to float64) (any, error) {
	span := to - from
	if span < 0 {
		return nil, fmt.Errorf("random range is invalid")
	}
	switch target {
	case "integer":
		low := int64(from)
		high := int64(to)
		if high < low {
			return nil, fmt.Errorf("random range From must be less than or equal to To")
		}
		count := high - low + 1
		if count <= 0 {
			// Range is too large for int64 arithmetic; fall back to a safe
			// modulus-based selection from a 62-bit value.
			n, err := rand.Int(rand.Reader, big.NewInt(1<<62))
			if err != nil {
				return nil, fmt.Errorf("generate random integer: %w", err)
			}
			return low + n.Int64()%count, nil
		}
		n, err := rand.Int(rand.Reader, big.NewInt(count))
		if err != nil {
			return nil, fmt.Errorf("generate random integer: %w", err)
		}
		return low + n.Int64(), nil
	default:
		n, err := rand.Int(rand.Reader, big.NewInt(1<<53))
		if err != nil {
			return nil, fmt.Errorf("generate random float: %w", err)
		}
		normalized := float64(n.Int64()) / float64(1<<53)
		return from + normalized*span, nil
	}
}

func config(node domain.FlowNode) map[string]any {
	if value, ok := node.Data["config"].(map[string]any); ok {
		return value
	}
	return node.Data
}

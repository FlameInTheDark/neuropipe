// Package mathnodes shares the immutable pin contract used by math modules.
package mathnodes

import (
	"fmt"
	"math"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// FiniteInput validates the strict number contract shared by arithmetic nodes.
func FiniteInput(inputs map[string]any, name string) (float64, error) {
	value, ok := asFloat(inputs[name])
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("math input %s must be a finite number", strings.ToUpper(name))
	}
	return value, nil
}

// EnsureFinite rejects arithmetic results that cannot be represented safely.
func EnsureFinite(nodeType string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%s produced a non-finite result", nodeType)
	}
	return nil
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
	default:
		return 0, false
	}
}

// Definition reports the complete math node metadata, including canonical
// numeric defaults and strict float input/output contracts.
func Definition(nodeType, label, description, icon string) domain.NodeDefinition {
	return domain.NodeDefinition{
		Type:        nodeType,
		Category:    "Math",
		Label:       label,
		Description: description,
		Icon:        icon,
		Color:       "#86efac",
		Mode:        domain.NodePure,
		Inputs: []domain.NodePort{
			numberPin("a", "A", domain.PinInput),
			numberPin("b", "B", domain.PinInput),
		},
		Outputs: []domain.NodePort{numberPin("result", "Result", domain.PinOutput)},
		Fields: []domain.ConfigField{
			{Name: "a", Label: "A", Kind: "number", Placeholder: "0"},
			{Name: "b", Label: "B", Kind: "number", Placeholder: "0"},
		},
		DefaultConfig: map[string]any{"a": 0.0, "b": 0.0},
		Source:        "builtin",
	}
}

func numberPin(id, label string, direction domain.PinDirection) domain.NodePort {
	typeSpec := typespec.Float()
	pin := domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataNumber, Type: &typeSpec, Color: "#86efac", MaxConnections: 1}
	if direction == domain.PinInput {
		pin.Default = 0.0
	}
	return pin
}

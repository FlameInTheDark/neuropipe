package pipeline

import (
	"fmt"
	"math"
	"strings"
)

// evaluateMath evaluates a pure two-operand arithmetic node. It deliberately
// accepts only finite numeric inputs and produces one finite numeric result.
func evaluateMath(nodeType string, inputs map[string]any) (map[string]any, error) {
	a, err := finiteMathInput(inputs, "a")
	if err != nil {
		return nil, err
	}
	b, err := finiteMathInput(inputs, "b")
	if err != nil {
		return nil, err
	}

	var result float64
	switch nodeType {
	case "math:add":
		result = a + b
	case "math:subtract":
		result = a - b
	case "math:multiply":
		result = a * b
	case "math:divide":
		if b == 0 {
			return nil, fmt.Errorf("divide requires a non-zero B input")
		}
		result = a / b
	default:
		return nil, fmt.Errorf("unsupported math node %q", nodeType)
	}
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return nil, fmt.Errorf("%s produced a non-finite result", nodeType)
	}
	return map[string]any{"result": result}, nil
}

func finiteMathInput(inputs map[string]any, name string) (float64, error) {
	value, ok := asNumber(inputs[name])
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("math input %s must be a finite number", strings.ToUpper(name))
	}
	return value, nil
}

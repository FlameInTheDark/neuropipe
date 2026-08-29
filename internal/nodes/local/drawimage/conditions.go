package drawimage

import (
	"strconv"
	"strings"
)

/* ------------------------------------------------------------------ */
/* visibility conditions                                               */
/* ------------------------------------------------------------------ */

// EvaluateCondition reports whether an element should render.
// pinType is "" for repeat pseudo-pins (item/item.field/index), which use
// generic value comparison.
func EvaluateCondition(visibility Visibility, values map[string]any, pinType PinType) bool {
	if visibility.Mode != "condition" {
		return true
	}
	var value any
	if isPseudoPin(visibility.Pin) {
		value = resolvePath(TemplateContext(values), visibility.Pin)
	} else {
		var ok bool
		value, ok = values[visibility.Pin]
		if !ok {
			// undeclared pins read as empty
			value = nil
		}
	}
	return applyOp(visibility.Op, value, visibility.Value, pinType)
}

// isPseudoPin matches the repetition-provided pseudo pins.
func isPseudoPin(name string) bool {
	return name == "item" || name == "index" || strings.HasPrefix(name, "item.")
}

// applyOp evaluates op against value with the configured comparison operand.
func applyOp(op string, value any, operand string, pinType PinType) bool {
	switch op {
	/* universal */
	case "isEmpty":
		return valueIsEmpty(value)
	case "notEmpty":
		return !valueIsEmpty(value)

	/* boolean */
	case "isTrue":
		return valueTruthy(value)
	case "isFalse":
		return !valueTruthy(value)

	/* text (and generic equality) */
	case "eq":
		return StringifyValue(value) == operand
	case "ne":
		return StringifyValue(value) != operand
	case "contains":
		return strings.Contains(StringifyValue(value), operand)
	case "notContains":
		return !strings.Contains(StringifyValue(value), operand)
	case "startsWith":
		return strings.HasPrefix(StringifyValue(value), operand)
	case "endsWith":
		return strings.HasSuffix(StringifyValue(value), operand)

	/* numeric comparisons — both sides coerced to float */
	case "gt":
		return numericPair(value, operand, func(a, b float64) bool { return a > b })
	case "ge":
		return numericPair(value, operand, func(a, b float64) bool { return a >= b })
	case "lt":
		return numericPair(value, operand, func(a, b float64) bool { return a < b })
	case "le":
		return numericPair(value, operand, func(a, b float64) bool { return a <= b })

	/* array membership */
	case "arrayContains":
		return arrayContains(value, operand)
	case "arrayNotContains":
		return !arrayContains(value, operand)

	/* array length */
	case "lenEq":
		return arrayLength(value) == parseIntOr(operand, -1)
	case "lenNe":
		return arrayLength(value) != parseIntOr(operand, -1)
	case "lenGt":
		return float64(arrayLength(value)) > parseFloatOr(operand, 0)
	case "lenGe":
		return float64(arrayLength(value)) >= parseFloatOr(operand, 0)
	case "lenLt":
		return float64(arrayLength(value)) < parseFloatOr(operand, 0)
	case "lenLe":
		return float64(arrayLength(value)) <= parseFloatOr(operand, 0)

	/* object keys */
	case "hasKey":
		if object, ok := value.(map[string]any); ok {
			_, exists := object[operand]
			return exists
		}
		return false
	}
	// unknown operator: render the element (fail open, visible in preview)
	return true
}

func valueIsEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return StringifyValue(value) == ""
	}
}

func valueTruthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "1"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return false
	}
}

func numericPair(value any, operand string, compare func(a, b float64) bool) bool {
	left, ok := toFloat(value)
	if !ok {
		left = 0
	}
	right, ok := toFloat(operand)
	if !ok {
		right = 0
	}
	return compare(left, right)
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case bool:
		if typed {
			return 1, true
		}
		return 0, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	case []any:
		return float64(len(typed)), true
	case map[string]any:
		return float64(len(typed)), true
	}
	return 0, false
}

func parseFloatOr(value string, fallback float64) float64 {
	parsed, ok := toFloat(strings.TrimSpace(value))
	if !ok {
		return fallback
	}
	return parsed
}

func parseIntOr(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func arrayContains(value any, operand string) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if StringifyValue(item) == operand {
			return true
		}
	}
	return false
}

func arrayLength(value any) int {
	switch typed := value.(type) {
	case []any:
		return len(typed)
	case string:
		return len(strings.Split(typed, ","))
	case map[string]any:
		return len(typed)
	default:
		return 0
	}
}

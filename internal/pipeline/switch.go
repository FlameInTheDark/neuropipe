package pipeline

import (
	"fmt"
	"math"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

type switchComparator string

const (
	switchEquals             switchComparator = "equals"
	switchNotEquals          switchComparator = "not_equals"
	switchContains           switchComparator = "contains"
	switchStartsWith         switchComparator = "starts_with"
	switchEndsWith           switchComparator = "ends_with"
	switchGreaterThan        switchComparator = "greater_than"
	switchGreaterThanOrEqual switchComparator = "greater_than_or_equal"
	switchLessThan           switchComparator = "less_than"
	switchLessThanOrEqual    switchComparator = "less_than_or_equal"
)

type switchCase struct {
	ID        string
	Label     string
	ValueType domain.DataType
	Value     any
}

type switchConfiguration struct {
	Comparator switchComparator
	Cases      []switchCase
	Legacy     bool
}

func switchConfigurationFor(node domain.FlowNode, defaults map[string]any) (switchConfiguration, error) {
	config := configFor(node)
	if raw, exists := config["switch"]; exists {
		return parseSwitchConfiguration(raw)
	}
	if legacy, exists := config["options"]; exists {
		return legacySwitchConfiguration(legacy)
	}
	return parseSwitchConfiguration(defaults["switch"])
}

func parseSwitchConfiguration(raw any) (switchConfiguration, error) {
	value, ok := raw.(map[string]any)
	if !ok {
		return switchConfiguration{}, fmt.Errorf("switch configuration must be an object")
	}
	comparator := switchComparator(strings.TrimSpace(fmt.Sprint(value["comparator"])))
	if !validSwitchComparator(comparator) {
		return switchConfiguration{}, fmt.Errorf("switch comparator %q is not supported", comparator)
	}
	items, ok := value["cases"].([]any)
	if !ok {
		return switchConfiguration{}, fmt.Errorf("switch cases must be a list")
	}
	if len(items) == 0 {
		return switchConfiguration{}, fmt.Errorf("add at least one switch case")
	}
	configuration := switchConfiguration{Comparator: comparator, Cases: make([]switchCase, 0, len(items))}
	ids := make(map[string]struct{}, len(items))
	labels := make(map[string]struct{}, len(items))
	for index, rawCase := range items {
		item, ok := rawCase.(map[string]any)
		if !ok {
			return switchConfiguration{}, fmt.Errorf("switch case %d must be an object", index+1)
		}
		caseValue, err := parseSwitchCase(item, comparator, index)
		if err != nil {
			return switchConfiguration{}, err
		}
		if _, duplicate := ids[caseValue.ID]; duplicate {
			return switchConfiguration{}, fmt.Errorf("switch cases contain duplicate ID %q", caseValue.ID)
		}
		ids[caseValue.ID] = struct{}{}
		labelKey := strings.ToLower(caseValue.Label)
		if _, duplicate := labels[labelKey]; duplicate {
			return switchConfiguration{}, fmt.Errorf("switch cases contain duplicate pin name %q", caseValue.Label)
		}
		labels[labelKey] = struct{}{}
		configuration.Cases = append(configuration.Cases, caseValue)
	}
	return configuration, nil
}

func parseSwitchCase(value map[string]any, comparator switchComparator, index int) (switchCase, error) {
	id := strings.TrimSpace(fmt.Sprint(value["id"]))
	if id == "" {
		return switchCase{}, fmt.Errorf("switch case %d needs an ID", index+1)
	}
	label := strings.TrimSpace(fmt.Sprint(value["label"]))
	if label == "" {
		return switchCase{}, fmt.Errorf("switch case %q needs a pin name", id)
	}
	valueType := domain.DataType(strings.TrimSpace(fmt.Sprint(value["valueType"])))
	if !switchValueTypeAllowed(comparator, valueType) {
		return switchCase{}, fmt.Errorf("switch case %q cannot use %s with comparator %q", id, valueType, comparator)
	}
	literal, err := switchLiteral(value["value"], valueType)
	if err != nil {
		return switchCase{}, fmt.Errorf("switch case %q has invalid %s value: %w", id, valueType, err)
	}
	return switchCase{ID: id, Label: label, ValueType: valueType, Value: literal}, nil
}

func legacySwitchConfiguration(raw any) (switchConfiguration, error) {
	ports, err := routeOptionPorts(raw)
	if err != nil {
		return switchConfiguration{}, err
	}
	cases := make([]switchCase, 0, len(ports))
	for _, port := range ports {
		cases = append(cases, switchCase{ID: port.ID, Label: port.Label, ValueType: domain.DataText, Value: port.ID})
	}
	return switchConfiguration{Comparator: switchEquals, Cases: cases, Legacy: true}, nil
}

func switchCasePorts(defaults map[string]any, node domain.FlowNode) ([]domain.NodePort, error) {
	configuration, err := switchConfigurationFor(node, defaults)
	if err != nil {
		return nil, err
	}
	ports := make([]domain.NodePort, 0, len(configuration.Cases))
	for _, item := range configuration.Cases {
		ports = append(ports, domain.NodePort{ID: item.ID, Label: item.Label, Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1})
	}
	return ports, nil
}

func validSwitchComparator(value switchComparator) bool {
	switch value {
	case switchEquals, switchNotEquals, switchContains, switchStartsWith, switchEndsWith, switchGreaterThan, switchGreaterThanOrEqual, switchLessThan, switchLessThanOrEqual:
		return true
	default:
		return false
	}
}

func switchValueTypeAllowed(comparator switchComparator, valueType domain.DataType) bool {
	switch comparator {
	case switchEquals, switchNotEquals:
		return valueType == domain.DataText || valueType == domain.DataNumber || valueType == domain.DataBoolean
	case switchContains, switchStartsWith, switchEndsWith:
		return valueType == domain.DataText
	case switchGreaterThan, switchGreaterThanOrEqual, switchLessThan, switchLessThanOrEqual:
		return valueType == domain.DataNumber
	default:
		return false
	}
}

func switchLiteral(value any, valueType domain.DataType) (any, error) {
	switch valueType {
	case domain.DataText:
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("must be text")
		}
		return text, nil
	case domain.DataBoolean:
		boolean, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("must be true or false")
		}
		return boolean, nil
	case domain.DataNumber:
		number, ok := strictNumber(value)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, fmt.Errorf("must be a finite number")
		}
		return number, nil
	default:
		return nil, fmt.Errorf("unsupported value type %q", valueType)
	}
}

func matchSwitchCase(input any, configuration switchConfiguration, item switchCase) (bool, error) {
	if configuration.Legacy {
		selection := strings.ToLower(strings.TrimSpace(fmt.Sprint(input)))
		return selection == strings.ToLower(item.ID), nil
	}
	switch configuration.Comparator {
	case switchEquals:
		return switchEqual(input, item.Value, item.ValueType), nil
	case switchNotEquals:
		return !switchEqual(input, item.Value, item.ValueType), nil
	case switchContains, switchStartsWith, switchEndsWith:
		text, ok := input.(string)
		if !ok {
			return false, fmt.Errorf("switch %s comparator requires a Text value, received %T", configuration.Comparator, input)
		}
		needle := item.Value.(string)
		switch configuration.Comparator {
		case switchContains:
			return strings.Contains(text, needle), nil
		case switchStartsWith:
			return strings.HasPrefix(text, needle), nil
		default:
			return strings.HasSuffix(text, needle), nil
		}
	case switchGreaterThan, switchGreaterThanOrEqual, switchLessThan, switchLessThanOrEqual:
		number, ok := strictNumber(input)
		if !ok {
			return false, fmt.Errorf("switch %s comparator requires a Number value, received %T", configuration.Comparator, input)
		}
		caseNumber := item.Value.(float64)
		switch configuration.Comparator {
		case switchGreaterThan:
			return number > caseNumber, nil
		case switchGreaterThanOrEqual:
			return number >= caseNumber, nil
		case switchLessThan:
			return number < caseNumber, nil
		default:
			return number <= caseNumber, nil
		}
	default:
		return false, fmt.Errorf("unsupported switch comparator %q", configuration.Comparator)
	}
}

func switchEqual(input, expected any, valueType domain.DataType) bool {
	switch valueType {
	case domain.DataText:
		actual, ok := input.(string)
		return ok && actual == expected.(string)
	case domain.DataBoolean:
		actual, ok := input.(bool)
		return ok && actual == expected.(bool)
	case domain.DataNumber:
		actual, ok := strictNumber(input)
		return ok && actual == expected.(float64)
	default:
		return false
	}
}

func strictNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	default:
		return 0, false
	}
}

package switchnode

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	flow "github.com/FlameInTheDark/neuropipe/internal/nodes/flow"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput), flow.Data("selection", "Value", domain.PinInput, domain.DataAny)}
	defaults := map[string]any{"switch": map[string]any{"comparator": "equals", "cases": []any{map[string]any{"id": "case-a", "label": "Case A", "valueType": "text", "value": "case-a"}, map[string]any{"id": "case-b", "label": "Case B", "valueType": "text", "value": "case-b"}}}}
	definition := flow.Node("flow:switch", "Flow", "Switch", "Route execution to a matching named output.", "split", inputs, []domain.NodePort{flow.Exec("default", "Default", domain.PinOutput)}, []domain.ConfigField{flow.SwitchCases("switch", "Cases")}, defaults)
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		configuration, err := configurationFor(config(node), definition.DefaultConfig)
		if err != nil {
			return definition, err
		}
		resolved := definition
		ports := make([]domain.NodePort, 0, len(configuration.Cases))
		for _, item := range configuration.Cases {
			ports = append(ports, flow.Exec(item.ID, item.Label, domain.PinOutput))
		}
		resolved.Outputs = append(ports, definition.Outputs...)
		return resolved, nil
	}, Executor: Execute})
}

// Execute evaluates one declared Switch case with strict input/literal types.
func Execute(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	configuration, err := configurationFor(invocation.Config, invocation.Definition.DefaultConfig)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	value := invocation.Inputs["selection"]
	result := map[string]any{"value": value, "comparator": string(configuration.Comparator), "matchedCase": nil}
	for _, item := range configuration.Cases {
		matched, err := match(value, configuration, item)
		if err != nil {
			return nodes.ExecutionResult{}, err
		}
		if matched {
			result["matchedCase"] = map[string]any{"id": item.ID, "label": item.Label}
			return nodes.ExecutionResult{Outputs: map[string]any{"result": result}, Ports: []string{item.ID}}, nil
		}
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": result}, Ports: []string{"default"}}, nil
}

type comparator string

const (
	equals             comparator = "equals"
	notEquals          comparator = "not_equals"
	contains           comparator = "contains"
	startsWith         comparator = "starts_with"
	endsWith           comparator = "ends_with"
	greaterThan        comparator = "greater_than"
	greaterThanOrEqual comparator = "greater_than_or_equal"
	lessThan           comparator = "less_than"
	lessThanOrEqual    comparator = "less_than_or_equal"
)

type caseDefinition struct {
	ID        string
	Label     string
	ValueType domain.DataType
	Value     any
}

type configuration struct {
	Comparator comparator
	Cases      []caseDefinition
	Legacy     bool
}

func configurationFor(config, defaults map[string]any) (configuration, error) {
	if raw, exists := config["switch"]; exists {
		return parseConfiguration(raw)
	}
	if legacy, exists := config["options"]; exists {
		return legacyConfiguration(legacy)
	}
	return parseConfiguration(defaults["switch"])
}

func parseConfiguration(raw any) (configuration, error) {
	value, ok := raw.(map[string]any)
	if !ok {
		return configuration{}, fmt.Errorf("switch configuration must be an object")
	}
	selected := comparator(strings.TrimSpace(fmt.Sprint(value["comparator"])))
	if !validComparator(selected) {
		return configuration{}, fmt.Errorf("switch comparator %q is not supported", selected)
	}
	items, ok := value["cases"].([]any)
	if !ok {
		return configuration{}, fmt.Errorf("switch cases must be a list")
	}
	if len(items) == 0 {
		return configuration{}, fmt.Errorf("add at least one switch case")
	}
	result := configuration{Comparator: selected, Cases: make([]caseDefinition, 0, len(items))}
	ids, labels := make(map[string]struct{}, len(items)), make(map[string]struct{}, len(items))
	for index, rawCase := range items {
		item, ok := rawCase.(map[string]any)
		if !ok {
			return configuration{}, fmt.Errorf("switch case %d must be an object", index+1)
		}
		parsed, err := parseCase(item, selected, index)
		if err != nil {
			return configuration{}, err
		}
		if _, exists := ids[parsed.ID]; exists {
			return configuration{}, fmt.Errorf("switch cases contain duplicate ID %q", parsed.ID)
		}
		if _, exists := labels[strings.ToLower(parsed.Label)]; exists {
			return configuration{}, fmt.Errorf("switch cases contain duplicate pin name %q", parsed.Label)
		}
		ids[parsed.ID] = struct{}{}
		labels[strings.ToLower(parsed.Label)] = struct{}{}
		result.Cases = append(result.Cases, parsed)
	}
	return result, nil
}

func parseCase(value map[string]any, selected comparator, index int) (caseDefinition, error) {
	id := strings.TrimSpace(fmt.Sprint(value["id"]))
	if id == "" {
		return caseDefinition{}, fmt.Errorf("switch case %d needs an ID", index+1)
	}
	label := strings.TrimSpace(fmt.Sprint(value["label"]))
	if label == "" {
		return caseDefinition{}, fmt.Errorf("switch case %q needs a pin name", id)
	}
	valueType := domain.DataType(strings.TrimSpace(fmt.Sprint(value["valueType"])))
	if !valueTypeAllowed(selected, valueType) {
		return caseDefinition{}, fmt.Errorf("switch case %q cannot use %s with comparator %q", id, valueType, selected)
	}
	literal, err := literal(value["value"], valueType)
	if err != nil {
		return caseDefinition{}, fmt.Errorf("switch case %q has invalid %s value: %w", id, valueType, err)
	}
	return caseDefinition{ID: id, Label: label, ValueType: valueType, Value: literal}, nil
}

func legacyConfiguration(value any) (configuration, error) {
	items, ok := value.([]any)
	if !ok {
		return configuration{}, fmt.Errorf("options must be a list")
	}
	if len(items) == 0 {
		return configuration{}, fmt.Errorf("add at least one option")
	}
	result := configuration{Comparator: equals, Legacy: true, Cases: make([]caseDefinition, 0, len(items))}
	seen := make(map[string]struct{}, len(items))
	for index, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return configuration{}, fmt.Errorf("option %d must be an object", index+1)
		}
		id := strings.TrimSpace(fmt.Sprint(item["id"]))
		if id == "" {
			return configuration{}, fmt.Errorf("option %d needs an ID", index+1)
		}
		if _, exists := seen[id]; exists {
			return configuration{}, fmt.Errorf("options contain duplicate ID %q", id)
		}
		seen[id] = struct{}{}
		label := strings.TrimSpace(fmt.Sprint(item["label"]))
		if label == "" {
			label = id
		}
		result.Cases = append(result.Cases, caseDefinition{ID: id, Label: label, ValueType: domain.DataText, Value: id})
	}
	return result, nil
}

func validComparator(value comparator) bool {
	switch value {
	case equals, notEquals, contains, startsWith, endsWith, greaterThan, greaterThanOrEqual, lessThan, lessThanOrEqual:
		return true
	default:
		return false
	}
}

func valueTypeAllowed(selected comparator, valueType domain.DataType) bool {
	switch selected {
	case equals, notEquals:
		return valueType == domain.DataText || valueType == domain.DataNumber || valueType == domain.DataBoolean
	case contains, startsWith, endsWith:
		return valueType == domain.DataText
	case greaterThan, greaterThanOrEqual, lessThan, lessThanOrEqual:
		return valueType == domain.DataNumber
	default:
		return false
	}
}

func literal(value any, valueType domain.DataType) (any, error) {
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
		number, ok := number(value)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, fmt.Errorf("must be a finite number")
		}
		return number, nil
	default:
		return nil, fmt.Errorf("unsupported value type %q", valueType)
	}
}

func match(input any, configuration configuration, item caseDefinition) (bool, error) {
	if configuration.Legacy {
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(input)), item.ID), nil
	}
	switch configuration.Comparator {
	case equals:
		return equal(input, item.Value, item.ValueType), nil
	case notEquals:
		return !equal(input, item.Value, item.ValueType), nil
	case contains, startsWith, endsWith:
		text, ok := input.(string)
		if !ok {
			return false, fmt.Errorf("switch %s comparator requires a Text value, received %T", configuration.Comparator, input)
		}
		needle := item.Value.(string)
		if configuration.Comparator == contains {
			return strings.Contains(text, needle), nil
		}
		if configuration.Comparator == startsWith {
			return strings.HasPrefix(text, needle), nil
		}
		return strings.HasSuffix(text, needle), nil
	case greaterThan, greaterThanOrEqual, lessThan, lessThanOrEqual:
		actual, ok := number(input)
		if !ok {
			return false, fmt.Errorf("switch %s comparator requires a Number value, received %T", configuration.Comparator, input)
		}
		expected := item.Value.(float64)
		switch configuration.Comparator {
		case greaterThan:
			return actual > expected, nil
		case greaterThanOrEqual:
			return actual >= expected, nil
		case lessThan:
			return actual < expected, nil
		default:
			return actual <= expected, nil
		}
	default:
		return false, fmt.Errorf("unsupported switch comparator %q", configuration.Comparator)
	}
}

func equal(input, expected any, valueType domain.DataType) bool {
	switch valueType {
	case domain.DataText:
		actual, ok := input.(string)
		return ok && actual == expected.(string)
	case domain.DataBoolean:
		actual, ok := input.(bool)
		return ok && actual == expected.(bool)
	case domain.DataNumber:
		actual, ok := number(input)
		return ok && actual == expected.(float64)
	default:
		return false
	}
}

func number(value any) (float64, bool) {
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

func config(node domain.FlowNode) map[string]any {
	if value, ok := node.Data["config"].(map[string]any); ok {
		return value
	}
	return node.Data
}

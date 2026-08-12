// Package setglobalvariable registers Set Global Variable, the write half of
// the workspace-scoped Global Variables feature. Unlike the execution-local
// Set Variable, writes are shared, durable, and safe under concurrent pipeline
// runs through the host's atomic Increment/Append operations.
package setglobalvariable

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	flownodes "github.com/FlameInTheDark/neuropipe/internal/nodes/flow"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{
		flownodes.Exec("in", "Exec", domain.PinInput),
		flownodes.Data("value", "Value", domain.PinInput, domain.DataAny),
	}
	inputs[1].IgnoreConfigFallback = true
	outputs := []domain.NodePort{
		flownodes.Exec("out", "Then", domain.PinOutput),
		flownodes.Data("result", "Value", domain.PinOutput, domain.DataAny),
	}
	fields := []domain.ConfigField{
		{Name: "name", Label: "Variable", Kind: "select", Required: true},
		{Name: "operation", Label: "Operation", Kind: "select", Required: true, Options: []domain.Option{
			{Value: "set", Label: "Set"},
			{Value: "increment", Label: "Increment (number)"},
			{Value: "append", Label: "Append (list)"},
		}},
		{Name: "value", Label: "Value", Kind: "string", Placeholder: "Literal value when the pin is not connected", Required: false},
	}
	metadata := flownodes.Node("flow:set_global_variable", "Data", "Set Global Variable", "Write a workspace variable shared across pipelines and runs. Persisted between restarts.", "database-plus", inputs, outputs, fields, map[string]any{"name": "", "operation": "set", "value": ""})
	return registrar.Register(Node{
		Metadata: metadata,
		Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
			return resolve(metadata, node), nil
		},
		Executor: Execute,
	})
}

func resolve(metadata domain.NodeDefinition, node domain.FlowNode) domain.NodeDefinition {
	resolved := metadata
	resolved.Fields = injectOptions(metadata.Fields)
	config, _ := node.Data["config"].(map[string]any)
	name, _ := config["name"].(string)
	operation, _ := config["operation"].(string)

	dataType := declaredTypeOf(strings.TrimSpace(name))
	resultType := dataType
	switch operation {
	case "increment":
		dataType = domain.DataNumber
		resultType = domain.DataNumber
	case "append":
		// Append consumes one item, not the variable's complete list value.
		dataType = domain.DataAny
		resultType = domain.DataList
	}
	if dataType != domain.DataAny {
		resolved.Inputs = typedPorts(metadata.Inputs, "value", dataType)
	}
	if resultType != domain.DataAny {
		resolved.Outputs = typedPorts(metadata.Outputs, "result", resultType)
	}
	return resolved
}

func typedPorts(ports []domain.NodePort, id string, dataType domain.DataType) []domain.NodePort {
	resolved := append([]domain.NodePort(nil), ports...)
	for index := range resolved {
		if resolved[index].ID != id {
			continue
		}
		resolved[index].DataType = dataType
		typeSpec := typespec.FromDataType(dataType)
		resolved[index].Type = &typeSpec
	}
	return resolved
}

// declaredOptions and declaredType are injected by the variables service so
// the resolved editor definition carries the live name picklist, and the
// literal fallback can coerce to the declaration's data type.
var (
	declaredOptions func() []domain.Option
	declaredType    func(name string) (domain.DataType, bool)
)

// SetDeclaredOptions wires the variable name picklist into the resolver.
func SetDeclaredOptions(source func() []domain.Option) {
	declaredOptions = source
}

// SetDeclaredType wires the variable type lookup used to coerce literals.
func SetDeclaredType(resolver func(name string) (domain.DataType, bool)) {
	declaredType = resolver
}

// declaredTypeOf resolves a variable name to its declared type. Unknown names
// fall back to Any so the literal can pass through untouched until the host
// reports the real configuration error.
func declaredTypeOf(name string) domain.DataType {
	if declaredType == nil {
		return domain.DataAny
	}
	resolved, ok := declaredType(name)
	if !ok {
		return domain.DataAny
	}
	return resolved
}

// literalValue coerces the inspector text field to the variable's declared
// type. An empty literal means "no value configured" so operators can use
// their natural default, e.g. increment-by-one.
func literalValue(raw any, declared domain.DataType) (any, bool, error) {
	text, ok := raw.(string)
	if !ok {
		if raw == nil {
			return nil, false, nil
		}
		return raw, true, nil
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, false, nil
	}
	switch declared {
	case domain.DataNumber:
		number, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return nil, false, fmt.Errorf("value %q is not a number: %w", trimmed, err)
		}
		return number, true, nil
	case domain.DataBoolean:
		boolean, err := strconv.ParseBool(trimmed)
		if err != nil {
			return nil, false, fmt.Errorf("value %q is not a boolean; use true or false", trimmed)
		}
		return boolean, true, nil
	case domain.DataObject:
		var object map[string]any
		if err := json.Unmarshal([]byte(trimmed), &object); err != nil {
			return nil, false, fmt.Errorf("value must be a JSON object: %w", err)
		}
		return object, true, nil
	case domain.DataList:
		var list []any
		if err := json.Unmarshal([]byte(trimmed), &list); err != nil {
			return nil, false, fmt.Errorf("value must be a JSON list: %w", err)
		}
		return list, true, nil
	default:
		return text, true, nil
	}
}

// injectOptions refreshes the name select from the live declaration list. The
// metadata slice is not shared with other nodes, but cloning keeps the helper
// safe against future reuse.
func injectOptions(fields []domain.ConfigField) []domain.ConfigField {
	cloned := append([]domain.ConfigField(nil), fields...)
	if declaredOptions == nil {
		return cloned
	}
	options := append([]domain.Option(nil), declaredOptions()...)
	for index, field := range cloned {
		if field.Name == "name" && field.Kind == "select" {
			field.Options = options
			cloned[index] = field
		}
	}
	return cloned
}

// Execute routes the write through the atomic host capability. The host
// validates the declared type, performs the mutation under a lock, and marks
// the value dirty for the next durability flush.
func Execute(_ context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	globals, ok := runtime.(nodes.GlobalVariableWriter)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("global variable runtime is unavailable")
	}
	name, _ := invocation.Config["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("select a variable to write")
	}
	operation, _ := invocation.Config["operation"].(string)
	if operation == "" {
		operation = "set"
	}
	value := invocation.Inputs["value"]
	if !invocation.ConnectedInputs["value"] {
		coerced, hasLiteral, err := literalValue(invocation.Config["value"], declaredTypeOf(name))
		if err != nil {
			return nodes.ExecutionResult{}, err
		}
		value = nil
		if hasLiteral {
			value = coerced
		}
	}
	switch operation {
	case "set":
		if value == nil {
			return nodes.ExecutionResult{}, fmt.Errorf("connect the Value pin or enter a literal value")
		}
		if err := globals.WriteGlobalVariable(name, value); err != nil {
			return nodes.ExecutionResult{}, err
		}
	case "increment":
		// A disconnected Value pin with an empty literal means "add one" —
		// the idiomatic counter step. Anything provided must be numeric.
		delta := float64(1)
		if value != nil {
			parsed, err := numeric(value)
			if err != nil {
				return nodes.ExecutionResult{}, fmt.Errorf("increment value: %w", err)
			}
			delta = parsed
		}
		result, err := globals.IncrementGlobalVariable(name, delta)
		if err != nil {
			return nodes.ExecutionResult{}, err
		}
		value = result
	case "append":
		list, err := globals.AppendGlobalVariable(name, value)
		if err != nil {
			return nodes.ExecutionResult{}, err
		}
		value = list
	default:
		return nodes.ExecutionResult{}, fmt.Errorf("unknown operation %q", operation)
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": value}, Ports: []string{"out"}}, nil
}

func numeric(value any) (float64, error) {
	switch number := value.(type) {
	case float64:
		return number, nil
	case float32:
		return float64(number), nil
	case int:
		return float64(number), nil
	case int8:
		return float64(number), nil
	case int16:
		return float64(number), nil
	case int32:
		return float64(number), nil
	case int64:
		return float64(number), nil
	case uint:
		return float64(number), nil
	case uint8:
		return float64(number), nil
	case uint16:
		return float64(number), nil
	case uint32:
		return float64(number), nil
	case uint64:
		return float64(number), nil
	}
	return 0, fmt.Errorf("value of type %T is not numeric", value)
}

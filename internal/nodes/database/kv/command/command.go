// Package command registers the generic Redis command node with dynamic
// typed argument pins. It is the key/value analogue of action:sql: every
// Redis command is reachable even when no curated node exists.
package command

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	kvnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

const commandInputPinID = "command"

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// reservedParameterIDs are pin IDs a configured argument cannot reuse because
// they belong to the node's static input contract.
var reservedParameterIDs = map[string]struct{}{
	commandInputPinID: {},
}

// New creates the generic command module implementation.
func New() Node {
	return Node{Metadata: definition(), Resolver: resolve, Executor: execute}
}

func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_command", "KV Command", "Run any Redis command with typed argument pins.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			commandPin(),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Any("value", "Value", domain.PinOutput),
			kvnodes.Text("valueText", "Value (text)", domain.PinOutput, false),
			kvnodes.Bool("isNil", "Is nil", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "command", Label: "Command", Kind: "string", Placeholder: "HGETALL", Required: true},
			{Name: "arguments", Label: "Arguments", Kind: "kv-arguments"},
			{Name: "allowDangerous", Label: "Allow dangerous commands", Kind: "boolean"},
		},
		map[string]any{"command": "", "arguments": []any{}, "allowDangerous": false},
	)
}

// commandPin exposes the command override. A wired value replaces the
// editor-configured command so pipelines can compute it at run time.
func commandPin() domain.NodePort {
	pin := kvnodes.Text(commandInputPinID, "Command", domain.PinInput, false)
	pin.IgnoreConfigFallback = true
	return pin
}

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	result := definition()
	arguments, err := configuredArguments(config(node))
	if err != nil {
		return result, err
	}
	inputs := append([]domain.NodePort(nil), result.Inputs...)
	for _, argument := range arguments {
		typeSpec := argument.Type
		pin := domain.NodePort{
			ID: argument.ID, Label: argument.Label, Kind: domain.PinData, Direction: domain.PinInput,
			DataType: dataType(typeSpec), Type: &typeSpec, Color: color(typeSpec),
			Required: argument.Required, MaxConnections: 1,
		}
		inputs = append(inputs, pin)
	}
	result.Inputs = inputs
	return result, nil
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	// The command pin is config-fallback-exempt (like action:sql's SQL pin):
	// the editor value is the source of truth and a wired pin overrides it.
	command, _ := invocation.Config["command"].(string)
	command = strings.TrimSpace(command)
	if invocation.ConnectedInputs[commandInputPinID] {
		if wired := strings.TrimSpace(kvnodes.String(invocation, commandInputPinID)); wired != "" {
			command = wired
		}
	}
	if command == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("redis command is required")
	}
	arguments, err := configuredArguments(invocation.Config)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	args := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		value, exists := invocation.Inputs[argument.ID]
		if !exists && argument.Required {
			return nodes.ExecutionResult{}, fmt.Errorf("command argument %q is required", argument.Label)
		}
		if !exists {
			continue
		}
		converted, err := kvnodes.Arg(value)
		if err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("command argument %q: %w", argument.Label, err)
		}
		args = append(args, converted)
	}
	allowDangerous := false
	if value, ok := invocation.Config["allowDangerous"].(bool); ok {
		allowDangerous = value
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{
		Command: command, Args: args, AllowDangerous: allowDangerous,
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	valueText := ""
	if !result.IsNil && result.Value != nil {
		switch typed := result.Value.(type) {
		case string:
			valueText = typed
		default:
			if encoded, err := json.Marshal(typed); err == nil {
				valueText = string(encoded)
			} else {
				valueText = fmt.Sprintf("%v", typed)
			}
		}
	}
	value := any(nil)
	if !result.IsNil {
		value = result.Value
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"value": value, "valueText": valueText, "isNil": result.IsNil},
		Ports:   []string{"out"},
	}, nil
}

// configuredArguments validates the persisted argument contract. The shape
// mirrors the SQL node's parameter contract so the editor reuses its UI.
func configuredArguments(values map[string]any) ([]domain.KVArgument, error) {
	raw, exists := values["arguments"]
	if !exists || raw == nil {
		return []domain.KVArgument{}, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("command arguments must be a list")
	}
	arguments := make([]domain.KVArgument, 0, len(items))
	ids := make(map[string]struct{})
	for index, item := range items {
		encoded, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("command argument %d: %w", index+1, err)
		}
		var argument domain.KVArgument
		if err := json.Unmarshal(encoded, &argument); err != nil {
			return nil, fmt.Errorf("command argument %d: %w", index+1, err)
		}
		argument.ID = strings.TrimSpace(argument.ID)
		argument.Name = strings.TrimLeft(strings.TrimSpace(argument.Name), ":@$")
		argument.Label = strings.TrimSpace(argument.Label)
		if !identifier.MatchString(argument.ID) {
			return nil, fmt.Errorf("command argument %d has an invalid pin ID", index+1)
		}
		if !identifier.MatchString(argument.Name) {
			return nil, fmt.Errorf("command argument %d has an invalid name", index+1)
		}
		if _, reserved := reservedParameterIDs[argument.ID]; reserved {
			return nil, fmt.Errorf("command argument %d uses a reserved pin ID %q", index+1, argument.ID)
		}
		if _, reserved := reservedParameterIDs[argument.Name]; reserved {
			return nil, fmt.Errorf("command argument %d uses a reserved name %q", index+1, argument.Name)
		}
		if _, duplicate := ids[argument.ID]; duplicate {
			return nil, fmt.Errorf("command arguments contain duplicate pin ID %q", argument.ID)
		}
		if err := typespec.ValidateSpec(argument.Type); err != nil {
			return nil, fmt.Errorf("command argument %q type: %w", argument.Name, err)
		}
		if argument.Label == "" {
			argument.Label = argument.Name
		}
		ids[argument.ID] = struct{}{}
		arguments = append(arguments, argument)
	}
	return arguments, nil
}

func config(node domain.FlowNode) map[string]any {
	if config, ok := node.Data["config"].(map[string]any); ok {
		return config
	}
	return map[string]any{}
}

func dataType(spec domain.TypeSpec) domain.DataType {
	switch spec.Kind {
	case domain.TypeString, domain.TypeBytes:
		return domain.DataText
	case domain.TypeInt, domain.TypeFloat:
		return domain.DataNumber
	case domain.TypeBool:
		return domain.DataBoolean
	case domain.TypeList:
		return domain.DataList
	case domain.TypeMap, domain.TypeRecord:
		return domain.DataObject
	default:
		return domain.DataAny
	}
}

func color(spec domain.TypeSpec) string {
	switch dataType(spec) {
	case domain.DataText:
		return "#e879f9"
	case domain.DataNumber:
		return "#86efac"
	case domain.DataBoolean:
		return "#f87171"
	case domain.DataObject:
		return "#60a5fa"
	case domain.DataList:
		return "#facc15"
	default:
		return "#a1a1aa"
	}
}

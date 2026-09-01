// Package appcommand registers the Discord application command trigger
// module: one dedicated trigger per slash / user / message command.
package appcommand

import (
	"context"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/discordspec"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Resolver: resolve, Executor: execute} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	return domain.NodeDefinition{
		Type: "discord:app_command", Category: "Discord", Label: "Discord Command Trigger", Description: "Start a trusted pipeline when a member runs one of the bot's application commands, with the command's options as typed pins.", Icon: "slash", Color: "#5865f2", Mode: domain.NodeEvent, TriggerKind: domain.TriggerDiscord, PortContractOwned: true,
		Inputs:  []domain.NodePort{},
		Outputs: []domain.NodePort{exec("out", "Start")},
		Fields: []domain.ConfigField{
			{Name: "eventType", Label: "Event", Kind: "select", Options: commandEventOptions(), Required: true},
			{Name: "identityId", Label: "Bot identity", Kind: "discord-identity", Required: true},
			{Name: "command", Label: "Command", Kind: "discord-command", Required: true},
			{Name: "responseMode", Label: "Response mode", Kind: "select", Options: []domain.Option{
				{Value: "", Label: "Auto defer (15 min to reply)"},
				{Value: "manual", Label: "Reply within 3 s (ephemeral possible)"},
			}},
			{Name: "guildId", Label: "Guild ID", Kind: "string", Placeholder: "optional guild snowflake"},
			{Name: "channelId", Label: "Channel ID", Kind: "string", Placeholder: "optional channel snowflake"},
		},
		DefaultConfig: map[string]any{"eventType": "application.command", "identityId": "", "command": map[string]any{}, "responseMode": "", "guildId": "", "channelId": ""}, Source: "builtin",
	}
}

func commandEventOptions() []domain.Option {
	return []domain.Option{{Value: "application.command", Label: "Application command used"}}
}

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := definition()
	selection, err := commandSelection(node)
	if err != nil {
		return domain.NodeDefinition{}, err
	}
	if selection.Name != "" {
		definition.Description = fmt.Sprintf("Start a trusted pipeline when a member runs /%s, with the command's options as typed pins.", selection.Name)
	}
	definition.Outputs = commandOutputs(selection)
	return definition, nil
}

// commandSelection extracts the command selector stored by the inspector's
// discord-command field: name, id, and the option schema that drives the
// dynamic output pins.
func commandSelection(node domain.FlowNode) (commandSelectionValue, error) {
	raw, _ := config(node)["command"].(map[string]any)
	result := commandSelectionValue{Name: stringOf(raw["commandName"])}
	if result.Name == "" {
		// The static contract (no command picked yet) is valid: pins appear
		// once a command is selected.
		return result, nil
	}
	result.ID = stringOf(raw["commandId"])
	if options, ok := raw["options"].([]any); ok {
		for _, item := range options {
			option, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := stringOf(option["name"])
			if name == "" {
				continue
			}
			result.Options = append(result.Options, commandOption{
				Name: name, Type: intOf(option["type"]), Required: boolOf(option["required"]),
			})
		}
	}
	return result, nil
}

type commandSelectionValue struct {
	Name    string
	ID      string
	Options []commandOption
}

type commandOption struct {
	Name     string
	Type     int
	Required bool
}

// commandOutputs exposes the envelope pins plus one typed data pin per
// command option. Option pins mirror the wire types Discord delivers:
// boolean options become booleans, integers and numbers become numbers,
// everything else stays text (snowflakes, strings, attachments).
func commandOutputs(command commandSelectionValue) []domain.NodePort {
	outputs := []domain.NodePort{exec("out", "Start"),
		eventPin("event", "Event", discordspec.EventType()),
		eventPin("command", "Command", discordspec.CommandEventType()),
		eventPin("interaction", "Interaction", discordspec.InteractionRefType()),
		stringPin("commandName", "Command name"),
		stringPin("commandId", "Command ID"),
		objectPin("options", "Options"),
		stringPin("userId", "User ID"),
		stringPin("username", "Username"),
		stringPin("nickname", "Nickname"),
		stringPin("channelId", "Channel ID"),
		stringPin("guildId", "Guild ID"),
		stringPin("targetUserId", "Target user ID"),
		stringPin("targetUsername", "Target username"),
		stringPin("targetMessageId", "Target message ID"),
		stringPin("locale", "Locale"),
	}
	for _, option := range command.Options {
		switch option.Type {
		case 5: // BOOLEAN
			outputs = append(outputs, boolOptionPin(option.Name, option.Required))
		case 4: // INTEGER
			outputs = append(outputs, intOptionPin(option.Name, option.Required))
		case 10: // NUMBER
			outputs = append(outputs, numberOptionPin(option.Name, option.Required))
		default:
			outputs = append(outputs, stringOptionPin(option.Name, option.Required))
		}
	}
	return outputs
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, err
	}
	if stringValue(invocation.Config, "eventType") != "application.command" {
		return nodes.ExecutionResult{}, fmt.Errorf("unsupported Discord command event")
	}
	eventValue, ok := invocation.Inputs["event"].(discordspec.DiscordEvent)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("discord trigger requires a typed gateway event")
	}
	if eventValue.Type != "application.command" {
		return nodes.ExecutionResult{}, fmt.Errorf("received Discord event %q for the command trigger", eventValue.Type)
	}
	command, ok := eventValue.Payload["command"].(discordspec.CommandEvent)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("discord application command payload is malformed")
	}
	outputs := map[string]any{
		"event": eventValue, "command": command, "interaction": command.Command,
		"commandName": command.CommandName, "commandId": command.CommandID,
		"options": command.Options,
		"userId":  command.UserID, "username": command.Username, "nickname": command.Nickname,
		"channelId": command.ChannelID, "guildId": command.GuildID,
		"targetUserId": command.TargetUserID, "targetUsername": command.TargetUsername,
		"targetMessageId": command.TargetMessageID, "locale": command.Locale,
	}
	selection, err := selectionFromConfig(invocation.Config)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	if selection.Name != "" {
		if command.CommandName != selection.Name {
			return nodes.ExecutionResult{Outputs: outputs}, nil
		}
		if selection.ID != "" && command.CommandID != "" && command.CommandID != selection.ID {
			return nodes.ExecutionResult{Outputs: outputs}, nil
		}
	}
	// One output per stored option, so a rewired command definition cannot
	// silently drop a wired pin until the graph is re-resolved.
	for _, option := range selection.Options {
		if value, found := command.Options[option.Name]; found {
			outputs[option.Name] = typedOptionValue(option.Type, value)
		}
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"out"}}, nil
}

// selectionFromConfig mirrors commandSelection for the executor's flat
// invocation config shape.
func selectionFromConfig(values map[string]any) (commandSelectionValue, error) {
	node := domain.FlowNode{Data: map[string]any{"config": values}}
	return commandSelection(node)
}

// typedOptionValue converts the wire's string representation into the pin's
// declared type where possible; a malformed value keeps its text so
// downstream nodes surface it instead of failing the whole trigger.
func typedOptionValue(optionType int, value string) any {
	switch optionType {
	case 5:
		return value == "true"
	case 4, 10:
		var parsed float64
		if _, err := fmt.Sscanf(strings.TrimSpace(value), "%g", &parsed); err == nil {
			if optionType == 4 {
				return int(parsed)
			}
			return parsed
		}
		return value
	default:
		return value
	}
}

func stringOf(value any) string {
	result, _ := value.(string)
	return strings.TrimSpace(result)
}
func intOf(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return 0
}
func boolOf(value any) bool {
	result, _ := value.(bool)
	return result
}
func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
func exec(id, label string) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1}
}
func stringPin(id, label string) domain.NodePort {
	spec := typespec.String()
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &spec, Color: "#5865f2", MaxConnections: 1}
}
func eventPin(id, label string, spec domain.TypeSpec) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &spec, Color: "#60a5fa", MaxConnections: 1}
}
func objectPin(id, label string) domain.NodePort {
	key := domain.TypeSpec{Kind: domain.TypeString}
	value := domain.TypeSpec{Kind: domain.TypeString}
	spec := domain.TypeSpec{Kind: domain.TypeMap, Key: &key, Value: &value}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &spec, Color: "#60a5fa", MaxConnections: 1}
}
func stringOptionPin(name string, required bool) domain.NodePort {
	spec := typespec.String()
	return domain.NodePort{ID: name, Label: name, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &spec, Color: "#5865f2", Required: required, MaxConnections: 1}
}
func boolOptionPin(name string, required bool) domain.NodePort {
	spec := typespec.Bool()
	return domain.NodePort{ID: name, Label: name, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataBoolean, Type: &spec, Color: "#f87171", Required: required, MaxConnections: 1}
}
func intOptionPin(name string, required bool) domain.NodePort {
	spec := typespec.Int()
	return domain.NodePort{ID: name, Label: name, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataNumber, Type: &spec, Color: "#a78bfa", Required: required, MaxConnections: 1}
}
func numberOptionPin(name string, required bool) domain.NodePort {
	spec := typespec.Float()
	return domain.NodePort{ID: name, Label: name, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataNumber, Type: &spec, Color: "#a78bfa", Required: required, MaxConnections: 1}
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

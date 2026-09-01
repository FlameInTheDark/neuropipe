// Package event registers the dynamic Discord event trigger module.
package event

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
		Type: "discord:event", Category: "Discord", Label: "Discord Event Trigger", Description: "Start a trusted pipeline from a Discord gateway event.", Icon: "hash", Color: "#5865f2", Mode: domain.NodeEvent, TriggerKind: domain.TriggerDiscord, PortContractOwned: true,
		Inputs:  []domain.NodePort{},
		Outputs: []domain.NodePort{exec("out", "Start")},
		// The static contract mirrors the default event (message.create);
		// resolve() swaps the filter set and output pins for the selected
		// event so every event exposes exactly its own configuration.
		Fields: []domain.ConfigField{
			{Name: "eventType", Label: "Event", Kind: "select", Options: eventOptions(), Required: true},
			{Name: "identityId", Label: "Bot identity", Kind: "discord-identity", Required: true},
			{Name: "prefix", Label: "Message prefix", Kind: "string"},
			{Name: "authorIDs", Label: "Author IDs", Kind: "string", Placeholder: "id-1, id-2"},
			{Name: "caseSensitivePrefix", Label: "Case-sensitive prefix", Kind: "boolean"},
		},
		DefaultConfig: map[string]any{"eventType": "message.create", "identityId": "", "prefix": "", "authorIDs": "", "caseSensitivePrefix": false}, Source: "builtin",
	}
}

func eventOptions() []domain.Option {
	options := make([]domain.Option, 0)
	for _, item := range discordspec.Catalog() {
		options = append(options, domain.Option{Value: item.Type, Label: item.Label})
	}
	return options
}

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := definition()
	eventType, _ := config(node)["eventType"].(string)
	descriptor, found := discordspec.Find(eventType)
	if !found {
		return domain.NodeDefinition{}, fmt.Errorf("unsupported Discord event %q", eventType)
	}
	definition.Fields = eventFields(descriptor)
	definition.Outputs = eventOutputs(descriptor)
	return definition, nil
}

// eventFields derives the inspector contract for one gateway event: the shared
// event and identity selectors, the descriptor's delivery conditions, and only
// the filters meaningful for that event kind. Intents are computed by the
// gateway service from trusted bindings; guild and channel conditions are
// matched before an event reaches this node.
func eventFields(descriptor domain.DiscordEventDescriptor) []domain.ConfigField {
	fields := []domain.ConfigField{
		{Name: "eventType", Label: "Event", Kind: "select", Options: eventOptions(), Required: true},
		{Name: "identityId", Label: "Bot identity", Kind: "discord-identity", Required: true},
	}
	fields = append(fields, conditionFields(descriptor)...)
	switch {
	case descriptor.ChatMessage:
		fields = append(fields,
			domain.ConfigField{Name: "prefix", Label: "Message prefix", Kind: "string"},
			domain.ConfigField{Name: "authorIDs", Label: "Author IDs", Kind: "string", Placeholder: "id-1, id-2"},
			domain.ConfigField{Name: "caseSensitivePrefix", Label: "Case-sensitive prefix", Kind: "boolean"},
		)
	case descriptor.Type == "interaction.create":
		fields = append(fields, domain.ConfigField{Name: "commandName", Label: "Command name", Kind: "string", Placeholder: "help"})
	case descriptor.Type == "message.reaction.add" || descriptor.Type == "message.reaction.remove":
		fields = append(fields, domain.ConfigField{Name: "emoji", Label: "Reaction emoji", Kind: "string", Placeholder: "👍 or thumbsup:1234"})
	case strings.HasPrefix(descriptor.Type, "guild.member."):
		fields = append(fields, domain.ConfigField{Name: "userIDs", Label: "User IDs", Kind: "string", Placeholder: "id-1, id-2"})
	}
	return fields
}

// eventOutputs exposes the typed convenience pins of the selected event in
// addition to the shared envelope pins.
func eventOutputs(descriptor domain.DiscordEventDescriptor) []domain.NodePort {
	outputs := []domain.NodePort{exec("out", "Start"),
		eventPin("event", "Event", discordspec.EventType()),
		stringPin("eventType", "Event type"),
		stringPin("gatewayEvent", "Gateway event"),
		stringPin("messageId", "Message ID"),
		stringPin("receivedAt", "Received at"),
	}
	if descriptor.ChatMessage {
		outputs = append(outputs,
			stringPin("text", "Text"),
			stringPin("commandText", "Command text"),
			stringPin("channelId", "Channel ID"),
			stringPin("channelName", "Channel name"),
			stringPin("guildId", "Guild ID"),
			stringPin("guildName", "Guild name"),
			stringPin("authorId", "Author ID"),
			eventPin("author", "Author", discordspec.AuthorType()),
		)
	}
	switch descriptor.Type {
	case "interaction.create":
		outputs = append(outputs,
			stringPin("commandName", "Command name"),
			objectPin("options", "Options"),
			stringPin("userId", "User ID"),
			stringPin("channelId", "Channel ID"),
			stringPin("guildId", "Guild ID"),
		)
	case "message.reaction.add", "message.reaction.remove":
		outputs = append(outputs,
			stringPin("emoji", "Emoji"),
			stringPin("userId", "User ID"),
			stringPin("channelId", "Channel ID"),
			stringPin("guildId", "Guild ID"),
		)
	case "guild.member.add", "guild.member.remove", "guild.member.update":
		outputs = append(outputs,
			stringPin("userId", "User ID"),
			stringPin("username", "Username"),
			stringPin("nickname", "Nickname"),
			stringPin("guildId", "Guild ID"),
			stringPin("joinedAt", "Joined at"),
		)
	case "guild.ban.add", "guild.ban.remove":
		outputs = append(outputs,
			stringPin("userId", "User ID"),
			stringPin("username", "Username"),
			stringPin("guildId", "Guild ID"),
		)
	case "voice.state.update":
		outputs = append(outputs,
			stringPin("userId", "User ID"),
			stringPin("channelId", "Channel ID"),
			stringPin("guildId", "Guild ID"),
			stringPin("sessionId", "Session ID"),
		)
	}
	return outputs
}

// conditionFields exposes only the descriptor's client-side filters. Intents
// are computed by the gateway service from trusted bindings; guild and
// channel conditions are matched before an event reaches this node.
func conditionFields(descriptor domain.DiscordEventDescriptor) []domain.ConfigField {
	fields := make([]domain.ConfigField, 0, len(descriptor.Conditions))
	for _, condition := range descriptor.Conditions {
		placeholder := "123456789012345678"
		if condition.ID == "guildId" {
			placeholder = "optional guild snowflake"
		}
		fields = append(fields, domain.ConfigField{Name: condition.ID, Label: condition.Label, Kind: "string", Placeholder: placeholder, Required: condition.Required})
	}
	return fields
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, err
	}
	descriptor, found := discordspec.Find(stringValue(invocation.Config, "eventType"))
	if !found {
		return nodes.ExecutionResult{}, fmt.Errorf("unsupported Discord event")
	}
	eventValue, ok := invocation.Inputs["event"].(discordspec.DiscordEvent)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("discord trigger requires a typed gateway event")
	}
	if eventValue.Type != descriptor.Type {
		return nodes.ExecutionResult{}, fmt.Errorf("received Discord event %q for trigger %q", eventValue.Type, descriptor.Type)
	}
	outputs := map[string]any{"event": eventValue, "eventType": eventValue.Type, "gatewayEvent": eventValue.GatewayEvent, "messageId": eventValue.MessageID, "receivedAt": eventValue.ReceivedAt}
	if descriptor.ChatMessage {
		message, ok := eventValue.Payload["chatMessage"].(discordspec.ChatMessage)
		if !ok {
			return nodes.ExecutionResult{}, fmt.Errorf("discord chat message payload is malformed")
		}
		if !matchesChatFilter(message, invocation.Config) {
			return nodes.ExecutionResult{Outputs: outputs}, nil
		}
		if prefix := stringValue(invocation.Config, "prefix"); prefix != "" && hasPrefix(message.Text, prefix, boolValue(invocation.Config, "caseSensitivePrefix")) {
			message.CommandText = strings.TrimSpace(message.Text[len(prefix):])
		} else {
			message.CommandText = message.Text
		}
		outputs["text"] = message.Text
		outputs["commandText"] = message.CommandText
		outputs["channelId"] = message.ChannelID
		outputs["channelName"] = message.ChannelName
		outputs["guildId"] = message.GuildID
		outputs["guildName"] = message.GuildName
		outputs["authorId"] = message.AuthorID
		outputs["author"] = discordspec.DiscordAuthor{ID: message.AuthorID, Username: message.AuthorUsername, Bot: message.AuthorBot}
	}
	switch descriptor.Type {
	case "interaction.create":
		interaction, ok := eventValue.Payload["interaction"].(discordspec.Interaction)
		if !ok {
			return nodes.ExecutionResult{}, fmt.Errorf("discord interaction payload is malformed")
		}
		if command := stringValue(invocation.Config, "commandName"); command != "" && interaction.CommandName != command {
			return nodes.ExecutionResult{Outputs: outputs}, nil
		}
		outputs["commandName"] = interaction.CommandName
		outputs["options"] = interaction.Options
		outputs["userId"] = interaction.UserID
		outputs["channelId"] = interaction.ChannelID
		outputs["guildId"] = interaction.GuildID
	case "message.reaction.add", "message.reaction.remove":
		reaction, ok := eventValue.Payload["reaction"].(discordspec.Reaction)
		if !ok {
			return nodes.ExecutionResult{}, fmt.Errorf("discord reaction payload is malformed")
		}
		if emoji := stringValue(invocation.Config, "emoji"); emoji != "" && reaction.Emoji != emoji {
			return nodes.ExecutionResult{Outputs: outputs}, nil
		}
		outputs["emoji"] = reaction.Emoji
		outputs["userId"] = reaction.UserID
		outputs["channelId"] = reaction.ChannelID
		outputs["guildId"] = reaction.GuildID
	case "guild.member.add", "guild.member.remove", "guild.member.update":
		member, ok := eventValue.Payload["member"].(discordspec.Member)
		if !ok {
			return nodes.ExecutionResult{}, fmt.Errorf("discord member payload is malformed")
		}
		if !matchesIDFilter(member.UserID, stringValue(invocation.Config, "userIDs")) {
			return nodes.ExecutionResult{Outputs: outputs}, nil
		}
		outputs["userId"] = member.UserID
		outputs["username"] = member.Username
		outputs["nickname"] = member.Nickname
		outputs["guildId"] = member.GuildID
		outputs["joinedAt"] = member.JoinedAt
	case "guild.ban.add", "guild.ban.remove":
		outputs["userId"] = eventValue.Payload["userId"]
		outputs["username"] = eventValue.Payload["username"]
		outputs["guildId"] = eventValue.Payload["guildId"]
	case "voice.state.update":
		outputs["userId"] = eventValue.Payload["userId"]
		outputs["channelId"] = eventValue.Payload["channelId"]
		outputs["guildId"] = eventValue.Payload["guildId"]
		outputs["sessionId"] = eventValue.Payload["sessionId"]
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"out"}}, nil
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
func boolValue(values map[string]any, key string) bool { value, _ := values[key].(bool); return value }
func matchesChatFilter(message discordspec.ChatMessage, values map[string]any) bool {
	prefix := stringValue(values, "prefix")
	if prefix != "" {
		if !hasPrefix(message.Text, prefix, boolValue(values, "caseSensitivePrefix")) {
			return false
		}
	}
	return matchesIDFilter(message.AuthorID, stringValue(values, "authorIDs"))
}
func matchesIDFilter(id string, allowed string) bool {
	if strings.TrimSpace(allowed) == "" {
		return true
	}
	for _, candidate := range strings.Split(allowed, ",") {
		if strings.TrimSpace(candidate) == id {
			return true
		}
	}
	return false
}
func hasPrefix(value, prefix string, caseSensitive bool) bool {
	if caseSensitive {
		return strings.HasPrefix(value, prefix)
	}
	return strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix))
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

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

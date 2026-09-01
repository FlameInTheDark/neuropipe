// Package event registers the dynamic Twitch Event Trigger module.
package event

import (
	"context"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/twitchspec"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Resolver: resolve, Executor: execute} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	options := make([]domain.Option, 0)
	for _, item := range twitchspec.Catalog() {
		options = append(options, domain.Option{Value: item.Type, Label: item.Label})
	}
	return domain.NodeDefinition{
		Type: "twitch:event", Category: "Twitch", Label: "Twitch Event Trigger", Description: "Start a trusted pipeline from a Twitch EventSub WebSocket event.", Icon: "radio", Color: "#a970ff", Mode: domain.NodeEvent, TriggerKind: domain.TriggerTwitch, PortContractOwned: true,
		Inputs:  []domain.NodePort{},
		Outputs: []domain.NodePort{exec("out", "Start")},
		Fields: []domain.ConfigField{
			{Name: "eventType", Label: "Event", Kind: "select", Options: options, Required: true},
			{Name: "identityId", Label: "Authorization identity", Kind: "twitch-identity", Required: true},
			{Name: "prefix", Label: "Message prefix", Kind: "string"},
			{Name: "authorIDs", Label: "Author IDs", Kind: "string", Placeholder: "id-1, id-2"},
			{Name: "caseSensitivePrefix", Label: "Case-sensitive prefix", Kind: "boolean"},
		},
		DefaultConfig: map[string]any{"eventType": "channel.chat.message", "identityId": "", "channel": "", "prefix": "", "authorIDs": "", "caseSensitivePrefix": false}, Source: "builtin",
	}
}

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := definition()
	eventType, _ := config(node)["eventType"].(string)
	descriptor, found := twitchspec.Find(eventType)
	if !found {
		return domain.NodeDefinition{}, fmt.Errorf("unsupported Twitch EventSub event %q", eventType)
	}
	definition.Fields = append(definition.Fields, conditionFields(descriptor)...)
	outputs := append([]domain.NodePort{}, definition.Outputs...)
	outputs = append(outputs, eventPin("event", "Event", twitchspec.EventType()), stringPin("eventType", "Event type"), stringPin("subscriptionId", "Subscription ID"), stringPin("receivedAt", "Received at"))
	if descriptor.ChatMessage {
		outputs = append(outputs, stringPin("text", "Text"), stringPin("commandText", "Command text"), stringPin("broadcasterId", "Broadcaster user ID"), stringPin("authorId", "Author ID"), stringPin("messageId", "Message ID"), eventPin("author", "Author", authorType()), stringPin("channel", "Channel"))
	}
	definition.Outputs = outputs
	return definition, nil
}

// conditionFields exposes only the EventSub conditions that the user must
// supply. Conditions derived from the selected identity or public Client ID
// are deliberately not duplicated as editable text fields.
func conditionFields(descriptor domain.TwitchEventDescriptor) []domain.ConfigField {
	fields := make([]domain.ConfigField, 0, len(descriptor.Conditions))
	for _, condition := range descriptor.Conditions {
		switch condition.ID {
		case "broadcaster_user_id":
			fields = append(fields, domain.ConfigField{Name: "channel", Label: "Channel name", Kind: "string", Placeholder: "your_channel", Required: condition.Required})
		case "to_broadcaster_user_id":
			fields = append(fields, domain.ConfigField{Name: "targetChannel", Label: "Target channel name", Kind: "string", Placeholder: "your_channel", Required: condition.Required})
		case "from_broadcaster_user_id":
			fields = append(fields, domain.ConfigField{Name: "sourceChannel", Label: "Source channel name", Kind: "string", Placeholder: "your_channel", Required: condition.Required})
		case "user_id", "moderator_user_id", "client_id":
			// Derived from the selected identity or app configuration.
		default:
			fields = append(fields, domain.ConfigField{Name: condition.ID, Label: condition.Label, Kind: "string", Placeholder: "123456", Required: condition.Required})
		}
	}
	return fields
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, err
	}
	descriptor, found := twitchspec.Find(stringValue(invocation.Config, "eventType"))
	if !found {
		return nodes.ExecutionResult{}, fmt.Errorf("unsupported Twitch EventSub event")
	}
	eventValue, ok := invocation.Inputs["event"].(twitchspec.Event)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("twitch trigger requires a typed EventSub event")
	}
	if eventValue.Type != descriptor.Type {
		return nodes.ExecutionResult{}, fmt.Errorf("received Twitch event %q for trigger %q", eventValue.Type, descriptor.Type)
	}
	if eventValue.Version != descriptor.Version {
		return nodes.ExecutionResult{}, fmt.Errorf("received Twitch event version %q for trigger version %q", eventValue.Version, descriptor.Version)
	}
	outputs := map[string]any{"event": eventValue, "eventType": eventValue.Type, "subscriptionId": eventValue.SubscriptionID, "receivedAt": eventValue.ReceivedAt}
	if descriptor.ChatMessage {
		message, ok := chatMessage(eventValue.Payload)
		if !ok {
			return nodes.ExecutionResult{}, fmt.Errorf("twitch chat message payload is malformed")
		}
		if !matchesChatFilter(message, invocation.Config) {
			return nodes.ExecutionResult{Outputs: outputs}, nil
		}
		if prefix := stringValue(invocation.Config, "prefix"); prefix != "" && hasPrefix(message.Text, prefix, boolValue(invocation.Config, "caseSensitivePrefix")) {
			message.CommandText = strings.TrimSpace(message.Text[len(prefix):])
		} else {
			message.CommandText = message.Text
		}
		outputs["text"], outputs["commandText"], outputs["broadcasterId"], outputs["authorId"], outputs["messageId"] = message.Text, message.CommandText, message.BroadcasterID, message.AuthorID, message.MessageID
		outputs["author"] = twitchspec.TwitchAuthor{Login: message.AuthorLogin, Name: message.AuthorName, ID: message.AuthorID}
		outputs["channel"] = message.ChannelLogin
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"out"}}, nil
}

func chatMessage(payload map[string]any) (twitchspec.ChatMessage, bool) {
	value, ok := payload["chatMessage"].(twitchspec.ChatMessage)
	return value, ok
}
func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
func matchesChatFilter(message twitchspec.ChatMessage, values map[string]any) bool {
	prefix := stringValue(values, "prefix")
	if prefix != "" {
		if !hasPrefix(message.Text, prefix, boolValue(values, "caseSensitivePrefix")) {
			return false
		}
	}
	allowed := stringValue(values, "authorIDs")
	if allowed == "" {
		return true
	}
	for _, id := range strings.Split(allowed, ",") {
		if strings.TrimSpace(id) == message.AuthorID {
			return true
		}
	}
	return false
}
func boolValue(values map[string]any, key string) bool { value, _ := values[key].(bool); return value }
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
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &spec, Color: "#e879f9", MaxConnections: 1}
}
func eventPin(id, label string, spec domain.TypeSpec) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &spec, Color: "#60a5fa", MaxConnections: 1}
}
func authorType() domain.TypeSpec {
	return domain.TypeSpec{Kind: domain.TypeRecord, Name: "TwitchAuthor", Fields: []domain.TypeFieldSpec{{ID: "login", Name: "login", Type: typespec.String()}, {ID: "name", Name: "name", Type: typespec.String()}, {ID: "id", Name: "id", Type: typespec.String()}}}
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

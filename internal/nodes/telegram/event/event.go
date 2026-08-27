// Package event registers the dynamic Telegram event trigger module.
package event

import (
	"context"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/telespec"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Resolver: resolve, Executor: execute} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	return domain.NodeDefinition{
		Type: "telegram:event", Category: "Telegram", Label: "Telegram Event Trigger", Description: "Start a trusted pipeline from a Telegram bot update.", Icon: "send", Color: "#229ed9", Mode: domain.NodeEvent, TriggerKind: domain.TriggerTelegram, PortContractOwned: true,
		Inputs:  []domain.NodePort{},
		Outputs: []domain.NodePort{exec("out", "Start")},
		// The static contract mirrors the default event (message);
		// resolve() swaps the filter set and output pins for the selected
		// update kind so every event exposes exactly its own configuration.
		Fields: []domain.ConfigField{
			{Name: "eventType", Label: "Event", Kind: "select", Options: eventOptions(), Required: true},
			{Name: "identityId", Label: "Bot identity", Kind: "telegram-identity", Required: true},
			{Name: "prefix", Label: "Message prefix", Kind: "string"},
			{Name: "fromUsernames", Label: "From usernames", Kind: "string", Placeholder: "alice, bob"},
			{Name: "caseSensitivePrefix", Label: "Case-sensitive prefix", Kind: "boolean"},
		},
		DefaultConfig: map[string]any{"eventType": "message", "identityId": "", "prefix": "", "fromUsernames": "", "caseSensitivePrefix": false}, Source: "builtin",
	}
}

func eventOptions() []domain.Option {
	options := make([]domain.Option, 0)
	for _, item := range telespec.Catalog() {
		options = append(options, domain.Option{Value: item.Type, Label: item.Label})
	}
	return options
}

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := definition()
	eventType, _ := config(node)["eventType"].(string)
	descriptor, found := telespec.Find(eventType)
	if !found {
		return domain.NodeDefinition{}, fmt.Errorf("unsupported Telegram event %q", eventType)
	}
	definition.Fields = eventFields(descriptor)
	definition.Outputs = eventOutputs(descriptor)
	return definition, nil
}

// eventFields derives the inspector contract for one Bot API update: the
// shared event and identity selectors, the descriptor's delivery conditions,
// and only the filters meaningful for that update kind. The chat allowlist is
// applied by the polling service before an update reaches this node.
func eventFields(descriptor domain.TelegramEventDescriptor) []domain.ConfigField {
	fields := []domain.ConfigField{
		{Name: "eventType", Label: "Event", Kind: "select", Options: eventOptions(), Required: true},
		{Name: "identityId", Label: "Bot identity", Kind: "telegram-identity", Required: true},
	}
	fields = append(fields, conditionFields(descriptor)...)
	switch {
	case descriptor.ChatMessage:
		fields = append(fields,
			domain.ConfigField{Name: "prefix", Label: "Message prefix", Kind: "string"},
			domain.ConfigField{Name: "fromUsernames", Label: "From usernames", Kind: "string", Placeholder: "alice, bob"},
			domain.ConfigField{Name: "caseSensitivePrefix", Label: "Case-sensitive prefix", Kind: "boolean"},
		)
	case descriptor.Callback:
		fields = append(fields, domain.ConfigField{Name: "callbackDataPrefix", Label: "Callback data prefix", Kind: "string", Placeholder: "approve:"})
	}
	return fields
}

// eventOutputs exposes the typed convenience pins of the selected update in
// addition to the shared envelope pins.
func eventOutputs(descriptor domain.TelegramEventDescriptor) []domain.NodePort {
	outputs := []domain.NodePort{exec("out", "Start"),
		eventPin("event", "Event", telespec.EventType()),
		stringPin("eventType", "Event type"),
		numberPin("updateId", "Update ID"),
		stringPin("receivedAt", "Received at"),
	}
	if descriptor.ChatMessage {
		outputs = append(outputs,
			stringPin("text", "Text"),
			stringPin("commandText", "Command text"),
			numberPin("messageId", "Message ID"),
			numberPin("chatId", "Chat ID"),
			stringPin("chatType", "Chat type"),
			stringPin("chatTitle", "Chat title"),
			eventPin("from", "From", telespec.FromType()),
			stringPin("fromId", "From user ID"),
		)
	}
	if descriptor.Callback {
		outputs = append(outputs,
			stringPin("callbackData", "Callback data"),
			stringPin("callbackQueryId", "Callback query ID"),
			stringPin("fromId", "From user ID"),
			numberPin("messageId", "Message ID"),
			numberPin("chatId", "Chat ID"),
		)
	}
	switch descriptor.Type {
	case "my_chat_member":
		outputs = append(outputs,
			numberPin("chatId", "Chat ID"),
			stringPin("chatTitle", "Chat title"),
			stringPin("userId", "User ID"),
			stringPin("oldStatus", "Old status"),
			stringPin("newStatus", "New status"),
		)
	case "chat_join_request":
		outputs = append(outputs,
			numberPin("chatId", "Chat ID"),
			stringPin("chatTitle", "Chat title"),
			stringPin("userId", "User ID"),
			stringPin("username", "Username"),
		)
	case "message_reaction":
		outputs = append(outputs,
			numberPin("chatId", "Chat ID"),
			numberPin("messageId", "Message ID"),
			stringPin("userId", "User ID"),
			stringPin("username", "Username"),
		)
	}
	return outputs
}

// conditionFields exposes the descriptor's client-side filters. The chat
// allowlist is applied by the polling service before an event reaches this
// node; it is exposed here so the inspector documents it beside the node's
// own fine-grained filters.
func conditionFields(descriptor domain.TelegramEventDescriptor) []domain.ConfigField {
	fields := make([]domain.ConfigField, 0, len(descriptor.Conditions))
	for _, condition := range descriptor.Conditions {
		fields = append(fields, domain.ConfigField{Name: condition.ID, Label: condition.Label, Kind: "string", Placeholder: "123456, @mychannel", Required: condition.Required})
	}
	return fields
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, err
	}
	descriptor, found := telespec.Find(stringValue(invocation.Config, "eventType"))
	if !found {
		return nodes.ExecutionResult{}, fmt.Errorf("unsupported Telegram event")
	}
	eventValue, ok := invocation.Inputs["event"].(telespec.TelegramEvent)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("telegram trigger requires a typed bot update")
	}
	if eventValue.Type != descriptor.Type {
		return nodes.ExecutionResult{}, fmt.Errorf("received Telegram event %q for trigger %q", eventValue.Type, descriptor.Type)
	}
	outputs := map[string]any{"event": eventValue, "eventType": eventValue.Type, "updateId": eventValue.UpdateID, "receivedAt": eventValue.ReceivedAt}
	if descriptor.ChatMessage {
		message, ok := eventValue.Payload["message"].(telespec.Message)
		if !ok {
			return nodes.ExecutionResult{}, fmt.Errorf("telegram message payload is malformed")
		}
		if !matchesMessageFilter(message, invocation.Config) {
			return nodes.ExecutionResult{Outputs: outputs}, nil
		}
		if prefix := stringValue(invocation.Config, "prefix"); prefix != "" && hasPrefix(message.Text, prefix, boolValue(invocation.Config, "caseSensitivePrefix")) {
			message.CommandText = strings.TrimSpace(message.Text[len(prefix):])
		} else {
			message.CommandText = message.Text
		}
		outputs["text"] = message.Text
		outputs["commandText"] = message.CommandText
		outputs["messageId"] = message.MessageID
		outputs["chatId"] = message.ChatID
		outputs["chatType"] = message.ChatType
		outputs["chatTitle"] = message.ChatTitle
		outputs["from"] = telespec.TelegramFrom{ID: fmt.Sprintf("%d", message.FromID), Username: message.FromUsername, Name: message.FromName}
		outputs["fromId"] = fmt.Sprintf("%d", message.FromID)
	}
	if descriptor.Callback {
		query, ok := eventValue.Payload["callbackQuery"].(telespec.CallbackQuery)
		if !ok {
			return nodes.ExecutionResult{}, fmt.Errorf("telegram callback payload is malformed")
		}
		if prefix := stringValue(invocation.Config, "callbackDataPrefix"); prefix != "" && !strings.HasPrefix(query.Data, prefix) {
			return nodes.ExecutionResult{Outputs: outputs}, nil
		}
		outputs["callbackData"] = query.Data
		outputs["callbackQueryId"] = query.ID
		outputs["fromId"] = fmt.Sprintf("%d", query.FromID)
		outputs["messageId"] = query.MessageID
		outputs["chatId"] = query.ChatID
	}
	switch descriptor.Type {
	case "my_chat_member":
		member, ok := eventValue.Payload["chatMember"].(telespec.ChatMemberUpdated)
		if !ok {
			return nodes.ExecutionResult{}, fmt.Errorf("telegram chat member payload is malformed")
		}
		outputs["chatId"] = member.ChatID
		outputs["chatTitle"] = member.ChatTitle
		outputs["userId"] = fmt.Sprintf("%d", member.UserID)
		outputs["oldStatus"] = member.OldStatus
		outputs["newStatus"] = member.NewStatus
	case "chat_join_request":
		request, ok := eventValue.Payload["joinRequest"].(map[string]any)
		if !ok {
			return nodes.ExecutionResult{}, fmt.Errorf("telegram join request payload is malformed")
		}
		outputs["chatId"] = request["chatId"]
		outputs["chatTitle"] = request["chatTitle"]
		outputs["userId"] = request["userId"]
		outputs["username"] = request["username"]
	case "message_reaction":
		reaction, ok := eventValue.Payload["reaction"].(map[string]any)
		if !ok {
			return nodes.ExecutionResult{}, fmt.Errorf("telegram reaction payload is malformed")
		}
		outputs["chatId"] = reaction["chatId"]
		outputs["messageId"] = reaction["messageId"]
		outputs["userId"] = reaction["userId"]
		outputs["username"] = reaction["username"]
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"out"}}, nil
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
func boolValue(values map[string]any, key string) bool { value, _ := values[key].(bool); return value }
func matchesMessageFilter(message telespec.Message, values map[string]any) bool {
	prefix := stringValue(values, "prefix")
	if prefix != "" {
		if !hasPrefix(message.Text, prefix, boolValue(values, "caseSensitivePrefix")) {
			return false
		}
	}
	allowed := stringValue(values, "fromUsernames")
	if allowed == "" {
		return true
	}
	for _, name := range strings.Split(allowed, ",") {
		if strings.TrimPrefix(strings.TrimSpace(strings.ToLower(name)), "@") == strings.ToLower(message.FromUsername) {
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
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &spec, Color: "#229ed9", MaxConnections: 1}
}
func numberPin(id, label string) domain.NodePort {
	spec := typespec.Float()
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataNumber, Type: &spec, Color: "#86efac", MaxConnections: 1}
}
func eventPin(id, label string, spec domain.TypeSpec) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &spec, Color: "#60a5fa", MaxConnections: 1}
}
func config(node domain.FlowNode) map[string]any {
	if value, ok := node.Data["config"].(map[string]any); ok {
		return value
	}
	return node.Data
}

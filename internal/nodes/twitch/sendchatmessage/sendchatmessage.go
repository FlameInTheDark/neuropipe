// Package sendchatmessage registers Twitch's typed chat action node.
package sendchatmessage

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Executor: execute} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	return domain.NodeDefinition{Type: "twitch:send_chat_message", Category: "Twitch", Label: "Send Twitch Chat Message", Description: "Send one explicit chat message through the selected bot identity.", Icon: "send", Color: "#a970ff", Mode: domain.NodeImpure, PortContractOwned: true, Capabilities: []domain.Capability{domain.CapabilityNetwork}, Inputs: []domain.NodePort{exec("in", "Send", domain.PinInput), textPin("message", "Message", domain.PinInput, true), textPin("channel", "Channel name", domain.PinInput, true), textPin("replyParentMessageId", "Reply to message ID", domain.PinInput, false), textPin("identityId", "Identity", domain.PinInput, false)}, Outputs: []domain.NodePort{exec("sent", "Sent", domain.PinOutput), exec("rejected", "Rejected", domain.PinOutput), textPin("messageId", "Message ID", domain.PinOutput, false), textPin("reason", "Reason", domain.PinOutput, false)}, Fields: []domain.ConfigField{{Name: "identityId", Label: "Bot identity", Kind: "twitch-identity"}, {Name: "channel", Label: "Channel name", Kind: "string", Placeholder: "your_channel", Required: true}, {Name: "message", Label: "Message", Kind: "string", Required: true}, {Name: "replyParentMessageId", Label: "Reply to message ID", Kind: "string"}}, DefaultConfig: map[string]any{"identityId": "", "channel": "", "message": "", "replyParentMessageId": ""}, Source: "builtin"}
}
func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	provider, ok := runtime.(nodes.TwitchChatSenderProvider)
	if !ok || provider.TwitchChatSender() == nil {
		return nodes.ExecutionResult{}, fmt.Errorf("twitch chat delivery is unavailable")
	}
	request := domain.TwitchChatMessageRequest{IdentityID: value(invocation.Inputs, "identityId"), Channel: value(invocation.Inputs, "channel"), Message: value(invocation.Inputs, "message"), ReplyParentID: value(invocation.Inputs, "replyParentMessageId")}
	if request.Channel == "" || request.Message == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("message and channel name are required")
	}
	if utf8.RuneCountInString(request.Message) > 500 {
		return nodes.ExecutionResult{Outputs: map[string]any{"messageId": "", "reason": "message exceeds Twitch's 500-character limit"}, Ports: []string{"rejected"}}, nil
	}
	result, err := provider.TwitchChatSender().SendTwitchChatMessage(ctx, request)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	outputs := map[string]any{"messageId": result.MessageID, "reason": result.Reason}
	if result.Sent {
		return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"sent"}}, nil
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"rejected"}}, nil
}
func value(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
func exec(id, label string, direction domain.PinDirection) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: direction, Color: "#fafafa", MaxConnections: 1}
}
func textPin(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	spec := typespec.String()
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataText, Type: &spec, Color: "#e879f9", Required: required, MaxConnections: 1}
}

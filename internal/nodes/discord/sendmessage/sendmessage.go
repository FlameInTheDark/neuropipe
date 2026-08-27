// Package sendmessage registers the Send Discord Message action node.
package sendmessage

import (
	"context"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/discord/dc"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Executor: execute} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	return dc.Definition("action:discord_send_message", "Send Discord Message", "Send one explicit message through the selected bot identity.", "send",
		[]domain.NodePort{
			dc.Exec("in", "Send", domain.PinInput),
			dc.Text("message", "Message", domain.PinInput, true),
			dc.Text("channel", "Channel ID", domain.PinInput, true),
			dc.Text("replyToMessageId", "Reply to message ID", domain.PinInput, false),
			dc.Text("identityId", "Identity", domain.PinInput, false),
		},
		[]domain.NodePort{
			dc.Exec("sent", "Sent", domain.PinOutput),
			dc.Exec("rejected", "Rejected", domain.PinOutput),
			dc.Text("messageId", "Message ID", domain.PinOutput, false),
			dc.Text("reason", "Reason", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "channel", Label: "Channel ID", Kind: "string", Placeholder: "123456789012345678", Required: true},
			{Name: "message", Label: "Message", Kind: "textarea", Required: true},
			{Name: "replyToMessageId", Label: "Reply to message ID", Kind: "string"},
		},
		map[string]any{"channel": "", "message": "", "replyToMessageId": ""},
	)
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	sender, err := dc.Sender(runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	channel, err := dc.RequiredString(invocation, "channel", "channel ID")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	message := dc.String(invocation, "message")
	if message == "" {
		return nodes.ExecutionResult{}, errEmptyMessage
	}
	if utf8.RuneCountInString(message) > 2000 {
		return nodes.ExecutionResult{Outputs: map[string]any{"messageId": "", "reason": "message exceeds Discord's 2,000-character limit"}, Ports: []string{"rejected"}}, nil
	}
	result, err := sender.SendDiscordMessage(ctx, domain.DiscordMessageRequest{
		IdentityID: dc.String(invocation, "identityId"), ChannelID: channel,
		Message: message, ReplyToID: dc.String(invocation, "replyToMessageId"),
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	outputs := map[string]any{"messageId": result.MessageID, "reason": result.Reason}
	if result.Sent {
		return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"sent"}}, nil
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"rejected"}}, nil
}

type validationError string

func (e validationError) Error() string { return string(e) }

const errEmptyMessage validationError = "message is required"

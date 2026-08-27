// Package editmessage registers the Edit Discord Message action node.
package editmessage

import (
	"context"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/discord/dc"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Executor: execute} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	return dc.Definition("action:discord_edit_message", "Edit Discord Message", "Edit the text of a message the bot previously sent.", "pencil",
		[]domain.NodePort{
			dc.Exec("in", "Edit", domain.PinInput),
			dc.Text("channel", "Channel ID", domain.PinInput, true),
			dc.Text("messageId", "Message ID", domain.PinInput, true),
			dc.Text("message", "New message", domain.PinInput, true),
			dc.Text("identityId", "Identity", domain.PinInput, false),
		},
		[]domain.NodePort{
			dc.Exec("done", "Done", domain.PinOutput),
			dc.Exec("rejected", "Rejected", domain.PinOutput),
			dc.Text("reason", "Reason", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "channel", Label: "Channel ID", Kind: "string", Placeholder: "123456789012345678", Required: true},
			{Name: "messageId", Label: "Message ID", Kind: "string", Required: true},
			{Name: "message", Label: "New message", Kind: "textarea", Required: true},
		},
		map[string]any{"channel": "", "messageId": "", "message": ""},
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
	messageID, err := dc.RequiredString(invocation, "messageId", "message ID")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	message := dc.String(invocation, "message")
	if message == "" {
		return nodes.ExecutionResult{}, errEmptyMessage
	}
	result, err := sender.EditDiscordMessage(ctx, domain.DiscordEditRequest{
		IdentityID: dc.String(invocation, "identityId"), ChannelID: channel, MessageID: messageID, Message: message,
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	outputs := map[string]any{"reason": result.Reason}
	if result.Done {
		return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"done"}}, nil
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"rejected"}}, nil
}

type validationError string

func (e validationError) Error() string { return string(e) }

const errEmptyMessage validationError = "message is required"

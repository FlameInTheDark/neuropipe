// Package deletemessage registers the Delete Discord Message action node.
package deletemessage

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
	return dc.Definition("action:discord_delete_message", "Delete Discord Message", "Delete one message the bot can moderate.", "trash",
		[]domain.NodePort{
			dc.Exec("in", "Delete", domain.PinInput),
			dc.Text("channel", "Channel ID", domain.PinInput, true),
			dc.Text("messageId", "Message ID", domain.PinInput, true),
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
		},
		map[string]any{"channel": "", "messageId": ""},
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
	result, err := sender.DeleteDiscordMessage(ctx, domain.DiscordDeleteRequest{
		IdentityID: dc.String(invocation, "identityId"), ChannelID: channel, MessageID: messageID,
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

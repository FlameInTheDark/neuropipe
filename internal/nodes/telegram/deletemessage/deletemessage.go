// Package deletemessage registers the Delete Telegram Message action node.
package deletemessage

import (
	"context"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/tg"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Executor: execute} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	return tg.Definition("action:telegram_delete_message", "Delete Telegram Message", "Delete a previously sent message.", "trash",
		[]domain.NodePort{
			tg.Exec("in", "Delete", domain.PinInput),
			tg.Text("chatId", "Chat ID", domain.PinInput, true),
			tg.Text("messageId", "Message ID", domain.PinInput, true),
			tg.Text("identityId", "Identity", domain.PinInput, false),
		},
		[]domain.NodePort{
			tg.Exec("done", "Done", domain.PinOutput),
			tg.Exec("rejected", "Rejected", domain.PinOutput),
			tg.Text("reason", "Reason", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "chatId", Label: "Chat ID", Kind: "string", Placeholder: "123456 or @mychannel", Required: true},
			{Name: "messageId", Label: "Message ID", Kind: "string", Required: true},
		},
		map[string]any{"chatId": "", "messageId": ""},
	)
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	sender, err := tg.Sender(runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	chatID, err := tg.RequiredString(invocation, "chatId", "chat ID")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	messageID, err := tg.RequiredString(invocation, "messageId", "message ID")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	result, err := sender.DeleteTelegramMessage(ctx, domain.TelegramDeleteRequest{
		IdentityID: tg.String(invocation, "identityId"), ChatID: chatID, MessageID: messageID,
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

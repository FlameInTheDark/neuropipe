// Package editmessage registers the Edit Telegram Message action node.
package editmessage

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
	return tg.Definition("action:telegram_edit_message", "Edit Telegram Message", "Edit the text of a previously sent message.", "pencil",
		[]domain.NodePort{
			tg.Exec("in", "Edit", domain.PinInput),
			tg.Text("chatId", "Chat ID", domain.PinInput, true),
			tg.Text("messageId", "Message ID", domain.PinInput, true),
			tg.Text("message", "New message", domain.PinInput, true),
			tg.Text("parseMode", "Parse mode", domain.PinInput, false),
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
			{Name: "message", Label: "New message", Kind: "textarea", Required: true},
			{Name: "parseMode", Label: "Parse mode", Kind: "select", Options: []domain.Option{{Value: "", Label: "Plain text"}, {Value: "HTML", Label: "HTML"}, {Value: "MarkdownV2", Label: "MarkdownV2"}}},
		},
		map[string]any{"chatId": "", "messageId": "", "message": "", "parseMode": ""},
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
	message := tg.String(invocation, "message")
	if message == "" {
		return nodes.ExecutionResult{}, errEmptyMessage
	}
	result, err := sender.EditTelegramMessage(ctx, domain.TelegramEditRequest{
		IdentityID: tg.String(invocation, "identityId"), ChatID: chatID,
		MessageID: messageID, Message: message, ParseMode: tg.String(invocation, "parseMode"),
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return outcome(result.Done, result.Reason), nil
}

func outcome(done bool, reason string) nodes.ExecutionResult {
	outputs := map[string]any{"reason": reason}
	if done {
		return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"done"}}
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"rejected"}}
}

type validationError string

func (e validationError) Error() string { return string(e) }

const errEmptyMessage validationError = "message is required"

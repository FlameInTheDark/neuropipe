// Package sendmessage registers the Send Telegram Message action node.
package sendmessage

import (
	"context"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/tg"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Executor: execute} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

var parseModeOptions = []domain.Option{
	{Value: "", Label: "Plain text"},
	{Value: "HTML", Label: "HTML"},
	{Value: "MarkdownV2", Label: "MarkdownV2"},
}

func definition() domain.NodeDefinition {
	return tg.Definition("action:telegram_send_message", "Send Telegram Message", "Send one text message through the selected bot identity.", "send",
		[]domain.NodePort{
			tg.Exec("in", "Send", domain.PinInput),
			tg.Text("message", "Message", domain.PinInput, true),
			tg.Text("chatId", "Chat ID", domain.PinInput, true),
			tg.Text("parseMode", "Parse mode", domain.PinInput, false),
			tg.Text("replyToMessageId", "Reply to message ID", domain.PinInput, false),
			tg.Bool("disableNotification", "Silent", domain.PinInput, false),
			tg.Text("identityId", "Identity", domain.PinInput, false),
		},
		[]domain.NodePort{
			tg.Exec("sent", "Sent", domain.PinOutput),
			tg.Exec("rejected", "Rejected", domain.PinOutput),
			tg.Text("messageId", "Message ID", domain.PinOutput, false),
			tg.Text("reason", "Reason", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "chatId", Label: "Chat ID", Kind: "string", Placeholder: "123456 or @mychannel", Required: true},
			{Name: "message", Label: "Message", Kind: "textarea", Required: true},
			{Name: "parseMode", Label: "Parse mode", Kind: "select", Options: parseModeOptions},
			{Name: "replyToMessageId", Label: "Reply to message ID", Kind: "string"},
			{Name: "disableNotification", Label: "Silent (no notification)", Kind: "boolean"},
		},
		map[string]any{"chatId": "", "message": "", "parseMode": "", "replyToMessageId": "", "disableNotification": false},
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
	message := tg.String(invocation, "message")
	if message == "" {
		return nodes.ExecutionResult{}, errEmptyMessage
	}
	if utf8.RuneCountInString(message) > 4096 {
		return nodes.ExecutionResult{Outputs: map[string]any{"messageId": "", "reason": "message exceeds Telegram's 4,096-character limit"}, Ports: []string{"rejected"}}, nil
	}
	parseMode := tg.String(invocation, "parseMode")
	switch parseMode {
	case "", "HTML", "MarkdownV2":
	default:
		return nodes.ExecutionResult{}, errParseMode
	}
	result, err := sender.SendTelegramMessage(ctx, domain.TelegramMessageRequest{
		IdentityID: tg.String(invocation, "identityId"), ChatID: chatID, Message: message,
		ParseMode: parseMode, ReplyToMessageID: tg.String(invocation, "replyToMessageId"),
		DisableNotification: tg.BoolValue(invocation, "disableNotification"),
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

const (
	errEmptyMessage validationError = "message is required"
	errParseMode    validationError = "parse mode must be empty, HTML, or MarkdownV2"
)

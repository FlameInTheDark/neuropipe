// Package sendphoto registers the Send Telegram Photo action node.
package sendphoto

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
	return tg.Definition("action:telegram_send_photo", "Send Telegram Photo", "Send one photo by URL through the selected bot identity. Telegram fetches the URL server-side.", "camera",
		[]domain.NodePort{
			tg.Exec("in", "Send", domain.PinInput),
			tg.Text("photoUrl", "Photo URL", domain.PinInput, true),
			tg.Text("chatId", "Chat ID", domain.PinInput, true),
			tg.Text("caption", "Caption", domain.PinInput, false),
			tg.Text("parseMode", "Parse mode", domain.PinInput, false),
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
			{Name: "photoUrl", Label: "Photo URL", Kind: "string", Placeholder: "https://example.com/photo.jpg", Required: true},
			{Name: "caption", Label: "Caption", Kind: "textarea"},
			{Name: "parseMode", Label: "Parse mode", Kind: "select", Options: []domain.Option{{Value: "", Label: "Plain text"}, {Value: "HTML", Label: "HTML"}, {Value: "MarkdownV2", Label: "MarkdownV2"}}},
		},
		map[string]any{"chatId": "", "photoUrl": "", "caption": "", "parseMode": ""},
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
	photoURL := tg.String(invocation, "photoUrl")
	if photoURL == "" {
		return nodes.ExecutionResult{}, errPhotoRequired
	}
	result, err := sender.SendTelegramPhoto(ctx, domain.TelegramPhotoRequest{
		IdentityID: tg.String(invocation, "identityId"), ChatID: chatID,
		PhotoURL: photoURL, Caption: tg.String(invocation, "caption"), ParseMode: tg.String(invocation, "parseMode"),
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

const errPhotoRequired validationError = "photo URL is required"

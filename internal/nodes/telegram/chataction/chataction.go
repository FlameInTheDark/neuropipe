// Package chataction registers the Send Telegram Chat Action action node.
package chataction

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

var actionOptions = []domain.Option{
	{Value: "typing", Label: "Typing"},
	{Value: "upload_photo", Label: "Upload photo"},
	{Value: "record_video", Label: "Record video"},
	{Value: "upload_video", Label: "Upload video"},
	{Value: "record_voice", Label: "Record voice"},
	{Value: "upload_voice", Label: "Upload voice"},
	{Value: "upload_document", Label: "Upload document"},
	{Value: "choose_sticker", Label: "Choose sticker"},
	{Value: "find_location", Label: "Find location"},
	{Value: "record_video_note", Label: "Record video note"},
	{Value: "upload_video_note", Label: "Upload video note"},
}

func definition() domain.NodeDefinition {
	return tg.Definition("action:telegram_chat_action", "Send Telegram Chat Action", "Show a typing or uploading indicator in one chat.", "clock",
		[]domain.NodePort{
			tg.Exec("in", "Send", domain.PinInput),
			tg.Text("chatId", "Chat ID", domain.PinInput, true),
			tg.Text("action", "Action", domain.PinInput, true),
			tg.Text("identityId", "Identity", domain.PinInput, false),
		},
		[]domain.NodePort{
			tg.Exec("done", "Done", domain.PinOutput),
			tg.Exec("rejected", "Rejected", domain.PinOutput),
			tg.Text("reason", "Reason", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "chatId", Label: "Chat ID", Kind: "string", Placeholder: "123456 or @mychannel", Required: true},
			{Name: "action", Label: "Action", Kind: "select", Options: actionOptions, Required: true},
		},
		map[string]any{"chatId": "", "action": "typing"},
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
	action := tg.String(invocation, "action")
	if action == "" {
		return nodes.ExecutionResult{}, errActionRequired
	}
	result, err := sender.SendTelegramChatAction(ctx, domain.TelegramChatActionRequest{
		IdentityID: tg.String(invocation, "identityId"), ChatID: chatID, Action: action,
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

const errActionRequired validationError = "action is required"

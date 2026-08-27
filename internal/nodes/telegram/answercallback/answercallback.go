// Package answercallback registers the Answer Telegram Callback action node.
package answercallback

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
	return tg.Definition("action:telegram_answer_callback", "Answer Telegram Callback", "Answer an inline-keyboard callback query, usually paired with the Callback Query trigger.", "pointer",
		[]domain.NodePort{
			tg.Exec("in", "Answer", domain.PinInput),
			tg.Text("callbackQueryId", "Callback query ID", domain.PinInput, true),
			tg.Text("text", "Answer text", domain.PinInput, false),
			tg.Bool("showAlert", "Show alert", domain.PinInput, false),
			tg.Text("identityId", "Identity", domain.PinInput, false),
		},
		[]domain.NodePort{
			tg.Exec("done", "Done", domain.PinOutput),
			tg.Exec("rejected", "Rejected", domain.PinOutput),
			tg.Text("reason", "Reason", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "callbackQueryId", Label: "Callback query ID", Kind: "string", Required: true},
			{Name: "text", Label: "Answer text", Kind: "string"},
			{Name: "showAlert", Label: "Show as alert", Kind: "boolean"},
		},
		map[string]any{"callbackQueryId": "", "text": "", "showAlert": false},
	)
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	sender, err := tg.Sender(runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	queryID, err := tg.RequiredString(invocation, "callbackQueryId", "callback query ID")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	result, err := sender.AnswerTelegramCallbackQuery(ctx, domain.TelegramCallbackAnswerRequest{
		IdentityID: tg.String(invocation, "identityId"), CallbackQueryID: queryID,
		Text: tg.String(invocation, "text"), ShowAlert: tg.BoolValue(invocation, "showAlert"),
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

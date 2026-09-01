package answercallback

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ sender nodes.TelegramSender }

func (r runtimeStub) TelegramSender() nodes.TelegramSender { return r.sender }

type senderStub struct {
	request domain.TelegramCallbackAnswerRequest
	result  domain.TelegramActionResult
	err     error
}

func (s *senderStub) AnswerTelegramCallbackQuery(_ context.Context, request domain.TelegramCallbackAnswerRequest) (domain.TelegramActionResult, error) {
	s.request = request
	return s.result, s.err
}
func (s *senderStub) SendTelegramMessage(context.Context, domain.TelegramMessageRequest) (domain.TelegramMessageResult, error) {
	panic("unused")
}
func (s *senderStub) SendTelegramPhoto(context.Context, domain.TelegramPhotoRequest) (domain.TelegramMessageResult, error) {
	panic("unused")
}
func (s *senderStub) SendTelegramDocument(context.Context, domain.TelegramDocumentRequest) (domain.TelegramMessageResult, error) {
	panic("unused")
}
func (s *senderStub) EditTelegramMessage(context.Context, domain.TelegramEditRequest) (domain.TelegramActionResult, error) {
	panic("unused")
}
func (s *senderStub) DeleteTelegramMessage(context.Context, domain.TelegramDeleteRequest) (domain.TelegramActionResult, error) {
	panic("unused")
}
func (s *senderStub) SendTelegramChatAction(context.Context, domain.TelegramChatActionRequest) (domain.TelegramActionResult, error) {
	panic("unused")
}
func (s *senderStub) PinTelegramMessage(context.Context, domain.TelegramPinRequest) (domain.TelegramActionResult, error) {
	panic("unused")
}

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("register: %v", err)
	}
	module, ok := registry.Get("action:telegram_answer_callback")
	if !ok {
		t.Fatal("action:telegram_answer_callback was not registered")
	}
	return module
}

func invocation(definition domain.NodeDefinition, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:telegram_answer_callback", Data: map[string]any{"config": map[string]any{}}},
		Definition:      definition,
		SchemaVersion:   3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func assertPinIDs(t *testing.T, ports []domain.NodePort, want []string) {
	t.Helper()
	got := make([]string, 0, len(ports))
	for _, port := range ports {
		got = append(got, port.ID)
	}
	if len(got) != len(want) {
		t.Fatalf("pin ids = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("pin ids = %v, want %v", got, want)
		}
	}
}

func TestRegistrationMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "action:telegram_answer_callback" || definition.Mode != domain.NodeImpure || definition.Category != "Telegram" {
		t.Fatalf("definition = %#v", definition)
	}
	assertPinIDs(t, definition.Inputs, []string{"in", "callbackQueryId", "text", "showAlert", "identityId"})
	assertPinIDs(t, definition.Outputs, []string{"done", "rejected", "reason"})
}

func TestAnswersCallbackAndFollowsDonePort(t *testing.T) {
	sender := &senderStub{result: domain.TelegramActionResult{Done: true}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"callbackQueryId": "agg-1", "text": "Picked!", "showAlert": true, "identityId": "bot-1",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "done" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	if sender.request.CallbackQueryID != "agg-1" || sender.request.Text != "Picked!" || !sender.request.ShowAlert || sender.request.IdentityID != "bot-1" {
		t.Fatalf("request = %#v", sender.request)
	}
}

// text and showAlert are optional: an empty answer with no alert is still a
// valid callback acknowledgment.
func TestOptionalPinsDefaultToZeroValues(t *testing.T) {
	sender := &senderStub{result: domain.TelegramActionResult{Done: true}}
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"callbackQueryId": "agg-2",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if sender.request.Text != "" || sender.request.ShowAlert || sender.request.IdentityID != "" {
		t.Fatalf("request = %#v", sender.request)
	}
}

func TestSoftRejectionFollowsRejectedPort(t *testing.T) {
	sender := &senderStub{result: domain.TelegramActionResult{Reason: "query is too old"}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"callbackQueryId": "agg-1",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "rejected" || result.Outputs["reason"] != "query is too old" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSenderErrorPropagates(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"callbackQueryId": "agg-1",
	}), runtimeStub{sender: &senderStub{err: errors.New("bot api down")}})
	if err == nil || !strings.Contains(err.Error(), "bot api down") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidationErrors(t *testing.T) {
	module := registeredModule(t)
	definition := module.Definition()
	cases := []struct {
		name    string
		inputs  map[string]any
		runtime nodes.Runtime
		want    string
	}{
		{"missing callback query id", map[string]any{"text": "hi"}, runtimeStub{sender: &senderStub{}}, "callback query ID is required"},
		{"nil runtime", map[string]any{"callbackQueryId": "agg-1"}, nil, "telegram delivery is unavailable"},
		{"runtime without provider", map[string]any{"callbackQueryId": "agg-1"}, struct{}{}, "telegram delivery is unavailable"},
		{"provider with nil sender", map[string]any{"callbackQueryId": "agg-1"}, runtimeStub{}, "telegram delivery is unavailable"},
	}
	for _, testCase := range cases {
		_, err := module.Execute(context.Background(), invocation(definition, testCase.inputs), testCase.runtime)
		if err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("%s: err = %v, want %q", testCase.name, err, testCase.want)
		}
	}
}

package sendmessage

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/tg"
)

type runtimeStub struct{ sender nodes.TelegramSender }

func (r runtimeStub) TelegramSender() nodes.TelegramSender { return r.sender }

type senderStub struct {
	request domain.TelegramMessageRequest
	result  domain.TelegramMessageResult
}

func (s *senderStub) SendTelegramMessage(_ context.Context, request domain.TelegramMessageRequest) (domain.TelegramMessageResult, error) {
	s.request = request
	return s.result, nil
}
func (s *senderStub) SendTelegramPhoto(context.Context, domain.TelegramPhotoRequest) (domain.TelegramMessageResult, error) {
	panic("unused")
}
func (s *senderStub) EditTelegramMessage(context.Context, domain.TelegramEditRequest) (domain.TelegramActionResult, error) {
	panic("unused")
}
func (s *senderStub) DeleteTelegramMessage(context.Context, domain.TelegramDeleteRequest) (domain.TelegramActionResult, error) {
	panic("unused")
}
func (s *senderStub) AnswerTelegramCallbackQuery(context.Context, domain.TelegramCallbackAnswerRequest) (domain.TelegramActionResult, error) {
	panic("unused")
}
func (s *senderStub) SendTelegramChatAction(context.Context, domain.TelegramChatActionRequest) (domain.TelegramActionResult, error) {
	panic("unused")
}
func (s *senderStub) PinTelegramMessage(context.Context, domain.TelegramPinRequest) (domain.TelegramActionResult, error) {
	panic("unused")
}

func invocation(config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:telegram_send_message", Data: map[string]any{"config": config}},
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestSendsMessageAndFollowsSentPort(t *testing.T) {
	sender := &senderStub{result: domain.TelegramMessageResult{MessageID: "321", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{},
		map[string]any{"chatId": "55", "message": "hello", "parseMode": "HTML", "identityId": "bot-1"},
	), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "sent" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	if result.Outputs["messageId"] != "321" {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if sender.request.ChatID != "55" || sender.request.Message != "hello" || sender.request.ParseMode != "HTML" || sender.request.IdentityID != "bot-1" {
		t.Fatalf("request = %#v", sender.request)
	}
}

func TestSoftRejectionFollowsRejectedPort(t *testing.T) {
	sender := &senderStub{result: domain.TelegramMessageResult{Reason: "Bad Request: chat not found"}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{}, map[string]any{"chatId": "55", "message": "hello"},
	), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "rejected" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	if result.Outputs["reason"] != "Bad Request: chat not found" {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestLengthCapPreValidated(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{}, map[string]any{"chatId": "55", "message": strings.Repeat("a", 4097)},
	), runtimeStub{sender: &senderStub{result: domain.TelegramMessageResult{Sent: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "rejected" || !strings.Contains(result.Outputs["reason"].(string), "4,096") {
		t.Fatalf("cap ports/outputs = %#v", result)
	}
}

func TestValidationErrors(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{"message": "x"}), runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("missing chat id accepted")
	}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{"chatId": "1"}), runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("missing message accepted")
	}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{"chatId": "1", "message": "x", "parseMode": "weird"}), runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("bad parse mode accepted")
	}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{"chatId": "1", "message": "x"}), nil); err == nil {
		t.Fatal("missing runtime accepted")
	}
}

func TestWiredPinsFeedTheRequest(t *testing.T) {
	sender := &senderStub{result: domain.TelegramMessageResult{Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	_, err := module.Execute(context.Background(), invocation(
		map[string]any{},
		map[string]any{"chatId": " 99 ", "message": "wired", "replyToMessageId": "7", "disableNotification": true},
	), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if sender.request.ChatID != "99" || sender.request.Message != "wired" || sender.request.ReplyToMessageID != "7" || !sender.request.DisableNotification {
		t.Fatalf("request = %#v", sender.request)
	}
}

var _ = tg.Sender

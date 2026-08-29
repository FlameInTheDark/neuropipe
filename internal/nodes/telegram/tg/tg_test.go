package tg

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type senderStub struct {
	message    domain.TelegramMessageRequest
	photo      domain.TelegramPhotoRequest
	document   domain.TelegramDocumentRequest
	edit       domain.TelegramEditRequest
	del        domain.TelegramDeleteRequest
	callback   domain.TelegramCallbackAnswerRequest
	chatAction domain.TelegramChatActionRequest
	pin        domain.TelegramPinRequest

	messageResult  domain.TelegramMessageResult
	photoResult    domain.TelegramMessageResult
	documentResult domain.TelegramMessageResult
	editResult     domain.TelegramActionResult
	deleteResult   domain.TelegramActionResult
	callbackResult domain.TelegramActionResult
	actionResult   domain.TelegramActionResult
	pinResult      domain.TelegramActionResult
}

func (s *senderStub) SendTelegramMessage(_ context.Context, request domain.TelegramMessageRequest) (domain.TelegramMessageResult, error) {
	s.message = request
	return s.messageResult, nil
}
func (s *senderStub) SendTelegramPhoto(_ context.Context, request domain.TelegramPhotoRequest) (domain.TelegramMessageResult, error) {
	s.photo = request
	return s.photoResult, nil
}
func (s *senderStub) SendTelegramDocument(_ context.Context, request domain.TelegramDocumentRequest) (domain.TelegramMessageResult, error) {
	s.document = request
	return s.documentResult, nil
}
func (s *senderStub) EditTelegramMessage(_ context.Context, request domain.TelegramEditRequest) (domain.TelegramActionResult, error) {
	s.edit = request
	return s.editResult, nil
}
func (s *senderStub) DeleteTelegramMessage(_ context.Context, request domain.TelegramDeleteRequest) (domain.TelegramActionResult, error) {
	s.del = request
	return s.deleteResult, nil
}
func (s *senderStub) AnswerTelegramCallbackQuery(_ context.Context, request domain.TelegramCallbackAnswerRequest) (domain.TelegramActionResult, error) {
	s.callback = request
	return s.callbackResult, nil
}
func (s *senderStub) SendTelegramChatAction(_ context.Context, request domain.TelegramChatActionRequest) (domain.TelegramActionResult, error) {
	s.chatAction = request
	return s.actionResult, nil
}
func (s *senderStub) PinTelegramMessage(_ context.Context, request domain.TelegramPinRequest) (domain.TelegramActionResult, error) {
	s.pin = request
	return s.pinResult, nil
}

type runtimeStub struct{ sender nodes.TelegramSender }

func (r runtimeStub) TelegramSender() nodes.TelegramSender { return r.sender }

func TestSenderRequiresProvider(t *testing.T) {
	if _, err := Sender(nil); err == nil {
		t.Fatal("nil runtime must fail")
	}
	if _, err := Sender(runtimeStub{}); err == nil {
		t.Fatal("nil sender must fail")
	}
	sender := &senderStub{}
	if resolved, err := Sender(runtimeStub{sender: sender}); err != nil || resolved != sender {
		t.Fatalf("sender = %v, err = %v", resolved, err)
	}
}

func TestRequiredStringAndBoolHelpers(t *testing.T) {
	invocation := nodes.Invocation{Inputs: map[string]any{"chatId": " 55 ", "silent": true}}
	if value := String(invocation, "chatId"); value != "55" {
		t.Fatalf("string = %q", value)
	}
	if !BoolValue(invocation, "silent") {
		t.Fatal("bool = false")
	}
	if _, err := RequiredString(invocation, "missing", "chat ID"); err == nil || err.Error() != "chat ID is required" {
		t.Fatalf("required error = %v", err)
	}
	if value, err := RequiredString(invocation, "chatId", "chat ID"); err != nil || value != "55" {
		t.Fatalf("required = %q, %v", value, err)
	}
}

func TestStringCoercesNumericValues(t *testing.T) {
	invocation := nodes.Invocation{Inputs: map[string]any{
		"jsonNumber": json.Number("1234567890"),
		"int64":      int64(42),
		"float":      float64(77),
	}}
	if value := String(invocation, "jsonNumber"); value != "1234567890" {
		t.Fatalf("json.Number = %q", value)
	}
	if value := String(invocation, "int64"); value != "42" {
		t.Fatalf("int64 = %q", value)
	}
	if value := String(invocation, "float"); value != "77" {
		t.Fatalf("float = %q", value)
	}
}

func TestDefinitionSkeletonIncludesIdentityField(t *testing.T) {
	definition := Definition("action:telegram_test", "Test", "desc.", "send", nil, nil, nil, nil)
	if definition.Type != "action:telegram_test" || definition.Category != "Telegram" || definition.Mode != domain.NodeImpure {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Capabilities) != 1 || definition.Capabilities[0] != domain.CapabilityNetwork {
		t.Fatalf("capabilities = %#v", definition.Capabilities)
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Kind != "telegram-identity" {
		t.Fatalf("fields = %#v", definition.Fields)
	}
	if definition.DefaultConfig["identityId"] != "" {
		t.Fatalf("defaults = %#v", definition.DefaultConfig)
	}
}

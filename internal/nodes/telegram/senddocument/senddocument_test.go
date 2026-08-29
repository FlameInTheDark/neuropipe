package senddocument

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ sender nodes.TelegramSender }

func (r runtimeStub) TelegramSender() nodes.TelegramSender { return r.sender }

type senderStub struct {
	request domain.TelegramDocumentRequest
	result  domain.TelegramMessageResult
}

func (s *senderStub) SendTelegramDocument(_ context.Context, request domain.TelegramDocumentRequest) (domain.TelegramMessageResult, error) {
	s.request = request
	return s.result, nil
}
func (s *senderStub) SendTelegramMessage(context.Context, domain.TelegramMessageRequest) (domain.TelegramMessageResult, error) {
	panic("unused")
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
		Node:            domain.FlowNode{Type: "action:telegram_send_document", Data: map[string]any{"config": config}},
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestSendsDocumentURL(t *testing.T) {
	sender := &senderStub{result: domain.TelegramMessageResult{MessageID: "77", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{
		"chatId": "55", "documentUrl": "https://example.com/report.pdf", "caption": "Monthly report",
		"parseMode": "HTML", "identityId": "bot-1",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "sent" || result.Outputs["messageId"] != "77" {
		t.Fatalf("result = %#v", result)
	}
	if sender.request.DocumentURL != "https://example.com/report.pdf" || sender.request.Caption != "Monthly report" ||
		sender.request.ParseMode != "HTML" || sender.request.ChatID != "55" || sender.request.IdentityID != "bot-1" {
		t.Fatalf("request = %#v", sender.request)
	}
	if len(sender.request.Data) != 0 {
		t.Fatalf("url document must not carry data: %#v", sender.request)
	}
}

func TestSendsDocumentFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(path, []byte("file body"), 0o600); err != nil {
		t.Fatal(err)
	}
	sender := &senderStub{result: domain.TelegramMessageResult{MessageID: "78", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{
		"chatId": "55", "documentPath": path, "caption": "from disk",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "sent" {
		t.Fatalf("result = %#v", result)
	}
	if sender.request.FileName != "report.txt" || string(sender.request.Data) != "file body" {
		t.Fatalf("request = %#v", sender.request)
	}
}

func TestSendsDocumentFromDataPin(t *testing.T) {
	sender := &senderStub{result: domain.TelegramMessageResult{MessageID: "79", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	inputs := map[string]any{
		"chatId": "55", "documentData": base64.StdEncoding.EncodeToString([]byte("drawn image")), "fileName": "weather.png",
	}
	result, err := module.Execute(context.Background(), invocation(map[string]any{}, inputs), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "sent" {
		t.Fatalf("result = %#v", result)
	}
	if sender.request.FileName != "weather.png" || string(sender.request.Data) != "drawn image" {
		t.Fatalf("request = %#v", sender.request)
	}
}

func TestCaptionLimitRejected(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{
		"chatId": "55", "documentUrl": "https://example.com/x.pdf", "caption": strings.Repeat("c", 1025),
	}), runtimeStub{sender: &senderStub{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "rejected" || !strings.Contains(result.Outputs["reason"].(string), "1,024") {
		t.Fatalf("result = %#v", result)
	}
}

func TestValidationErrors(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{"documentUrl": "https://example.com"}), runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("missing chat id accepted")
	}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{"chatId": "55"}), runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("missing document accepted")
	}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{
		"chatId": "55", "documentUrl": "https://example.com", "parseMode": "Markdown",
	}), runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("invalid parse mode accepted")
	}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{
		"chatId": "55", "documentUrl": "https://example.com",
	}), nil); err == nil {
		t.Fatal("missing runtime accepted")
	}
}

func TestMissingPathRejected(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{
		"chatId": "55", "documentPath": "/definitely/missing/report.pdf",
	}), runtimeStub{sender: &senderStub{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "rejected" {
		t.Fatalf("result = %#v", result)
	}
}

/* ------------------------------------------------------------------ */
/* document source selector                                            */
/* ------------------------------------------------------------------ */

func TestSendsDocumentFromBase64(t *testing.T) {
	sender := &senderStub{result: domain.TelegramMessageResult{MessageID: "80", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	encoded := base64.StdEncoding.EncodeToString([]byte("report body"))
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"documentSource": "base64"},
		map[string]any{"chatId": "55", "documentBase64": "data:application/pdf;base64," + encoded, "fileName": "report.pdf"},
	), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "sent" {
		t.Fatalf("result = %#v", result)
	}
	if sender.request.DocumentURL != "" || sender.request.FileName != "report.pdf" || sender.request.ContentType != "application/pdf" || string(sender.request.Data) != "report body" {
		t.Fatalf("request = %#v", sender.request)
	}
}

// Auto mode keeps accepting a base64 document even without a selected mode,
// with the bytes pin taking precedence when both are set.
func TestAutoModeFallsBackToBase64(t *testing.T) {
	sender := &senderStub{result: domain.TelegramMessageResult{MessageID: "81", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	encoded := base64.StdEncoding.EncodeToString([]byte("b64 body"))
	if _, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{
		"chatId": "55", "documentBase64": encoded, "fileName": "notes.txt",
	}), runtimeStub{sender: sender}); err != nil {
		t.Fatal(err)
	}
	if sender.request.FileName != "notes.txt" || string(sender.request.Data) != "b64 body" {
		t.Fatalf("auto base64 request = %#v", sender.request)
	}
}

func TestMissingSourcePerModeIsHardError(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	cases := []struct {
		config map[string]any
		want   string
	}{
		{map[string]any{"documentSource": "url"}, "document URL is required when the document source is URL"},
		{map[string]any{"documentSource": "file"}, "document path is required when the document source is Local file"},
		{map[string]any{"documentSource": "base64"}, "document base64 is required when the document source is Base64"},
		{map[string]any{"documentSource": "bytes"}, "document data is required when the document source is Bytes from another node"},
	}
	for _, testCase := range cases {
		_, err := module.Execute(context.Background(), invocation(testCase.config, map[string]any{"chatId": "55"}), runtimeStub{sender: &senderStub{}})
		if err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("config %v: err = %v, want %q", testCase.config, err, testCase.want)
		}
	}
}

func TestResolveDocumentSourceFiltersPins(t *testing.T) {
	idsFor := func(config map[string]any) map[string]bool {
		definition, err := resolve(domain.FlowNode{Type: "action:telegram_send_document", Data: map[string]any{"config": config}})
		if err != nil {
			t.Fatal(err)
		}
		ids := map[string]bool{}
		for _, port := range definition.Inputs {
			ids[port.ID] = true
		}
		return ids
	}
	fileOnly := idsFor(map[string]any{"documentSource": "file"})
	if !fileOnly["documentPath"] || fileOnly["documentUrl"] || fileOnly["documentBase64"] || fileOnly["documentData"] || fileOnly["fileName"] {
		t.Fatalf("file mode pins = %v", fileOnly)
	}
	base64Only := idsFor(map[string]any{"documentSource": "base64"})
	if !base64Only["documentBase64"] || !base64Only["fileName"] || base64Only["documentUrl"] || base64Only["documentData"] {
		t.Fatalf("base64 mode pins = %v", base64Only)
	}
	auto := idsFor(map[string]any{})
	for _, pin := range []string{"documentUrl", "documentPath", "documentBase64", "documentData", "fileName", "caption", "chatId"} {
		if !auto[pin] {
			t.Fatalf("auto mode lost pin %q", pin)
		}
	}
}

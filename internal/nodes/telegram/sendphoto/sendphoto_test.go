package sendphoto

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
	request domain.TelegramPhotoRequest
	result  domain.TelegramMessageResult
}

func (s *senderStub) SendTelegramPhoto(_ context.Context, request domain.TelegramPhotoRequest) (domain.TelegramMessageResult, error) {
	s.request = request
	return s.result, nil
}
func (s *senderStub) SendTelegramMessage(context.Context, domain.TelegramMessageRequest) (domain.TelegramMessageResult, error) {
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
		Node:            domain.FlowNode{Type: "action:telegram_send_photo", Data: map[string]any{"config": config}},
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestSendsPhotoURL(t *testing.T) {
	sender := &senderStub{result: domain.TelegramMessageResult{MessageID: "31", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{
		"chatId": "55", "photoUrl": "https://example.com/photo.jpg", "caption": "sunset", "identityId": "bot-1",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "sent" || result.Outputs["messageId"] != "31" {
		t.Fatalf("result = %#v", result)
	}
	if sender.request.PhotoURL != "https://example.com/photo.jpg" || sender.request.Caption != "sunset" || sender.request.ChatID != "55" || sender.request.IdentityID != "bot-1" {
		t.Fatalf("request = %#v", sender.request)
	}
	if len(sender.request.Data) != 0 {
		t.Fatalf("url photo must not carry data: %#v", sender.request)
	}
}

func TestSendsPhotoFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "holiday.jpg")
	if err := os.WriteFile(path, []byte("jpeg bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	sender := &senderStub{result: domain.TelegramMessageResult{MessageID: "32", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"photoSource": "file"},
		map[string]any{"chatId": "55", "photoPath": path},
	), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "sent" {
		t.Fatalf("result = %#v", result)
	}
	if sender.request.PhotoURL != "" || sender.request.FileName != "holiday.jpg" || sender.request.ContentType != "image/jpeg" || string(sender.request.Data) != "jpeg bytes" {
		t.Fatalf("request = %#v", sender.request)
	}
}

func TestSendsPhotoFromBase64(t *testing.T) {
	sender := &senderStub{result: domain.TelegramMessageResult{MessageID: "33", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	encoded := base64.StdEncoding.EncodeToString([]byte("pixel data"))
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"photoSource": "base64"},
		map[string]any{"chatId": "55", "photoBase64": "data:image/png;base64," + encoded, "photoName": "chart.png"},
	), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "sent" {
		t.Fatalf("result = %#v", result)
	}
	if sender.request.FileName != "chart.png" || sender.request.ContentType != "image/png" || string(sender.request.Data) != "pixel data" {
		t.Fatalf("request = %#v", sender.request)
	}
}

func TestSendsPhotoFromBytesPin(t *testing.T) {
	sender := &senderStub{result: domain.TelegramMessageResult{MessageID: "34", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"photoSource": "bytes"},
		map[string]any{"chatId": "55", "photoData": []byte("drawn image"), "photoName": "weather.png"},
	), runtimeStub{sender: sender})
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

// Auto mode keeps the legacy contract: a photo URL passes through untouched
// while an upload source (path, base64, or bytes) takes precedence.
func TestAutoModePrefersUploadSourcesAndFallsBackToURL(t *testing.T) {
	sender := &senderStub{result: domain.TelegramMessageResult{MessageID: "35", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{
		"chatId": "55", "photoUrl": "https://example.com/photo.jpg",
	}), runtimeStub{sender: sender}); err != nil {
		t.Fatal(err)
	}
	if sender.request.PhotoURL != "https://example.com/photo.jpg" || len(sender.request.Data) != 0 {
		t.Fatalf("auto url fallback = %#v", sender.request)
	}

	encoded := base64.StdEncoding.EncodeToString([]byte("local wins"))
	if _, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{
		"chatId": "55", "photoUrl": "https://example.com/photo.jpg", "photoBase64": encoded, "photoName": "local.png",
	}), runtimeStub{sender: sender}); err != nil {
		t.Fatal(err)
	}
	if sender.request.PhotoURL != "" || sender.request.FileName != "local.png" || string(sender.request.Data) != "local wins" {
		t.Fatalf("auto upload precedence = %#v", sender.request)
	}
}

func TestMissingSourcePerModeIsHardError(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	cases := []struct {
		config map[string]any
		want   string
	}{
		{map[string]any{"photoSource": "url"}, "photo URL is required when the photo source is URL"},
		{map[string]any{"photoSource": "file"}, "photo path is required when the photo source is Local file"},
		{map[string]any{"photoSource": "base64"}, "photo base64 is required when the photo source is Base64"},
		{map[string]any{"photoSource": "bytes"}, "photo data is required when the photo source is Bytes from another node"},
		{map[string]any{}, "a photo URL, file, base64, or data is required"},
	}
	for _, testCase := range cases {
		_, err := module.Execute(context.Background(), invocation(testCase.config, map[string]any{"chatId": "55"}), runtimeStub{sender: &senderStub{}})
		if err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("config %v: err = %v, want %q", testCase.config, err, testCase.want)
		}
	}
}

func TestMissingPhotoFileRejectedSoftly(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"photoSource": "file"},
		map[string]any{"chatId": "55", "photoPath": "/definitely/missing/photo.jpg"},
	), runtimeStub{sender: &senderStub{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "rejected" || !strings.Contains(result.Outputs["reason"].(string), "no such file or directory") {
		t.Fatalf("result = %#v", result)
	}
}

func TestCaptionLimitRejected(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{}, map[string]any{
		"chatId": "55", "photoUrl": "https://example.com/x.jpg", "caption": strings.Repeat("c", 1025),
	}), runtimeStub{sender: &senderStub{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "rejected" || !strings.Contains(result.Outputs["reason"].(string), "1,024") {
		t.Fatalf("result = %#v", result)
	}
}

func TestResolvePhotoSourceFiltersPins(t *testing.T) {
	idsFor := func(config map[string]any) map[string]bool {
		definition, err := resolve(domain.FlowNode{Type: "action:telegram_send_photo", Data: map[string]any{"config": config}})
		if err != nil {
			t.Fatal(err)
		}
		ids := map[string]bool{}
		for _, port := range definition.Inputs {
			ids[port.ID] = true
		}
		return ids
	}
	urlOnly := idsFor(map[string]any{"photoSource": "url"})
	if !urlOnly["photoUrl"] || urlOnly["photoPath"] || urlOnly["photoBase64"] || urlOnly["photoData"] || urlOnly["photoName"] {
		t.Fatalf("url mode pins = %v", urlOnly)
	}
	bytesOnly := idsFor(map[string]any{"photoSource": "bytes"})
	if !bytesOnly["photoData"] || !bytesOnly["photoName"] || bytesOnly["photoUrl"] || bytesOnly["photoPath"] || bytesOnly["photoBase64"] {
		t.Fatalf("bytes mode pins = %v", bytesOnly)
	}
	auto := idsFor(map[string]any{})
	for _, pin := range []string{"photoUrl", "photoPath", "photoBase64", "photoData", "photoName", "caption", "chatId"} {
		if !auto[pin] {
			t.Fatalf("auto mode lost pin %q", pin)
		}
	}
}

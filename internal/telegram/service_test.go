package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/FlameInTheDark/neuropipe/internal/telespec"
)

// memoryVault mirrors the production security.Vault concurrency contract:
// the service reads tokens from its validation loop goroutine while request
// goroutines Put/Delete, so the fake must be safe for concurrent use.
type memoryVault struct {
	mu     sync.Mutex
	values map[string]string
}

func (v *memoryVault) Get(key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	value, found := v.values[key]
	if !found {
		return "", fmt.Errorf("missing %s", key)
	}
	return value, nil
}
func (v *memoryVault) Put(key, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.values[key] = value
	return nil
}
func (v *memoryVault) Delete(key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.values, key)
	return nil
}

type fakeBindings struct {
	mu        sync.Mutex
	bindings  []domain.TriggerBinding
	listCalls int
}

func (f *fakeBindings) ListTriggers(context.Context, domain.TriggerKind) ([]domain.TriggerBinding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	return append([]domain.TriggerBinding(nil), f.bindings...), nil
}

type fakeRunner struct {
	mu       sync.Mutex
	bindings []string
	packets  []pipeline.Packet
	signal   chan struct{}
}

func newFakeRunner() *fakeRunner { return &fakeRunner{signal: make(chan struct{}, 32)} }

func (f *fakeRunner) QueueBinding(_ context.Context, bindingID string, packet pipeline.Packet, _ bool) (domain.Execution, error) {
	f.mu.Lock()
	f.bindings = append(f.bindings, bindingID)
	f.packets = append(f.packets, packet)
	f.mu.Unlock()
	f.signal <- struct{}{}
	return domain.Execution{ID: "run"}, nil
}
func (f *fakeRunner) snapshot() ([]string, []pipeline.Packet) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.bindings...), append([]pipeline.Packet(nil), f.packets...)
}
func (f *fakeRunner) waitFor(t *testing.T, count int) bool {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		f.mu.Lock()
		have := len(f.bindings)
		f.mu.Unlock()
		if have >= count {
			return true
		}
		select {
		case <-deadline:
			return false
		case <-f.signal:
		}
	}
}

type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

type fakeAPI struct {
	mu        sync.Mutex
	requests  []recordedRequest
	responses []fakeResponse
	handler   func(method string, body map[string]any) (any, int)
	server    *httptest.Server
}

type fakeResponse struct {
	status int
	body   string
}

func okResult(result any) string {
	encoded, _ := json.Marshal(map[string]any{"ok": true, "result": result})
	return string(encoded)
}

func apiFailure(status int, description string) string {
	encoded, _ := json.Marshal(map[string]any{"ok": false, "description": description})
	return string(encoded)
}

func newFakeAPI(t *testing.T, handler func(method string, body map[string]any) (any, int)) *fakeAPI {
	api := &fakeAPI{handler: handler}
	api.server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(request.Body).Decode(&body)
		api.mu.Lock()
		parts := strings.Split(request.URL.Path, "/")
		method := parts[len(parts)-1]
		api.requests = append(api.requests, recordedRequest{Method: method, Path: request.URL.Path, Body: body})
		api.mu.Unlock()
		result, status := handler(method, body)
		if status == 0 {
			status = http.StatusOK
		}
		if result == nil {
			// No canned result: block like a real long poll until the client
			// goes away so the poll loop does not spin hot.
			<-request.Context().Done()
			return
		}
		if encoded, ok := result.(string); ok {
			response.WriteHeader(status)
			_, _ = response.Write([]byte(encoded))
			return
		}
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(status)
		_, _ = response.Write([]byte(okResult(result)))
	}))
	t.Cleanup(api.server.Close)
	return api
}

func (api *fakeAPI) recorded() []recordedRequest {
	api.mu.Lock()
	defer api.mu.Unlock()
	return append([]recordedRequest(nil), api.requests...)
}

func newTestService(t *testing.T, api *fakeAPI, vault *memoryVault, bindings *fakeBindings, runner *fakeRunner) *Service {
	if vault == nil {
		vault = &memoryVault{values: map[string]string{}}
	}
	if bindings == nil {
		bindings = &fakeBindings{}
	}
	if runner == nil {
		runner = newFakeRunner()
	}
	service := New(vault, bindings, runner, nil, nil)
	service.SetBaseURL(api.server.URL)
	t.Cleanup(service.Stop)
	return service
}

func botIdentity(id string) domain.TelegramIdentity {
	return domain.TelegramIdentity{ID: id, Label: id, BotUserID: "1", Username: "bot", Status: domain.TelegramIdentityConnected}
}

func TestAddManualIdentityValidatesAndStoresToken(t *testing.T) {
	api := newFakeAPI(t, func(method string, _ map[string]any) (any, int) {
		if method != "getMe" {
			t.Fatalf("unexpected method %q", method)
		}
		return map[string]any{"id": 42, "is_bot": true, "username": "NeuropipeBot", "first_name": "Neuropipe"}, 0
	})
	vault := &memoryVault{values: map[string]string{}}
	service := newTestService(t, api, vault, nil, nil)

	identity, err := service.AddManualIdentity(context.Background(), domain.TelegramManualIdentityRequest{Label: "", Token: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if identity.Username != "NeuropipeBot" || identity.BotUserID != "42" || identity.Label != "@NeuropipeBot" || identity.Status != domain.TelegramIdentityConnected {
		t.Fatalf("identity = %#v", identity)
	}
	if got, err := vault.Get(tokenKey(identity.ID)); err != nil || got != "abc" {
		t.Fatalf("vault token = %q, %v", got, err)
	}

	if _, err := service.AddManualIdentity(context.Background(), domain.TelegramManualIdentityRequest{Token: "abc"}); err == nil || !strings.Contains(err.Error(), "already connected") {
		t.Fatalf("duplicate token error = %v", err)
	}
}

func TestAddManualIdentityRejectsNonBotTokens(t *testing.T) {
	api := newFakeAPI(t, func(_ string, _ map[string]any) (any, int) {
		return map[string]any{"id": 7, "is_bot": false, "username": "human"}, 0
	})
	service := newTestService(t, api, nil, nil, nil)
	if _, err := service.AddManualIdentity(context.Background(), domain.TelegramManualIdentityRequest{Token: "user-token"}); err == nil || !strings.Contains(err.Error(), "bot") {
		t.Fatalf("non-bot error = %v", err)
	}
}

func TestAddManualIdentitySurfacesInvalidTokens(t *testing.T) {
	api := newFakeAPI(t, func(_ string, _ map[string]any) (any, int) {
		return apiFailure(http.StatusUnauthorized, "Unauthorized"), http.StatusUnauthorized
	})
	service := newTestService(t, api, nil, nil, nil)
	_, err := service.AddManualIdentity(context.Background(), domain.TelegramManualIdentityRequest{Token: "bad"})
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("invalid token error = %v", err)
	}
}

func messageUpdate(updateID int64, chatID int64, text string) telegramUpdate {
	return telegramUpdate{UpdateID: updateID, Message: &telegramMessage{
		MessageID: updateID, Date: 1700000000, Text: text,
		Chat: telegramChat{ID: chatID, Type: "private", Username: "chat"},
		From: &telegramUser{ID: 5, Username: "alice", FirstName: "Alice"},
	}}
}

func binding(id, eventType, identityID, chatIDs string, trusted, enabled bool) domain.TriggerBinding {
	config := map[string]any{"eventType": eventType, "identityId": identityID}
	if chatIDs != "" {
		config["chatIds"] = chatIDs
	}
	return domain.TriggerBinding{ID: id, Kind: domain.TriggerTelegram, Config: config, Trusted: trusted, Enabled: enabled}
}

func TestPollLoopDeliversOnlyTrustedMatchingBindings(t *testing.T) {
	delivered := make(chan struct{}, 4)
	api := newFakeAPI(t, func(method string, body map[string]any) (any, int) {
		if method == "getMe" {
			return map[string]any{"id": 1, "is_bot": true, "username": "bot"}, 0
		}
		if method != "getUpdates" {
			t.Errorf("unexpected method %q", method)
		}
		if offset, ok := body["offset"].(float64); ok && offset < 0 {
			// backlog flush: newest pending update, discarded
			return []any{map[string]any{"update_id": 90}}, 0
		}
		if offset, ok := body["offset"].(float64); ok && offset == 91 {
			delivered <- struct{}{}
			return []any{
				map[string]any{"update_id": 100, "message": map[string]any{"message_id": 100, "date": 1, "chat": map[string]any{"id": 55, "type": "private"}, "from": map[string]any{"id": 5, "username": "alice"}, "text": "hello"}},
				map[string]any{"update_id": 101, "callback_query": map[string]any{"id": "cb1", "data": "pressed", "from": map[string]any{"id": 5}, "message": map[string]any{"message_id": 9, "chat": map[string]any{"id": 55}}}},
			}, 0
		}
		return nil, 0
	})
	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	bindings := &fakeBindings{bindings: []domain.TriggerBinding{
		binding("trusted", "message", "", "", true, true),
		binding("callback", "callback_query", "", "", true, true),
		binding("untrusted", "message", "", "", false, true),
		binding("disabled", "message", "", "", true, false),
		binding("otherchat", "message", "", "999", true, true),
	}}
	runner := newFakeRunner()
	service := newTestService(t, api, vault, bindings, runner)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})
	service.Start(context.Background())

	if !runner.waitFor(t, 2) {
		bindingIDs, _ := runner.snapshot()
		t.Fatalf("expected 2 deliveries, got %v", bindingIDs)
	}
	bindingIDs, packets := runner.snapshot()
	if bindingIDs[0] != "trusted" || bindingIDs[1] != "callback" {
		t.Fatalf("delivered bindings = %v", bindingIDs)
	}
	first, _ := packets[0]["event"].(telespec.TelegramEvent)
	if first.Type != "message" || first.UpdateID != 100 {
		t.Fatalf("first event = %#v", first)
	}
	message, _ := first.Payload["message"].(telespec.Message)
	if message.Text != "hello" || message.ChatID != 55 || message.FromUsername != "alice" {
		t.Fatalf("message payload = %#v", message)
	}
	second, _ := packets[1]["event"].(telespec.TelegramEvent)
	if second.Type != "callback_query" {
		t.Fatalf("second event = %#v", second)
	}
	query, _ := second.Payload["callbackQuery"].(telespec.CallbackQuery)
	if query.Data != "pressed" || query.ChatID != 55 {
		t.Fatalf("callback payload = %#v", query)
	}

	// The loop must confirm both updates by advancing its offset cursor.
	deadline := time.After(5 * time.Second)
	for {
		confirmed := false
		for _, request := range api.recorded() {
			if request.Method == "getUpdates" && request.Body["offset"] != nil {
				if offset, ok := request.Body["offset"].(float64); ok && offset >= 102 {
					confirmed = true
				}
			}
		}
		if confirmed {
			break
		}
		select {
		case <-deadline:
			t.Fatal("poll loop never confirmed update 101")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestAllowedUpdatesAreTrustGated(t *testing.T) {
	var seenAllowed []any
	seenMu := sync.Mutex{}
	api := newFakeAPI(t, func(method string, body map[string]any) (any, int) {
		if method == "getMe" {
			return map[string]any{"id": 1, "is_bot": true, "username": "bot"}, 0
		}
		if method != "getUpdates" {
			t.Errorf("unexpected method %q", method)
		}
		seenMu.Lock()
		if body["allowed_updates"] != nil {
			seenAllowed = append(seenAllowed, body["allowed_updates"])
		}
		seenMu.Unlock()
		return []any{}, 0
	})
	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	bindings := &fakeBindings{bindings: []domain.TriggerBinding{
		binding("trusted", "message", "", "", true, true),
		binding("untrusted", "callback_query", "", "", false, true),
	}}
	service := newTestService(t, api, vault, bindings, nil)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})
	service.Start(context.Background())

	deadline := time.After(5 * time.Second)
	for {
		seenMu.Lock()
		count := len(seenAllowed)
		seenMu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("poll loop never issued a getUpdates with allowed_updates")
		case <-time.After(10 * time.Millisecond):
		}
	}
	seenMu.Lock()
	defer seenMu.Unlock()
	first := fmt.Sprint(seenAllowed[0])
	if !strings.Contains(first, "message") {
		t.Fatalf("allowed_updates = %v", first)
	}
	if strings.Contains(first, "callback_query") {
		t.Fatalf("untrusted update type leaked into allowed_updates: %v", first)
	}
}

func TestChatAllowlistFiltering(t *testing.T) {
	event := telespec.TelegramEvent{Type: "message", Payload: map[string]any{"message": telespec.Message{ChatID: -100123, ChatUsername: "mychannel"}}}
	if !chatAllowed("", event) {
		t.Fatal("empty allowlist must match everything")
	}
	if !chatAllowed("-100123", event) {
		t.Fatal("numeric allowlist must match the chat id")
	}
	if !chatAllowed("@mychannel", event) {
		t.Fatal("username allowlist must match the channel")
	}
	if !chatAllowed("7, -100123", event) {
		t.Fatal("comma-separated allowlist must match")
	}
	if chatAllowed("7", event) {
		t.Fatal("unlisted chat must be rejected")
	}
	if chatAllowed("@other", event) {
		t.Fatal("unlisted username must be rejected")
	}
}

func TestConflictSurfacesActionableError(t *testing.T) {
	api := newFakeAPI(t, func(method string, _ map[string]any) (any, int) {
		if method == "getMe" {
			return map[string]any{"id": 1, "is_bot": true, "username": "bot"}, 0
		}
		return apiFailure(http.StatusConflict, "Conflict: terminated by other getUpdates request; make sure that only one bot instance is running"), http.StatusConflict
	})
	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	service := newTestService(t, api, vault, nil, nil)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})
	service.Start(context.Background())

	deadline := time.After(5 * time.Second)
	for {
		status := service.Status()
		if strings.Contains(status.LastError, "Conflict") {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("conflict never surfaced, status = %#v", status)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestSendMessageActionRequestShape(t *testing.T) {
	api := newFakeAPI(t, func(method string, body map[string]any) (any, int) {
		if method != "sendMessage" {
			t.Errorf("unexpected method %q", method)
		}
		if body["chat_id"] != "55" || body["text"] != "hi there" || body["parse_mode"] != "HTML" {
			t.Fatalf("sendMessage body = %#v", body)
		}
		// The Bot API types reply_to_message_id as Integer — it must be a
		// JSON number, not the string pin value it arrives as.
		if body["reply_to_message_id"] != float64(77) || body["disable_notification"] != true {
			t.Fatalf("sendMessage body = %#v", body)
		}
		return map[string]any{"message_id": 321}, 0
	})
	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	service := newTestService(t, api, vault, nil, nil)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})

	result, err := service.SendTelegramMessage(context.Background(), domain.TelegramMessageRequest{ChatID: "55", Message: "hi there", ParseMode: "HTML", ReplyToMessageID: "77", DisableNotification: true})
	if err != nil || !result.Sent || result.MessageID != "321" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

func TestSendMessageRejectsNonNumericReplyID(t *testing.T) {
	service := New(&memoryVault{values: map[string]string{}}, &fakeBindings{}, newFakeRunner(), nil, nil)
	result, err := service.SendTelegramMessage(context.Background(), domain.TelegramMessageRequest{ChatID: "55", Message: "hi", ReplyToMessageID: "7.9e+16"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent || !strings.Contains(result.Reason, "not numeric") {
		t.Fatalf("reply result = %#v", result)
	}
}

func TestSendMessageSoftRejectsAPIFailures(t *testing.T) {
	api := newFakeAPI(t, func(_ string, _ map[string]any) (any, int) {
		return apiFailure(http.StatusBadRequest, "Bad Request: message text is empty"), http.StatusBadRequest
	})
	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	service := newTestService(t, api, vault, nil, nil)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})

	result, err := service.SendTelegramMessage(context.Background(), domain.TelegramMessageRequest{ChatID: "55", Message: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent || result.Reason != "Bad Request: message text is empty" {
		t.Fatalf("soft reject = %#v", result)
	}
}

func TestSendMessagePreValidatesLengthCap(t *testing.T) {
	api := newFakeAPI(t, func(method string, _ map[string]any) (any, int) {
		t.Fatalf("no HTTP call expected, got %q", method)
		return nil, 0
	})
	service := newTestService(t, api, nil, nil, nil)
	result, err := service.SendTelegramMessage(context.Background(), domain.TelegramMessageRequest{ChatID: "1", Message: strings.Repeat("a", 4097)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent || !strings.Contains(result.Reason, "4,096") {
		t.Fatalf("cap result = %#v", result)
	}
}

func TestPinMessageUsesUnpinMethod(t *testing.T) {
	var methods []string
	api := newFakeAPI(t, func(method string, _ map[string]any) (any, int) {
		methods = append(methods, method)
		return true, 0
	})
	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	service := newTestService(t, api, vault, nil, nil)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})

	if _, err := service.PinTelegramMessage(context.Background(), domain.TelegramPinRequest{ChatID: "1", MessageID: "2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PinTelegramMessage(context.Background(), domain.TelegramPinRequest{ChatID: "1", MessageID: "2", Unpin: true}); err != nil {
		t.Fatal(err)
	}
	if len(methods) != 2 || methods[0] != "pinChatMessage" || methods[1] != "unpinChatMessage" {
		t.Fatalf("methods = %v", methods)
	}
}

func TestRemoveIdentityDeletesVaultAndStopsLoop(t *testing.T) {
	api := newFakeAPI(t, func(method string, _ map[string]any) (any, int) {
		if method == "getMe" {
			return map[string]any{"id": 1, "is_bot": true, "username": "bot"}, 0
		}
		return []any{}, 0
	})
	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	service := newTestService(t, api, vault, nil, nil)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})
	service.Start(context.Background())
	if !service.Status().Connected {
		deadline := time.After(5 * time.Second)
		for !service.Status().Connected {
			select {
			case <-deadline:
				t.Fatal("loop never started")
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	if err := service.RemoveIdentity(context.Background(), "bot"); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Get(tokenKey("bot")); err == nil {
		t.Fatal("vault record survived removal")
	}
	// Read under the service lock: the validation-loop goroutine started by
	// Start() may still be persisting identity state concurrently.
	service.mu.RLock()
	settings := service.settings
	service.mu.RUnlock()
	if len(settings.Identities) != 0 || settings.DefaultBotIdentityID != "" {
		t.Fatalf("settings = %#v", settings)
	}
}

func TestBacklogFlushDiscardsOfflineUpdates(t *testing.T) {
	api := newFakeAPI(t, func(method string, body map[string]any) (any, int) {
		if method == "getMe" {
			return map[string]any{"id": 1, "is_bot": true, "username": "bot"}, 0
		}
		if offset, ok := body["offset"].(float64); ok && offset < 0 {
			return []any{map[string]any{"update_id": 90}}, 0
		}
		if offset, ok := body["offset"].(float64); ok && offset > 0 && offset != 91 {
			t.Errorf("backlog was not discarded, polling from offset %v", offset)
		}
		return nil, 0
	})
	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	runner := newFakeRunner()
	service := newTestService(t, api, vault, nil, runner)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})
	service.Start(context.Background())
	time.Sleep(150 * time.Millisecond)
	bindingIDs, _ := runner.snapshot()
	if len(bindingIDs) != 0 {
		t.Fatalf("backlog was delivered: %v", bindingIDs)
	}
}

func TestSendDocumentByURLRequestShape(t *testing.T) {
	api := newFakeAPI(t, func(method string, body map[string]any) (any, int) {
		if method != "sendDocument" {
			t.Errorf("unexpected method %q", method)
		}
		if body["chat_id"] != "55" || body["document"] != "https://example.com/report.pdf" || body["caption"] != "monthly" {
			t.Fatalf("sendDocument body = %#v", body)
		}
		if _, exists := body["parse_mode"]; exists {
			t.Fatalf("empty parse mode must be omitted: %#v", body)
		}
		return map[string]any{"message_id": 900}, 0
	})
	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	service := newTestService(t, api, vault, nil, nil)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})

	result, err := service.SendTelegramDocument(context.Background(), domain.TelegramDocumentRequest{ChatID: "55", DocumentURL: "https://example.com/report.pdf", Caption: "monthly"})
	if err != nil || !result.Sent || result.MessageID != "900" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

func TestSendDocumentUploadsMultipart(t *testing.T) {
	var captured struct {
		contentType string
		chatID      string
		caption     string
		parseMode   string
		fileName    string
		fileType    string
		fileBody    string
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		captured.contentType = request.Header.Get("Content-Type")
		if err := request.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			return
		}
		captured.chatID = request.FormValue("chat_id")
		captured.caption = request.FormValue("caption")
		captured.parseMode = request.FormValue("parse_mode")
		file, header, err := request.FormFile("document")
		if err != nil {
			t.Errorf("form file: %v", err)
			return
		}
		defer func() { _ = file.Close() }()
		captured.fileName = header.Filename
		captured.fileType = header.Header.Get("Content-Type")
		body, _ := io.ReadAll(file)
		captured.fileBody = string(body)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(okResult(map[string]any{"message_id": 901})))
	}))
	defer server.Close()

	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	service := New(vault, &fakeBindings{}, newFakeRunner(), nil, nil)
	service.SetBaseURL(server.URL)
	t.Cleanup(service.Stop)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})

	result, err := service.SendTelegramDocument(context.Background(), domain.TelegramDocumentRequest{
		ChatID: "55", Caption: "rendered", ParseMode: "HTML",
		FileName: "weather.png", ContentType: "image/png", Data: []byte("PNGDATA"),
	})
	if err != nil || !result.Sent || result.MessageID != "901" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	if !strings.HasPrefix(captured.contentType, "multipart/form-data") {
		t.Fatalf("content type = %q", captured.contentType)
	}
	if captured.chatID != "55" || captured.caption != "rendered" || captured.parseMode != "HTML" {
		t.Fatalf("form fields = %+v", captured)
	}
	if captured.fileName != "weather.png" || captured.fileType != "image/png" || captured.fileBody != "PNGDATA" {
		t.Fatalf("file part = %+v", captured)
	}
}

func TestSendDocumentPreValidatesCaptionCap(t *testing.T) {
	service := New(&memoryVault{values: map[string]string{}}, &fakeBindings{}, newFakeRunner(), nil, nil)
	result, err := service.SendTelegramDocument(context.Background(), domain.TelegramDocumentRequest{ChatID: "1", DocumentURL: "https://example.com/x", Caption: strings.Repeat("a", 1025)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent || !strings.Contains(result.Reason, "1,024") {
		t.Fatalf("cap result = %#v", result)
	}
}

func TestSendPhotoByURLRequestShape(t *testing.T) {
	api := newFakeAPI(t, func(method string, body map[string]any) (any, int) {
		if method != "sendPhoto" {
			t.Errorf("unexpected method %q", method)
		}
		if body["chat_id"] != "55" || body["photo"] != "https://example.com/photo.jpg" || body["caption"] != "sunset" {
			t.Fatalf("sendPhoto body = %#v", body)
		}
		if _, exists := body["parse_mode"]; exists {
			t.Fatalf("empty parse mode must be omitted: %#v", body)
		}
		return map[string]any{"message_id": 910}, 0
	})
	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	service := newTestService(t, api, vault, nil, nil)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})

	result, err := service.SendTelegramPhoto(context.Background(), domain.TelegramPhotoRequest{ChatID: "55", PhotoURL: "https://example.com/photo.jpg", Caption: "sunset"})
	if err != nil || !result.Sent || result.MessageID != "910" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

func TestSendPhotoUploadsMultipart(t *testing.T) {
	var captured struct {
		contentType string
		chatID      string
		caption     string
		parseMode   string
		fileName    string
		fileType    string
		fileBody    string
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		captured.contentType = request.Header.Get("Content-Type")
		if err := request.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			return
		}
		captured.chatID = request.FormValue("chat_id")
		captured.caption = request.FormValue("caption")
		captured.parseMode = request.FormValue("parse_mode")
		file, header, err := request.FormFile("photo")
		if err != nil {
			t.Errorf("form file: %v", err)
			return
		}
		defer func() { _ = file.Close() }()
		captured.fileName = header.Filename
		captured.fileType = header.Header.Get("Content-Type")
		body, _ := io.ReadAll(file)
		captured.fileBody = string(body)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(okResult(map[string]any{"message_id": 911})))
	}))
	defer server.Close()

	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	service := New(vault, &fakeBindings{}, newFakeRunner(), nil, nil)
	service.SetBaseURL(server.URL)
	t.Cleanup(service.Stop)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})

	result, err := service.SendTelegramPhoto(context.Background(), domain.TelegramPhotoRequest{
		ChatID: "55", Caption: "rendered", ParseMode: "HTML",
		FileName: "weather.png", ContentType: "image/png", Data: []byte("PNGDATA"),
	})
	if err != nil || !result.Sent || result.MessageID != "911" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	if !strings.HasPrefix(captured.contentType, "multipart/form-data") {
		t.Fatalf("content type = %q", captured.contentType)
	}
	if captured.chatID != "55" || captured.caption != "rendered" || captured.parseMode != "HTML" {
		t.Fatalf("form fields = %+v", captured)
	}
	if captured.fileName != "weather.png" || captured.fileType != "image/png" || captured.fileBody != "PNGDATA" {
		t.Fatalf("file part = %+v", captured)
	}
}

func TestSendPhotoUploadDefaultsNameAndType(t *testing.T) {
	var captured struct {
		fileName string
		fileType string
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if err := request.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			return
		}
		file, header, err := request.FormFile("photo")
		if err != nil {
			t.Errorf("form file: %v", err)
			return
		}
		defer func() { _ = file.Close() }()
		captured.fileName = header.Filename
		captured.fileType = header.Header.Get("Content-Type")
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(okResult(map[string]any{"message_id": 912})))
	}))
	defer server.Close()

	vault := &memoryVault{values: map[string]string{tokenKey("bot"): "token"}}
	service := New(vault, &fakeBindings{}, newFakeRunner(), nil, nil)
	service.SetBaseURL(server.URL)
	t.Cleanup(service.Stop)
	service.Configure(domain.TelegramSettings{Identities: []domain.TelegramIdentity{botIdentity("bot")}, DefaultBotIdentityID: "bot"})

	_, err := service.SendTelegramPhoto(context.Background(), domain.TelegramPhotoRequest{ChatID: "55", Data: []byte("RAW")})
	if err != nil {
		t.Fatal(err)
	}
	if captured.fileName != "photo.jpg" || captured.fileType != "image/jpeg" {
		t.Fatalf("defaults = %+v", captured)
	}
}

func TestSendPhotoPreValidatesCaptionCap(t *testing.T) {
	service := New(&memoryVault{values: map[string]string{}}, &fakeBindings{}, newFakeRunner(), nil, nil)
	result, err := service.SendTelegramPhoto(context.Background(), domain.TelegramPhotoRequest{ChatID: "1", PhotoURL: "https://example.com/x.jpg", Caption: strings.Repeat("a", 1025)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent || !strings.Contains(result.Reason, "1,024") {
		t.Fatalf("cap result = %#v", result)
	}
}

package event

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/telespec"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

func TestCatalogIsUniqueAndValid(t *testing.T) {
	seen := map[string]bool{}
	for _, descriptor := range telespec.Catalog() {
		if descriptor.Type == "" || seen[descriptor.Type] {
			t.Fatalf("duplicate or empty event %q", descriptor.Type)
		}
		seen[descriptor.Type] = true
		if err := typespec.ValidateSpec(telespec.EventType()); err != nil {
			t.Fatalf("%s event contract: %v", descriptor.Type, err)
		}
	}
	if len(seen) < 8 {
		t.Fatalf("catalog has %d entries, expected the full v1 update coverage", len(seen))
	}
	allowed := telespec.AllowedUpdates([]string{"callback_query", "message", "message", "unknown"})
	if len(allowed) != 2 || allowed[0] != "callback_query" || allowed[1] != "message" {
		t.Fatalf("allowed updates = %#v", allowed)
	}
}

func TestChatEventResolvesTypedOutputsAndFilters(t *testing.T) {
	node := New()
	definition, err := node.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": "message"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"text", "commandText", "messageId", "chatId", "chatType", "chatTitle", "from", "fromId", "updateId"} {
		found := false
		for _, pin := range definition.Outputs {
			if pin.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing chat output %q", id)
		}
	}
	event := telespec.TelegramEvent{Type: "message", UpdateID: 77, ReceivedAt: "2026-08-20T12:00:00Z", Payload: map[string]any{
		"message": telespec.Message{Text: "/HeLp  now", MessageID: 5, ChatID: 55, ChatType: "private", ChatTitle: "Alice", FromID: 9, FromUsername: "alice", FromName: "Alice"},
	}}
	for _, pin := range definition.Outputs {
		if pin.ID == "event" {
			if err := typespec.ValidateValue(event, *pin.Type); err != nil {
				t.Fatalf("event violates its strict output contract: %v", err)
			}
		}
	}
	result, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "message", "prefix": "/help", "fromUsernames": "alice", "caseSensitivePrefix": false}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	if result.Outputs["text"] != "/HeLp  now" || result.Outputs["commandText"] != "now" {
		t.Fatalf("chat values = %#v", result.Outputs)
	}
	if result.Outputs["chatId"] != int64(55) || result.Outputs["messageId"] != int64(5) {
		t.Fatalf("numeric outputs = %#v", result.Outputs)
	}

	rejected, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "message", "prefix": "/help", "fromUsernames": "someone-else"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rejected.Ports) != 0 {
		t.Fatalf("filtered event followed ports %#v", rejected.Ports)
	}
}

func TestCallbackEventResolvesCallbackOutputs(t *testing.T) {
	node := New()
	definition, err := node.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": "callback_query"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"callbackData", "callbackQueryId", "fromId", "messageId", "chatId"} {
		found := false
		for _, pin := range definition.Outputs {
			if pin.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing callback output %q", id)
		}
	}
	event := telespec.TelegramEvent{Type: "callback_query", UpdateID: 8, Payload: map[string]any{"callbackQuery": telespec.CallbackQuery{ID: "cb9", Data: "pressed", FromID: 4, ChatID: 55, MessageID: 6}}}
	result, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "callback_query"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outputs["callbackData"] != "pressed" || result.Outputs["callbackQueryId"] != "cb9" {
		t.Fatalf("callback values = %#v", result.Outputs)
	}
}

func TestResolveRejectsUnknownEvents(t *testing.T) {
	if _, err := New().Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": "nope"}}}); err == nil {
		t.Fatal("unknown event accepted")
	}
}

func TestResolveDerivesPerEventFilterFields(t *testing.T) {
	node := New()
	cases := []struct {
		eventType string
		want      []string
		ban       []string
	}{
		{eventType: "message", want: []string{"prefix", "fromUsernames", "caseSensitivePrefix"}, ban: []string{"callbackDataPrefix"}},
		{eventType: "edited_message", want: []string{"prefix", "fromUsernames", "caseSensitivePrefix"}},
		{eventType: "channel_post", want: []string{"prefix", "fromUsernames", "caseSensitivePrefix"}},
		{eventType: "edited_channel_post", want: []string{"prefix", "fromUsernames", "caseSensitivePrefix"}},
		{eventType: "callback_query", want: []string{"callbackDataPrefix"}, ban: []string{"prefix", "fromUsernames", "caseSensitivePrefix"}},
		{eventType: "my_chat_member", want: []string{}, ban: []string{"prefix", "fromUsernames", "callbackDataPrefix"}},
		{eventType: "chat_join_request", want: []string{}, ban: []string{"prefix", "callbackDataPrefix"}},
		{eventType: "message_reaction", want: []string{}, ban: []string{"prefix", "callbackDataPrefix"}},
	}
	for _, tc := range cases {
		definition, err := node.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": tc.eventType}}})
		if err != nil {
			t.Fatal(err)
		}
		fields := map[string]bool{}
		for _, field := range definition.Fields {
			fields[field.Name] = true
		}
		if !fields["eventType"] || !fields["identityId"] {
			t.Fatalf("%s is missing its shared selector fields: %#v", tc.eventType, fields)
		}
		for _, name := range tc.want {
			if !fields[name] {
				t.Fatalf("%s is missing event filter %q: %#v", tc.eventType, name, fields)
			}
		}
		for _, name := range tc.ban {
			if fields[name] {
				t.Fatalf("%s must not expose filter %q: %#v", tc.eventType, name, fields)
			}
		}
	}
}

func TestResolveDerivesPerEventTypedOutputs(t *testing.T) {
	node := New()
	cases := map[string][]string{
		"my_chat_member":    {"chatId", "chatTitle", "userId", "oldStatus", "newStatus"},
		"chat_join_request": {"chatId", "chatTitle", "userId", "username"},
		"message_reaction":  {"chatId", "messageId", "userId", "username"},
	}
	for eventType, want := range cases {
		definition, err := node.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": eventType}}})
		if err != nil {
			t.Fatal(err)
		}
		pins := map[string]bool{}
		for _, pin := range definition.Outputs {
			pins[pin.ID] = true
		}
		for _, id := range want {
			if !pins[id] {
				t.Fatalf("%s is missing output %q: %#v", eventType, id, pins)
			}
		}
	}
}

func TestCallbackDataPrefixFilterStopsNonMatchingButtons(t *testing.T) {
	node := New()
	event := telespec.TelegramEvent{Type: "callback_query", UpdateID: 8, Payload: map[string]any{"callbackQuery": telespec.CallbackQuery{ID: "cb9", Data: "approve:42", FromID: 4, ChatID: 55, MessageID: 6}}}
	matching, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "callback_query", "callbackDataPrefix": "approve:"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(matching.Ports) != 1 || matching.Outputs["callbackData"] != "approve:42" {
		t.Fatalf("matching callback was not delivered: %#v", matching)
	}
	rejected, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "callback_query", "callbackDataPrefix": "deny:"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rejected.Ports) != 0 {
		t.Fatalf("non-matching callback followed ports %#v", rejected.Ports)
	}
}

func TestMemberUpdateExposesStatusTransitions(t *testing.T) {
	node := New()
	event := telespec.TelegramEvent{Type: "my_chat_member", UpdateID: 9, Payload: map[string]any{
		"chatMember": telespec.ChatMemberUpdated{ChatID: 55, ChatTitle: "Ops", UserID: 4, OldStatus: "member", NewStatus: "administrator"},
	}}
	result, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "my_chat_member"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outputs["oldStatus"] != "member" || result.Outputs["newStatus"] != "administrator" || result.Outputs["chatId"] != int64(55) {
		t.Fatalf("member values = %#v", result.Outputs)
	}
}

func TestExecuteRejectsMismatchedEvents(t *testing.T) {
	event := telespec.TelegramEvent{Type: "message", Payload: map[string]any{"message": telespec.Message{Text: "x"}}}
	if _, err := New().Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "callback_query"}, Inputs: map[string]any{"event": event}}, nil); err == nil {
		t.Fatal("mismatched event accepted")
	}
	if _, err := New().Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "message"}, Inputs: map[string]any{"event": "raw-string"}}, nil); err == nil {
		t.Fatal("untyped event accepted")
	}
}

func TestChatFilterMatchesAtSignAndCaseInsensitive(t *testing.T) {
	message := telespec.Message{Text: "hello", FromUsername: "Alice"}
	if !matchesMessageFilter(message, map[string]any{"fromUsernames": "@alice"}) {
		t.Fatal("@-prefixed username must match")
	}
	if matchesMessageFilter(message, map[string]any{"prefix": "!"}) {
		t.Fatal("prefix mismatch must filter out")
	}
	if !matchesMessageFilter(telespec.Message{Text: "!go"}, map[string]any{"prefix": "!GO"}) {
		t.Fatal("prefix must be case-insensitive by default")
	}
}

package event

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/twitchspec"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

func TestCatalogIsUniqueAndStrict(t *testing.T) {
	seen := map[string]bool{}
	for _, descriptor := range twitchspec.Catalog() {
		if descriptor.Type == "" || seen[descriptor.Type] {
			t.Fatalf("duplicate or empty event %q", descriptor.Type)
		}
		seen[descriptor.Type] = true
		if err := typespec.ValidateSpec(descriptor.EventType); err != nil {
			t.Fatalf("%s event contract: %v", descriptor.Type, err)
		}
	}
	if len(seen) < 70 {
		t.Fatalf("catalog has %d entries, expected complete current EventSub coverage", len(seen))
	}
}

func TestChatEventResolvesTypedOutputsAndFilters(t *testing.T) {
	node := New()
	definition, err := node.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": "channel.chat.message"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"text", "commandText", "broadcasterId", "authorId", "messageId", "author", "channel"} {
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
	event := twitchspec.Event{Type: "channel.chat.message", Version: "1", MessageID: "delivery", SubscriptionID: "sub", ReceivedAt: "2026-08-10T12:00:00Z", Payload: map[string]any{"chatMessage": twitchspec.ChatMessage{Text: "!HeLp  now", BroadcasterID: "channel", AuthorID: "author", MessageID: "message", AuthorLogin: "viewer", AuthorName: "Viewer", ChannelLogin: "channel-login"}}}
	for _, pin := range definition.Outputs {
		if pin.ID == "event" {
			if err := typespec.ValidateValue(event, *pin.Type); err != nil {
				t.Fatalf("event violates its strict output contract: %v", err)
			}
		}
	}
	result, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "channel.chat.message", "prefix": "!help", "authorIDs": "author", "caseSensitivePrefix": false}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	if result.Outputs["text"] != "!HeLp  now" || result.Outputs["commandText"] != "now" {
		t.Fatalf("chat values = %#v", result.Outputs)
	}
	rejected, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "channel.chat.message", "prefix": "!help", "authorIDs": "someone-else"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rejected.Ports) != 0 {
		t.Fatalf("filtered event followed ports %#v", rejected.Ports)
	}
}

func TestResolveAddsOnlySelectedEventConditions(t *testing.T) {
	definition, err := New().Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": "channel.raid"}}})
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]bool{}
	for _, field := range definition.Fields {
		fields[field.Name] = true
	}
	if !fields["targetChannel"] || !fields["sourceChannel"] {
		t.Fatalf("raid condition fields = %#v", fields)
	}
	if fields["channel"] {
		t.Fatal("raid unexpectedly received a generic channel field")
	}
}

func TestCatalogUsesCurrentVersionsForRevisedSubscriptions(t *testing.T) {
	for eventType, version := range map[string]string{
		"channel.follow":              "2",
		"channel.update":              "2",
		"channel.hype_train.begin":    "2",
		"channel.hype_train.progress": "2",
		"channel.hype_train.end":      "2",
	} {
		descriptor, found := twitchspec.Find(eventType)
		if !found || descriptor.Version != version {
			t.Fatalf("%s version = %#v, want %s", eventType, descriptor, version)
		}
	}
}

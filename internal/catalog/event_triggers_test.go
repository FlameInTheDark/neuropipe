package catalog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// Event-source triggers follow the Twitch convention: their type uses the
// provider prefix (discord:event, telegram:event, kv:subscribe) so the
// editor's node library — which hides the seeded trigger:* core triggers —
// lists them as addable nodes.
func TestEventSourceTriggersStayLibraryVisible(t *testing.T) {
	registry := New()
	expected := map[string]domain.TriggerKind{
		"twitch:event":   domain.TriggerTwitch,
		"discord:event":  domain.TriggerDiscord,
		"telegram:event": domain.TriggerTelegram,
		"kv:subscribe":   domain.TriggerKV,
	}
	for nodeType, kind := range expected {
		definition, found := registry.Get(nodeType)
		if !found {
			t.Fatalf("%s is not registered", nodeType)
		}
		if strings.HasPrefix(nodeType, "trigger:") {
			t.Fatalf("%s must not use the library-hidden trigger: prefix", nodeType)
		}
		if definition.TriggerKind != kind {
			t.Fatalf("%s trigger kind = %q, want %q", nodeType, definition.TriggerKind, kind)
		}
		if definition.Category == "" {
			t.Fatalf("%s needs a category so the library can group it", nodeType)
		}
	}
}

// Per-event trigger contracts must adapt to the configured event type: the
// Discord trigger exposes chat filters only for message events and its own
// filters for interactions, reactions, and member updates; the Telegram
// trigger exposes message filters only for message-like updates and a
// callback-data prefix for callback queries.
func TestEventSourceTriggerContractsAdaptPerEvent(t *testing.T) {
	registry := New()
	cases := []struct {
		nodeType  string
		eventType string
		want      []string
		ban       []string
	}{
		{nodeType: "discord:event", eventType: "message.create", want: []string{"prefix", "authorIDs", "caseSensitivePrefix"}, ban: []string{"commandName", "emoji", "userIDs"}},
		{nodeType: "discord:event", eventType: "interaction.create", want: []string{"commandName"}, ban: []string{"prefix", "authorIDs", "emoji", "userIDs"}},
		{nodeType: "discord:event", eventType: "message.reaction.add", want: []string{"emoji"}, ban: []string{"prefix", "commandName", "userIDs"}},
		{nodeType: "discord:event", eventType: "guild.member.add", want: []string{"userIDs"}, ban: []string{"prefix", "commandName", "emoji"}},
		{nodeType: "discord:event", eventType: "voice.state.update", want: []string{}, ban: []string{"prefix", "commandName", "emoji", "userIDs"}},
		{nodeType: "telegram:event", eventType: "message", want: []string{"prefix", "fromUsernames", "caseSensitivePrefix"}, ban: []string{"callbackDataPrefix"}},
		{nodeType: "telegram:event", eventType: "callback_query", want: []string{"callbackDataPrefix"}, ban: []string{"prefix", "fromUsernames"}},
		{nodeType: "telegram:event", eventType: "my_chat_member", want: []string{}, ban: []string{"prefix", "callbackDataPrefix"}},
	}
	for _, tc := range cases {
		module, found := registry.Node(tc.nodeType)
		if !found {
			t.Fatalf("%s is not registered", tc.nodeType)
		}
		definition, err := module.Resolve(domain.FlowNode{Type: tc.nodeType, Data: map[string]any{"config": map[string]any{"eventType": tc.eventType}}})
		if err != nil {
			t.Fatalf("resolve %s %s: %v", tc.nodeType, tc.eventType, err)
		}
		fields := map[string]bool{}
		for _, field := range definition.Fields {
			fields[field.Name] = true
		}
		if !fields["eventType"] || !fields["identityId"] {
			t.Fatalf("%s %s lost its shared selectors: %#v", tc.nodeType, tc.eventType, fields)
		}
		for _, name := range tc.want {
			if !fields[name] {
				t.Fatalf("%s %s is missing filter %q", tc.nodeType, tc.eventType, name)
			}
		}
		for _, name := range tc.ban {
			if fields[name] {
				t.Fatalf("%s %s must not expose filter %q", tc.nodeType, tc.eventType, name)
			}
		}
	}
}

// Object-typed trigger pins carry named record TypeSpecs whose fields describe
// the payload structure shown in the editor's pin tooltips. A pin without its
// spec renders as an opaque "Object" chip, so the envelope and author records
// must survive resolution for every event variant.
func TestEventSourceTriggerObjectPinsCarryRecordSpecs(t *testing.T) {
	registry := New()
	named := []struct {
		nodeType  string
		eventType string
		pinID     string
		record    string
		minFields int
	}{
		{nodeType: "twitch:event", eventType: "channel.chat.message", pinID: "event", record: "Event", minFields: 5},
		{nodeType: "twitch:event", eventType: "channel.chat.message", pinID: "author", record: "TwitchAuthor", minFields: 3},
		{nodeType: "discord:event", eventType: "message.create", pinID: "event", record: "DiscordEvent", minFields: 5},
		{nodeType: "discord:event", eventType: "message.create", pinID: "author", record: "DiscordAuthor", minFields: 3},
		{nodeType: "discord:event", eventType: "message.update", pinID: "author", record: "DiscordAuthor", minFields: 3},
		{nodeType: "telegram:event", eventType: "message", pinID: "event", record: "TelegramEvent", minFields: 4},
		{nodeType: "telegram:event", eventType: "message", pinID: "from", record: "TelegramFrom", minFields: 3},
		{nodeType: "telegram:event", eventType: "edited_message", pinID: "from", record: "TelegramFrom", minFields: 3},
	}
	for _, tc := range named {
		pin, err := resolvePin(registry, tc.nodeType, tc.eventType, tc.pinID)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.nodeType, tc.eventType, err)
		}
		if pin.DataType != domain.DataObject {
			t.Fatalf("%s %s pin %q dataType = %q, want object", tc.nodeType, tc.eventType, tc.pinID, pin.DataType)
		}
		if pin.Type == nil || pin.Type.Kind != domain.TypeRecord {
			t.Fatalf("%s %s pin %q needs a record TypeSpec so tooltips can show its structure", tc.nodeType, tc.eventType, tc.pinID)
		}
		if pin.Type.Name != tc.record {
			t.Fatalf("%s %s pin %q record name = %q, want %q", tc.nodeType, tc.eventType, tc.pinID, pin.Type.Name, tc.record)
		}
		if len(pin.Type.Fields) < tc.minFields {
			t.Fatalf("%s %s pin %q record %q has %d fields, want at least %d", tc.nodeType, tc.eventType, tc.pinID, tc.record, len(pin.Type.Fields), tc.minFields)
		}
	}
}

// Every object-typed output across the full event catalogs must describe its
// structure — either a record spec with fields or a map spec with key/value
// types — so no tooltip ever degrades to a bare "Object" chip.
func TestEventSourceTriggerObjectPinsNeverOpaque(t *testing.T) {
	registry := New()
	type catalogEntry struct {
		nodeType  string
		eventType string
	}
	entries := []catalogEntry{
		{nodeType: "discord:event", eventType: "message.create"},
		{nodeType: "discord:event", eventType: "message.update"},
		{nodeType: "discord:event", eventType: "message.delete"},
		{nodeType: "discord:event", eventType: "message.reaction.add"},
		{nodeType: "discord:event", eventType: "message.reaction.remove"},
		{nodeType: "discord:event", eventType: "message.reaction.remove_all"},
		{nodeType: "discord:event", eventType: "guild.member.add"},
		{nodeType: "discord:event", eventType: "guild.member.remove"},
		{nodeType: "discord:event", eventType: "guild.member.update"},
		{nodeType: "discord:event", eventType: "guild.ban.add"},
		{nodeType: "discord:event", eventType: "guild.ban.remove"},
		{nodeType: "discord:event", eventType: "interaction.create"},
		{nodeType: "discord:event", eventType: "voice.state.update"},
		{nodeType: "telegram:event", eventType: "message"},
		{nodeType: "telegram:event", eventType: "edited_message"},
		{nodeType: "telegram:event", eventType: "channel_post"},
		{nodeType: "telegram:event", eventType: "edited_channel_post"},
		{nodeType: "telegram:event", eventType: "callback_query"},
		{nodeType: "telegram:event", eventType: "my_chat_member"},
		{nodeType: "telegram:event", eventType: "chat_join_request"},
		{nodeType: "telegram:event", eventType: "message_reaction"},
	}
	for _, entry := range entries {
		module, found := registry.Node(entry.nodeType)
		if !found {
			t.Fatalf("%s is not registered", entry.nodeType)
		}
		definition, err := module.Resolve(domain.FlowNode{Type: entry.nodeType, Data: map[string]any{"config": map[string]any{"eventType": entry.eventType}}})
		if err != nil {
			t.Fatalf("resolve %s %s: %v", entry.nodeType, entry.eventType, err)
		}
		for _, pin := range definition.Outputs {
			if pin.DataType != domain.DataObject {
				continue
			}
			if pin.Type == nil {
				t.Fatalf("%s %s pin %q is object-typed without a TypeSpec", entry.nodeType, entry.eventType, pin.ID)
			}
			switch pin.Type.Kind {
			case domain.TypeRecord:
				if len(pin.Type.Fields) == 0 {
					t.Fatalf("%s %s pin %q record spec has no fields", entry.nodeType, entry.eventType, pin.ID)
				}
			case domain.TypeMap:
				if pin.Type.Key == nil || pin.Type.Value == nil {
					t.Fatalf("%s %s pin %q map spec needs key and value types", entry.nodeType, entry.eventType, pin.ID)
				}
			default:
				t.Fatalf("%s %s pin %q spec kind = %q, want record or map", entry.nodeType, entry.eventType, pin.ID, pin.Type.Kind)
			}
		}
	}
}

func resolvePin(registry *Registry, nodeType, eventType, pinID string) (domain.NodePort, error) {
	module, found := registry.Node(nodeType)
	if !found {
		return domain.NodePort{}, fmt.Errorf("%s is not registered", nodeType)
	}
	definition, err := module.Resolve(domain.FlowNode{Type: nodeType, Data: map[string]any{"config": map[string]any{"eventType": eventType}}})
	if err != nil {
		return domain.NodePort{}, err
	}
	for _, pin := range definition.Outputs {
		if pin.ID == pinID {
			return pin, nil
		}
	}
	return domain.NodePort{}, fmt.Errorf("pin %q not found on %s %s", pinID, nodeType, eventType)
}

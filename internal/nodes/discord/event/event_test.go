package event

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/discordspec"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

func TestCatalogIsUniqueAndValid(t *testing.T) {
	seen := map[string]bool{}
	for _, descriptor := range discordspec.Catalog() {
		if descriptor.Type == "" || seen[descriptor.Type] {
			t.Fatalf("duplicate or empty event %q", descriptor.Type)
		}
		seen[descriptor.Type] = true
		if descriptor.GatewayEvent == "" {
			t.Fatalf("%s has no gateway event", descriptor.Type)
		}
		if descriptor.Intents == 0 {
			t.Fatalf("%s requires no intents", descriptor.Type)
		}
	}
	if len(seen) < 13 {
		t.Fatalf("catalog has %d entries, expected the full v1 gateway coverage", len(seen))
	}
	if err := typespec.ValidateSpec(discordspec.EventType()); err != nil {
		t.Fatalf("event contract: %v", err)
	}
	if err := typespec.ValidateSpec(discordspec.AuthorType()); err != nil {
		t.Fatalf("author contract: %v", err)
	}
	if discordspec.RequiredIntents([]string{"message.create", "guild.member.add"}) != (discordspec.IntentGuildMessages | discordspec.IntentDirectMessages | discordspec.IntentMessageContent | discordspec.IntentGuildMembers) {
		t.Fatal("intent union is wrong")
	}
}

func TestChatEventResolvesTypedOutputsAndFilters(t *testing.T) {
	node := New()
	definition, err := node.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": "message.create"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"text", "commandText", "channelId", "channelName", "guildId", "guildName", "authorId", "author", "gatewayEvent"} {
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
	chat := discordspec.ChatMessage{Text: "!HeLp  now", MessageID: "m1", ChannelID: "c1", ChannelName: "general", GuildID: "g1", GuildName: "Server", AuthorID: "author", AuthorUsername: "viewer"}
	event := discordspec.DiscordEvent{Type: "message.create", GatewayEvent: "MESSAGE_CREATE", MessageID: "m1", ReceivedAt: "2026-08-20T12:00:00Z", Payload: map[string]any{"chatMessage": chat}}
	for _, pin := range definition.Outputs {
		if pin.ID == "event" {
			if err := typespec.ValidateValue(event, *pin.Type); err != nil {
				t.Fatalf("event violates its strict output contract: %v", err)
			}
		}
	}
	result, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "message.create", "prefix": "!help", "authorIDs": "author", "caseSensitivePrefix": false}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	if result.Outputs["text"] != "!HeLp  now" || result.Outputs["commandText"] != "now" {
		t.Fatalf("chat values = %#v", result.Outputs)
	}
	if result.Outputs["channelName"] != "general" || result.Outputs["guildName"] != "Server" {
		t.Fatalf("name outputs = %#v", result.Outputs)
	}

	rejected, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "message.create", "prefix": "!help", "authorIDs": "someone-else"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rejected.Ports) != 0 {
		t.Fatalf("filtered event followed ports %#v", rejected.Ports)
	}
}

func TestInteractionEventResolvesCommandOutputs(t *testing.T) {
	node := New()
	definition, err := node.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": "interaction.create"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"commandName", "options", "userId", "channelId", "guildId"} {
		found := false
		for _, pin := range definition.Outputs {
			if pin.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing interaction output %q", id)
		}
	}
	event := discordspec.DiscordEvent{Type: "interaction.create", GatewayEvent: "INTERACTION_CREATE", Payload: map[string]any{
		"interaction": discordspec.Interaction{CommandName: "greet", UserID: "u1", ChannelID: "c1", GuildID: "g1", Options: map[string]string{"who": "world"}},
	}}
	result, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "interaction.create"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outputs["commandName"] != "greet" || result.Outputs["userId"] != "u1" {
		t.Fatalf("interaction values = %#v", result.Outputs)
	}
	options, _ := result.Outputs["options"].(map[string]string)
	if options["who"] != "world" {
		t.Fatalf("options = %#v", result.Outputs["options"])
	}
}

func TestResolveAddsOnlySelectedEventConditions(t *testing.T) {
	definition, err := New().Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": "message.create"}}})
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]bool{}
	for _, field := range definition.Fields {
		fields[field.Name] = true
	}
	if !fields["guildId"] || !fields["channelId"] {
		t.Fatalf("message condition fields = %#v", fields)
	}
	definition, err = New().Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": "guild.member.add"}}})
	if err != nil {
		t.Fatal(err)
	}
	fields = map[string]bool{}
	for _, field := range definition.Fields {
		fields[field.Name] = true
	}
	if !fields["guildId"] || fields["channelId"] {
		t.Fatalf("member condition fields = %#v", fields)
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
		{eventType: "message.create", want: []string{"prefix", "authorIDs", "caseSensitivePrefix"}},
		{eventType: "message.update", want: []string{"prefix", "authorIDs", "caseSensitivePrefix"}},
		{eventType: "message.delete", want: []string{}, ban: []string{"prefix", "authorIDs", "caseSensitivePrefix", "commandName", "emoji", "userIDs"}},
		{eventType: "interaction.create", want: []string{"commandName"}, ban: []string{"prefix", "authorIDs", "emoji", "userIDs"}},
		{eventType: "message.reaction.add", want: []string{"emoji"}, ban: []string{"prefix", "authorIDs", "commandName", "userIDs"}},
		{eventType: "message.reaction.remove", want: []string{"emoji"}},
		{eventType: "message.reaction.remove_all", want: []string{}, ban: []string{"emoji", "prefix"}},
		{eventType: "guild.member.add", want: []string{"userIDs"}, ban: []string{"prefix", "authorIDs", "commandName", "emoji"}},
		{eventType: "guild.member.update", want: []string{"userIDs"}},
		{eventType: "guild.ban.add", want: []string{}, ban: []string{"prefix", "userIDs", "emoji", "commandName"}},
		{eventType: "voice.state.update", want: []string{}, ban: []string{"prefix", "userIDs", "emoji", "commandName"}},
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
		"message.reaction.add":    {"emoji", "userId", "channelId", "guildId"},
		"message.reaction.remove": {"emoji", "userId"},
		"guild.member.add":        {"userId", "username", "nickname", "guildId", "joinedAt"},
		"guild.member.remove":     {"userId", "username", "guildId"},
		"guild.ban.add":           {"userId", "username", "guildId"},
		"guild.ban.remove":        {"userId", "username"},
		"voice.state.update":      {"userId", "channelId", "guildId", "sessionId"},
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

func TestInteractionCommandFilterStopsNonMatchingCommands(t *testing.T) {
	node := New()
	event := discordspec.DiscordEvent{Type: "interaction.create", GatewayEvent: "INTERACTION_CREATE", Payload: map[string]any{
		"interaction": discordspec.Interaction{CommandName: "greet", UserID: "u1", Options: map[string]string{}},
	}}
	matching, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "interaction.create", "commandName": "greet"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(matching.Ports) != 1 || matching.Outputs["commandName"] != "greet" {
		t.Fatalf("matching command was not delivered: %#v", matching)
	}
	rejected, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "interaction.create", "commandName": "help"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rejected.Ports) != 0 {
		t.Fatalf("non-matching command followed ports %#v", rejected.Ports)
	}
}

func TestReactionEmojiFilterStopsNonMatchingReactions(t *testing.T) {
	node := New()
	event := discordspec.DiscordEvent{Type: "message.reaction.add", GatewayEvent: "MESSAGE_REACTION_ADD", Payload: map[string]any{
		"reaction": discordspec.Reaction{MessageID: "m1", ChannelID: "c1", GuildID: "g1", UserID: "u1", Emoji: "thumbsup:1234"},
	}}
	matching, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "message.reaction.add", "emoji": "thumbsup:1234"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(matching.Ports) != 1 || matching.Outputs["emoji"] != "thumbsup:1234" || matching.Outputs["userId"] != "u1" {
		t.Fatalf("matching reaction was not delivered: %#v", matching)
	}
	rejected, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "message.reaction.add", "emoji": "👍"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rejected.Ports) != 0 {
		t.Fatalf("non-matching emoji followed ports %#v", rejected.Ports)
	}
}

func TestMemberUserFilterStopsNonListedUsers(t *testing.T) {
	node := New()
	event := discordspec.DiscordEvent{Type: "guild.member.add", GatewayEvent: "GUILD_MEMBER_ADD", Payload: map[string]any{
		"member": discordspec.Member{UserID: "u9", Username: "newbie", Nickname: "new", GuildID: "g1", JoinedAt: "2026-08-20T12:00:00Z"},
	}}
	matching, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "guild.member.add", "userIDs": "u8, u9"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(matching.Ports) != 1 || matching.Outputs["userId"] != "u9" || matching.Outputs["nickname"] != "new" {
		t.Fatalf("listed member was not delivered: %#v", matching)
	}
	rejected, err := node.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "guild.member.add", "userIDs": "u1"}, Inputs: map[string]any{"event": event}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rejected.Ports) != 0 {
		t.Fatalf("unlisted member followed ports %#v", rejected.Ports)
	}
}

func TestExecuteRejectsMismatchedEvents(t *testing.T) {
	event := discordspec.DiscordEvent{Type: "message.create", Payload: map[string]any{"chatMessage": discordspec.ChatMessage{Text: "x"}}}
	if _, err := New().Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "message.delete"}, Inputs: map[string]any{"event": event}}, nil); err == nil {
		t.Fatal("mismatched event accepted")
	}
	if _, err := New().Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "message.create"}, Inputs: map[string]any{"event": "raw"}}, nil); err == nil {
		t.Fatal("untyped event accepted")
	}
}

package dc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ sender nodes.DiscordSender }

func (r runtimeStub) DiscordSender() nodes.DiscordSender { return r.sender }

type senderStub struct{ connected bool }

func (s *senderStub) SendDiscordMessage(context.Context, domain.DiscordMessageRequest) (domain.DiscordMessageResult, error) {
	panic("unused")
}
func (s *senderStub) SendDiscordDirectMessage(context.Context, domain.DiscordDMRequest) (domain.DiscordMessageResult, error) {
	panic("unused")
}
func (s *senderStub) AddDiscordReaction(context.Context, domain.DiscordReactionRequest) (domain.DiscordActionResult, error) {
	panic("unused")
}
func (s *senderStub) EditDiscordMessage(context.Context, domain.DiscordEditRequest) (domain.DiscordActionResult, error) {
	panic("unused")
}
func (s *senderStub) DeleteDiscordMessage(context.Context, domain.DiscordDeleteRequest) (domain.DiscordActionResult, error) {
	panic("unused")
}
func (s *senderStub) RespondDiscordInteraction(context.Context, domain.DiscordCommandReplyRequest) (domain.DiscordMessageResult, error) {
	panic("unused")
}
func (s *senderStub) SendDiscordFollowup(context.Context, domain.DiscordFollowupRequest) (domain.DiscordMessageResult, error) {
	panic("unused")
}
func (s *senderStub) EditDiscordInteractionMessage(context.Context, domain.DiscordCommandEditRequest) (domain.DiscordActionResult, error) {
	panic("unused")
}

func TestSenderRequiresProvider(t *testing.T) {
	if _, err := Sender(nil); err == nil {
		t.Fatal("nil runtime must fail")
	}
	if _, err := Sender(runtimeStub{}); err == nil {
		t.Fatal("nil sender must fail")
	}
	sender := &senderStub{}
	resolved, err := Sender(runtimeStub{sender: sender})
	if err != nil || resolved != nodes.DiscordSender(sender) {
		t.Fatalf("sender = %v, err = %v", resolved, err)
	}
}

func TestStringHelpers(t *testing.T) {
	invocation := nodes.Invocation{Inputs: map[string]any{"channel": " 55 ", "silent": true}}
	if value := String(invocation, "channel"); value != "55" {
		t.Fatalf("string = %q", value)
	}
	if !BoolValue(invocation, "silent") {
		t.Fatal("bool = false")
	}
	if _, err := RequiredString(invocation, "missing", "channel ID"); err == nil || err.Error() != "channel ID is required" {
		t.Fatalf("required error = %v", err)
	}
	if value, err := RequiredString(invocation, "channel", "channel ID"); err != nil || value != "55" {
		t.Fatalf("required = %q, %v", value, err)
	}
}

func TestStringCoercesNumericValues(t *testing.T) {
	invocation := nodes.Invocation{Inputs: map[string]any{
		"jsonNumber": json.Number("79216925611139072"),
		"int64":      int64(42),
		"int":        7,
		"float":      float64(100000000000000000),
		"fraction":   1.5,
	}}
	if value := String(invocation, "jsonNumber"); value != "79216925611139072" {
		t.Fatalf("json.Number = %q", value)
	}
	if value := String(invocation, "int64"); value != "42" {
		t.Fatalf("int64 = %q", value)
	}
	if value := String(invocation, "int"); value != "7" {
		t.Fatalf("int = %q", value)
	}
	if value := String(invocation, "float"); value != "100000000000000000" {
		t.Fatalf("float = %q (must never be scientific notation)", value)
	}
	if value := String(invocation, "fraction"); value != "1.5" {
		t.Fatalf("fraction = %q", value)
	}
	if value := String(invocation, "absent"); value != "" {
		t.Fatalf("absent = %q", value)
	}
}

func TestDefinitionSkeletonIncludesIdentityField(t *testing.T) {
	definition := Definition("action:discord_test", "Test", "desc.", "send", nil, nil, nil, nil)
	if definition.Type != "action:discord_test" || definition.Category != "Discord" || definition.Mode != domain.NodeImpure {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Capabilities) != 1 || definition.Capabilities[0] != domain.CapabilityNetwork {
		t.Fatalf("capabilities = %#v", definition.Capabilities)
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Kind != "discord-identity" {
		t.Fatalf("fields = %#v", definition.Fields)
	}
	if definition.DefaultConfig["identityId"] != "" {
		t.Fatalf("defaults = %#v", definition.DefaultConfig)
	}
}

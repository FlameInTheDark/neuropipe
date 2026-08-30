package replycommand

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/discord/dc"
)

type runtimeStub struct{ sender nodes.DiscordSender }

func (r runtimeStub) DiscordSender() nodes.DiscordSender { return r.sender }

type senderStub struct {
	reply  domain.DiscordCommandReplyRequest
	result domain.DiscordMessageResult
}

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
func (s *senderStub) RespondDiscordInteraction(_ context.Context, request domain.DiscordCommandReplyRequest) (domain.DiscordMessageResult, error) {
	s.reply = request
	return s.result, nil
}
func (s *senderStub) SendDiscordFollowup(context.Context, domain.DiscordFollowupRequest) (domain.DiscordMessageResult, error) {
	panic("unused")
}
func (s *senderStub) EditDiscordInteractionMessage(context.Context, domain.DiscordCommandEditRequest) (domain.DiscordActionResult, error) {
	panic("unused")
}

var reference = domain.DiscordInteractionRef{InteractionID: "111", ApplicationID: "222", Token: "token", CommandName: "weather", Deferred: true}

func TestReplySendsRequestAndPassesDeferredFlag(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "m-1", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{},
		Inputs: map[string]any{"interaction": reference, "message": "It is sunny"},
	}, runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "sent" || result.Outputs["messageId"] != "m-1" {
		t.Fatalf("result = %#v", result)
	}
	if sender.reply.Message != "It is sunny" || sender.reply.Interaction.Token != "token" || !sender.reply.Interaction.Deferred {
		t.Fatalf("request = %#v", sender.reply)
	}
}

func TestReplyRejectedReasonFlows(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{Reason: "Unknown interaction"}}
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{},
		Inputs: map[string]any{"interaction": reference, "message": "hello"},
	}, runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "rejected" || result.Outputs["reason"] != "Unknown interaction" {
		t.Fatalf("result = %#v", result)
	}
}

func TestReplyEmptyBodyRejected(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{},
		Inputs: map[string]any{"interaction": reference},
	}, runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "rejected" || result.Outputs["reason"] == "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestReplyWithoutInteractionIsHardError(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	if _, err := module.Execute(context.Background(), nodes.Invocation{Config: map[string]any{}, Inputs: map[string]any{"message": "x"}}, runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("missing interaction should be a hard error")
	}
}

func TestReplyAcceptsMapShapedInteraction(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	shaped := map[string]any{
		"interactionId": "111", "applicationId": "222", "token": "token", "deferred": true,
	}
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{},
		Inputs: map[string]any{"interaction": shaped, "message": "hi"},
	}, runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "sent" {
		t.Fatalf("result = %#v", result)
	}
	if sender.reply.Interaction.InteractionID != "111" || !sender.reply.Interaction.Deferred {
		t.Fatalf("request = %#v", sender.reply)
	}
}

func TestResolveAddsEmbedTemplatePins(t *testing.T) {
	node := New()
	definition, err := node.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{
		"embeds": map[string]any{
			"embeds": []any{map[string]any{"title": "Weather in {{city}}"}},
			"pins":   []any{map[string]any{"name": "city", "type": "text"}},
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, port := range definition.Inputs {
		if port.ID == "city" {
			found = true
		}
	}
	if !found {
		t.Fatalf("embed template pin missing: %#v", definition.Inputs)
	}
}

var _ = dc.String

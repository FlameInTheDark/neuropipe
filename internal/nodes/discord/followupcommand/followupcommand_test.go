package followupcommand

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ sender nodes.DiscordSender }

func (r runtimeStub) DiscordSender() nodes.DiscordSender { return r.sender }

type senderStub struct {
	followup domain.DiscordFollowupRequest
	result   domain.DiscordMessageResult
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
func (s *senderStub) RespondDiscordInteraction(context.Context, domain.DiscordCommandReplyRequest) (domain.DiscordMessageResult, error) {
	panic("unused")
}
func (s *senderStub) SendDiscordFollowup(_ context.Context, request domain.DiscordFollowupRequest) (domain.DiscordMessageResult, error) {
	s.followup = request
	return s.result, nil
}
func (s *senderStub) EditDiscordInteractionMessage(context.Context, domain.DiscordCommandEditRequest) (domain.DiscordActionResult, error) {
	panic("unused")
}

var reference = domain.DiscordInteractionRef{InteractionID: "111", ApplicationID: "222", Token: "token", CommandName: "weather", Deferred: true}

func TestFollowupSendsRequest(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "m-9", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{},
		Inputs: map[string]any{"interaction": reference, "message": "Extra info"},
	}, runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "sent" || result.Outputs["messageId"] != "m-9" {
		t.Fatalf("result = %#v", result)
	}
	if sender.followup.Message != "Extra info" || sender.followup.Interaction.Token != "token" {
		t.Fatalf("request = %#v", sender.followup)
	}
}

func TestFollowupWithoutInteractionIsHardError(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	if _, err := module.Execute(context.Background(), nodes.Invocation{Config: map[string]any{}, Inputs: map[string]any{"message": "x"}}, runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("missing interaction should be a hard error")
	}
}

func TestFollowupEmptyBodyRejected(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{},
		Inputs: map[string]any{"interaction": reference},
	}, runtimeStub{sender: &senderStub{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "rejected" || result.Outputs["reason"] == "" {
		t.Fatalf("result = %#v", result)
	}
}

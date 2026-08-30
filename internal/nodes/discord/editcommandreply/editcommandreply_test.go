package editcommandreply

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ sender nodes.DiscordSender }

func (r runtimeStub) DiscordSender() nodes.DiscordSender { return r.sender }

type senderStub struct {
	edit   domain.DiscordCommandEditRequest
	result domain.DiscordActionResult
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
func (s *senderStub) SendDiscordFollowup(context.Context, domain.DiscordFollowupRequest) (domain.DiscordMessageResult, error) {
	panic("unused")
}
func (s *senderStub) EditDiscordInteractionMessage(_ context.Context, request domain.DiscordCommandEditRequest) (domain.DiscordActionResult, error) {
	s.edit = request
	return s.result, nil
}

var reference = domain.DiscordInteractionRef{InteractionID: "111", ApplicationID: "222", Token: "token", CommandName: "weather", Deferred: true}

func TestEditDefaultsToOriginal(t *testing.T) {
	sender := &senderStub{result: domain.DiscordActionResult{Done: true}}
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{},
		Inputs: map[string]any{"interaction": reference, "message": "Updated"},
	}, runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "done" {
		t.Fatalf("result = %#v", result)
	}
	if sender.edit.MessageID != "" || sender.edit.Message != "Updated" {
		t.Fatalf("request = %#v", sender.edit)
	}
}

func TestEditTargetsFollowupByID(t *testing.T) {
	sender := &senderStub{result: domain.DiscordActionResult{Done: true}}
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	_, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{},
		Inputs: map[string]any{"interaction": reference, "message": "Updated", "messageId": "9999"},
	}, runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if sender.edit.MessageID != "9999" {
		t.Fatalf("request = %#v", sender.edit)
	}
}

func TestEditWithoutInteractionIsHardError(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	if _, err := module.Execute(context.Background(), nodes.Invocation{Config: map[string]any{}, Inputs: map[string]any{"message": "x"}}, runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("missing interaction should be a hard error")
	}
}

func TestEditRejectedReasonFlows(t *testing.T) {
	sender := &senderStub{result: domain.DiscordActionResult{Reason: "Unknown message"}}
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{},
		Inputs: map[string]any{"interaction": reference, "message": "Updated"},
	}, runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "rejected" || result.Outputs["reason"] != "Unknown message" {
		t.Fatalf("result = %#v", result)
	}
}

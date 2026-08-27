package sendmessage

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ sender nodes.DiscordSender }

func (r runtimeStub) DiscordSender() nodes.DiscordSender { return r.sender }

type senderStub struct {
	request domain.DiscordMessageRequest
	result  domain.DiscordMessageResult
}

func (s *senderStub) SendDiscordMessage(_ context.Context, request domain.DiscordMessageRequest) (domain.DiscordMessageResult, error) {
	s.request = request
	return s.result, nil
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

func invocation(inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:discord_send_message", Data: map[string]any{"config": map[string]any{}}},
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestSendsMessageAndFollowsSentPort(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-9", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{
		"channel": "55", "message": "hello", "replyToMessageId": "77", "identityId": "bot-1",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "sent" || result.Outputs["messageId"] != "msg-9" {
		t.Fatalf("result = %#v", result)
	}
	if sender.request.ChannelID != "55" || sender.request.Message != "hello" || sender.request.ReplyToID != "77" || sender.request.IdentityID != "bot-1" {
		t.Fatalf("request = %#v", sender.request)
	}
}

func TestSoftRejectionFollowsRejectedPort(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{Reason: "Missing Permissions"}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{"channel": "55", "message": "hello"}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "rejected" || result.Outputs["reason"] != "Missing Permissions" {
		t.Fatalf("result = %#v", result)
	}
}

// Message and channel IDs frequently arrive as numbers (numeric constants,
// JSON extraction); they must be coerced to strings instead of silently
// dropping the value — a dropped reply reference would change the message
// from a reply into a plain send.
func TestNumericIDInputsAreCoerced(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-1", Sent: true}}
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	_, err := module.Execute(context.Background(), invocation(map[string]any{
		"channel": json.Number("565062979255795712"), "message": "hello",
		"replyToMessageId": json.Number("79216925611139072"),
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if sender.request.ChannelID != "565062979255795712" || sender.request.ReplyToID != "79216925611139072" {
		t.Fatalf("json.Number request = %#v", sender.request)
	}

	sender = &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-2", Sent: true}}
	_, err = module.Execute(context.Background(), invocation(map[string]any{
		"channel": float64(55), "message": "hello", "replyToMessageId": float64(100000000000000000),
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	// 1e17 is exactly representable in float64; the coercion must render it
	// as a plain digit string, never scientific notation.
	if sender.request.ChannelID != "55" || sender.request.ReplyToID != "100000000000000000" {
		t.Fatalf("float64 request = %#v", sender.request)
	}
}

func TestLengthCapPreValidated(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	result, err := module.Execute(context.Background(), invocation(map[string]any{"channel": "55", "message": strings.Repeat("a", 2001)}), runtimeStub{sender: &senderStub{result: domain.DiscordMessageResult{Sent: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "rejected" || !strings.Contains(result.Outputs["reason"].(string), "2,000") {
		t.Fatalf("cap result = %#v", result)
	}
}

func TestValidationErrors(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Executor: execute}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{"message": "x"}), runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("missing channel accepted")
	}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{"channel": "1"}), runtimeStub{sender: &senderStub{}}); err == nil {
		t.Fatal("missing message accepted")
	}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{"channel": "1", "message": "x"}), nil); err == nil {
		t.Fatal("missing runtime accepted")
	}
}

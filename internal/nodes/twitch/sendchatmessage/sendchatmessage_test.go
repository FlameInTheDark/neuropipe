package sendchatmessage

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type sender struct {
	request domain.TwitchChatMessageRequest
}

func (s *sender) SendTwitchChatMessage(_ context.Context, request domain.TwitchChatMessageRequest) (domain.TwitchChatMessageResult, error) {
	s.request = request
	return domain.TwitchChatMessageResult{Sent: true, MessageID: "sent-id"}, nil
}

type runtime struct{ sender *sender }

func (r runtime) TwitchChatSender() nodes.TwitchChatSender { return r.sender }

func TestSendChatMessageUsesNarrowRuntimePort(t *testing.T) {
	s := &sender{}
	result, err := New().Execute(context.Background(), nodes.Invocation{Inputs: map[string]any{"message": "hello", "channel": "channel", "replyParentMessageId": "incoming", "identityId": "bot"}}, runtime{s})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "sent" || s.request.Channel != "channel" || s.request.ReplyParentID != "incoming" || s.request.IdentityID != "bot" {
		t.Fatalf("result=%#v request=%#v", result, s.request)
	}
}
func TestSendChatMessageRejectsLongMessageBeforeDelivery(t *testing.T) {
	s := &sender{}
	result, err := New().Execute(context.Background(), nodes.Invocation{Inputs: map[string]any{"message": strings.Repeat("x", 501), "channel": "channel"}}, runtime{s})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "rejected" || s.request.Message != "" {
		t.Fatalf("result=%#v request=%#v", result, s.request)
	}
}

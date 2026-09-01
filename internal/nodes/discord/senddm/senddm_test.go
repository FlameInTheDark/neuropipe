package senddm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ sender nodes.DiscordSender }

func (r runtimeStub) DiscordSender() nodes.DiscordSender { return r.sender }

type senderStub struct {
	request domain.DiscordDMRequest
	result  domain.DiscordMessageResult
	err     error
}

func (s *senderStub) SendDiscordDirectMessage(_ context.Context, request domain.DiscordDMRequest) (domain.DiscordMessageResult, error) {
	s.request = request
	return s.result, s.err
}
func (s *senderStub) SendDiscordMessage(context.Context, domain.DiscordMessageRequest) (domain.DiscordMessageResult, error) {
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

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("register: %v", err)
	}
	module, ok := registry.Get("action:discord_send_dm")
	if !ok {
		t.Fatal("action:discord_send_dm was not registered")
	}
	return module
}

func invocation(definition domain.NodeDefinition, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:discord_send_dm", Data: map[string]any{"config": map[string]any{}}},
		Definition:      definition,
		SchemaVersion:   3,
		Config:          map[string]any{},
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func assertPinIDs(t *testing.T, ports []domain.NodePort, want []string) {
	t.Helper()
	got := make([]string, 0, len(ports))
	for _, port := range ports {
		got = append(got, port.ID)
	}
	if len(got) != len(want) {
		t.Fatalf("pin ids = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("pin ids = %v, want %v", got, want)
		}
	}
}

func TestRegistrationMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "action:discord_send_dm" || definition.Mode != domain.NodeImpure || definition.Category != "Discord" {
		t.Fatalf("definition = %#v", definition)
	}
	assertPinIDs(t, definition.Inputs, []string{"in", "message", "userId", "identityId"})
	assertPinIDs(t, definition.Outputs, []string{"sent", "rejected", "messageId", "reason"})
}

func TestSendsDMAndFollowsSentPort(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{MessageID: "msg-9", Sent: true}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"userId": "42", "message": "psst", "identityId": "bot-1",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "sent" || result.Outputs["messageId"] != "msg-9" {
		t.Fatalf("result = %#v", result)
	}
	if sender.request.UserID != "42" || sender.request.Message != "psst" || sender.request.IdentityID != "bot-1" {
		t.Fatalf("request = %#v", sender.request)
	}
}

func TestSoftRejectionFollowsRejectedPort(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{Reason: "Cannot send messages to this user"}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"userId": "42", "message": "psst",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "rejected" || result.Outputs["reason"] != "Cannot send messages to this user" {
		t.Fatalf("result = %#v", result)
	}
}

// The 2,000-character cap is pre-validated so oversized DMs are rejected
// without burning an API call.
func TestLengthCapPreValidated(t *testing.T) {
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"userId": "42", "message": strings.Repeat("a", 2001),
	}), runtimeStub{sender: &senderStub{result: domain.DiscordMessageResult{Sent: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "rejected" || !strings.Contains(result.Outputs["reason"].(string), "2,000") {
		t.Fatalf("result = %#v", result)
	}
}

func TestSenderErrorPropagates(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"userId": "42", "message": "psst",
	}), runtimeStub{sender: &senderStub{err: errors.New("gateway down")}})
	if err == nil || !strings.Contains(err.Error(), "gateway down") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidationErrors(t *testing.T) {
	module := registeredModule(t)
	definition := module.Definition()
	cases := []struct {
		name    string
		inputs  map[string]any
		runtime nodes.Runtime
		want    string
	}{
		{"missing user id", map[string]any{"message": "psst"}, runtimeStub{sender: &senderStub{}}, "user ID is required"},
		{"empty message", map[string]any{"userId": "42"}, runtimeStub{sender: &senderStub{}}, "message is required"},
		{"nil runtime", map[string]any{"userId": "42", "message": "psst"}, nil, "discord delivery is unavailable"},
		{"runtime without provider", map[string]any{"userId": "42", "message": "psst"}, struct{}{}, "discord delivery is unavailable"},
		{"provider with nil sender", map[string]any{"userId": "42", "message": "psst"}, runtimeStub{}, "discord delivery is unavailable"},
	}
	for _, testCase := range cases {
		_, err := module.Execute(context.Background(), invocation(definition, testCase.inputs), testCase.runtime)
		if err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("%s: err = %v, want %q", testCase.name, err, testCase.want)
		}
	}
}

// User IDs frequently arrive as numbers (numeric constants, JSON extraction);
// they must be coerced to digit strings instead of being silently dropped.
func TestNumericUserIDInputIsCoerced(t *testing.T) {
	sender := &senderStub{result: domain.DiscordMessageResult{Sent: true}}
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"userId": json.Number("565062979255795712"), "message": "psst",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if sender.request.UserID != "565062979255795712" {
		t.Fatalf("request = %#v", sender.request)
	}
}

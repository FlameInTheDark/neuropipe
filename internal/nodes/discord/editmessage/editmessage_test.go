package editmessage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ sender nodes.DiscordSender }

func (r runtimeStub) DiscordSender() nodes.DiscordSender { return r.sender }

type senderStub struct {
	request domain.DiscordEditRequest
	result  domain.DiscordActionResult
	err     error
}

func (s *senderStub) EditDiscordMessage(_ context.Context, request domain.DiscordEditRequest) (domain.DiscordActionResult, error) {
	s.request = request
	return s.result, s.err
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
	module, ok := registry.Get("action:discord_edit_message")
	if !ok {
		t.Fatal("action:discord_edit_message was not registered")
	}
	return module
}

func invocation(definition domain.NodeDefinition, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:discord_edit_message", Data: map[string]any{"config": map[string]any{}}},
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
	if definition.Type != "action:discord_edit_message" || definition.Mode != domain.NodeImpure || definition.Category != "Discord" {
		t.Fatalf("definition = %#v", definition)
	}
	assertPinIDs(t, definition.Inputs, []string{"in", "channel", "messageId", "message", "identityId"})
	assertPinIDs(t, definition.Outputs, []string{"done", "rejected", "reason"})
}

func TestEditsMessageAndFollowsDonePort(t *testing.T) {
	sender := &senderStub{result: domain.DiscordActionResult{Done: true}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"channel": "55", "messageId": "77", "message": "edited body", "identityId": "bot-1",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "done" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	if sender.request.ChannelID != "55" || sender.request.MessageID != "77" || sender.request.Message != "edited body" || sender.request.IdentityID != "bot-1" {
		t.Fatalf("request = %#v", sender.request)
	}
}

func TestSoftRejectionFollowsRejectedPort(t *testing.T) {
	sender := &senderStub{result: domain.DiscordActionResult{Reason: "Missing Access"}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"channel": "55", "messageId": "77", "message": "edited body",
	}), runtimeStub{sender: sender})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "rejected" || result.Outputs["reason"] != "Missing Access" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSenderErrorPropagates(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"channel": "55", "messageId": "77", "message": "edited body",
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
		{"missing channel", map[string]any{"messageId": "77", "message": "x"}, runtimeStub{sender: &senderStub{}}, "channel ID is required"},
		{"missing message id", map[string]any{"channel": "55", "message": "x"}, runtimeStub{sender: &senderStub{}}, "message ID is required"},
		{"empty message", map[string]any{"channel": "55", "messageId": "77"}, runtimeStub{sender: &senderStub{}}, "message is required"},
		{"nil runtime", map[string]any{"channel": "55", "messageId": "77", "message": "x"}, nil, "discord delivery is unavailable"},
		{"runtime without provider", map[string]any{"channel": "55", "messageId": "77", "message": "x"}, struct{}{}, "discord delivery is unavailable"},
		{"provider with nil sender", map[string]any{"channel": "55", "messageId": "77", "message": "x"}, runtimeStub{}, "discord delivery is unavailable"},
	}
	for _, testCase := range cases {
		_, err := module.Execute(context.Background(), invocation(definition, testCase.inputs), testCase.runtime)
		if err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("%s: err = %v, want %q", testCase.name, err, testCase.want)
		}
	}
}

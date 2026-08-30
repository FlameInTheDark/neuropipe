package appcommand

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/discordspec"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func testEvent(command discordspec.CommandEvent) discordspec.DiscordEvent {
	return discordspec.DiscordEvent{
		Type: "application.command", GatewayEvent: "INTERACTION_CREATE", MessageID: command.Command.InteractionID,
		Payload: map[string]any{"command": command},
	}
}

func validCommandEvent() discordspec.CommandEvent {
	return discordspec.CommandEvent{
		Command: domain.DiscordInteractionRef{
			InteractionID: "111", ApplicationID: "222", Token: "token", CommandName: "weather", CommandID: "333",
		},
		CommandID: "333", CommandName: "weather", CommandType: 1,
		Options: map[string]string{"city": "berlin", "days": "3", "metric": "true"},
		UserID:  "42", Username: "tester", ChannelID: "55", GuildID: "66",
	}
}

func TestResolveGrowsOptionPins(t *testing.T) {
	node := New()
	definition, err := node.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{
		"eventType": "application.command",
		"command": map[string]any{
			"commandName": "weather", "commandId": "333",
			"options": []any{
				map[string]any{"name": "city", "type": 3, "required": true},
				map[string]any{"name": "days", "type": 4, "required": false},
				map[string]any{"name": "metric", "type": 5, "required": false},
			},
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]domain.NodePort{}
	for _, port := range definition.Outputs {
		byID[port.ID] = port
	}
	for _, id := range []string{"commandName", "commandId", "options", "userId", "username", "channelId", "guildId", "interaction", "command", "event"} {
		if _, found := byID[id]; !found {
			t.Fatalf("missing output pin %q", id)
		}
	}
	if byID["city"].DataType != domain.DataText {
		t.Fatalf("city pin = %v, want text", byID["city"].DataType)
	}
	if byID["days"].DataType != domain.DataNumber {
		t.Fatalf("days pin = %v, want number", byID["days"].DataType)
	}
	if byID["metric"].DataType != domain.DataBoolean {
		t.Fatalf("metric pin = %v, want boolean", byID["metric"].DataType)
	}
	if !byID["city"].Required || byID["days"].Required {
		t.Fatalf("required flags wrong: city=%v days=%v", byID["city"].Required, byID["days"].Required)
	}
	if byID["days"].Type == nil || byID["days"].Type.Kind != domain.TypeInt {
		t.Fatalf("days type spec = %#v", byID["days"].Type)
	}
	if byID["metric"].Type == nil || byID["metric"].Type.Kind != domain.TypeBool {
		t.Fatalf("metric type spec = %#v", byID["metric"].Type)
	}
}

func TestResolveWithoutSelectionKeepsEnvelope(t *testing.T) {
	node := New()
	definition, err := node.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"eventType": "application.command"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Outputs) != 16 {
		t.Fatalf("outputs = %d, want the 16 envelope pins", len(definition.Outputs))
	}
}

func TestExecuteFiltersWrongCommand(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	config := map[string]any{
		"eventType": "application.command",
		"command":   map[string]any{"commandName": "weather", "commandId": "333", "options": []any{map[string]any{"name": "city", "type": 3}}},
	}
	event := validCommandEvent()
	event.CommandName = "other"
	event.Command.CommandName = "other"
	result, err := module.Execute(context.Background(), nodes.Invocation{Config: config, Inputs: map[string]any{"event": testEvent(event)}}, runtimeStub{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 0 {
		t.Fatalf("wrong command should not start the flow: %#v", result)
	}
}

func TestExecuteExposesTypedOptions(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	config := map[string]any{
		"eventType": "application.command",
		"command": map[string]any{
			"commandName": "weather", "commandId": "333",
			"options": []any{
				map[string]any{"name": "city", "type": 3},
				map[string]any{"name": "days", "type": 4},
				map[string]any{"name": "metric", "type": 5},
			},
		},
	}
	result, err := module.Execute(context.Background(), nodes.Invocation{Config: config, Inputs: map[string]any{"event": testEvent(validCommandEvent())}}, runtimeStub{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ports[0] != "out" {
		t.Fatalf("ports = %v", result.Ports)
	}
	if result.Outputs["city"] != "berlin" {
		t.Fatalf("city = %#v", result.Outputs["city"])
	}
	if days, ok := result.Outputs["days"].(int); !ok || days != 3 {
		t.Fatalf("days = %#v, want int 3", result.Outputs["days"])
	}
	if metric, ok := result.Outputs["metric"].(bool); !ok || !metric {
		t.Fatalf("metric = %#v, want true", result.Outputs["metric"])
	}
	ref, ok := result.Outputs["interaction"].(domain.DiscordInteractionRef)
	if !ok || ref.Token != "token" || ref.InteractionID != "111" || ref.ApplicationID != "222" {
		t.Fatalf("interaction ref = %#v", result.Outputs["interaction"])
	}
	if result.Outputs["commandName"] != "weather" || result.Outputs["userId"] != "42" {
		t.Fatalf("envelope outputs = %#v", result.Outputs)
	}
}

func TestExecuteRejectsForeignEvent(t *testing.T) {
	module := nodes.Implementation{Metadata: definition(), Resolver: resolve, Executor: execute}
	event := discordspec.DiscordEvent{Type: "message.create", Payload: map[string]any{}}
	if _, err := module.Execute(context.Background(), nodes.Invocation{Config: map[string]any{"eventType": "application.command"}, Inputs: map[string]any{"event": event}}, runtimeStub{}); err == nil {
		t.Fatal("foreign event should be a hard error")
	}
}

func TestTypedOptionValueLenientOnGarbage(t *testing.T) {
	if typedOptionValue(4, "not-a-number") != "not-a-number" {
		t.Fatal("garbage numbers keep their text")
	}
	if typedOptionValue(5, "false") != false {
		t.Fatal("false parses")
	}
	if typedOptionValue(10, "2.5") != 2.5 {
		t.Fatal("floats parse")
	}
}

type runtimeStub struct{}

func (runtimeStub) DiscordSender() nodes.DiscordSender { return nil }

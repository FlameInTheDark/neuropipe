package discord

import (
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestValidateCommand(t *testing.T) {
	cases := []struct {
		name    string
		command domain.DiscordApplicationCommand
		want    string
	}{
		{"valid slash", domain.DiscordApplicationCommand{Name: "weather", Description: "Get the weather", Type: 1}, ""},
		{"valid user command", domain.DiscordApplicationCommand{Name: "Ask AI", Type: 2}, ""},
		{"valid message command", domain.DiscordApplicationCommand{Name: "Summarize", Type: 3}, ""},
		{
			"name too long",
			domain.DiscordApplicationCommand{Name: strings.Repeat("a", 33), Description: "d", Type: 1},
			"command name must be 1-32 characters",
		},
		{
			"uppercase slash name",
			domain.DiscordApplicationCommand{Name: "Weather", Description: "d", Type: 1},
			"must be lowercase",
		},
		{
			"space in slash name",
			domain.DiscordApplicationCommand{Name: "get weather", Description: "d", Type: 1},
			"must be lowercase",
		},
		{
			"empty description",
			domain.DiscordApplicationCommand{Name: "weather", Type: 1},
			"description must be 1-100 characters",
		},
		{
			"description too long",
			domain.DiscordApplicationCommand{Name: "weather", Description: strings.Repeat("d", 101), Type: 1},
			"description must be 1-100 characters",
		},
		{
			"user command with description",
			domain.DiscordApplicationCommand{Name: "Ask AI", Description: "no", Type: 2},
			"cannot have a description",
		},
		{
			"user command with options",
			domain.DiscordApplicationCommand{Name: "Ask AI", Type: 2, Options: []domain.DiscordApplicationCommandOption{{Type: 3, Name: "x", Description: "d"}}},
			"only slash commands can have options",
		},
		{
			"unknown type",
			domain.DiscordApplicationCommand{Name: "x", Description: "d", Type: 7},
			"not one of Discord's types",
		},
		{
			"unknown option type",
			domain.DiscordApplicationCommand{Name: "x", Description: "d", Options: []domain.DiscordApplicationCommandOption{{Type: 42, Name: "opt", Description: "d"}}},
			"option type 42",
		},
		{
			"option name too long",
			domain.DiscordApplicationCommand{Name: "x", Description: "d", Options: []domain.DiscordApplicationCommandOption{{Type: 3, Name: strings.Repeat("a", 33), Description: "d"}}},
			"option name must be 1-32 characters",
		},
		{
			"option description empty",
			domain.DiscordApplicationCommand{Name: "x", Description: "d", Options: []domain.DiscordApplicationCommandOption{{Type: 3, Name: "city", Description: ""}}},
			"option description must be 1-100 characters",
		},
		{
			"choices on boolean",
			domain.DiscordApplicationCommand{Name: "x", Description: "d", Options: []domain.DiscordApplicationCommandOption{{Type: 5, Name: "flag", Description: "d", Choices: []domain.DiscordApplicationCommandChoice{{Name: "yes", Value: "1"}}}}},
			"choices are only allowed on text, integer, and number options",
		},
		{
			"too many choices",
			domain.DiscordApplicationCommand{Name: "x", Description: "d", Options: []domain.DiscordApplicationCommandOption{{
				Type: 3, Name: "city", Description: "d",
				Choices: makeChoices(26),
			}}},
			"at most 25 choices",
		},
		{
			"required after optional",
			domain.DiscordApplicationCommand{Name: "x", Description: "d", Options: []domain.DiscordApplicationCommandOption{
				{Type: 3, Name: "opt", Description: "d"},
				{Type: 3, Name: "req", Description: "d", Required: true},
			}},
			"required options must be listed before optional options",
		},
		{
			"mixed subcommands and values",
			domain.DiscordApplicationCommand{Name: "x", Description: "d", Options: []domain.DiscordApplicationCommandOption{
				{Type: 1, Name: "sub", Description: "d", Options: []domain.DiscordApplicationCommandOption{{Type: 3, Name: "v", Description: "d"}}},
				{Type: 3, Name: "stray", Description: "d"},
			}},
			"cannot mix subcommands with value options",
		},
		{
			"nested too deep",
			domain.DiscordApplicationCommand{Name: "x", Description: "d", Options: []domain.DiscordApplicationCommandOption{deepGroup(3)}},
			"only nest two levels",
		},
		{
			"min above max",
			domain.DiscordApplicationCommand{Name: "x", Description: "d", Options: []domain.DiscordApplicationCommandOption{{
				Type: 4, Name: "days", Description: "d", MinValue: ptrFloat(10), MaxValue: ptrFloat(5),
			}}},
			"minimum value 10.00 is above the maximum 5.00",
		},
		{
			"min length above max",
			domain.DiscordApplicationCommand{Name: "x", Description: "d", Options: []domain.DiscordApplicationCommandOption{{
				Type: 3, Name: "q", Description: "d", MinLength: ptrInt(50), MaxLength: 10,
			}}},
			"minimum length 50 is above the maximum 10",
		},
		{
			"valid nesting",
			domain.DiscordApplicationCommand{Name: "config", Description: "d", Options: []domain.DiscordApplicationCommandOption{{
				Type: 2, Name: "set", Description: "d",
				Options: []domain.DiscordApplicationCommandOption{{
					Type: 1, Name: "theme", Description: "d",
					Options: []domain.DiscordApplicationCommandOption{{Type: 3, Name: "value", Description: "d", Required: true}},
				}},
			}}},
			"",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := validateCommand(testCase.command)
			if testCase.want == "" {
				if got != "" {
					t.Fatalf("validateCommand() = %q, want accepted", got)
				}
				return
			}
			if !strings.Contains(got, testCase.want) {
				t.Fatalf("validateCommand() = %q, want it to contain %q", got, testCase.want)
			}
		})
	}
}

func makeChoices(count int) []domain.DiscordApplicationCommandChoice {
	choices := make([]domain.DiscordApplicationCommandChoice, 0, count)
	for i := 0; i < count; i++ {
		choices = append(choices, domain.DiscordApplicationCommandChoice{Name: "c", Value: i})
	}
	return choices
}

func deepGroup(depth int) domain.DiscordApplicationCommandOption {
	option := domain.DiscordApplicationCommandOption{Type: 2, Name: "g", Description: "d"}
	if depth > 1 {
		option.Options = []domain.DiscordApplicationCommandOption{deepGroup(depth - 1)}
	} else {
		option.Options = []domain.DiscordApplicationCommandOption{{Type: 3, Name: "v", Description: "d"}}
	}
	return option
}

func ptrFloat(value float64) *float64 { return &value }
func ptrInt(value int) *int           { return &value }

func TestValidateInteractionRef(t *testing.T) {
	valid := domain.DiscordInteractionRef{InteractionID: "123456789012345678", ApplicationID: "123456789012345678", Token: "a"}
	if reason := validateInteractionRef(valid); reason != "" {
		t.Fatalf("validateInteractionRef(valid) = %q", reason)
	}
	if reason := validateInteractionRef(domain.DiscordInteractionRef{}); reason != "wire the Command Trigger's Interaction output into this node — the interaction token is required" {
		t.Fatalf("empty ref reason = %q", reason)
	}
	badID := valid
	badID.InteractionID = "not-an-id"
	if reason := validateInteractionRef(badID); !strings.Contains(reason, "is not a valid Discord ID") {
		t.Fatalf("bad interaction id reason = %q", reason)
	}
}

func TestValidateInteractionReplyRules(t *testing.T) {
	valid := domain.DiscordInteractionRef{InteractionID: "123", ApplicationID: "456", Token: "t"}
	if reason := validateInteractionReply(valid, "hello", nil); reason != "" {
		t.Fatalf("valid reply reason = %q", reason)
	}
	if reason := validateInteractionReply(valid, "", nil); !strings.Contains(reason, "a message or at least one embed is required") {
		t.Fatalf("empty reply reason = %q", reason)
	}
	if reason := validateInteractionReply(valid, strings.Repeat("x", 2001), nil); !strings.Contains(reason, "2,000-character limit") {
		t.Fatalf("long reply reason = %q", reason)
	}
}

func TestTransportCommandRoundTrip(t *testing.T) {
	command := domain.DiscordApplicationCommand{
		Type: 1, Name: "weather", Description: "Get the weather",
		Options: []domain.DiscordApplicationCommandOption{{
			Type: 3, Name: "city", Description: "City name", Required: true,
			Choices:   []domain.DiscordApplicationCommandChoice{{Name: "Berlin", Value: "berlin"}},
			MinLength: ptrInt(2), MaxLength: 32,
		}},
	}
	transport := transportCommand(command)
	if transport.Name != "weather" || len(transport.Options) != 1 {
		t.Fatalf("transport = %#v", transport)
	}
	option := transport.Options[0]
	if !option.Required || len(option.Choices) != 1 || option.Choices[0].Value != "berlin" {
		t.Fatalf("option = %#v", option)
	}
	if option.MinLength == nil || *option.MinLength != 2 || option.MaxLength != 32 {
		t.Fatalf("length constraints = %#v / %d", option.MinLength, option.MaxLength)
	}
	back := domainCommand(transport)
	if back.Name != command.Name || len(back.Options) != 1 || back.Options[0].Name != "city" {
		t.Fatalf("round trip = %#v", back)
	}
	if back.Options[0].MinLength == nil || *back.Options[0].MinLength != 2 {
		t.Fatalf("round trip min length = %#v", back.Options[0].MinLength)
	}
	if back.Type != domain.DiscordCommandChatInput {
		t.Fatalf("round trip type = %d", back.Type)
	}
}

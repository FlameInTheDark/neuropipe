package execution

// Mirrors the Desktop composition in internal/app/desktop.go for the Discord
// and Telegram services. The app package cannot compile in headless
// containers (Wails GTK), so this test keeps the wiring honest.

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/discord"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/telegram"
)

type wiringVault struct{}

func (wiringVault) Get(string) (string, error) { return "", context.Canceled }
func (wiringVault) Put(string, string) error   { return nil }
func (wiringVault) Delete(string) error        { return nil }

type wiringBindings struct{}

func (wiringBindings) ListTriggers(context.Context, domain.TriggerKind) ([]domain.TriggerBinding, error) {
	return nil, nil
}

func TestDiscordAndTelegramServicesSatisfyEnginePorts(t *testing.T) {
	emit := func(string, any) {}
	saveDiscord := func(context.Context, domain.DiscordIdentity) error { return nil }
	saveTelegram := func(context.Context, domain.TelegramIdentity) error { return nil }

	discordService := discord.New(wiringVault{}, wiringBindings{}, nil, saveDiscord, emit)
	discordService.Configure(domain.DiscordSettings{})
	var discordSender nodes.DiscordSender = discordService

	telegramService := telegram.New(wiringVault{}, wiringBindings{}, nil, saveTelegram, emit)
	telegramService.Configure(domain.TelegramSettings{})
	var telegramSender nodes.TelegramSender = telegramService

	if discordSender == nil || telegramSender == nil {
		t.Fatal("services do not satisfy the engine ports")
	}
	if len(discordService.Catalog()) < 13 {
		t.Fatalf("discord catalog has %d entries", len(discordService.Catalog()))
	}
	if len(telegramService.Catalog()) < 8 {
		t.Fatalf("telegram catalog has %d entries", len(telegramService.Catalog()))
	}
	if discordService.Status().ConnectionState != "stopped" || telegramService.Status().ConnectionState != "stopped" {
		t.Fatal("services must start stopped")
	}

	// The execution service accepts them exactly like Desktop does.
	service := NewService(nil, nil, nil, nil)
	service.SetDiscordSender(discordSender)
	service.SetTelegramSender(telegramSender)
	if service.discord != discordSender || service.telegram != telegramSender {
		t.Fatal("senders were not retained")
	}
}

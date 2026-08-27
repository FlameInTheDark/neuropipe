package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/discordspec"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/bwmarrin/discordgo"
)

// memoryVault mirrors the production security.Vault concurrency contract:
// the service reads tokens from its validation loop goroutine while request
// goroutines Put/Delete, so the fake must be safe for concurrent use.
type memoryVault struct {
	mu     sync.Mutex
	values map[string]string
}

func (v *memoryVault) Get(key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	value, found := v.values[key]
	if !found {
		return "", fmt.Errorf("missing %s", key)
	}
	return value, nil
}
func (v *memoryVault) Put(key, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.values[key] = value
	return nil
}
func (v *memoryVault) Delete(key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.values, key)
	return nil
}

type fakeBindings struct {
	mu       sync.Mutex
	bindings []domain.TriggerBinding
}

func (f *fakeBindings) ListTriggers(context.Context, domain.TriggerKind) ([]domain.TriggerBinding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.TriggerBinding(nil), f.bindings...), nil
}

type fakeRunner struct {
	mu       sync.Mutex
	bindings []string
	packets  []pipeline.Packet
}

func (f *fakeRunner) QueueBinding(_ context.Context, bindingID string, packet pipeline.Packet, _ bool) (domain.Execution, error) {
	f.mu.Lock()
	f.bindings = append(f.bindings, bindingID)
	f.packets = append(f.packets, packet)
	f.mu.Unlock()
	return domain.Execution{ID: "run"}, nil
}
func (f *fakeRunner) snapshot() ([]string, []pipeline.Packet) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.bindings...), append([]pipeline.Packet(nil), f.packets...)
}

func binding(id, eventType, identityID, guildID, channelID string, trusted, enabled bool) domain.TriggerBinding {
	config := map[string]any{"eventType": eventType, "identityId": identityID}
	if guildID != "" {
		config["guildId"] = guildID
	}
	if channelID != "" {
		config["channelId"] = channelID
	}
	return domain.TriggerBinding{ID: id, Kind: domain.TriggerDiscord, Config: config, Trusted: trusted, Enabled: enabled}
}

func botIdentity(id string) domain.DiscordIdentity {
	return domain.DiscordIdentity{ID: id, Label: id, BotUserID: "1", Username: "bot", Status: domain.DiscordIdentityConnected}
}

// withFakeREST redirects the discordgo REST endpoints at an httptest server
// for the duration of one test and restores them afterwards.
func withFakeREST(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	originalUsers, originalChannels := discordgo.EndpointUsers, discordgo.EndpointChannels
	discordgo.EndpointUsers = server.URL + "/users/"
	discordgo.EndpointChannels = server.URL + "/channels/"
	t.Cleanup(func() {
		discordgo.EndpointUsers, discordgo.EndpointChannels = originalUsers, originalChannels
		server.Close()
	})
	return server
}

func TestIntentSetsAreTrustGated(t *testing.T) {
	identities := []domain.DiscordIdentity{botIdentity("main"), botIdentity("mod")}
	bindings := []domain.TriggerBinding{
		binding("trusted", "message.create", "", "", "", true, true),
		binding("untrusted", "guild.member.add", "", "", "", false, true),
		binding("disabled", "message.reaction.add", "", "", "", true, false),
		binding("mods", "guild.ban.add", "mod", "", "", true, true),
	}
	sets := intentSets(bindings, identities, "main")
	messageIntents := discordspec.IntentGuildMessages | discordspec.IntentDirectMessages | discordspec.IntentMessageContent
	if sets["main"] != messageIntents {
		t.Fatalf("main intents = %d, want %d", sets["main"], messageIntents)
	}
	if sets["mod"] != discordspec.IntentGuildModeration {
		t.Fatalf("mod intents = %d", sets["mod"])
	}
	if intents, leaked := sets["untrusted"]; leaked || intents != 0 {
		t.Fatal("untrusted binding leaked intents")
	}
	if discordspec.RequiresPrivilegedIntents([]string{"message.create"}) != true {
		t.Fatal("message.create must require a privileged intent")
	}
	if discordspec.RequiresPrivilegedIntents([]string{"message.delete"}) {
		t.Fatal("message.delete must not require a privileged intent")
	}
}

func TestAddManualIdentityValidatesAndStoresToken(t *testing.T) {
	withFakeREST(t, func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/users/@me" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		if !strings.HasPrefix(request.Header.Get("Authorization"), "Bot ") {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		_, _ = response.Write([]byte(`{"id":"42","username":"neuropipe","bot":true}`))
	})
	vault := &memoryVault{values: map[string]string{}}
	service := New(vault, nil, nil, nil, nil)

	identity, err := service.AddManualIdentity(context.Background(), domain.DiscordManualIdentityRequest{Token: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if identity.Username != "neuropipe" || identity.BotUserID != "42" || identity.Label != "neuropipe" || identity.Status != domain.DiscordIdentityConnected {
		t.Fatalf("identity = %#v", identity)
	}
	if got, err := vault.Get(tokenKey(identity.ID)); err != nil || got != "abc" {
		t.Fatalf("vault token = %q, %v", got, err)
	}
	if _, err := service.AddManualIdentity(context.Background(), domain.DiscordManualIdentityRequest{Token: "abc"}); err == nil || !strings.Contains(err.Error(), "already connected") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestAddManualIdentityRejectsInvalidAndNonBotTokens(t *testing.T) {
	withFakeREST(t, func(response http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.Header.Get("Authorization"), "bad") {
			response.WriteHeader(http.StatusUnauthorized)
			_, _ = response.Write([]byte(`{"message": "401: Unauthorized"}`))
			return
		}
		_, _ = response.Write([]byte(`{"id":"7","username":"human","bot":false}`))
	})
	service := New(&memoryVault{values: map[string]string{}}, nil, nil, nil, nil)
	if _, err := service.AddManualIdentity(context.Background(), domain.DiscordManualIdentityRequest{Token: "bad"}); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("invalid token error = %v", err)
	}
	if _, err := service.AddManualIdentity(context.Background(), domain.DiscordManualIdentityRequest{Token: "user"}); err == nil || !strings.Contains(err.Error(), "bot") {
		t.Fatalf("non-bot error = %v", err)
	}
}

func TestDeliverMatchesTrustedBindingsAndConditions(t *testing.T) {
	bindings := &fakeBindings{bindings: []domain.TriggerBinding{
		binding("trusted", "message.create", "", "", "", true, true),
		binding("guild", "message.create", "", "10", "", true, true),
		binding("channel", "message.create", "", "", "20", true, true),
		binding("untrusted", "message.create", "", "", "", false, true),
		binding("other-identity", "message.create", "other", "", "", true, true),
	}}
	runner := &fakeRunner{}
	service := New(&memoryVault{values: map[string]string{}}, bindings, runner, nil, nil)
	service.Configure(domain.DiscordSettings{Identities: []domain.DiscordIdentity{botIdentity("main")}, DefaultBotIdentityID: "main"})

	chat := discordspec.ChatMessage{Text: "hello", MessageID: "m1", ChannelID: "20", GuildID: "10", AuthorID: "5", AuthorUsername: "alice"}
	service.emitEvent("main", "message.create", "MESSAGE_CREATE", "m1", map[string]any{"chatMessage": chat, "messageId": "m1", "channelId": "20", "guildId": "10", "authorId": "5"})

	bindingIDs, packets := runner.snapshot()
	if len(bindingIDs) != 3 {
		t.Fatalf("delivered bindings = %v", bindingIDs)
	}
	first, _ := packets[0]["event"].(discordspec.DiscordEvent)
	if first.Type != "message.create" || first.GatewayEvent != "MESSAGE_CREATE" || first.MessageID != "m1" {
		t.Fatalf("event = %#v", first)
	}
	message, _ := first.Payload["chatMessage"].(discordspec.ChatMessage)
	if message.Text != "hello" || message.ChannelID != "20" {
		t.Fatalf("chat message = %#v", message)
	}
}

func TestDeliverSkipsWhenConditionsDoNotMatch(t *testing.T) {
	bindings := &fakeBindings{bindings: []domain.TriggerBinding{
		binding("guild", "message.create", "", "999", "", true, true),
	}}
	runner := &fakeRunner{}
	service := New(&memoryVault{values: map[string]string{}}, bindings, runner, nil, nil)
	service.Configure(domain.DiscordSettings{Identities: []domain.DiscordIdentity{botIdentity("main")}, DefaultBotIdentityID: "main"})
	service.emitEvent("main", "message.create", "MESSAGE_CREATE", "m1", map[string]any{"guildId": "10", "channelId": "20"})
	if bindingIDs, _ := runner.snapshot(); len(bindingIDs) != 0 {
		t.Fatalf("non-matching conditions delivered: %v", bindingIDs)
	}
}

func TestHandleInteractionFlattensOptions(t *testing.T) {
	bindings := &fakeBindings{bindings: []domain.TriggerBinding{binding("cmd", "interaction.create", "", "", "", true, true)}}
	runner := &fakeRunner{}
	service := New(&memoryVault{values: map[string]string{}}, bindings, runner, nil, nil)
	service.Configure(domain.DiscordSettings{Identities: []domain.DiscordIdentity{botIdentity("main")}, DefaultBotIdentityID: "main"})

	service.handleInteraction("main", &discordgo.Interaction{
		Type:      discordgo.InteractionApplicationCommand,
		Member:    &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		ChannelID: "c1", GuildID: "g1",
		Data: discordgo.ApplicationCommandInteractionData{Name: "greet", Options: []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "who", Type: discordgo.ApplicationCommandOptionString, Value: "world"},
			{Name: "times", Type: discordgo.ApplicationCommandOptionInteger, Value: float64(3)},
		}},
	})
	bindingIDs, packets := runner.snapshot()
	if len(bindingIDs) != 1 {
		t.Fatalf("bindings = %v", bindingIDs)
	}
	event, _ := packets[0]["event"].(discordspec.DiscordEvent)
	interaction, _ := event.Payload["interaction"].(discordspec.Interaction)
	if interaction.CommandName != "greet" || interaction.Options["who"] != "world" || interaction.Options["times"] != "3" || interaction.UserID != "u1" {
		t.Fatalf("interaction = %#v", interaction)
	}
}

func TestHandleReactionUsesCustomEmojiForm(t *testing.T) {
	bindings := &fakeBindings{bindings: []domain.TriggerBinding{binding("r", "message.reaction.add", "", "", "", true, true)}}
	runner := &fakeRunner{}
	service := New(&memoryVault{values: map[string]string{}}, bindings, runner, nil, nil)
	service.Configure(domain.DiscordSettings{Identities: []domain.DiscordIdentity{botIdentity("main")}, DefaultBotIdentityID: "main"})
	service.handleReaction("main", "message.reaction.add", "MESSAGE_REACTION_ADD", &discordgo.MessageReaction{
		UserID: "u1", MessageID: "m1", ChannelID: "c1", GuildID: "g1", Emoji: discordgo.Emoji{Name: "wavie", ID: "777"},
	})
	_, packets := runner.snapshot()
	event, _ := packets[0]["event"].(discordspec.DiscordEvent)
	reaction, _ := event.Payload["reaction"].(discordspec.Reaction)
	if reaction.Emoji != "wavie:777" {
		t.Fatalf("emoji = %q", reaction.Emoji)
	}
}

func TestSendMessageActionShapeAndSoftRejection(t *testing.T) {
	var lastBody map[string]any
	withFakeREST(t, func(response http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.URL.Path, "/messages") {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		if strings.Contains(request.URL.Path, "403") {
			response.WriteHeader(http.StatusForbidden)
			_, _ = response.Write([]byte(`{"message": "Missing Permissions"}`))
			return
		}
		raw, _ := io.ReadAll(request.Body)
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &lastBody); err != nil {
				t.Fatalf("body is not JSON: %s", raw)
			}
		}
		_, _ = response.Write([]byte(`{"id":"msg-1"}`))
	})
	vault := &memoryVault{values: map[string]string{tokenKey("main"): "token"}}
	service := New(vault, nil, nil, nil, nil)
	service.Configure(domain.DiscordSettings{Identities: []domain.DiscordIdentity{botIdentity("main")}, DefaultBotIdentityID: "main"})

	result, err := service.SendDiscordMessage(context.Background(), domain.DiscordMessageRequest{ChannelID: "55", Message: "hello", ReplyToID: "77"})
	if err != nil || !result.Sent || result.MessageID != "msg-1" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	if lastBody["content"] != "hello" {
		t.Fatalf("request body = %#v", lastBody)
	}
	reference, _ := lastBody["message_reference"].(map[string]any)
	if reference == nil || reference["message_id"] != "77" {
		t.Fatalf("reply reference = %#v", lastBody["message_reference"])
	}

	result, err = service.SendDiscordMessage(context.Background(), domain.DiscordMessageRequest{ChannelID: "403", Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent || result.Reason != "Missing Permissions" {
		t.Fatalf("soft reject = %#v", result)
	}
}

func TestSendMessagePreValidatesLengthCap(t *testing.T) {
	service := New(&memoryVault{values: map[string]string{}}, nil, nil, nil, nil)
	result, err := service.SendDiscordMessage(context.Background(), domain.DiscordMessageRequest{ChannelID: "1", Message: strings.Repeat("a", 2001)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent || !strings.Contains(result.Reason, "2,000") {
		t.Fatalf("cap result = %#v", result)
	}
}

// A reply reference to a message Discord cannot resolve answers 50035 with
// the offending field nested in the errors object; the reason pin must carry
// that detail instead of the opaque top-level message alone.
func TestSendMessageSurfacesDiscordFieldErrors(t *testing.T) {
	withFakeREST(t, func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadRequest)
		_, _ = response.Write([]byte(`{
                        "code": 50035,
                        "message": "Invalid Form Body",
                        "errors": {
                                "message_reference": {
                                        "_errors": [{"code": "REPLIES_UNKNOWN_MESSAGE", "message": "Unknown message"}]
                                }
                        }
                }`))
	})
	vault := &memoryVault{values: map[string]string{tokenKey("main"): "token"}}
	service := New(vault, nil, nil, nil, nil)
	service.Configure(domain.DiscordSettings{Identities: []domain.DiscordIdentity{botIdentity("main")}, DefaultBotIdentityID: "main"})

	result, err := service.SendDiscordMessage(context.Background(), domain.DiscordMessageRequest{ChannelID: "55", Message: "hello", ReplyToID: "77"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent {
		t.Fatalf("result = %#v", result)
	}
	expected := "Invalid Form Body — message_reference: Unknown message (REPLIES_UNKNOWN_MESSAGE) — the reply target does not exist in channel 55; the referenced message may live in another channel or thread, or the ID is wrong. Wire the trigger's Message ID and Channel ID outputs instead of typing IDs"
	if result.Reason != expected {
		t.Fatalf("reason = %q, want %q", result.Reason, expected)
	}
}

func TestSendMessageSurfacesNestedFieldErrors(t *testing.T) {
	withFakeREST(t, func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadRequest)
		_, _ = response.Write([]byte(`{
                        "code": 50035,
                        "message": "Invalid Form Body",
                        "errors": {
                                "message_reference": {
                                        "message_id": {
                                                "_errors": [{"code": "NUMBER_TYPE_COERCE", "message": "Value is not snowflake."}]
                                        }
                                }
                        }
                }`))
	})
	vault := &memoryVault{values: map[string]string{tokenKey("main"): "token"}}
	service := New(vault, nil, nil, nil, nil)
	service.Configure(domain.DiscordSettings{Identities: []domain.DiscordIdentity{botIdentity("main")}, DefaultBotIdentityID: "main"})

	result, err := service.SendDiscordMessage(context.Background(), domain.DiscordMessageRequest{ChannelID: "55", Message: "hello", ReplyToID: "77"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent {
		t.Fatalf("result = %#v", result)
	}
	expected := "Invalid Form Body — message_reference.message_id: Value is not snowflake. (NUMBER_TYPE_COERCE)"
	if result.Reason != expected {
		t.Fatalf("reason = %q, want %q", result.Reason, expected)
	}
}

func TestSendMessagePreValidatesSnowflakeIDs(t *testing.T) {
	service := New(&memoryVault{values: map[string]string{}}, nil, nil, nil, nil)

	result, err := service.SendDiscordMessage(context.Background(), domain.DiscordMessageRequest{ChannelID: "7.9216925611139072e+16", Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent || !strings.Contains(result.Reason, "channel ID") || !strings.Contains(result.Reason, "not a valid Discord ID") {
		t.Fatalf("channel result = %#v", result)
	}

	result, err = service.SendDiscordMessage(context.Background(), domain.DiscordMessageRequest{ChannelID: "55", Message: "hello", ReplyToID: "79216925611139072abc"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent || !strings.Contains(result.Reason, "reply to message ID") || !strings.Contains(result.Reason, "not a valid Discord message ID") {
		t.Fatalf("reply result = %#v", result)
	}
}

// The exact pair from a real report: the reply ID decodes to Aug 2015 while
// the target channel was created Apr 2019. A message cannot predate its
// channel, so Discord can only answer MESSAGE_REFERENCE_UNKNOWN_MESSAGE; the
// service must reject the reference before any request and explain the
// decoded dates.
func TestSendMessagePreValidatesImpossibleReplyReference(t *testing.T) {
	withFakeREST(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("impossible reply reference must be rejected before any REST call")
	})
	vault := &memoryVault{values: map[string]string{tokenKey("main"): "token"}}
	service := New(vault, nil, nil, nil, nil)
	service.Configure(domain.DiscordSettings{Identities: []domain.DiscordIdentity{botIdentity("main")}, DefaultBotIdentityID: "main"})

	result, err := service.SendDiscordMessage(context.Background(), domain.DiscordMessageRequest{
		ChannelID: "565062979255795712", Message: "Pong", ReplyToID: "79216925611139072",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent {
		t.Fatalf("result = %#v", result)
	}
	for _, want := range []string{"Aug 2015", "Apr 2019", "before channel 565062979255795712", "Message ID"} {
		if !strings.Contains(result.Reason, want) {
			t.Fatalf("reason = %q, want it to mention %q", result.Reason, want)
		}
	}
}

func TestSendMessagePreValidatesFutureReplyReference(t *testing.T) {
	service := New(&memoryVault{values: map[string]string{}}, nil, nil, nil, nil)
	replyID := snowflakeForMS(time.Now().UTC().Add(24 * time.Hour).UnixMilli())
	result, err := service.SendDiscordMessage(context.Background(), domain.DiscordMessageRequest{
		ChannelID: "565062979255795712", Message: "Pong", ReplyToID: replyID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent || !strings.Contains(result.Reason, "in the future") {
		t.Fatalf("future result = %#v", result)
	}
}

// A reply newer than the channel's creation is legitimate and must reach
// Discord with the exact wired ID in message_reference.
func TestSendMessageSendsReplyNewerThanChannel(t *testing.T) {
	var lastBody map[string]any
	withFakeREST(t, func(response http.ResponseWriter, request *http.Request) {
		raw, _ := io.ReadAll(request.Body)
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &lastBody); err != nil {
				t.Fatalf("body is not JSON: %s", raw)
			}
		}
		_, _ = response.Write([]byte(`{"id":"msg-2"}`))
	})
	vault := &memoryVault{values: map[string]string{tokenKey("main"): "token"}}
	service := New(vault, nil, nil, nil, nil)
	service.Configure(domain.DiscordSettings{Identities: []domain.DiscordIdentity{botIdentity("main")}, DefaultBotIdentityID: "main"})

	now := time.Now().UTC()
	channelID := snowflakeForMS(now.Add(-365 * 24 * time.Hour).UnixMilli())
	replyID := snowflakeForMS(now.Add(-time.Hour).UnixMilli())
	result, err := service.SendDiscordMessage(context.Background(), domain.DiscordMessageRequest{
		ChannelID: channelID, Message: "Pong", ReplyToID: replyID,
	})
	if err != nil || !result.Sent || result.MessageID != "msg-2" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	reference, _ := lastBody["message_reference"].(map[string]any)
	if reference == nil || reference["message_id"] != replyID {
		t.Fatalf("reply reference = %#v, want message_id %s", lastBody["message_reference"], replyID)
	}
}

// snowflakeForMS builds the smallest snowflake carrying the given creation
// time so tests can construct realistic Discord IDs.
func snowflakeForMS(ms int64) string {
	if ms < discordEpochMS {
		ms = discordEpochMS
	}
	return strconv.FormatUint(uint64(ms-discordEpochMS)<<22, 10)
}

func TestCloseCode4014SurfacesRemediation(t *testing.T) {
	service := New(&memoryVault{values: map[string]string{}}, nil, nil, nil, nil)
	service.setError(fmt.Errorf("websocket: close 4014: Disallowed intent(s)"))
	status := service.Status()
	if !strings.Contains(status.LastError, "Developer Portal") {
		t.Fatalf("remediation = %q", status.LastError)
	}
}

func TestRemoveIdentityDeletesVaultRecord(t *testing.T) {
	vault := &memoryVault{values: map[string]string{tokenKey("main"): "token"}}
	service := New(vault, nil, nil, nil, nil)
	service.Configure(domain.DiscordSettings{Identities: []domain.DiscordIdentity{botIdentity("main")}, DefaultBotIdentityID: "main"})
	if err := service.RemoveIdentity(context.Background(), "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Get(tokenKey("main")); err == nil {
		t.Fatal("vault record survived removal")
	}
	// Read under the service lock, mirroring how callers observe settings
	// while background goroutines may persist identity state.
	service.mu.RLock()
	settings := service.settings
	service.mu.RUnlock()
	if len(settings.Identities) != 0 || settings.DefaultBotIdentityID != "" {
		t.Fatalf("settings = %#v", settings)
	}
}

func TestMessageDeduplicationIsUnnecessaryButStateNamesResolve(t *testing.T) {
	// Channel and guild names come from the optional state cache; without a
	// session the lookup must degrade to empty strings instead of panicking.
	if stateChannelName(nil, "1") != "" || stateGuildName(nil, "1") != "" {
		t.Fatal("nil session must degrade to empty names")
	}
	_ = time.Now()
}

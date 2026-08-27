// Package discordspec contains the stable Discord gateway contracts shared by
// the Discord infrastructure and first-party Discord node modules. It has no
// Wails, persistence, transport, or graph-engine dependencies — and it does
// not import discordgo, so node packages never see the client library.
package discordspec

import (
	"sort"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Gateway intent bits as defined by the Discord gateway protocol. They are
// mirrored here (instead of importing discordgo) so this package stays
// dependency-free; internal/discord converts them to discordgo.Intent.
const (
	IntentGuilds                 = 1 << 0
	IntentGuildMembers           = 1 << 1 // privileged
	IntentGuildModeration        = 1 << 2
	IntentGuildEmojisAndStickers = 1 << 3
	IntentGuildIntegrations      = 1 << 4
	IntentGuildWebhooks          = 1 << 5
	IntentGuildInvites           = 1 << 6
	IntentGuildVoiceStates       = 1 << 7
	IntentGuildPresences         = 1 << 8 // privileged
	IntentGuildMessages          = 1 << 9
	IntentGuildMessageReactions  = 1 << 10
	IntentGuildMessageTyping     = 1 << 11
	IntentDirectMessages         = 1 << 12
	IntentDirectMessageReactions = 1 << 13
	IntentDirectMessageTyping    = 1 << 14
	IntentMessageContent         = 1 << 15 // privileged
	IntentGuildScheduledEvents   = 1 << 16
)

// Event is the common, JSON-safe gateway delivery contract. Event-specific
// nodes expose their selected typed convenience fields in addition to this
// complete envelope; unrecognised Discord additions remain isolated in Payload
// rather than weakening other pins to an implicit conversion.
type DiscordEvent struct {
	Type         string         `json:"type"`
	GatewayEvent string         `json:"gatewayEvent"`
	MessageID    string         `json:"messageId"`
	ReceivedAt   string         `json:"receivedAt"`
	Payload      map[string]any `json:"payload"`
}

func EventType() domain.TypeSpec {
	key, value := typespec.String(), typespec.Any()
	payload := domain.TypeSpec{Kind: domain.TypeMap, Key: &key, Value: &value}
	return domain.TypeSpec{Kind: domain.TypeRecord, Name: "DiscordEvent", Fields: []domain.TypeFieldSpec{
		{ID: "type", Name: "type", Type: typespec.String()},
		{ID: "gatewayEvent", Name: "gatewayEvent", Type: typespec.String()},
		{ID: "messageId", Name: "messageId", Type: typespec.String()},
		{ID: "receivedAt", Name: "receivedAt", Type: typespec.String()},
		{ID: "payload", Name: "payload", Type: payload},
	}}
}

// ChatMessage describes the message-specific portion of MESSAGE_CREATE and
// MESSAGE_UPDATE deliveries. It is intentionally a native named value so
// strict runtime TypeSpecs can verify it without treating a map as a hidden
// any value.
type ChatMessage struct {
	Text           string `json:"text"`
	CommandText    string `json:"commandText"`
	MessageID      string `json:"messageId"`
	ChannelID      string `json:"channelId"`
	ChannelName    string `json:"channelName"`
	GuildID        string `json:"guildId"`
	GuildName      string `json:"guildName"`
	AuthorID       string `json:"authorId"`
	AuthorUsername string `json:"authorUsername"`
	AuthorBot      bool   `json:"authorBot"`
}

// DiscordAuthor is the native value behind the trigger's named Author record.
type DiscordAuthor struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

func ChatMessageType() domain.TypeSpec {
	return domain.TypeSpec{Kind: domain.TypeRecord, Name: "DiscordChatMessage", Fields: []domain.TypeFieldSpec{
		{ID: "text", Name: "text", Type: typespec.String()},
		{ID: "commandText", Name: "commandText", Type: typespec.String()},
		{ID: "messageId", Name: "messageId", Type: typespec.String()},
		{ID: "channelId", Name: "channelId", Type: typespec.String()},
		{ID: "channelName", Name: "channelName", Type: typespec.String()},
		{ID: "guildId", Name: "guildId", Type: typespec.String()},
		{ID: "guildName", Name: "guildName", Type: typespec.String()},
		{ID: "authorId", Name: "authorId", Type: typespec.String()},
		{ID: "authorUsername", Name: "authorUsername", Type: typespec.String()},
		{ID: "authorBot", Name: "authorBot", Type: typespec.Bool()},
	}}
}

func AuthorType() domain.TypeSpec {
	return domain.TypeSpec{Kind: domain.TypeRecord, Name: "DiscordAuthor", Fields: []domain.TypeFieldSpec{
		{ID: "id", Name: "id", Type: typespec.String()},
		{ID: "username", Name: "username", Type: typespec.String()},
		{ID: "bot", Name: "bot", Type: typespec.Bool()},
	}}
}

// Reaction describes the reaction-specific portion of reaction deliveries.
type Reaction struct {
	MessageID string `json:"messageId"`
	ChannelID string `json:"channelId"`
	GuildID   string `json:"guildId"`
	UserID    string `json:"userId"`
	Emoji     string `json:"emoji"` // unicode emoji or name:id
}

// Interaction describes the application-command portion of an
// INTERACTION_CREATE delivery. Options maps option names to string values.
type Interaction struct {
	CommandName string            `json:"commandName"`
	UserID      string            `json:"userId"`
	ChannelID   string            `json:"channelId"`
	GuildID     string            `json:"guildId"`
	Options     map[string]string `json:"options"`
}

// Member describes the member-specific portion of guild member deliveries.
type Member struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	GuildID  string `json:"guildId"`
	JoinedAt string `json:"joinedAt"`
}

// Catalog returns the gateway events which Neuropipe currently understands.
// Every entry has a deterministic gateway event name, intent union, and a
// strict envelope contract.
func Catalog() []domain.DiscordEventDescriptor {
	guildFilter := []domain.DiscordEventConditionField{
		{ID: "guildId", Label: "Guild ID", Description: "Optional Discord server (guild) snowflake. Leave empty to match every server.", Required: false},
	}
	channelFilter := append(append([]domain.DiscordEventConditionField{}, guildFilter...), domain.DiscordEventConditionField{
		ID: "channelId", Label: "Channel ID", Description: "Optional channel snowflake. Leave empty to match every channel.", Required: false,
	})
	entries := []domain.DiscordEventDescriptor{
		event("message.create", "MESSAGE_CREATE", "Message sent", "A message is sent in a server or direct channel.", IntentGuildMessages|IntentDirectMessages|IntentMessageContent, true, true, channelFilter),
		event("message.update", "MESSAGE_UPDATE", "Message edited", "A message is edited in a server or direct channel.", IntentGuildMessages|IntentDirectMessages|IntentMessageContent, true, true, channelFilter),
		event("message.delete", "MESSAGE_DELETE", "Message deleted", "A message is deleted.", IntentGuildMessages|IntentDirectMessages, false, false, channelFilter),
		event("message.reaction.add", "MESSAGE_REACTION_ADD", "Reaction added", "A member reacts to a message.", IntentGuildMessageReactions|IntentDirectMessageReactions, false, false, channelFilter),
		event("message.reaction.remove", "MESSAGE_REACTION_REMOVE", "Reaction removed", "A member removes a reaction from a message.", IntentGuildMessageReactions|IntentDirectMessageReactions, false, false, channelFilter),
		event("message.reaction.remove_all", "MESSAGE_REACTION_REMOVE_ALL", "All reactions removed", "All reactions are removed from a message.", IntentGuildMessageReactions, false, false, channelFilter),
		event("guild.member.add", "GUILD_MEMBER_ADD", "Member joined", "A member joins a server.", IntentGuildMembers, true, false, guildFilter),
		event("guild.member.remove", "GUILD_MEMBER_REMOVE", "Member left", "A member is removed from or leaves a server.", IntentGuildMembers, true, false, guildFilter),
		event("guild.member.update", "GUILD_MEMBER_UPDATE", "Member updated", "A member's nickname or roles change.", IntentGuildMembers, true, false, guildFilter),
		event("guild.ban.add", "GUILD_BAN_ADD", "Member banned", "A member is banned from a server.", IntentGuildModeration, false, false, guildFilter),
		event("guild.ban.remove", "GUILD_BAN_REMOVE", "Ban removed", "A ban is removed from a server.", IntentGuildModeration, false, false, guildFilter),
		event("interaction.create", "INTERACTION_CREATE", "Interaction created", "A member uses an application command (slash command).", IntentGuilds, false, false, guildFilter),
		event("voice.state.update", "VOICE_STATE_UPDATE", "Voice state updated", "A member joins, leaves, or moves between voice channels.", IntentGuildVoiceStates, false, false, guildFilter),
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Type < entries[j].Type })
	return entries
}

func event(eventType, gatewayEvent, label, description string, intents int, privileged, chat bool, conditions []domain.DiscordEventConditionField) domain.DiscordEventDescriptor {
	return domain.DiscordEventDescriptor{
		Type:         eventType,
		GatewayEvent: gatewayEvent,
		Label:        label,
		Description:  description,
		Intents:      intents,
		Privileged:   privileged,
		ChatMessage:  chat,
		Conditions:   append([]domain.DiscordEventConditionField(nil), conditions...),
	}
}

func Find(eventType string) (domain.DiscordEventDescriptor, bool) {
	for _, descriptor := range Catalog() {
		if descriptor.Type == strings.TrimSpace(eventType) {
			return descriptor, true
		}
	}
	return domain.DiscordEventDescriptor{}, false
}

// RequiredIntents returns the union of intent bits required by the given
// trusted event types. Unknown types contribute nothing.
func RequiredIntents(eventTypes []string) int {
	union := 0
	for _, eventType := range eventTypes {
		if descriptor, ok := Find(eventType); ok {
			union |= descriptor.Intents
		}
	}
	return union
}

// RequiresPrivilegedIntents reports whether any of the given trusted event
// types needs a Developer Portal privileged-intent toggle.
func RequiresPrivilegedIntents(eventTypes []string) bool {
	for _, eventType := range eventTypes {
		if descriptor, ok := Find(eventType); ok && descriptor.Privileged {
			return true
		}
	}
	return false
}

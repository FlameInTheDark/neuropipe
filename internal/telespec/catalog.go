// Package telespec contains the stable Telegram Bot API contracts shared by
// the Telegram infrastructure and first-party Telegram node modules. It has
// no Wails, persistence, transport, or graph-engine dependencies.
package telespec

import (
	"sort"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Event is the common, JSON-safe update delivery contract. Event-specific
// nodes expose their selected typed convenience fields in addition to this
// complete envelope; unrecognised Telegram additions remain isolated in
// Payload rather than weakening other pins to an implicit conversion.
type TelegramEvent struct {
	Type       string         `json:"type"`
	UpdateID   int64          `json:"updateId"`
	ReceivedAt string         `json:"receivedAt"`
	Payload    map[string]any `json:"payload"`
}

func EventType() domain.TypeSpec {
	key, value := typespec.String(), typespec.Any()
	payload := domain.TypeSpec{Kind: domain.TypeMap, Key: &key, Value: &value}
	return domain.TypeSpec{Kind: domain.TypeRecord, Name: "TelegramEvent", Fields: []domain.TypeFieldSpec{
		{ID: "type", Name: "type", Type: typespec.String()},
		{ID: "updateId", Name: "updateId", Type: typespec.Float()},
		{ID: "receivedAt", Name: "receivedAt", Type: typespec.String()},
		{ID: "payload", Name: "payload", Type: payload},
	}}
}

// Message describes the message-like portion of message, edited_message,
// channel_post, and edited_channel_post updates.
type Message struct {
	Text         string `json:"text"`
	CommandText  string `json:"commandText"`
	MessageID    int64  `json:"messageId"`
	ChatID       int64  `json:"chatId"`
	ChatType     string `json:"chatType"` // private | group | supergroup | channel
	ChatTitle    string `json:"chatTitle"`
	ChatUsername string `json:"chatUsername"` // without @
	FromID       int64  `json:"fromId"`
	FromUsername string `json:"fromUsername"` // without @
	FromName     string `json:"fromName"`
	Date         int64  `json:"date"`
}

// TelegramFrom is the native value behind the trigger's named From record.
type TelegramFrom struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// CallbackQuery describes the callback_query update.
type CallbackQuery struct {
	ID        string `json:"id"`
	Data      string `json:"data"`
	FromID    int64  `json:"fromId"`
	ChatID    int64  `json:"chatId"`
	MessageID int64  `json:"messageId"`
}

// ChatMemberUpdated describes the my_chat_member / chat_member updates.
type ChatMemberUpdated struct {
	ChatID    int64  `json:"chatId"`
	ChatTitle string `json:"chatTitle"`
	UserID    int64  `json:"userId"`
	OldStatus string `json:"oldStatus"`
	NewStatus string `json:"newStatus"`
}

func MessageType() domain.TypeSpec {
	return domain.TypeSpec{Kind: domain.TypeRecord, Name: "TelegramMessage", Fields: []domain.TypeFieldSpec{
		{ID: "text", Name: "text", Type: typespec.String()},
		{ID: "commandText", Name: "commandText", Type: typespec.String()},
		{ID: "messageId", Name: "messageId", Type: typespec.Float()},
		{ID: "chatId", Name: "chatId", Type: typespec.Float()},
		{ID: "chatType", Name: "chatType", Type: typespec.String()},
		{ID: "chatTitle", Name: "chatTitle", Type: typespec.String()},
		{ID: "chatUsername", Name: "chatUsername", Type: typespec.String()},
		{ID: "fromId", Name: "fromId", Type: typespec.Float()},
		{ID: "fromUsername", Name: "fromUsername", Type: typespec.String()},
		{ID: "fromName", Name: "fromName", Type: typespec.String()},
		{ID: "date", Name: "date", Type: typespec.Float()},
	}}
}

func FromType() domain.TypeSpec {
	return domain.TypeSpec{Kind: domain.TypeRecord, Name: "TelegramFrom", Fields: []domain.TypeFieldSpec{
		{ID: "id", Name: "id", Type: typespec.String()},
		{ID: "username", Name: "username", Type: typespec.String()},
		{ID: "name", Name: "name", Type: typespec.String()},
	}}
}

// Catalog returns the Bot API update types which Neuropipe currently
// understands. Type is the update field name used by getUpdates.
func Catalog() []domain.TelegramEventDescriptor {
	chatFilter := []domain.TelegramEventConditionField{
		{ID: "chatIds", Label: "Chat IDs", Description: "Optional comma-separated allowlist of numeric chat IDs or @channel usernames. Leave empty to match every chat.", Required: false},
	}
	entries := []domain.TelegramEventDescriptor{
		event("message", "Message received", "A message is sent in a private chat or group.", true, false, chatFilter),
		event("edited_message", "Message edited", "A sent message is edited.", true, false, chatFilter),
		event("channel_post", "Channel post", "A post is published in a channel.", true, false, chatFilter),
		event("edited_channel_post", "Channel post edited", "A channel post is edited.", true, false, chatFilter),
		event("callback_query", "Callback query", "An inline-keyboard button is pressed.", false, true, chatFilter),
		event("my_chat_member", "Bot chat member updated", "The bot is added to, removed from, or promoted in a chat.", false, false, chatFilter),
		event("chat_join_request", "Chat join request", "A user requests to join a chat.", false, false, chatFilter),
		event("message_reaction", "Message reaction", "A user reacts to a message.", false, false, chatFilter),
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Type < entries[j].Type })
	return entries
}

func event(eventType, label, description string, chat, callback bool, conditions []domain.TelegramEventConditionField) domain.TelegramEventDescriptor {
	return domain.TelegramEventDescriptor{
		Type:        eventType,
		Label:       label,
		Description: description,
		ChatMessage: chat,
		Callback:    callback,
		Conditions:  append([]domain.TelegramEventConditionField(nil), conditions...),
	}
}

func Find(eventType string) (domain.TelegramEventDescriptor, bool) {
	for _, descriptor := range Catalog() {
		if descriptor.Type == strings.TrimSpace(eventType) {
			return descriptor, true
		}
	}
	return domain.TelegramEventDescriptor{}, false
}

// AllowedUpdates returns the sorted, deduplicated union of update types for
// the given trusted event types; unknown types contribute nothing.
func AllowedUpdates(eventTypes []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		if descriptor, ok := Find(eventType); ok && !seen[descriptor.Type] {
			seen[descriptor.Type] = true
			result = append(result, descriptor.Type)
		}
	}
	sort.Strings(result)
	return result
}

package telegram

import (
	"strconv"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/telespec"
)

// telegramUpdate mirrors one Bot API update object. Exactly one event field
// is present per update.
type telegramUpdate struct {
	UpdateID          int64                      `json:"update_id"`
	Message           *telegramMessage           `json:"message"`
	EditedMessage     *telegramMessage           `json:"edited_message"`
	ChannelPost       *telegramMessage           `json:"channel_post"`
	EditedChannelPost *telegramMessage           `json:"edited_channel_post"`
	CallbackQuery     *telegramCallbackQuery     `json:"callback_query"`
	MyChatMember      *telegramChatMemberUpdated `json:"my_chat_member"`
	ChatJoinRequest   *telegramJoinRequest       `json:"chat_join_request"`
	MessageReaction   *telegramReaction          `json:"message_reaction"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (u telegramUser) displayName() string {
	if u.FirstName == "" {
		return u.LastName
	}
	if u.LastName == "" {
		return u.FirstName
	}
	return u.FirstName + " " + u.LastName
}

type telegramChat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Username string `json:"username"`
}

type telegramMessage struct {
	MessageID int64         `json:"message_id"`
	From      *telegramUser `json:"from"`
	Date      int64         `json:"date"`
	Chat      telegramChat  `json:"chat"`
	Text      string        `json:"text"`
	Caption   string        `json:"caption"`
}

type telegramCallbackQuery struct {
	ID      string           `json:"id"`
	From    telegramUser     `json:"from"`
	Message *telegramMessage `json:"message"`
	Data    string           `json:"data"`
}

type telegramChatMember struct {
	Status string       `json:"status"`
	User   telegramUser `json:"user"`
}

type telegramChatMemberUpdated struct {
	Chat          telegramChat       `json:"chat"`
	From          telegramUser       `json:"from"`
	Date          int64              `json:"date"`
	OldChatMember telegramChatMember `json:"old_chat_member"`
	NewChatMember telegramChatMember `json:"new_chat_member"`
}

type telegramJoinRequest struct {
	Chat telegramChat `json:"chat"`
	From telegramUser `json:"from"`
	Date int64        `json:"date"`
}

type telegramReaction struct {
	Chat      telegramChat  `json:"chat"`
	MessageID int64         `json:"message_id"`
	User      *telegramUser `json:"user"`
	Date      int64         `json:"date"`
}

// buildEvent converts a raw update into the JSON-safe envelope plus typed
// convenience payload. Unknown update shapes report ok=false so the poll loop
// still advances its offset without queuing anything.
func buildEvent(update telegramUpdate) (telespec.TelegramEvent, bool) {
	event := telespec.TelegramEvent{UpdateID: update.UpdateID, ReceivedAt: time.Now().UTC().Format(time.RFC3339Nano), Payload: map[string]any{}}
	switch {
	case update.Message != nil:
		event.Type = "message"
		event.Payload["message"] = chatMessage(update.Message)
	case update.EditedMessage != nil:
		event.Type = "edited_message"
		event.Payload["message"] = chatMessage(update.EditedMessage)
	case update.ChannelPost != nil:
		event.Type = "channel_post"
		event.Payload["message"] = chatMessage(update.ChannelPost)
	case update.EditedChannelPost != nil:
		event.Type = "edited_channel_post"
		event.Payload["message"] = chatMessage(update.EditedChannelPost)
	case update.CallbackQuery != nil:
		event.Type = "callback_query"
		query := update.CallbackQuery
		message := telespec.CallbackQuery{ID: query.ID, Data: query.Data, FromID: query.From.ID}
		if query.Message != nil {
			message.ChatID = query.Message.Chat.ID
			message.MessageID = query.Message.MessageID
		}
		event.Payload["callbackQuery"] = message
	case update.MyChatMember != nil:
		event.Type = "my_chat_member"
		member := update.MyChatMember
		event.Payload["chatMember"] = telespec.ChatMemberUpdated{
			ChatID: member.Chat.ID, ChatTitle: member.Chat.Title, UserID: member.From.ID,
			OldStatus: member.OldChatMember.Status, NewStatus: member.NewChatMember.Status,
		}
	case update.ChatJoinRequest != nil:
		event.Type = "chat_join_request"
		request := update.ChatJoinRequest
		event.Payload["joinRequest"] = map[string]any{"chatId": request.Chat.ID, "chatTitle": request.Chat.Title, "userId": request.From.ID, "username": request.From.Username, "date": request.Date}
	case update.MessageReaction != nil:
		event.Type = "message_reaction"
		reaction := update.MessageReaction
		payload := map[string]any{"chatId": reaction.Chat.ID, "messageId": reaction.MessageID, "date": reaction.Date}
		if reaction.User != nil {
			payload["userId"] = reaction.User.ID
			payload["username"] = reaction.User.Username
		}
		event.Payload["reaction"] = payload
	default:
		return telespec.TelegramEvent{}, false
	}
	return event, true
}

func chatMessage(message *telegramMessage) telespec.Message {
	value := telespec.Message{
		MessageID: message.MessageID, ChatID: message.Chat.ID, ChatType: message.Chat.Type,
		ChatTitle: message.Chat.Title, ChatUsername: message.Chat.Username,
		Date: message.Date, Text: message.Text,
	}
	if message.Text == "" {
		value.Text = message.Caption
	}
	if message.From != nil {
		value.FromID = message.From.ID
		value.FromUsername = message.From.Username
		value.FromName = message.From.displayName()
	}
	value.CommandText = value.Text
	return value
}

// chatAllowed applies the binding's optional comma-separated chat allowlist.
// Entries match the numeric chat ID (including -100… supergroup and channel
// forms) or the @username of public channels.
func chatAllowed(config string, event telespec.TelegramEvent) bool {
	if config == "" {
		return true
	}
	var chatID int64
	var username string
	switch event.Type {
	case "message", "edited_message", "channel_post", "edited_channel_post":
		if message, ok := event.Payload["message"].(telespec.Message); ok {
			chatID, username = message.ChatID, message.ChatUsername
		}
	case "callback_query":
		if query, ok := event.Payload["callbackQuery"].(telespec.CallbackQuery); ok {
			chatID = query.ChatID
		}
	case "my_chat_member":
		if member, ok := event.Payload["chatMember"].(telespec.ChatMemberUpdated); ok {
			chatID = member.ChatID
		}
	case "chat_join_request":
		if request, ok := event.Payload["joinRequest"].(map[string]any); ok {
			if id, ok := request["chatId"].(int64); ok {
				chatID = id
			}
		}
	case "message_reaction":
		if reaction, ok := event.Payload["reaction"].(map[string]any); ok {
			if id, ok := reaction["chatId"].(int64); ok {
				chatID = id
			}
		}
	}
	for _, entry := range splitList(config) {
		if entry == "" {
			continue
		}
		if entry == "@"+username {
			return true
		}
		if entry[0] == '@' || entry[0] == '-' || (entry[0] >= '0' && entry[0] <= '9') {
			if value, err := strconv.ParseInt(entry, 10, 64); err == nil && value == chatID {
				return true
			}
		}
	}
	return false
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		result = append(result, strings.TrimSpace(part))
	}
	return result
}

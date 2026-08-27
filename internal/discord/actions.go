package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/bwmarrin/discordgo"
)

// fetchSelf validates one bot token with a REST users/@me call. A session is
// created but never opened, so no gateway connection is made.
func (s *Service) fetchSelf(_ context.Context, token string) (*discordgo.User, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	user, err := session.User("@me")
	if err != nil {
		if strings.Contains(err.Error(), "401") {
			return nil, fmt.Errorf("discord bot token was rejected")
		}
		return nil, fmt.Errorf("validate Discord token: %w", err)
	}
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return nil, fmt.Errorf("discord returned no user for this token")
	}
	return user, nil
}

// SendDiscordMessage sends one channel message. Transport failures are hard
// errors; Discord REST rejections are soft results carrying the API's own
// message.
func (s *Service) SendDiscordMessage(ctx context.Context, request domain.DiscordMessageRequest) (domain.DiscordMessageResult, error) {
	if utf8.RuneCountInString(request.Message) > maxMessageRunes {
		return domain.DiscordMessageResult{Reason: "message exceeds Discord's 2,000-character limit"}, nil
	}
	if !validSnowflake(request.ChannelID) {
		return domain.DiscordMessageResult{Reason: fmt.Sprintf("channel ID %q is not a valid Discord ID (expected up to 20 digits)", request.ChannelID)}, nil
	}
	if request.ReplyToID != "" && !validSnowflake(request.ReplyToID) {
		return domain.DiscordMessageResult{Reason: fmt.Sprintf("reply to message ID %q is not a valid Discord message ID (expected up to 20 digits)", request.ReplyToID)}, nil
	}
	if request.ReplyToID != "" {
		if reason := replyReferenceReason(request.ChannelID, request.ReplyToID, time.Now().UTC().UnixMilli()); reason != "" {
			return domain.DiscordMessageResult{Reason: reason}, nil
		}
	}
	session, err := s.session(request.IdentityID)
	if err != nil {
		return domain.DiscordMessageResult{}, err
	}
	data := &discordgo.MessageSend{Content: request.Message}
	if request.ReplyToID != "" {
		data.Reference = &discordgo.MessageReference{MessageID: request.ReplyToID}
	}
	message, err := session.ChannelMessageSendComplex(request.ChannelID, data)
	if err != nil {
		return messageRejection(request, err), nil
	}
	return domain.DiscordMessageResult{MessageID: message.ID, Sent: true}, nil
}

func (s *Service) SendDiscordDirectMessage(ctx context.Context, request domain.DiscordDMRequest) (domain.DiscordMessageResult, error) {
	if utf8.RuneCountInString(request.Message) > maxMessageRunes {
		return domain.DiscordMessageResult{Reason: "message exceeds Discord's 2,000-character limit"}, nil
	}
	session, err := s.session(request.IdentityID)
	if err != nil {
		return domain.DiscordMessageResult{}, err
	}
	channel, err := session.UserChannelCreate(request.UserID)
	if err != nil {
		return softMessage(err), nil
	}
	message, err := session.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{Content: request.Message})
	if err != nil {
		return softMessage(err), nil
	}
	return domain.DiscordMessageResult{MessageID: message.ID, Sent: true}, nil
}

func (s *Service) AddDiscordReaction(ctx context.Context, request domain.DiscordReactionRequest) (domain.DiscordActionResult, error) {
	session, err := s.session(request.IdentityID)
	if err != nil {
		return domain.DiscordActionResult{}, err
	}
	if err := session.MessageReactionAdd(request.ChannelID, request.MessageID, request.Emoji); err != nil {
		return softAction(err), nil
	}
	return domain.DiscordActionResult{Done: true}, nil
}

func (s *Service) EditDiscordMessage(ctx context.Context, request domain.DiscordEditRequest) (domain.DiscordActionResult, error) {
	if utf8.RuneCountInString(request.Message) > maxMessageRunes {
		return domain.DiscordActionResult{Reason: "message exceeds Discord's 2,000-character limit"}, nil
	}
	session, err := s.session(request.IdentityID)
	if err != nil {
		return domain.DiscordActionResult{}, err
	}
	if _, err := session.ChannelMessageEdit(request.ChannelID, request.MessageID, request.Message); err != nil {
		return softAction(err), nil
	}
	return domain.DiscordActionResult{Done: true}, nil
}

func (s *Service) DeleteDiscordMessage(ctx context.Context, request domain.DiscordDeleteRequest) (domain.DiscordActionResult, error) {
	session, err := s.session(request.IdentityID)
	if err != nil {
		return domain.DiscordActionResult{}, err
	}
	if err := session.ChannelMessageDelete(request.ChannelID, request.MessageID); err != nil {
		return softAction(err), nil
	}
	return domain.DiscordActionResult{Done: true}, nil
}

// softMessage converts a discordgo REST error into a rejected message result.
func softMessage(err error) domain.DiscordMessageResult {
	return domain.DiscordMessageResult{Reason: restReason(err)}
}

// softAction converts a discordgo REST error into a rejected action result.
func softAction(err error) domain.DiscordActionResult {
	return domain.DiscordActionResult{Reason: restReason(err)}
}

// reasonDetailLimit bounds how much of a Discord error detail is inlined into
// the reason output pin.
const reasonDetailLimit = 400

// restReason renders a discordgo REST error as the reason shown on the
// rejected port. Discord validation failures nest field-level detail under
// an "errors" object; flattening it turns the opaque top-level message
// ("Invalid Form Body") into the actual cause ("message_reference: Unknown
// message (REPLIES_UNKNOWN_MESSAGE)").
func restReason(err error) string {
	if restError, ok := err.(*discordgo.RESTError); ok {
		if detail := restErrorDetail(restError); detail != "" {
			return detail
		}
		if restError.Message != nil && restError.Message.Message != "" {
			return restError.Message.Message
		}
		if restError.Response != nil && len(restError.ResponseBody) > 0 {
			return fmt.Sprintf("HTTP %d: %s", restError.Response.StatusCode, truncateForReason(string(restError.ResponseBody)))
		}
	}
	return err.Error()
}

// restErrorDetail flattens the nested "errors" object of a Discord REST
// rejection into "path: message (code)" fragments joined with "; ". It
// returns an empty string when the body carries no field-level detail.
func restErrorDetail(restError *discordgo.RESTError) string {
	if len(restError.ResponseBody) == 0 {
		return ""
	}
	var payload struct {
		Message string `json:"message"`
		Errors  any    `json:"errors"`
	}
	if err := json.Unmarshal(restError.ResponseBody, &payload); err != nil || payload.Errors == nil {
		return ""
	}
	var fragments []string
	var walk func(path string, value any)
	walk = func(path string, value any) {
		switch typed := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if key == "_errors" {
					for _, item := range toArray(typed[key]) {
						if entry, ok := item.(map[string]any); ok {
							fragments = append(fragments, errorFragment(path, entry))
						}
					}
					continue
				}
				child := key
				if path != "" {
					child = path + "." + key
				}
				walk(child, typed[key])
			}
		case []any:
			for index, item := range typed {
				walk(fmt.Sprintf("%s[%d]", path, index), item)
			}
		}
	}
	walk("", payload.Errors)
	if len(fragments) == 0 {
		return ""
	}
	base := payload.Message
	if base == "" {
		base = "request rejected"
	}
	return base + " — " + truncateForReason(strings.Join(fragments, "; "))
}

// errorFragment renders one _errors entry as "path: message (code)".
func errorFragment(path string, entry map[string]any) string {
	message, _ := entry["message"].(string)
	code, _ := entry["code"].(string)
	fragment := message
	if path != "" {
		fragment = path + ": " + fragment
	}
	if code != "" {
		fragment += " (" + code + ")"
	}
	return fragment
}

func toArray(value any) []any {
	if list, ok := value.([]any); ok {
		return list
	}
	return nil
}

func truncateForReason(text string) string {
	runes := []rune(text)
	if len(runes) > reasonDetailLimit {
		return string(runes[:reasonDetailLimit]) + "..."
	}
	return text
}

// discordEpochMS is the Discord snowflake epoch, 2015-01-01T00:00:00 UTC.
// Every Discord snowflake encodes its entity's creation time as milliseconds
// since this moment, shifted left by 22 bits.
const discordEpochMS = int64(1420070400000)

// futureSnowflakeSkewMS is the clock drift tolerated before a snowflake that
// decodes past the current moment is treated as invalid.
const futureSnowflakeSkewMS = int64(10 * 60 * 1000)

// snowflakeTimeMS decodes the creation timestamp embedded in a Discord
// snowflake. Messages and channels alike carry the moment they were created,
// which makes a reply reference that predates its target channel provably
// impossible.
func snowflakeTimeMS(id string) (int64, bool) {
	value, err := strconv.ParseUint(id, 10, 64)
	if err != nil || value == 0 {
		return 0, false
	}
	return discordEpochMS + int64(value>>22), true
}

// snowflakeMonth renders a decoded snowflake timestamp as "Jan 2006", the
// precision snowflake forensics can honestly claim.
func snowflakeMonth(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("Jan 2006")
}

// replyReferenceReason rejects reply references no Discord channel can
// resolve before the request is made. A message cannot have been created
// before the channel it is replied to, nor in the future: an ID decoding that
// way was truncated or miscopied by hand (a current message ID has 19 digits),
// and sending it would only come back as MESSAGE_REFERENCE_UNKNOWN_MESSAGE.
func replyReferenceReason(channelID, replyID string, nowMS int64) string {
	replyMS, ok := snowflakeTimeMS(replyID)
	if !ok {
		return ""
	}
	if replyMS > nowMS+futureSnowflakeSkewMS {
		return fmt.Sprintf("reply to message ID %q decodes to %s, in the future — the ID is invalid; wire the trigger's Message ID output instead of typing it by hand", replyID, snowflakeMonth(replyMS))
	}
	channelMS, ok := snowflakeTimeMS(channelID)
	if !ok {
		return ""
	}
	if replyMS < channelMS {
		return fmt.Sprintf("reply to message ID %q decodes to %s, before channel %s existed (created %s) — the reply ID looks truncated or miscopied; wire the trigger's Message ID output instead of typing it by hand", replyID, snowflakeMonth(replyMS), channelID, snowflakeMonth(channelMS))
	}
	return ""
}

// messageRejection renders a REST rejection as a soft result. When the request
// carried a reply reference and Discord could not resolve it, the reason gains
// the two real-world causes and the reliable wiring.
func messageRejection(request domain.DiscordMessageRequest, err error) domain.DiscordMessageResult {
	result := softMessage(err)
	if request.ReplyToID == "" || !referencesUnknownMessage(result.Reason) {
		return result
	}
	guidance := fmt.Sprintf(" — the reply target does not exist in channel %s; the referenced message may live in another channel or thread, or the ID is wrong. Wire the trigger's Message ID and Channel ID outputs instead of typing IDs", request.ChannelID)
	result.Reason = truncateForReason(result.Reason + guidance)
	return result
}

// referencesUnknownMessage reports whether a Discord rejection reason says the
// referenced message could not be resolved, either as a nested form-body error
// (MESSAGE_REFERENCE_UNKNOWN_MESSAGE, REPLIES_UNKNOWN_MESSAGE) or as a plain
// Unknown Message REST error.
func referencesUnknownMessage(reason string) bool {
	lowered := strings.ToLower(reason)
	return strings.Contains(lowered, "message_reference_unknown_message") ||
		strings.Contains(lowered, "replies_unknown_message") ||
		strings.Contains(lowered, "unknown message")
}

// validSnowflake reports whether id plausibly identifies a Discord entity:
// non-empty, all ASCII digits, at most 20 digits, and inside the uint64
// range. It catches values mangled by numeric pipelines (scientific
// notation, truncated pastes) before they hit the API.
func validSnowflake(id string) bool {
	if id == "" || len(id) > 20 {
		return false
	}
	for _, char := range id {
		if char < '0' || char > '9' {
			return false
		}
	}
	value, err := strconv.ParseUint(id, 10, 64)
	return err == nil && value > 0
}

// Service satisfies the node module port so Desktop can inject it directly.
var _ nodes.DiscordSender = (*Service)(nil)

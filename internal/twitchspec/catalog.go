// Package twitchspec contains the stable EventSub contracts shared by the
// Twitch infrastructure and first-party Twitch node modules. It has no Wails,
// persistence, transport, or graph-engine dependencies.
package twitchspec

import (
	"sort"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Event is the common, JSON-safe EventSub delivery contract. Event-specific
// nodes expose their selected typed convenience fields in addition to this
// complete envelope; unrecognised Twitch additions remain isolated in Payload
// rather than weakening other pins to an implicit conversion.
type Event struct {
	Type           string         `json:"type"`
	Version        string         `json:"version"`
	MessageID      string         `json:"messageId"`
	SubscriptionID string         `json:"subscriptionId"`
	ReceivedAt     string         `json:"receivedAt"`
	Payload        map[string]any `json:"payload"`
}

func EventType() domain.TypeSpec {
	key, value := typespec.String(), typespec.Any()
	payload := domain.TypeSpec{Kind: domain.TypeMap, Key: &key, Value: &value}
	return domain.TypeSpec{Kind: domain.TypeRecord, Name: "Event", Fields: []domain.TypeFieldSpec{
		{ID: "type", Name: "type", Type: typespec.String()},
		{ID: "version", Name: "version", Type: typespec.String()},
		{ID: "messageId", Name: "messageId", Type: typespec.String()},
		{ID: "subscriptionId", Name: "subscriptionId", Type: typespec.String()},
		{ID: "receivedAt", Name: "receivedAt", Type: typespec.String()},
		{ID: "payload", Name: "payload", Type: payload},
	}}
}

// ChatMessage describes the chat-specific portion of channel.chat.message.
// It is intentionally a native named value so strict runtime TypeSpecs can
// verify it without treating a map as a hidden any value.
type ChatMessage struct {
	Text          string `json:"text"`
	CommandText   string `json:"commandText"`
	BroadcasterID string `json:"broadcasterId"`
	AuthorID      string `json:"authorId"`
	MessageID     string `json:"messageId"`
	AuthorLogin   string `json:"authorLogin"`
	AuthorName    string `json:"authorName"`
	ChannelLogin  string `json:"channelLogin"`
}

// TwitchAuthor is the native value behind the trigger's named Author record.
type TwitchAuthor struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	ID    string `json:"id"`
}

func ChatMessageType() domain.TypeSpec {
	return domain.TypeSpec{Kind: domain.TypeRecord, Name: "ChatMessage", Fields: []domain.TypeFieldSpec{
		{ID: "text", Name: "text", Type: typespec.String()},
		{ID: "commandText", Name: "commandText", Type: typespec.String()},
		{ID: "broadcasterId", Name: "broadcasterId", Type: typespec.String()},
		{ID: "authorId", Name: "authorId", Type: typespec.String()},
		{ID: "messageId", Name: "messageId", Type: typespec.String()},
		{ID: "authorLogin", Name: "authorLogin", Type: typespec.String()},
		{ID: "authorName", Name: "authorName", Type: typespec.String()},
		{ID: "channelLogin", Name: "channelLogin", Type: typespec.String()},
	}}
}

// Catalog returns the WebSocket-compatible, non-deprecated event types which
// Neuropipe currently understands. Every entry has a deterministic version,
// conditions, scopes, and a strict envelope contract.
func Catalog() []domain.TwitchEventDescriptor {
	broadcaster := []domain.TwitchEventConditionField{{ID: "broadcaster_user_id", Label: "Broadcaster user ID", Description: "The numeric Twitch user ID of the channel that owns this subscription; this is not the channel login.", Required: true}}
	moderator := append(append([]domain.TwitchEventConditionField{}, broadcaster...), domain.TwitchEventConditionField{ID: "moderator_user_id", Label: "Moderator ID", Description: "The moderator identity used for this subscription.", Required: true})
	chat := append(append([]domain.TwitchEventConditionField{}, broadcaster...), domain.TwitchEventConditionField{ID: "user_id", Label: "Bot user ID", Description: "The connected chat identity.", Required: true})
	user := []domain.TwitchEventConditionField{{ID: "user_id", Label: "User ID", Description: "The connected Twitch identity.", Required: true}}
	extension := []domain.TwitchEventConditionField{{ID: "extension_client_id", Label: "Extension client ID", Description: "The Twitch extension client ID.", Required: true}}
	organization := []domain.TwitchEventConditionField{{ID: "organization_id", Label: "Organization ID", Description: "The Twitch organization ID.", Required: true}}
	entries := []domain.TwitchEventDescriptor{
		eventVersion("automod.message.hold", "2", "AutoMod message held", "A public blocked chat message is held for review.", []string{"moderator:manage:automod"}, moderator, false),
		eventVersion("automod.message.update", "2", "AutoMod message updated", "A held AutoMod message changes status.", []string{"moderator:manage:automod"}, moderator, false),
		event("automod.settings.update", "AutoMod settings updated", "A broadcaster's AutoMod settings change.", []string{"moderator:read:automod_settings"}, moderator, false),
		event("automod.terms.update", "AutoMod terms updated", "A broadcaster's public blocked terms change.", []string{"moderator:manage:automod"}, moderator, false),
		event("channel.bits.use", "Bits used", "Bits are used in a channel.", []string{"bits:read"}, broadcaster, false),
		event("channel.chat.message", "Channel chat message", "A message is sent in a channel chat.", []string{"user:read:chat"}, chat, true),
		event("channel.chat.clear", "Channel chat cleared", "All messages are cleared from a channel chat.", []string{"user:read:chat"}, chat, false),
		event("channel.chat.clear_user_messages", "Channel user messages cleared", "A user's messages are cleared from a channel chat.", []string{"user:read:chat"}, chat, false),
		event("channel.chat.notification", "Channel chat notification", "A chat notification is sent.", []string{"user:read:chat"}, chat, false),
		event("channel.chat.message_delete", "Channel chat message deleted", "A chat message is deleted.", []string{"user:read:chat"}, chat, false),
		event("channel.chat_settings.update", "Channel chat settings updated", "A channel's chat settings change.", []string{"moderator:read:chat_settings"}, moderator, false),
		event("channel.chat.user_message_hold", "Chat user message held", "A user's chat message is held by AutoMod.", []string{"user:read:chat"}, chat, false),
		event("channel.chat.user_message_update", "Chat user message updated", "A held chat message changes AutoMod status.", []string{"user:read:chat"}, chat, false),
		eventVersion("channel.follow", "2", "Channel follow", "A user follows a channel.", []string{"moderator:read:followers"}, moderator, false),
		event("channel.subscribe", "Channel subscribe", "A user subscribes to a channel.", []string{"channel:read:subscriptions"}, broadcaster, false),
		event("channel.subscription.end", "Subscription ended", "A subscription ends.", []string{"channel:read:subscriptions"}, broadcaster, false),
		event("channel.subscription.gift", "Subscription gift", "Subscriptions are gifted.", []string{"channel:read:subscriptions"}, broadcaster, false),
		event("channel.subscription.message", "Subscription message", "A subscriber sends a resubscription message.", []string{"channel:read:subscriptions"}, broadcaster, false),
		event("channel.cheer", "Channel cheer", "A viewer cheers with Bits.", []string{"bits:read"}, broadcaster, false),
		event("channel.raid", "Channel raid", "A channel raid starts.", []string{}, []domain.TwitchEventConditionField{{ID: "to_broadcaster_user_id", Label: "Target broadcaster ID", Description: "The broadcaster being raided.", Required: false}, {ID: "from_broadcaster_user_id", Label: "Source broadcaster ID", Description: "The broadcaster starting a raid.", Required: false}}, false),
		eventVersion("channel.update", "2", "Channel updated", "Channel title, category, or other settings change.", []string{"channel:manage:broadcast"}, broadcaster, false),
		event("stream.online", "Stream online", "A broadcaster goes live.", []string{}, broadcaster, false),
		event("stream.offline", "Stream offline", "A broadcaster goes offline.", []string{}, broadcaster, false),
		event("channel.channel_points_custom_reward_redemption.add", "Channel point redemption", "A channel point reward is redeemed.", []string{"channel:read:redemptions"}, broadcaster, false),
		event("channel.channel_points_custom_reward_redemption.update", "Channel point redemption updated", "A channel point redemption status changes.", []string{"channel:read:redemptions"}, broadcaster, false),
		eventVersion("channel.channel_points_automatic_reward_redemption.add", "2", "Automatic reward redemption", "An automatic channel point reward is redeemed.", []string{"channel:read:redemptions"}, broadcaster, false),
		event("channel.channel_points_custom_reward.add", "Channel point reward created", "A custom channel point reward is created.", []string{"channel:manage:redemptions"}, broadcaster, false),
		event("channel.channel_points_custom_reward.update", "Channel point reward updated", "A custom channel point reward changes.", []string{"channel:manage:redemptions"}, broadcaster, false),
		event("channel.channel_points_custom_reward.remove", "Channel point reward removed", "A custom channel point reward is removed.", []string{"channel:manage:redemptions"}, broadcaster, false),
		event("channel.custom_power_up_redemption.add", "Custom power-up redeemed", "A viewer redeems a custom power-up.", []string{"channel:read:redemptions"}, broadcaster, false),
		event("channel.poll.begin", "Poll started", "A channel poll starts.", []string{"channel:read:polls"}, broadcaster, false),
		event("channel.poll.progress", "Poll progress", "A channel poll changes.", []string{"channel:read:polls"}, broadcaster, false),
		event("channel.poll.end", "Poll ended", "A channel poll ends.", []string{"channel:read:polls"}, broadcaster, false),
		event("channel.prediction.begin", "Prediction started", "A prediction starts.", []string{"channel:read:predictions"}, broadcaster, false),
		event("channel.prediction.progress", "Prediction progress", "A prediction changes.", []string{"channel:read:predictions"}, broadcaster, false),
		event("channel.prediction.lock", "Prediction locked", "A prediction locks.", []string{"channel:read:predictions"}, broadcaster, false),
		event("channel.prediction.end", "Prediction ended", "A prediction resolves.", []string{"channel:read:predictions"}, broadcaster, false),
		event("channel.goal.begin", "Goal started", "A creator goal starts.", []string{"channel:read:goals"}, broadcaster, false),
		event("channel.goal.progress", "Goal progress", "A creator goal changes.", []string{"channel:read:goals"}, broadcaster, false),
		event("channel.goal.end", "Goal ended", "A creator goal ends.", []string{"channel:read:goals"}, broadcaster, false),
		eventVersion("channel.hype_train.begin", "2", "Hype Train started", "A Hype Train starts.", []string{"channel:read:hype_train"}, broadcaster, false),
		eventVersion("channel.hype_train.progress", "2", "Hype Train progress", "A Hype Train changes.", []string{"channel:read:hype_train"}, broadcaster, false),
		eventVersion("channel.hype_train.end", "2", "Hype Train ended", "A Hype Train ends.", []string{"channel:read:hype_train"}, broadcaster, false),
		event("channel.ban", "Channel ban", "A user is banned or timed out.", []string{"moderator:read:banned_users"}, moderator, false),
		event("channel.unban", "Channel unban", "A user is unbanned.", []string{"moderator:read:banned_users"}, moderator, false),
		event("channel.unban_request.create", "Unban request created", "A user creates an unban request.", []string{"moderator:read:unban_requests"}, moderator, false),
		event("channel.unban_request.resolve", "Unban request resolved", "An unban request is resolved.", []string{"moderator:read:unban_requests"}, moderator, false),
		eventVersion("channel.moderate", "2", "Channel moderated", "A moderator performs a moderation action.", []string{"channel:moderate"}, moderator, false),
		event("channel.moderator.add", "Moderator added", "A moderator is added to a channel.", []string{"channel:manage:moderators"}, broadcaster, false),
		event("channel.moderator.remove", "Moderator removed", "A moderator is removed from a channel.", []string{"channel:manage:moderators"}, broadcaster, false),
		event("channel.vip.add", "VIP added", "A channel VIP is added.", []string{"channel:read:vips"}, broadcaster, false),
		event("channel.vip.remove", "VIP removed", "A channel VIP is removed.", []string{"channel:read:vips"}, broadcaster, false),
		event("channel.ad_break.begin", "Ad break", "An ad break begins.", []string{"channel:read:ads"}, broadcaster, false),
		event("channel.shield_mode.begin", "Shield mode enabled", "Shield mode is enabled.", []string{"moderator:read:shield_mode"}, moderator, false),
		event("channel.shield_mode.end", "Shield mode disabled", "Shield mode is disabled.", []string{"moderator:read:shield_mode"}, moderator, false),
		event("channel.suspicious_user.message", "Suspicious user message", "A suspicious user posts in chat.", []string{"moderator:read:suspicious_users"}, moderator, false),
		event("channel.suspicious_user.update", "Suspicious user updated", "A suspicious user's status changes.", []string{"moderator:read:suspicious_users"}, moderator, false),
		event("channel.warning.send", "Warning sent", "A moderator warns a user.", []string{"moderator:read:warnings"}, moderator, false),
		event("channel.warning.acknowledge", "Warning acknowledged", "A warned user acknowledges the warning.", []string{"moderator:read:warnings"}, moderator, false),
		event("channel.charity_campaign.donate", "Charity donation", "A viewer donates to a charity campaign.", []string{"channel:read:charity"}, broadcaster, false),
		event("channel.charity_campaign.start", "Charity campaign started", "A charity campaign starts.", []string{"channel:read:charity"}, broadcaster, false),
		event("channel.charity_campaign.progress", "Charity campaign progress", "A charity campaign changes.", []string{"channel:read:charity"}, broadcaster, false),
		event("channel.charity_campaign.stop", "Charity campaign stopped", "A charity campaign stops.", []string{"channel:read:charity"}, broadcaster, false),
		event("channel.shared_chat.begin", "Shared chat started", "A shared chat session starts.", []string{}, broadcaster, false),
		event("channel.shared_chat.update", "Shared chat updated", "A shared chat session changes.", []string{}, broadcaster, false),
		event("channel.shared_chat.end", "Shared chat ended", "A shared chat session ends.", []string{}, broadcaster, false),
		event("channel.shoutout.create", "Shoutout created", "A broadcaster sends a shoutout.", []string{"moderator:read:shoutouts"}, moderator, false),
		event("channel.shoutout.receive", "Shoutout received", "A broadcaster receives a shoutout.", []string{"moderator:read:shoutouts"}, moderator, false),
		event("drop.entitlement.grant", "Drop entitlement granted", "A Twitch Drop entitlement is granted.", []string{}, organization, false),
		event("extension.bits_transaction.create", "Extension Bits transaction", "A Bits transaction occurs in an extension.", []string{}, extension, false),
		event("user.authorization.grant", "Authorization granted", "A user authorizes the application.", []string{}, []domain.TwitchEventConditionField{{ID: "client_id", Label: "Client ID", Description: "Your Twitch application client ID.", Required: true}}, false),
		event("user.authorization.revoke", "Authorization revoked", "A user revokes the application authorization.", []string{}, []domain.TwitchEventConditionField{{ID: "client_id", Label: "Client ID", Description: "Your Twitch application client ID.", Required: true}}, false),
		event("user.update", "User updated", "A user updates their Twitch account.", []string{}, user, false),
		event("user.whisper.message", "Whisper received", "A user receives a whisper.", []string{"user:read:whispers"}, user, false),
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Type < entries[j].Type })
	return entries
}

func event(eventType, label, description string, scopes []string, conditions []domain.TwitchEventConditionField, chat bool) domain.TwitchEventDescriptor {
	return eventVersion(eventType, "1", label, description, scopes, conditions, chat)
}

func eventVersion(eventType, version, label, description string, scopes []string, conditions []domain.TwitchEventConditionField, chat bool) domain.TwitchEventDescriptor {
	return domain.TwitchEventDescriptor{Type: eventType, Version: version, Label: label, Description: description, RequiredScopes: append([]string(nil), scopes...), Conditions: append([]domain.TwitchEventConditionField(nil), conditions...), EventType: EventType(), ChatMessage: chat}
}

func Find(eventType string) (domain.TwitchEventDescriptor, bool) {
	for _, descriptor := range Catalog() {
		if descriptor.Type == strings.TrimSpace(eventType) {
			return descriptor, true
		}
	}
	return domain.TwitchEventDescriptor{}, false
}

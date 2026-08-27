package discord

import (
	"context"
	"fmt"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/discordspec"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/bwmarrin/discordgo"
)

// registerHandlers subscribes the session to every gateway event the catalog
// understands. The intent bitmask controls what Discord actually delivers;
// handlers for events the intents exclude are simply never invoked.
func (s *Service) registerHandlers(session *discordgo.Session, identityID string) {
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageCreate) {
		s.handleMessage(identityID, "message.create", "MESSAGE_CREATE", m.Message, session)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageUpdate) {
		s.handleMessage(identityID, "message.update", "MESSAGE_UPDATE", m.Message, session)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageDelete) {
		payload := map[string]any{"messageId": m.ID, "channelId": m.ChannelID, "guildId": m.GuildID}
		s.emitEvent(identityID, "message.delete", "MESSAGE_DELETE", m.ID, payload)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageReactionAdd) {
		s.handleReaction(identityID, "message.reaction.add", "MESSAGE_REACTION_ADD", m.MessageReaction)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageReactionRemove) {
		s.handleReaction(identityID, "message.reaction.remove", "MESSAGE_REACTION_REMOVE", m.MessageReaction)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageReactionRemoveAll) {
		payload := map[string]any{"messageId": m.MessageID, "channelId": m.ChannelID, "guildId": m.GuildID}
		s.emitEvent(identityID, "message.reaction.remove_all", "MESSAGE_REACTION_REMOVE_ALL", m.MessageID, payload)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.GuildMemberAdd) {
		s.handleMember(identityID, "guild.member.add", "GUILD_MEMBER_ADD", m.Member)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.GuildMemberRemove) {
		s.handleMember(identityID, "guild.member.remove", "GUILD_MEMBER_REMOVE", m.Member)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.GuildMemberUpdate) {
		s.handleMember(identityID, "guild.member.update", "GUILD_MEMBER_UPDATE", m.Member)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.GuildBanAdd) {
		s.handleBan(identityID, "guild.ban.add", "GUILD_BAN_ADD", m.User, m.GuildID)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.GuildBanRemove) {
		s.handleBan(identityID, "guild.ban.remove", "GUILD_BAN_REMOVE", m.User, m.GuildID)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.InteractionCreate) {
		s.handleInteraction(identityID, m.Interaction)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.VoiceStateUpdate) {
		payload := map[string]any{"userId": m.UserID, "channelId": m.ChannelID, "guildId": m.GuildID, "sessionId": m.SessionID, "selfMute": m.SelfMute, "selfDeaf": m.SelfDeaf}
		s.emitEvent(identityID, "voice.state.update", "VOICE_STATE_UPDATE", "", payload)
	})
}

func (s *Service) handleMessage(identityID, eventType, gatewayEvent string, message *discordgo.Message, session *discordgo.Session) {
	if message == nil {
		return
	}
	chat := discordspec.ChatMessage{
		Text: message.Content, MessageID: message.ID, ChannelID: message.ChannelID, GuildID: message.GuildID,
		ChannelName: stateChannelName(session, message.ChannelID), GuildName: stateGuildName(session, message.GuildID),
	}
	if message.Author != nil {
		chat.AuthorID, chat.AuthorUsername, chat.AuthorBot = message.Author.ID, message.Author.Username, message.Author.Bot
	}
	payload := map[string]any{
		"chatMessage": chat,
		"messageId":   message.ID,
		"channelId":   message.ChannelID,
		"guildId":     message.GuildID,
		"authorId":    chat.AuthorID,
		"content":     message.Content,
	}
	s.emitEvent(identityID, eventType, gatewayEvent, message.ID, payload)
}

func (s *Service) handleReaction(identityID, eventType, gatewayEvent string, reaction *discordgo.MessageReaction) {
	if reaction == nil {
		return
	}
	emoji := reaction.Emoji.Name
	if reaction.Emoji.ID != "" {
		emoji = reaction.Emoji.Name + ":" + reaction.Emoji.ID
	}
	typed := discordspec.Reaction{MessageID: reaction.MessageID, ChannelID: reaction.ChannelID, GuildID: reaction.GuildID, UserID: reaction.UserID, Emoji: emoji}
	payload := map[string]any{
		"reaction": typed, "messageId": reaction.MessageID, "channelId": reaction.ChannelID,
		"guildId": reaction.GuildID, "userId": reaction.UserID, "emoji": emoji,
	}
	s.emitEvent(identityID, eventType, gatewayEvent, reaction.MessageID, payload)
}

func (s *Service) handleMember(identityID, eventType, gatewayEvent string, member *discordgo.Member) {
	if member == nil {
		return
	}
	typed := discordspec.Member{GuildID: member.GuildID, Nickname: member.Nick}
	if !member.JoinedAt.IsZero() {
		typed.JoinedAt = member.JoinedAt.UTC().Format(time.RFC3339Nano)
	}
	if member.User != nil {
		typed.UserID, typed.Username = member.User.ID, member.User.Username
	}
	payload := map[string]any{"member": typed, "guildId": member.GuildID, "userId": typed.UserID, "username": typed.Username, "nickname": member.Nick}
	s.emitEvent(identityID, eventType, gatewayEvent, "", payload)
}

func (s *Service) handleBan(identityID, eventType, gatewayEvent string, user *discordgo.User, guildID string) {
	payload := map[string]any{"guildId": guildID}
	if user != nil {
		payload["userId"], payload["username"] = user.ID, user.Username
	}
	s.emitEvent(identityID, eventType, gatewayEvent, "", payload)
}

func (s *Service) handleInteraction(identityID string, interaction *discordgo.Interaction) {
	if interaction == nil {
		return
	}
	typed := discordspec.Interaction{ChannelID: interaction.ChannelID, GuildID: interaction.GuildID, Options: map[string]string{}}
	if interaction.Member != nil && interaction.Member.User != nil {
		typed.UserID = interaction.Member.User.ID
	} else if interaction.User != nil {
		typed.UserID = interaction.User.ID
	}
	if interaction.Type == discordgo.InteractionApplicationCommand {
		data := interaction.ApplicationCommandData()
		typed.CommandName = data.Name
		collectOptions(data.Options, typed.Options)
	}
	payload := map[string]any{
		"interaction": typed, "commandName": typed.CommandName, "options": typed.Options,
		"userId": typed.UserID, "channelId": typed.ChannelID, "guildId": typed.GuildID,
	}
	s.emitEvent(identityID, "interaction.create", "INTERACTION_CREATE", interaction.ID, payload)
}

func collectOptions(options []*discordgo.ApplicationCommandInteractionDataOption, destination map[string]string) {
	for _, option := range options {
		if option == nil {
			continue
		}
		if len(option.Options) > 0 {
			collectOptions(option.Options, destination)
			continue
		}
		if option.Value != nil {
			destination[option.Name] = fmt.Sprint(option.Value)
		} else {
			destination[option.Name] = ""
		}
	}
}

func stateChannelName(session *discordgo.Session, channelID string) string {
	if session == nil || session.State == nil {
		return ""
	}
	channel, err := session.State.Channel(channelID)
	if err != nil || channel == nil {
		return ""
	}
	return channel.Name
}

func stateGuildName(session *discordgo.Session, guildID string) string {
	if session == nil || session.State == nil {
		return ""
	}
	guild, err := session.State.Guild(guildID)
	if err != nil || guild == nil {
		return ""
	}
	return guild.Name
}

func (s *Service) emitEvent(identityID, eventType, gatewayEvent, messageID string, payload map[string]any) {
	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()
	if ctx == nil {
		ctx = context.Background()
	}
	event := discordspec.DiscordEvent{
		Type: eventType, GatewayEvent: gatewayEvent, MessageID: messageID,
		ReceivedAt: time.Now().UTC().Format(time.RFC3339Nano), Payload: payload,
	}
	s.deliver(ctx, identityID, event)
}

// deliver matches one gateway event against the trusted Discord bindings
// assigned to the polled identity. The guild and channel conditions mirror
// Twitch's server-side subscription match; prefix and author filters are the
// trigger node's responsibility.
func (s *Service) deliver(ctx context.Context, identityID string, event discordspec.DiscordEvent) {
	bindings, err := s.listBindings(ctx)
	if err != nil {
		return
	}
	s.mu.RLock()
	defaultID := s.settings.DefaultBotIdentityID
	s.mu.RUnlock()
	for _, binding := range bindings {
		if !binding.Enabled || !binding.Trusted || stringConfig(binding.Config, "eventType") != event.Type {
			continue
		}
		bindingIdentity := stringConfig(binding.Config, "identityId")
		if bindingIdentity == "" {
			bindingIdentity = defaultID
		}
		if bindingIdentity != identityID {
			continue
		}
		if !conditionMatches(binding.Config, event.Payload) {
			continue
		}
		if s.runner == nil {
			continue
		}
		_, _ = s.runner.QueueBinding(ctx, binding.ID, pipeline.Packet{"event": event}, true)
	}
}

// conditionMatches applies the binding's optional guild and channel filters
// against the event payload.
func conditionMatches(config map[string]any, payload map[string]any) bool {
	if guildID := stringConfig(config, "guildId"); guildID != "" {
		if fmt.Sprint(payload["guildId"]) != guildID {
			return false
		}
	}
	if channelID := stringConfig(config, "channelId"); channelID != "" {
		if fmt.Sprint(payload["channelId"]) != channelID {
			return false
		}
	}
	return true
}

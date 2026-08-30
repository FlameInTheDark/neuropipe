package discord

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/bwmarrin/discordgo"
)

/* ------------------------------------------------------------------ */
/* Interaction responses                                               */
/* ------------------------------------------------------------------ */

// interactionFromRef rebuilds the minimal transport interaction the
// response endpoints need: they address everything through the interaction
// id, application id, and token.
func interactionFromRef(reference domain.DiscordInteractionRef) *discordgo.Interaction {
	return &discordgo.Interaction{
		ID:    reference.InteractionID,
		AppID: reference.ApplicationID,
		Token: reference.Token,
	}
}

// RespondDiscordInteraction answers one application-command interaction.
// Manual mode (the trigger did not auto-defer) sends the initial callback
// with the message, optionally ephemeral. Deferred mode replaces the
// loading placeholder by editing the original response. Transport failures
// are hard errors; Discord REST rejections are soft results carrying the
// API's own message.
func (s *Service) RespondDiscordInteraction(ctx context.Context, request domain.DiscordCommandReplyRequest) (domain.DiscordMessageResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.DiscordMessageResult{}, err
	}
	if reason := validateInteractionReply(request.Interaction, request.Message, request.Embeds); reason != "" {
		return domain.DiscordMessageResult{Reason: reason}, nil
	}
	if request.Ephemeral && request.Interaction.Deferred {
		return domain.DiscordMessageResult{Reason: "ephemeral replies need the trigger's manual response mode — a deferred response is already public"}, nil
	}
	session, err := s.session(request.IdentityID)
	if err != nil {
		return domain.DiscordMessageResult{}, err
	}
	interaction := interactionFromRef(request.Interaction)
	if request.Interaction.Deferred {
		content := request.Message
		embeds := transportEmbeds(request.Embeds)
		message, err := session.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &content, Embeds: &embeds,
		})
		if err != nil {
			return softMessage(err), nil
		}
		return domain.DiscordMessageResult{MessageID: messageID(message), Sent: true}, nil
	}
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: request.Message, Embeds: transportEmbeds(request.Embeds),
		},
	}
	if request.Ephemeral {
		response.Data.Flags = discordgo.MessageFlagsEphemeral
	}
	if err := session.InteractionRespond(interaction, response); err != nil {
		return softMessage(err), nil
	}
	return domain.DiscordMessageResult{Sent: true}, nil
}

// SendDiscordFollowup sends one additional followup message while the
// interaction token is valid (15 minutes after the command ran).
func (s *Service) SendDiscordFollowup(ctx context.Context, request domain.DiscordFollowupRequest) (domain.DiscordMessageResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.DiscordMessageResult{}, err
	}
	if reason := validateInteractionReply(request.Interaction, request.Message, request.Embeds); reason != "" {
		return domain.DiscordMessageResult{Reason: reason}, nil
	}
	if !request.Interaction.Deferred {
		return domain.DiscordMessageResult{Reason: "followups need the trigger's auto-defer response mode — reply to the command first, then send followups"}, nil
	}
	session, err := s.session(request.IdentityID)
	if err != nil {
		return domain.DiscordMessageResult{}, err
	}
	message, err := session.FollowupMessageCreate(interactionFromRef(request.Interaction), true, &discordgo.WebhookParams{
		Content: request.Message, Embeds: transportEmbeds(request.Embeds),
	})
	if err != nil {
		return softMessage(err), nil
	}
	return domain.DiscordMessageResult{MessageID: messageID(message), Sent: true}, nil
}

// EditDiscordInteractionMessage edits the original interaction reply (empty
// MessageID) or one followup message by id.
func (s *Service) EditDiscordInteractionMessage(ctx context.Context, request domain.DiscordCommandEditRequest) (domain.DiscordActionResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.DiscordActionResult{}, err
	}
	if reason := validateInteractionRef(request.Interaction); reason != "" {
		return domain.DiscordActionResult{Reason: reason}, nil
	}
	if utf8.RuneCountInString(request.Message) > maxMessageRunes {
		return domain.DiscordActionResult{Reason: "message exceeds Discord's 2,000-character limit"}, nil
	}
	if request.Message == "" && len(request.Embeds) == 0 {
		return domain.DiscordActionResult{Reason: "a message or at least one embed is required"}, nil
	}
	messageID := strings.TrimSpace(request.MessageID)
	if messageID != "" && messageID != "@original" && !validSnowflake(messageID) {
		return domain.DiscordActionResult{Reason: fmt.Sprintf("message ID %q is not a valid Discord message ID (expected up to 20 digits)", messageID)}, nil
	}
	if messageID == "" {
		messageID = "@original"
	}
	session, err := s.session(request.IdentityID)
	if err != nil {
		return domain.DiscordActionResult{}, err
	}
	content := request.Message
	embeds := transportEmbeds(request.Embeds)
	if _, err := session.FollowupMessageEdit(interactionFromRef(request.Interaction), messageID, &discordgo.WebhookEdit{
		Content: &content, Embeds: &embeds,
	}); err != nil {
		return softAction(err), nil
	}
	return domain.DiscordActionResult{Done: true}, nil
}

// validateInteractionReply checks the shared reply invariants: a usable
// interaction reference and a non-empty body.
func validateInteractionReply(reference domain.DiscordInteractionRef, message string, embeds []*domain.DiscordEmbed) string {
	if reason := validateInteractionRef(reference); reason != "" {
		return reason
	}
	if utf8.RuneCountInString(message) > maxMessageRunes {
		return "message exceeds Discord's 2,000-character limit"
	}
	if strings.TrimSpace(message) == "" && len(embeds) == 0 {
		return "a message or at least one embed is required"
	}
	return ""
}

// validateInteractionRef reports whether the handoff object carries the
// three identifiers every interaction endpoint needs.
func validateInteractionRef(reference domain.DiscordInteractionRef) string {
	if strings.TrimSpace(reference.Token) == "" {
		return "wire the Command Trigger's Interaction output into this node — the interaction token is required"
	}
	if !validSnowflake(reference.InteractionID) {
		return fmt.Sprintf("interaction ID %q is not a valid Discord ID (expected up to 20 digits)", reference.InteractionID)
	}
	if !validSnowflake(reference.ApplicationID) {
		return fmt.Sprintf("application ID %q is not a valid Discord ID (expected up to 20 digits)", reference.ApplicationID)
	}
	return ""
}

func messageID(message *discordgo.Message) string {
	if message == nil {
		return ""
	}
	return message.ID
}

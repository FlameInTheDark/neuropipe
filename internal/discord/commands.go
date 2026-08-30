package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/bwmarrin/discordgo"
)

/* ------------------------------------------------------------------ */
/* Application command management                                      */
/* ------------------------------------------------------------------ */

// maxCommandOptions is Discord's documented per-command option cap.
const maxCommandOptions = 25
const maxCommandChoices = 25

// ListDiscordGuilds returns the guilds the bot identity is a member of,
// reduced to the id/name pairs the command scope picker needs.
func (s *Service) ListDiscordGuilds(ctx context.Context, identityID string) ([]domain.DiscordGuildLite, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	session, err := s.session(identityID)
	if err != nil {
		return nil, err
	}
	guilds, err := session.UserGuilds(200, "", "", false)
	if err != nil {
		return nil, fmt.Errorf("list Discord guilds: %w", err)
	}
	result := make([]domain.DiscordGuildLite, 0, len(guilds))
	for _, guild := range guilds {
		if guild == nil {
			continue
		}
		result = append(result, domain.DiscordGuildLite{ID: guild.ID, Name: guild.Name, Icon: guild.Icon})
	}
	return result, nil
}

// ListDiscordApplicationCommands returns the commands registered on the
// bot. An empty guildID lists global commands; otherwise the commands of
// that guild.
func (s *Service) ListDiscordApplicationCommands(ctx context.Context, identityID, guildID string) ([]domain.DiscordApplicationCommand, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if guildID != "" && !validSnowflake(guildID) {
		return nil, fmt.Errorf("guild ID %q is not a valid Discord ID (expected up to 20 digits)", guildID)
	}
	appID, session, err := s.applicationSession(identityID)
	if err != nil {
		return nil, err
	}
	commands, err := session.ApplicationCommands(appID, guildID)
	if err != nil {
		return nil, fmt.Errorf("list Discord application commands: %w", err)
	}
	result := make([]domain.DiscordApplicationCommand, 0, len(commands))
	for _, command := range commands {
		if command == nil {
			continue
		}
		result = append(result, domainCommand(command))
	}
	return result, nil
}

// CreateDiscordApplicationCommand registers one new command after local
// validation, so obvious mistakes fail fast with precise guidance instead
// of Discord's flattened form-body errors.
func (s *Service) CreateDiscordApplicationCommand(ctx context.Context, request domain.DiscordCommandRequest) (domain.DiscordApplicationCommand, error) {
	transport, err := s.prepareCommandRequest(ctx, request, false)
	if err != nil {
		return domain.DiscordApplicationCommand{}, err
	}
	appID, session, err := s.applicationSession(request.IdentityID)
	if err != nil {
		return domain.DiscordApplicationCommand{}, err
	}
	created, err := session.ApplicationCommandCreate(appID, request.GuildID, transport)
	if err != nil {
		return domain.DiscordApplicationCommand{}, fmt.Errorf("create Discord application command: %w", err)
	}
	return domainCommand(created), nil
}

// UpdateDiscordApplicationCommand edits one registered command. The
// command ID inside the request addresses it.
func (s *Service) UpdateDiscordApplicationCommand(ctx context.Context, request domain.DiscordCommandRequest) (domain.DiscordApplicationCommand, error) {
	transport, err := s.prepareCommandRequest(ctx, request, true)
	if err != nil {
		return domain.DiscordApplicationCommand{}, err
	}
	appID, session, err := s.applicationSession(request.IdentityID)
	if err != nil {
		return domain.DiscordApplicationCommand{}, err
	}
	updated, err := session.ApplicationCommandEdit(appID, request.GuildID, request.Command.ID, transport)
	if err != nil {
		return domain.DiscordApplicationCommand{}, fmt.Errorf("update Discord application command: %w", err)
	}
	return domainCommand(updated), nil
}

// DeleteDiscordApplicationCommand removes one registered command.
func (s *Service) DeleteDiscordApplicationCommand(ctx context.Context, identityID, guildID, commandID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if commandID == "" || !validSnowflake(commandID) {
		return fmt.Errorf("command ID %q is not a valid Discord ID (expected up to 20 digits)", commandID)
	}
	if guildID != "" && !validSnowflake(guildID) {
		return fmt.Errorf("guild ID %q is not a valid Discord ID (expected up to 20 digits)", guildID)
	}
	appID, session, err := s.applicationSession(identityID)
	if err != nil {
		return err
	}
	if err := session.ApplicationCommandDelete(appID, guildID, commandID); err != nil {
		return fmt.Errorf("delete Discord application command: %w", err)
	}
	return nil
}

// applicationSession resolves the identity and its application ID. For bot
// accounts the application ID equals the bot user ID, which the identity
// already carries; a fallback REST lookup covers identities that were
// connected before the ID was captured.
func (s *Service) applicationSession(identityID string) (string, *discordgo.Session, error) {
	identity, token, err := s.identityToken(identityID)
	if err != nil {
		return "", nil, err
	}
	session, err := s.session(identityID)
	if err != nil {
		return "", nil, err
	}
	if identity.BotUserID != "" {
		return identity.BotUserID, session, nil
	}
	user, err := session.User("@me")
	if err != nil || user == nil || user.ID == "" {
		_ = token
		return "", nil, fmt.Errorf("resolve Discord application ID for %q", identity.Label)
	}
	return user.ID, session, nil
}

// prepareCommandRequest validates the command body and converts it to the
// transport library's wire type. requireID selects between update (ID
// required) and create (ID must be empty) semantics.
func (s *Service) prepareCommandRequest(ctx context.Context, request domain.DiscordCommandRequest, requireID bool) (*discordgo.ApplicationCommand, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.GuildID != "" && !validSnowflake(request.GuildID) {
		return nil, fmt.Errorf("guild ID %q is not a valid Discord ID (expected up to 20 digits)", request.GuildID)
	}
	command := request.Command
	if requireID {
		if command.ID == "" || !validSnowflake(command.ID) {
			return nil, fmt.Errorf("command ID %q is not a valid Discord ID (expected up to 20 digits)", command.ID)
		}
	} else if strings.TrimSpace(command.ID) != "" {
		return nil, fmt.Errorf("a new command must not carry an ID")
	}
	if reason := validateCommand(command); reason != "" {
		return nil, fmt.Errorf("%s", reason)
	}
	transport := transportCommand(command)
	transport.ID = command.ID
	return transport, nil
}

// validateCommand applies Discord's documented command rules locally.
// It returns an empty string when the command is acceptable.
func validateCommand(command domain.DiscordApplicationCommand) string {
	commandType := command.Type
	if commandType == 0 {
		commandType = domain.DiscordCommandChatInput
	}
	if commandType != domain.DiscordCommandChatInput && commandType != domain.DiscordCommandUser && commandType != domain.DiscordCommandMessage {
		return fmt.Sprintf("command type %d is not one of Discord's types 1 (slash), 2 (user), 3 (message)", commandType)
	}
	name := command.Name
	if runeLen(name) < 1 || runeLen(name) > 32 {
		return "command name must be 1-32 characters"
	}
	if commandType == domain.DiscordCommandChatInput && !validSlashName(name) {
		return fmt.Sprintf("slash command name %q must be lowercase and contain only letters, numbers, hyphens, and underscores", name)
	}
	if commandType == domain.DiscordCommandChatInput {
		if runeLen(command.Description) < 1 || runeLen(command.Description) > 100 {
			return "command description must be 1-100 characters"
		}
	} else if strings.TrimSpace(command.Description) != "" {
		return "user and message commands cannot have a description"
	}
	if len(command.Options) > maxCommandOptions {
		return fmt.Sprintf("a command can have at most %d options, got %d", maxCommandOptions, len(command.Options))
	}
	return validateOptions("options", command.Options, commandType, 0)
}

// validateOptions checks one option list and, recursively, subcommand
// nesting. Depth 0 is the command's own option list.
func validateOptions(path string, options []domain.DiscordApplicationCommandOption, commandType int, depth int) string {
	if len(options) == 0 {
		return ""
	}
	if commandType != domain.DiscordCommandChatInput {
		return "only slash commands can have options"
	}
	seenOptional := false
	seenSubcommand := false
	for index, option := range options {
		label := fmt.Sprintf("%s[%d] %q", path, index, option.Name)
		if runeLen(option.Name) < 1 || runeLen(option.Name) > 32 {
			return fmt.Sprintf("%s: option name must be 1-32 characters", label)
		}
		if !validSlashName(option.Name) {
			return fmt.Sprintf("%s: option names must be lowercase and contain only letters, numbers, hyphens, and underscores", label)
		}
		if runeLen(option.Description) < 1 || runeLen(option.Description) > 100 {
			return fmt.Sprintf("%s: option description must be 1-100 characters", label)
		}
		switch option.Type {
		case 1, 2: // subcommand / subcommand group
			if depth >= 2 {
				return fmt.Sprintf("%s: subcommands can only nest two levels (group > subcommand > options)", label)
			}
			if option.Required {
				return fmt.Sprintf("%s: subcommands cannot be required", label)
			}
			if len(option.Choices) > 0 {
				return fmt.Sprintf("%s: subcommands cannot have choices", label)
			}
			if option.MinValue != nil || option.MaxValue != nil || option.MinLength != nil || option.MaxLength != 0 {
				return fmt.Sprintf("%s: subcommands cannot have value or length constraints", label)
			}
			if len(option.Options) == 0 {
				return fmt.Sprintf("%s: a subcommand needs at least one option", label)
			}
			if reason := validateOptions(label, option.Options, commandType, depth+1); reason != "" {
				return reason
			}
			seenSubcommand = true
		case 3, 4, 5, 6, 7, 8, 9, 10, 11:
			if seenSubcommand {
				return fmt.Sprintf("%s: a command cannot mix subcommands with value options — move every value option inside a subcommand", label)
			}
			if option.Required && seenOptional {
				return fmt.Sprintf("%s: required options must be listed before optional options", label)
			}
			if !option.Required {
				seenOptional = true
			}
			if len(option.Choices) > 0 && option.Type != 3 && option.Type != 4 && option.Type != 10 {
				return fmt.Sprintf("%s: choices are only allowed on text, integer, and number options", label)
			}
			if len(option.Choices) > maxCommandChoices {
				return fmt.Sprintf("%s: an option can have at most %d choices, got %d", label, maxCommandChoices, len(option.Choices))
			}
			for choiceIndex, choice := range option.Choices {
				choiceLabel := fmt.Sprintf("%s choice %d", label, choiceIndex)
				if runeLen(choice.Name) < 1 || runeLen(choice.Name) > 100 {
					return fmt.Sprintf("%s: choice names must be 1-100 characters", choiceLabel)
				}
				if choice.Value == nil || fmt.Sprint(choice.Value) == "" {
					return fmt.Sprintf("%s: a choice needs a value", choiceLabel)
				}
			}
			if option.Autocomplete && len(option.Choices) > 0 {
				return fmt.Sprintf("%s: autocomplete options cannot also define choices", label)
			}
			if option.Type == 4 || option.Type == 10 {
				if option.MinValue != nil && option.MaxValue != nil && *option.MinValue > *option.MaxValue {
					return fmt.Sprintf("%s: minimum value %.2f is above the maximum %.2f", label, *option.MinValue, *option.MaxValue)
				}
			}
			if option.Type == 3 {
				if option.MinLength != nil && option.MaxLength != 0 && *option.MinLength > option.MaxLength {
					return fmt.Sprintf("%s: minimum length %d is above the maximum %d", label, *option.MinLength, option.MaxLength)
				}
			}
		default:
			return fmt.Sprintf("%s: option type %d is not one of Discord's option types 1-11", label, option.Type)
		}
	}
	return ""
}

// validSlashName reports whether the name follows Discord's command and
// option naming rules: lowercase word characters and hyphens.
func validSlashName(name string) bool {
	if name == "" || runeLen(name) > 32 {
		return false
	}
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func runeLen(value string) int { return len([]rune(value)) }

// domainCommand converts a transport command into the domain wire type.
func domainCommand(command *discordgo.ApplicationCommand) domain.DiscordApplicationCommand {
	if command == nil {
		return domain.DiscordApplicationCommand{}
	}
	result := domain.DiscordApplicationCommand{
		ID:                      command.ID,
		ApplicationID:           command.ApplicationID,
		GuildID:                 command.GuildID,
		Version:                 command.Version,
		Type:                    int(command.Type),
		Name:                    command.Name,
		Description:             command.Description,
		DefaultMemberPermission: command.DefaultMemberPermissions,
		DMPermission:            command.DMPermission,
		NSFW:                    command.NSFW != nil && *command.NSFW,
	}
	if command.Type == 0 {
		result.Type = domain.DiscordCommandChatInput
	}
	result.Options = domainOptions(command.Options)
	return result
}

// domainOptions recursively converts the option tree.
func domainOptions(options []*discordgo.ApplicationCommandOption) []domain.DiscordApplicationCommandOption {
	if len(options) == 0 {
		return nil
	}
	result := make([]domain.DiscordApplicationCommandOption, 0, len(options))
	for _, option := range options {
		if option == nil {
			continue
		}
		converted := domain.DiscordApplicationCommandOption{
			Type:         int(option.Type),
			Name:         option.Name,
			Description:  option.Description,
			Required:     option.Required,
			MinValue:     option.MinValue,
			MaxLength:    int(option.MaxLength),
			Autocomplete: option.Autocomplete,
			Options:      domainOptions(option.Options),
		}
		if option.MinValue != nil {
			value := *option.MinValue
			converted.MinValue = &value
		}
		if option.MaxValue != 0 {
			value := option.MaxValue
			converted.MaxValue = &value
		}
		if option.MinLength != nil {
			value := *option.MinLength
			converted.MinLength = &value
		}
		if option.MaxLength != 0 {
			converted.MaxLength = int(option.MaxLength)
		}
		for _, channelType := range option.ChannelTypes {
			converted.ChannelTypes = append(converted.ChannelTypes, int(channelType))
		}
		for _, choice := range option.Choices {
			if choice == nil {
				continue
			}
			converted.Choices = append(converted.Choices, domain.DiscordApplicationCommandChoice{Name: choice.Name, Value: choice.Value})
		}
		result = append(result, converted)
	}
	return result
}

// transportCommand converts the domain command into the transport wire
// type, trimming fields Discord rejects for non-slash commands.
func transportCommand(command domain.DiscordApplicationCommand) *discordgo.ApplicationCommand {
	commandType := command.Type
	if commandType == 0 {
		commandType = domain.DiscordCommandChatInput
	}
	result := &discordgo.ApplicationCommand{
		ID:                       command.ID,
		Type:                     discordgo.ApplicationCommandType(commandType),
		Name:                     command.Name,
		DefaultMemberPermissions: command.DefaultMemberPermission,
		DMPermission:             command.DMPermission,
		Options:                  transportOptions(command.Options),
	}
	if commandType == domain.DiscordCommandChatInput {
		result.Description = command.Description
		if command.NSFW {
			value := true
			result.NSFW = &value
		}
	}
	return result
}

// transportOptions recursively converts the option tree.
func transportOptions(options []domain.DiscordApplicationCommandOption) []*discordgo.ApplicationCommandOption {
	if len(options) == 0 {
		return nil
	}
	result := make([]*discordgo.ApplicationCommandOption, 0, len(options))
	for _, option := range options {
		converted := &discordgo.ApplicationCommandOption{
			Type:         discordgo.ApplicationCommandOptionType(option.Type),
			Name:         option.Name,
			Description:  option.Description,
			Required:     option.Required,
			MaxLength:    option.MaxLength,
			Autocomplete: option.Autocomplete,
			Options:      transportOptions(option.Options),
		}
		if option.MinValue != nil {
			value := *option.MinValue
			converted.MinValue = &value
		}
		if option.MaxValue != nil {
			converted.MaxValue = *option.MaxValue
		}
		if option.MinLength != nil {
			value := *option.MinLength
			converted.MinLength = &value
		}
		for _, channelType := range option.ChannelTypes {
			converted.ChannelTypes = append(converted.ChannelTypes, discordgo.ChannelType(channelType))
		}
		for _, choice := range option.Choices {
			converted.Choices = append(converted.Choices, &discordgo.ApplicationCommandOptionChoice{Name: choice.Name, Value: choice.Value})
		}
		result = append(result, converted)
	}
	return result
}

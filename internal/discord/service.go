// Package discord owns Discord bot identity and gateway lifecycle management.
// Node modules only see the tiny nodes.DiscordSender port; they never import
// this package, discordgo, Wails, persistence, or the graph interpreter.
package discord

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/discordspec"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// Vault is deliberately smaller than the DPAPI implementation. This keeps
// token storage testable and prevents the service from learning persistence
// details.
type Vault interface {
	Get(string) (string, error)
	Put(string, string) error
	Delete(string) error
}
type BindingSource interface {
	ListTriggers(context.Context, domain.TriggerKind) ([]domain.TriggerBinding, error)
}
type ExecutionQueue interface {
	QueueBinding(context.Context, string, pipeline.Packet, bool) (domain.Execution, error)
}
type IdentitySink func(context.Context, domain.DiscordIdentity) error
type EventSink func(string, any)

// maxMessageRunes matches Discord's documented message length limit.
const maxMessageRunes = 2000

// interactionTokenTTL is how long an interaction token stays valid after
// Discord created it: 15 minutes, per the interactions documentation.
const interactionTokenTTL = 15 * time.Minute

// Service is an owned lifecycle. Every gateway session shares ctx and stops
// synchronously, so reconnects and validation cannot outlive Desktop.
type Service struct {
	vault      Vault
	bindings   BindingSource
	runner     ExecutionQueue
	onIdentity IdentitySink
	emit       EventSink

	mu       sync.RWMutex
	settings domain.DiscordSettings
	status   domain.DiscordStatus
	sessions map[string]*discordgo.Session
	acked    map[string]time.Time
	opening  sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func New(vault Vault, bindings BindingSource, runner ExecutionQueue, identities IdentitySink, emit EventSink) *Service {
	return &Service{
		vault: vault, bindings: bindings, runner: runner, onIdentity: identities, emit: emit,
		sessions: make(map[string]*discordgo.Session),
		status:   domain.DiscordStatus{ConnectionState: "stopped"},
	}
}

func (s *Service) Configure(settings domain.DiscordSettings) {
	s.mu.Lock()
	settings.Identities = append([]domain.DiscordIdentity(nil), settings.Identities...)
	s.settings = settings
	s.mu.Unlock()
	s.Reconcile()
}
func (s *Service) Status() domain.DiscordStatus             { s.mu.RLock(); defer s.mu.RUnlock(); return s.status }
func (s *Service) Catalog() []domain.DiscordEventDescriptor { return discordspec.Catalog() }

func (s *Service) Start(parent context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	s.ctx, s.cancel = context.WithCancel(parent)
	s.status.ConnectionState = "connecting"
	s.mu.Unlock()
	s.wg.Add(1)
	go s.validationLoop(s.ctx)
	s.reconcileSessions()
}
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.ctx = nil
	s.status.Connected = false
	s.status.ConnectionState = "stopped"
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		s.opening.Wait()
		s.closeSessions()
		s.wg.Wait()
	}
}

// Reconcile restarts gateway sessions so trusted-binding changes (publish,
// trust, enable, settings) immediately recompute the intent union.
func (s *Service) Reconcile() { s.reconcileSessions() }

func (s *Service) reconcileSessions() {
	s.mu.Lock()
	ctx := s.ctx
	if ctx == nil {
		s.mu.Unlock()
		return
	}
	identities := make([]domain.DiscordIdentity, 0, len(s.settings.Identities))
	for _, identity := range s.settings.Identities {
		if identity.Status == domain.DiscordIdentityConnected {
			identities = append(identities, identity)
		}
	}
	defaultID := s.settings.DefaultBotIdentityID
	s.mu.Unlock()

	s.closeSessions()
	if ctx.Err() != nil {
		return
	}

	bindings, err := s.listBindings(ctx)
	if err != nil {
		s.setError(fmt.Errorf("list Discord triggers: %w", err))
	}
	intentSets := intentSets(bindings, identities, defaultID)
	active := 0
	for _, binding := range bindings {
		if !binding.Enabled || !binding.Trusted {
			continue
		}
		if _, found := discordspec.Find(stringConfig(binding.Config, "eventType")); !found {
			continue
		}
		if !identityExists(identities, stringConfig(binding.Config, "identityId"), defaultID) {
			continue
		}
		active++
	}

	started := 0
	for _, identity := range identities {
		if ctx.Err() != nil {
			break
		}
		token, err := s.tokens(identity.ID)
		if err != nil {
			s.setError(fmt.Errorf("discord identity %q needs reconnection", identity.Label))
			continue
		}
		intents := intentSets[identity.ID]
		session, err := discordgo.New("Bot " + token)
		if err != nil {
			s.setError(fmt.Errorf("create Discord session: %w", err))
			continue
		}
		session.Identify.Intents = discordgo.Intent(intents)
		s.registerHandlers(session, identity.ID)
		s.mu.Lock()
		if s.ctx == nil {
			s.mu.Unlock()
			_ = session.Close()
			break
		}
		s.sessions[identity.ID] = session
		s.mu.Unlock()
		s.opening.Add(1)
		go func(session *discordgo.Session) {
			defer s.opening.Done()
			if err := session.Open(); err != nil {
				s.setError(fmt.Errorf("discord gateway: %w", err))
				_ = session.Close()
				s.mu.Lock()
				if s.sessions[identity.ID] == session {
					delete(s.sessions, identity.ID)
				}
				s.mu.Unlock()
			}
		}(session)
		started++
	}
	s.mu.Lock()
	s.status.ActiveSubscriptions = active
	s.status.Connected = started > 0
	if started > 0 {
		s.status.ConnectionState = "connected"
	} else if s.cancel != nil {
		s.status.ConnectionState = "stopped"
	}
	s.mu.Unlock()
}

// closeSessions closes every owned gateway session. discordgo reconnects a
// closed session internally, so Close fully terminates it.
func (s *Service) closeSessions() {
	s.mu.Lock()
	sessions := s.sessions
	s.sessions = make(map[string]*discordgo.Session)
	s.mu.Unlock()
	for _, session := range sessions {
		_ = session.Close()
	}
}

func identityExists(identities []domain.DiscordIdentity, id, defaultID string) bool {
	if id == "" {
		id = defaultID
	}
	for _, identity := range identities {
		if identity.ID == id {
			return true
		}
	}
	return false
}

// intentSets maps each connected identity to the intent union required by its
// trusted+enabled bindings. A binding belongs to its configured identity, or
// to the default identity when unconfigured. Untrusted bindings contribute
// nothing, so a pasted pipeline can never silently enable a privileged event
// stream.
func intentSets(bindings []domain.TriggerBinding, identities []domain.DiscordIdentity, defaultID string) map[string]int {
	sets := make(map[string]int)
	for _, binding := range bindings {
		if !binding.Enabled || !binding.Trusted {
			continue
		}
		identityID := stringConfig(binding.Config, "identityId")
		if identityID == "" {
			identityID = defaultID
		}
		if !identityExists(identities, identityID, defaultID) {
			continue
		}
		descriptor, found := discordspec.Find(stringConfig(binding.Config, "eventType"))
		if !found {
			continue
		}
		sets[identityID] |= descriptor.Intents
	}
	return sets
}

// listBindings is nil-safe so tests and pre-Start reconciles stay cheap.
func (s *Service) listBindings(ctx context.Context) ([]domain.TriggerBinding, error) {
	if s.bindings == nil {
		return nil, nil
	}
	return s.bindings.ListTriggers(ctx, domain.TriggerDiscord)
}

func (s *Service) validationLoop(ctx context.Context) {
	defer s.wg.Done()
	s.validateAll(ctx)
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.validateAll(ctx)
		}
	}
}
func (s *Service) validateAll(ctx context.Context) {
	s.mu.RLock()
	identities := append([]domain.DiscordIdentity(nil), s.settings.Identities...)
	s.mu.RUnlock()
	for _, identity := range identities {
		token, err := s.tokens(identity.ID)
		if err != nil {
			continue
		}
		user, err := s.fetchSelf(ctx, token)
		if err != nil {
			identity.Status = domain.DiscordIdentityInvalid
			_ = s.recordIdentity(ctx, identity)
			continue
		}
		identity.Username, identity.BotUserID = user.Username, user.ID
		identity.Status = domain.DiscordIdentityConnected
		_ = s.recordIdentity(ctx, identity)
	}
}

func (s *Service) AddManualIdentity(ctx context.Context, request domain.DiscordManualIdentityRequest) (domain.DiscordIdentity, error) {
	token := strings.TrimSpace(request.Token)
	if token == "" {
		return domain.DiscordIdentity{}, fmt.Errorf("a bot token is required")
	}
	user, err := s.fetchSelf(ctx, token)
	if err != nil {
		return domain.DiscordIdentity{}, err
	}
	if !user.Bot {
		return domain.DiscordIdentity{}, fmt.Errorf("this token does not belong to a bot account")
	}
	s.mu.RLock()
	identities := append([]domain.DiscordIdentity(nil), s.settings.Identities...)
	s.mu.RUnlock()
	for _, identity := range identities {
		if existing, err := s.tokens(identity.ID); err == nil && existing == token {
			return domain.DiscordIdentity{}, fmt.Errorf("this bot token is already connected as %q", identity.Label)
		}
	}
	identity := domain.DiscordIdentity{
		ID: uuid.NewString(), Label: strings.TrimSpace(request.Label), BotUserID: user.ID,
		Username: user.Username, Status: domain.DiscordIdentityConnected,
	}
	if identity.Label == "" {
		identity.Label = user.Username
	}
	if err := s.vault.Put(tokenKey(identity.ID), token); err != nil {
		return domain.DiscordIdentity{}, err
	}
	if err := s.recordIdentity(ctx, identity); err != nil {
		return domain.DiscordIdentity{}, err
	}
	s.Reconcile()
	return identity, nil
}
func (s *Service) RemoveIdentity(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("identity ID is required")
	}
	_ = s.vault.Delete(tokenKey(id))
	s.mu.Lock()
	identities := s.settings.Identities[:0]
	for _, item := range s.settings.Identities {
		if item.ID != id {
			identities = append(identities, item)
		}
	}
	s.settings.Identities = identities
	if s.settings.DefaultBotIdentityID == id {
		s.settings.DefaultBotIdentityID = ""
	}
	s.mu.Unlock()
	if s.onIdentity == nil {
		s.Reconcile()
		return nil
	}
	err := s.onIdentity(ctx, domain.DiscordIdentity{ID: id, Status: domain.DiscordIdentityRevoked})
	s.Reconcile()
	return err
}

func (s *Service) recordIdentity(ctx context.Context, identity domain.DiscordIdentity) error {
	s.mu.Lock()
	found := false
	for index := range s.settings.Identities {
		if s.settings.Identities[index].ID == identity.ID {
			s.settings.Identities[index] = identity
			found = true
		}
	}
	if !found && identity.Status != domain.DiscordIdentityRevoked {
		s.settings.Identities = append(s.settings.Identities, identity)
	}
	if identity.Status == domain.DiscordIdentityConnected && s.settings.DefaultBotIdentityID == "" {
		s.settings.DefaultBotIdentityID = identity.ID
	}
	s.mu.Unlock()
	if s.onIdentity == nil {
		return nil
	}
	return s.onIdentity(ctx, identity)
}

func (s *Service) identityToken(id string) (domain.DiscordIdentity, string, error) {
	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	if id == "" {
		id = settings.DefaultBotIdentityID
	}
	for _, identity := range settings.Identities {
		if identity.ID == id {
			if identity.Status != domain.DiscordIdentityConnected {
				return domain.DiscordIdentity{}, "", fmt.Errorf("discord identity %q is invalid and needs to be reconnected", identity.Label)
			}
			token, err := s.tokens(identity.ID)
			return identity, token, err
		}
	}
	return domain.DiscordIdentity{}, "", fmt.Errorf("select a connected Discord identity")
}

// session returns the live gateway session for an identity. Actions only
// need the REST half, so a session is created on demand when reconcile has
// not opened one (for example before Start).
func (s *Service) session(identityID string) (*discordgo.Session, error) {
	_, token, err := s.identityToken(identityID)
	if err != nil {
		return nil, err
	}
	id := identityID
	if id == "" {
		s.mu.RLock()
		id = s.settings.DefaultBotIdentityID
		s.mu.RUnlock()
	}
	s.mu.RLock()
	session, found := s.sessions[id]
	s.mu.RUnlock()
	if found {
		return session, nil
	}
	session, err = discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	// Actions never consume gateway events; keep the state cache light.
	session.Identify.Intents = discordgo.Intent(0)
	s.mu.Lock()
	if s.ctx != nil {
		if existing, exists := s.sessions[id]; !exists {
			s.sessions[id] = session
		} else {
			session = existing
		}
	}
	s.mu.Unlock()
	return session, nil
}

func tokenKey(id string) string { return "discord:" + id + ":token" }
func (s *Service) tokens(id string) (string, error) {
	token, err := s.vault.Get(tokenKey(id))
	if err != nil {
		return "", err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("discord bot token is missing")
	}
	return token, nil
}

func (s *Service) setError(err error) {
	message := err.Error()
	if strings.Contains(message, "4014") {
		message = "Discord refused the connection: privileged intents are not enabled for this bot. Open the Discord Developer Portal, select the application, choose Bot, and enable the required Privileged Gateway Intents."
	}
	s.mu.Lock()
	s.status.LastError = message
	s.mu.Unlock()
	if s.emit != nil {
		s.emit("discord.status.error", message)
	}
}

func stringConfig(value map[string]any, key string) string {
	raw, ok := value[key]
	if !ok {
		return ""
	}
	result, _ := raw.(string)
	return strings.TrimSpace(result)
}

// Package telegram owns Telegram Bot API long-polling lifecycle management.
// Node modules only see the tiny nodes.TelegramSender port; they never import
// this package, Wails, persistence, or the graph interpreter.
package telegram

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/FlameInTheDark/neuropipe/internal/telespec"
	"github.com/google/uuid"
)

const (
	defaultBaseURL  = "https://api.telegram.org"
	pollTimeoutSecs = 50
	// maxMessageRunes matches Telegram's documented text length limit.
	maxMessageRunes = 4096
	// maxCaptionRunes matches Telegram's documented caption length limit for
	// media messages — a document's caption is its message text.
	maxCaptionRunes = 1024
	// backlogFlushUpdateID asks Telegram for only the newest pending update so
	// a restarted loop can confirm (discard) the offline backlog instead of
	// replaying stale events.
	backlogFlushUpdateID = -1
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
type IdentitySink func(context.Context, domain.TelegramIdentity) error
type EventSink func(string, any)

// pollLoop is one owned long-polling goroutine for one bot identity. Telegram
// allows exactly one concurrent getUpdates consumer per token, so loops are
// stopped and awaited before replacements start.
type pollLoop struct {
	identityID string
	cancel     context.CancelFunc
	done       chan struct{}
}

// Service is an owned lifecycle. All background goroutines share ctx and stop
// synchronously, so poll loops and validation cannot outlive Desktop.
type Service struct {
	vault      Vault
	bindings   BindingSource
	runner     ExecutionQueue
	onIdentity IdentitySink
	emit       EventSink
	http       *http.Client
	baseURL    string

	mu       sync.RWMutex
	settings domain.TelegramSettings
	status   domain.TelegramStatus
	loops    map[string]*pollLoop
	seen     map[int64]time.Time
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func New(vault Vault, bindings BindingSource, runner ExecutionQueue, identities IdentitySink, emit EventSink) *Service {
	return &Service{
		vault: vault, bindings: bindings, runner: runner, onIdentity: identities, emit: emit,
		http: &http.Client{}, baseURL: defaultBaseURL,
		loops: make(map[string]*pollLoop), seen: make(map[int64]time.Time),
		status: domain.TelegramStatus{ConnectionState: "stopped"},
	}
}

// SetBaseURL redirects every Bot API call. Production uses the Telegram host;
// tests point it at an httptest server.
func (s *Service) SetBaseURL(url string) {
	s.mu.Lock()
	s.baseURL = strings.TrimRight(url, "/")
	s.mu.Unlock()
}

func (s *Service) Configure(settings domain.TelegramSettings) {
	s.mu.Lock()
	settings.Identities = append([]domain.TelegramIdentity(nil), settings.Identities...)
	s.settings = settings
	s.mu.Unlock()
	s.Reconcile()
}
func (s *Service) Status() domain.TelegramStatus             { s.mu.RLock(); defer s.mu.RUnlock(); return s.status }
func (s *Service) Catalog() []domain.TelegramEventDescriptor { return telespec.Catalog() }

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
	s.reconcileLoops()
}
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	loops := s.loops
	s.cancel = nil
	s.loops = make(map[string]*pollLoop)
	s.status.Connected = false
	s.status.ConnectionState = "stopped"
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		for _, loop := range loops {
			<-loop.done
		}
		s.wg.Wait()
	}
}

// Reconcile restarts poll loops so trusted-binding changes (publish, trust,
// enable, settings) immediately recompute allowed_updates.
func (s *Service) Reconcile() { s.reconcileLoops() }

func (s *Service) reconcileLoops() {
	s.mu.Lock()
	ctx := s.ctx
	if ctx == nil {
		s.mu.Unlock()
		return
	}
	loops := s.loops
	s.loops = make(map[string]*pollLoop)
	identities := make([]domain.TelegramIdentity, 0, len(s.settings.Identities))
	for _, identity := range s.settings.Identities {
		if identity.Status == domain.TelegramIdentityConnected {
			identities = append(identities, identity)
		}
	}
	defaultID := s.settings.DefaultBotIdentityID
	s.mu.Unlock()

	// A Telegram token permits exactly one concurrent getUpdates consumer, so
	// every old loop must fully exit before replacements are started.
	for _, loop := range loops {
		loop.cancel()
	}
	for _, loop := range loops {
		<-loop.done
	}
	if ctx.Err() != nil {
		return
	}

	bindings, err := s.listBindings(ctx)
	if err != nil {
		s.setError(fmt.Errorf("list Telegram triggers: %w", err))
	}
	subscriptions := subscriptionSets(bindings, identities, defaultID)
	active := 0
	for _, binding := range bindings {
		if !binding.Enabled || !binding.Trusted {
			continue
		}
		if _, found := telespec.Find(stringConfig(binding.Config, "eventType")); !found {
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
			s.setError(fmt.Errorf("telegram identity %q needs reconnection", identity.Label))
			continue
		}
		allowed := subscriptions[identity.ID]
		loopCtx, cancel := context.WithCancel(ctx)
		loop := &pollLoop{identityID: identity.ID, cancel: cancel, done: make(chan struct{})}
		s.mu.Lock()
		if s.ctx == nil {
			s.mu.Unlock()
			cancel()
			break
		}
		s.loops[identity.ID] = loop
		s.mu.Unlock()
		s.wg.Add(1)
		go s.pollLoop(loopCtx, loop, token, allowed)
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

func identityExists(identities []domain.TelegramIdentity, id, defaultID string) bool {
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

// subscriptionSets maps each connected identity to the update types its
// trusted+enabled bindings require. A binding belongs to its configured
// identity, or to the default identity when unconfigured.
func subscriptionSets(bindings []domain.TriggerBinding, identities []domain.TelegramIdentity, defaultID string) map[string][]string {
	sets := make(map[string][]string)
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
		eventType := stringConfig(binding.Config, "eventType")
		if _, found := telespec.Find(eventType); !found {
			continue
		}
		sets[identityID] = append(sets[identityID], eventType)
	}
	result := make(map[string][]string, len(sets))
	for identityID, types := range sets {
		result[identityID] = telespec.AllowedUpdates(types)
	}
	return result
}

// pollLoop owns one bot identity's getUpdates stream: it first confirms (and
// discards) the offline backlog so only live events reach pipelines, then long
// polls with a persistent offset cursor.
func (s *Service) pollLoop(ctx context.Context, loop *pollLoop, token string, allowed []string) {
	defer s.wg.Done()
	defer close(loop.done)
	offset, err := s.flushBacklog(ctx, token)
	if err != nil && ctx.Err() == nil {
		s.setError(err)
	}
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := s.getUpdates(ctx, token, offset, allowed)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.setError(err)
			if !wait(ctx, backoff) {
				return
			}
			backoff = minDuration(backoff*2, time.Minute)
			continue
		}
		backoff = time.Second
		for _, update := range updates {
			if update.UpdateID+1 > offset {
				offset = update.UpdateID + 1
			}
			s.deliver(ctx, loop.identityID, update)
		}
	}
}

// flushBacklog confirms pending updates without delivering them, so a restart
// never replays events that arrived while the app was offline. This matches
// the Twitch EventSub semantics of live-only delivery.
func (s *Service) flushBacklog(ctx context.Context, token string) (int64, error) {
	updates, err := s.getUpdates(ctx, token, backlogFlushUpdateID, nil)
	if err != nil {
		return 0, err
	}
	if len(updates) == 0 {
		return 0, nil
	}
	return updates[len(updates)-1].UpdateID + 1, nil
}

func (s *Service) getUpdates(ctx context.Context, token string, offset int64, allowed []string) ([]telegramUpdate, error) {
	body := map[string]any{"timeout": pollTimeoutSecs}
	if offset != 0 {
		body["offset"] = offset
	}
	if allowed != nil {
		body["allowed_updates"] = allowed
	}
	pollCtx, cancel := context.WithTimeout(ctx, (pollTimeoutSecs+15)*time.Second)
	defer cancel()
	var result []telegramUpdate
	if err := s.call(pollCtx, token, "getUpdates", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// deliver matches one update against the trusted Telegram bindings assigned
// to the polled identity and queues each match as an unattended run.
func (s *Service) deliver(ctx context.Context, identityID string, update telegramUpdate) {
	if !s.markUpdate(update.UpdateID) {
		return
	}
	event, ok := buildEvent(update)
	if !ok {
		return
	}
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
		if !chatAllowed(stringConfig(binding.Config, "chatIds"), event) {
			continue
		}
		if s.runner == nil {
			continue
		}
		_, _ = s.runner.QueueBinding(ctx, binding.ID, pipeline.Packet{"event": event}, true)
	}
}

// listBindings is nil-safe so tests and pre-Start reconciles stay cheap.
func (s *Service) listBindings(ctx context.Context) ([]domain.TriggerBinding, error) {
	if s.bindings == nil {
		return nil, nil
	}
	return s.bindings.ListTriggers(ctx, domain.TriggerTelegram)
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
	identities := append([]domain.TelegramIdentity(nil), s.settings.Identities...)
	s.mu.RUnlock()
	for _, identity := range identities {
		token, err := s.tokens(identity.ID)
		if err != nil {
			continue
		}
		me, err := s.getMe(ctx, token)
		if err != nil {
			identity.Status = domain.TelegramIdentityInvalid
			_ = s.recordIdentity(ctx, identity)
			continue
		}
		identity.Username, identity.BotUserID = me.Username, strconv.FormatInt(me.ID, 10)
		identity.Status = domain.TelegramIdentityConnected
		_ = s.recordIdentity(ctx, identity)
	}
}

func (s *Service) AddManualIdentity(ctx context.Context, request domain.TelegramManualIdentityRequest) (domain.TelegramIdentity, error) {
	token := strings.TrimSpace(request.Token)
	if token == "" {
		return domain.TelegramIdentity{}, fmt.Errorf("a bot token is required")
	}
	me, err := s.getMe(ctx, token)
	if err != nil {
		return domain.TelegramIdentity{}, err
	}
	if !me.IsBot {
		return domain.TelegramIdentity{}, fmt.Errorf("this token does not belong to a bot")
	}
	s.mu.RLock()
	identities := append([]domain.TelegramIdentity(nil), s.settings.Identities...)
	s.mu.RUnlock()
	for _, identity := range identities {
		if existing, err := s.tokens(identity.ID); err == nil && existing == token {
			return domain.TelegramIdentity{}, fmt.Errorf("this bot token is already connected as %q", identity.Label)
		}
	}
	identity := domain.TelegramIdentity{
		ID: uuid.NewString(), Label: strings.TrimSpace(request.Label), BotUserID: strconv.FormatInt(me.ID, 10),
		Username: me.Username, Status: domain.TelegramIdentityConnected,
	}
	if identity.Label == "" {
		identity.Label = "@" + me.Username
	}
	if err := s.vault.Put(tokenKey(identity.ID), token); err != nil {
		return domain.TelegramIdentity{}, err
	}
	if err := s.recordIdentity(ctx, identity); err != nil {
		return domain.TelegramIdentity{}, err
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
	err := s.onIdentity(ctx, domain.TelegramIdentity{ID: id, Status: domain.TelegramIdentityRevoked})
	s.Reconcile()
	return err
}

func (s *Service) recordIdentity(ctx context.Context, identity domain.TelegramIdentity) error {
	s.mu.Lock()
	found := false
	for index := range s.settings.Identities {
		if s.settings.Identities[index].ID == identity.ID {
			s.settings.Identities[index] = identity
			found = true
		}
	}
	if !found && identity.Status != domain.TelegramIdentityRevoked {
		s.settings.Identities = append(s.settings.Identities, identity)
	}
	if identity.Status == domain.TelegramIdentityConnected && s.settings.DefaultBotIdentityID == "" {
		s.settings.DefaultBotIdentityID = identity.ID
	}
	s.mu.Unlock()
	if s.onIdentity == nil {
		return nil
	}
	return s.onIdentity(ctx, identity)
}

func (s *Service) identityToken(id string) (domain.TelegramIdentity, string, error) {
	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	if id == "" {
		id = settings.DefaultBotIdentityID
	}
	for _, identity := range settings.Identities {
		if identity.ID == id {
			if identity.Status != domain.TelegramIdentityConnected {
				return domain.TelegramIdentity{}, "", fmt.Errorf("telegram identity %q is invalid and needs to be reconnected", identity.Label)
			}
			token, err := s.tokens(identity.ID)
			return identity, token, err
		}
	}
	return domain.TelegramIdentity{}, "", fmt.Errorf("select a connected Telegram identity")
}

func tokenKey(id string) string { return "telegram:" + id + ":token" }
func (s *Service) tokens(id string) (string, error) {
	token, err := s.vault.Get(tokenKey(id))
	if err != nil {
		return "", err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("telegram bot token is missing")
	}
	return token, nil
}

func (s *Service) markUpdate(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for key, when := range s.seen {
		if now.Sub(when) > 10*time.Minute {
			delete(s.seen, key)
		}
	}
	if _, exists := s.seen[id]; exists {
		return false
	}
	s.seen[id] = now
	return true
}

func (s *Service) setError(err error) {
	s.mu.Lock()
	s.status.LastError = err.Error()
	s.mu.Unlock()
	if s.emit != nil {
		s.emit("telegram.status.error", err.Error())
	}
}

func minDuration(value, maximum time.Duration) time.Duration {
	if value > maximum {
		return maximum
	}
	return value
}
func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
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

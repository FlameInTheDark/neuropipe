// Package twitch owns Twitch OAuth, Helix and EventSub lifecycle management.
// Node modules only see the tiny nodes.TwitchChatSender port; they never import
// this package, Wails, persistence, or the graph interpreter.
package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/FlameInTheDark/neuropipe/internal/twitchspec"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	oauthBase   = "https://id.twitch.tv/oauth2"
	helixBase   = "https://api.twitch.tv/helix"
	eventSubURL = "wss://eventsub.wss.twitch.tv/ws"
)

// Vault is deliberately smaller than the DPAPI implementation. This makes
// token rotation testable and prevents the service from learning persistence
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
type IdentitySink func(context.Context, domain.TwitchIdentity) error
type EventSink func(string, any)

// Service is an owned lifecycle. All background goroutines share ctx and stop
// synchronously, so reconnects and device-code polling cannot outlive Desktop.
type Service struct {
	vault      Vault
	bindings   BindingSource
	runner     ExecutionQueue
	onIdentity IdentitySink
	emit       EventSink
	http       *http.Client

	mu       sync.RWMutex
	settings domain.TwitchSettings
	status   domain.TwitchStatus
	pending  map[string]pendingDevice
	seen     map[string]time.Time
	channels map[string]string
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	conn     *websocket.Conn
}

type pendingDevice struct {
	Request  domain.TwitchDeviceAuthorizationRequest
	Code     string
	Expires  time.Time
	Interval time.Duration
}
type tokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

// tokenBundle is deliberately kept as one vault record. OAuth refresh tokens
// rotate, so writing the replacement access and refresh tokens in one Put
// avoids leaving a usable identity in a partially-rotated state.
type tokenBundle struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
}
type tokenValidation struct {
	ClientID  string   `json:"client_id"`
	Login     string   `json:"login"`
	UserID    string   `json:"user_id"`
	Scopes    []string `json:"scopes"`
	ExpiresIn int      `json:"expires_in"`
}

func New(vault Vault, bindings BindingSource, runner ExecutionQueue, identities IdentitySink, emit EventSink) *Service {
	return &Service{vault: vault, bindings: bindings, runner: runner, onIdentity: identities, emit: emit, http: &http.Client{Timeout: 20 * time.Second}, pending: make(map[string]pendingDevice), seen: make(map[string]time.Time), channels: make(map[string]string), status: domain.TwitchStatus{ConnectionState: "stopped"}}
}

func (s *Service) Configure(settings domain.TwitchSettings) {
	s.mu.Lock()
	settings.Identities = append([]domain.TwitchIdentity(nil), settings.Identities...)
	s.settings = settings
	s.mu.Unlock()
	s.Reconcile()
}
func (s *Service) Status() domain.TwitchStatus             { s.mu.RLock(); defer s.mu.RUnlock(); return s.status }
func (s *Service) Catalog() []domain.TwitchEventDescriptor { return twitchspec.Catalog() }

func (s *Service) Start(parent context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	s.ctx, s.cancel = context.WithCancel(parent)
	s.status.ConnectionState = "connecting"
	ctx := s.ctx
	s.mu.Unlock()
	s.wg.Add(2)
	go s.validationLoop(ctx)
	go s.eventLoop(ctx)
	s.Reconcile()
}
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	conn := s.conn
	s.cancel = nil
	s.status.Connected = false
	s.status.ConnectionState = "stopped"
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		if conn != nil {
			_ = conn.Close()
		}
		s.wg.Wait()
	}
}
func (s *Service) Reconcile() {
	// A WebSocket subscription is bound to a session. Closing only the owned
	// connection makes the event loop establish a fresh session and recreate
	// the coalesced set of trusted bindings without an extra background reader.
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()
	if conn != nil {
		_ = conn.Close()
	}
}

func (s *Service) BeginDeviceAuthorization(ctx context.Context, request domain.TwitchDeviceAuthorizationRequest) (domain.TwitchDeviceAuthorization, error) {
	clientID := s.clientID()
	if clientID == "" {
		return domain.TwitchDeviceAuthorization{}, fmt.Errorf("set a Twitch Client ID before connecting an identity")
	}
	form := url.Values{"client_id": {clientID}, "scopes": {strings.Join(normalizeScopes(request.Scopes), " ")}}
	var response struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	if err := s.form(ctx, oauthBase+"/device", form, &response); err != nil {
		return domain.TwitchDeviceAuthorization{}, err
	}
	if response.DeviceCode == "" || response.UserCode == "" {
		return domain.TwitchDeviceAuthorization{}, fmt.Errorf("twitch returned an incomplete device authorization")
	}
	if response.Interval < 1 {
		response.Interval = 5
	}
	id := uuid.NewString()
	result := domain.TwitchDeviceAuthorization{ID: id, UserCode: response.UserCode, VerificationURI: response.VerificationURI, ExpiresAt: time.Now().UTC().Add(time.Duration(response.ExpiresIn) * time.Second), IntervalSeconds: response.Interval}
	s.mu.Lock()
	s.pending[id] = pendingDevice{Request: request, Code: response.DeviceCode, Expires: result.ExpiresAt, Interval: time.Duration(response.Interval) * time.Second}
	parent := s.ctx
	s.mu.Unlock()
	if parent == nil {
		parent = ctx
	}
	s.wg.Add(1)
	go s.pollDevice(parent, id)
	return result, nil
}
func (s *Service) CancelDeviceAuthorization(id string) {
	s.mu.Lock()
	delete(s.pending, id)
	s.mu.Unlock()
}

func (s *Service) AddManualIdentity(ctx context.Context, request domain.TwitchManualIdentityRequest) (domain.TwitchIdentity, error) {
	if strings.TrimSpace(request.AccessToken) == "" {
		return domain.TwitchIdentity{}, fmt.Errorf("an access token is required")
	}
	validation, err := s.validate(ctx, request.AccessToken)
	if err != nil {
		return domain.TwitchIdentity{}, err
	}
	identity := identityFromValidation(uuid.NewString(), request.Label, validation, domain.TwitchConnectionManual)
	if err := s.putTokens(identity.ID, tokenBundle{AccessToken: request.AccessToken}); err != nil {
		return domain.TwitchIdentity{}, err
	}
	if err := s.recordIdentity(ctx, identity); err != nil {
		return domain.TwitchIdentity{}, err
	}
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
		return nil
	}
	return s.onIdentity(ctx, domain.TwitchIdentity{ID: id, Status: domain.TwitchIdentityRevoked})
}

func (s *Service) SendTwitchChatMessage(ctx context.Context, request domain.TwitchChatMessageRequest) (domain.TwitchChatMessageResult, error) {
	if len([]rune(request.Message)) > 500 {
		return domain.TwitchChatMessageResult{Reason: "message exceeds Twitch's 500-character limit"}, nil
	}
	identity, token, err := s.identityToken(ctx, request.IdentityID)
	if err != nil {
		return domain.TwitchChatMessageResult{}, err
	}
	broadcasterID, err := s.resolveChannelID(ctx, token, request.Channel)
	if err != nil {
		return domain.TwitchChatMessageResult{}, err
	}
	body := map[string]string{"broadcaster_id": broadcasterID, "sender_id": identity.UserID, "message": request.Message}
	if request.ReplyParentID != "" {
		body["reply_parent_message_id"] = request.ReplyParentID
	}
	var response struct {
		Data []struct {
			MessageID  string `json:"message_id"`
			IsSent     bool   `json:"is_sent"`
			DropReason struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"drop_reason"`
		} `json:"data"`
	}
	if err := s.helix(ctx, http.MethodPost, "/chat/messages", token, body, &response); err != nil {
		return domain.TwitchChatMessageResult{}, err
	}
	if len(response.Data) == 0 {
		return domain.TwitchChatMessageResult{}, fmt.Errorf("twitch returned no chat result")
	}
	item := response.Data[0]
	reason := strings.TrimSpace(item.DropReason.Message)
	if reason == "" {
		reason = item.DropReason.Code
	}
	return domain.TwitchChatMessageResult{MessageID: item.MessageID, Sent: item.IsSent, Reason: reason}, nil
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
	identities := append([]domain.TwitchIdentity(nil), s.settings.Identities...)
	s.mu.RUnlock()
	for _, identity := range identities {
		tokens, err := s.tokens(identity.ID)
		if err != nil {
			continue
		}
		validation, err := s.validate(ctx, tokens.AccessToken)
		if err != nil {
			identity.Status = domain.TwitchIdentityReconnectRequired
			_ = s.recordIdentity(ctx, identity)
			continue
		}
		identity.Login, identity.UserID, identity.Scopes = validation.Login, validation.UserID, normalizeScopes(validation.Scopes)
		expiry := time.Now().UTC().Add(time.Duration(validation.ExpiresIn) * time.Second)
		identity.ExpiresAt = &expiry
		identity.Status = domain.TwitchIdentityConnected
		_ = s.recordIdentity(ctx, identity)
	}
}

// eventLoop owns exactly one EventSub connection. Twitch reconnect messages
// supply a new URL that preserves subscriptions; any genuine disconnect is
// followed by a fresh connection and deterministic subscription reconciliation.
func (s *Service) eventLoop(ctx context.Context) {
	defer s.wg.Done()
	endpoint := eventSubURL
	createSubscriptions := true
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, endpoint, nil)
		if err != nil {
			s.setError(err)
			if !wait(ctx, 5*time.Second) {
				return
			}
			continue
		}
		endpoint = eventSubURL
		s.setConnection(conn)
		s.setState("connected", true)
		next, err := s.consumeConnection(ctx, conn, createSubscriptions)
		s.clearConnection(conn)
		_ = conn.Close()
		s.setState("reconnecting", false)
		if next != "" {
			endpoint = next
			// Twitch reconnect URLs transfer the existing subscriptions. Only a
			// genuine disconnect creates a new session that must be resubscribed.
			createSubscriptions = false
		} else if err != nil {
			s.setError(err)
			createSubscriptions = true
		} else {
			createSubscriptions = true
		}
		if !wait(ctx, 2*time.Second) {
			return
		}
	}
}
func (s *Service) consumeConnection(ctx context.Context, conn *websocket.Conn, createSubscriptions bool) (string, error) {
	keepalive := 90 * time.Second
	if err := conn.SetReadDeadline(time.Now().Add(keepalive)); err != nil {
		return "", err
	}
	for {
		var message eventMessage
		if err := conn.ReadJSON(&message); err != nil {
			return "", err
		}
		if message.Metadata.MessageType == "session_welcome" && message.Payload.Session.KeepaliveTimeoutSeconds > 0 {
			keepalive = time.Duration(message.Payload.Session.KeepaliveTimeoutSeconds+10) * time.Second
		}
		if err := conn.SetReadDeadline(time.Now().Add(keepalive)); err != nil {
			return "", err
		}
		switch message.Metadata.MessageType {
		case "session_welcome":
			sessionID := message.Payload.Session.ID
			if sessionID == "" {
				return "", fmt.Errorf("twitch EventSub welcome has no session ID")
			}
			if createSubscriptions {
				if err := s.createSubscriptions(ctx, sessionID); err != nil {
					s.setError(err)
				}
			}
		case "session_reconnect":
			return message.Payload.Session.ReconnectURL, nil
		case "session_keepalive":
			// Receiving it refreshed the read deadline above.
			continue
		case "revocation":
			s.setError(fmt.Errorf("twitch revoked subscription %s", message.Payload.Subscription.ID))
		case "notification":
			s.deliver(ctx, message)
		}
	}
}

type eventMessage struct {
	Metadata struct {
		MessageID   string `json:"message_id"`
		MessageType string `json:"message_type"`
	} `json:"metadata"`
	Payload struct {
		Session struct {
			ID                      string `json:"id"`
			ReconnectURL            string `json:"reconnect_url"`
			KeepaliveTimeoutSeconds int    `json:"keepalive_timeout_seconds"`
		} `json:"session"`
		Subscription struct {
			ID        string            `json:"id"`
			Type      string            `json:"type"`
			Version   string            `json:"version"`
			Condition map[string]string `json:"condition"`
		} `json:"subscription"`
		Event map[string]any `json:"event"`
	} `json:"payload"`
}

func (s *Service) createSubscriptions(ctx context.Context, sessionID string) error {
	if s.bindings == nil {
		return nil
	}
	bindings, err := s.bindings.ListTriggers(ctx, domain.TriggerTwitch)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	active := 0
	for _, binding := range bindings {
		if !binding.Enabled || !binding.Trusted {
			continue
		}
		eventType, _ := binding.Config["eventType"].(string)
		descriptor, ok := twitchspec.Find(eventType)
		if !ok {
			continue
		}
		identity, token, err := s.identityToken(ctx, stringConfig(binding.Config, "identityId"))
		if err != nil {
			continue
		}
		if !identityHasScopes(identity, descriptor.RequiredScopes) {
			s.setError(fmt.Errorf("twitch identity %q is missing scopes for %s", identity.Label, descriptor.Label))
			continue
		}
		condition, err := s.resolvedSubscriptionCondition(ctx, descriptor, binding.Config, identity, token)
		if err != nil {
			s.setError(fmt.Errorf("twitch trigger %q: %w", binding.Label, err))
			continue
		}
		key := descriptor.Type + "|" + descriptor.Version + "|" + identity.UserID + "|" + canonicalCondition(condition)
		if seen[key] {
			continue
		}
		seen[key] = true
		body := map[string]any{"type": descriptor.Type, "version": descriptor.Version, "condition": condition, "transport": map[string]string{"method": "websocket", "session_id": sessionID}}
		if err := s.helix(ctx, http.MethodPost, "/eventsub/subscriptions", token, body, nil); err == nil {
			active++
		}
	}
	s.mu.Lock()
	s.status.ActiveSubscriptions = active
	s.mu.Unlock()
	return nil
}
func (s *Service) deliver(ctx context.Context, message eventMessage) {
	if message.Metadata.MessageID == "" || !s.markMessage(message.Metadata.MessageID) {
		return
	}
	event := twitchspec.Event{Type: message.Payload.Subscription.Type, Version: message.Payload.Subscription.Version, MessageID: message.Metadata.MessageID, SubscriptionID: message.Payload.Subscription.ID, ReceivedAt: time.Now().UTC().Format(time.RFC3339Nano), Payload: message.Payload.Event}
	if event.Type == "channel.chat.message" {
		event.Payload["chatMessage"] = decodeChat(message.Payload.Event)
	}
	bindings, err := s.bindings.ListTriggers(ctx, domain.TriggerTwitch)
	if err != nil {
		return
	}
	for _, binding := range bindings {
		if !binding.Enabled || !binding.Trusted || stringConfig(binding.Config, "eventType") != event.Type {
			continue
		}
		descriptor, found := twitchspec.Find(event.Type)
		if !found || !s.matchesSubscription(ctx, binding, descriptor, message.Payload.Subscription.Condition) {
			continue
		}
		_, _ = s.runner.QueueBinding(ctx, binding.ID, pipeline.Packet{"event": event}, true)
	}
}

func (s *Service) matchesSubscription(ctx context.Context, binding domain.TriggerBinding, descriptor domain.TwitchEventDescriptor, actual map[string]string) bool {
	identity, found := s.identity(stringConfig(binding.Config, "identityId"))
	if !found || identity.Status != domain.TwitchIdentityConnected {
		return false
	}
	// EventSub subscriptions are created through resolvedSubscriptionCondition,
	// which primes the channel cache. Delivery matching deliberately avoids a
	// DPAPI vault read or network request on every incoming event.
	expected, err := s.resolvedSubscriptionCondition(ctx, descriptor, binding.Config, identity, "")
	if err != nil {
		return false
	}
	for key, value := range expected {
		if actual[key] != value {
			return false
		}
	}
	return true
}
func decodeChat(value map[string]any) twitchspec.ChatMessage {
	return twitchspec.ChatMessage{Text: nestedString(value, "message", "text"), BroadcasterID: stringConfig(value, "broadcaster_user_id"), AuthorID: stringConfig(value, "chatter_user_id"), MessageID: stringConfig(value, "message_id"), AuthorLogin: stringConfig(value, "chatter_user_login"), AuthorName: stringConfig(value, "chatter_user_name"), ChannelLogin: stringConfig(value, "broadcaster_user_login")}
}

func (s *Service) pollDevice(ctx context.Context, id string) {
	defer s.wg.Done()
	for {
		s.mu.RLock()
		pending, exists := s.pending[id]
		clientID := s.settings.ClientID
		s.mu.RUnlock()
		if !exists || time.Now().After(pending.Expires) || ctx.Err() != nil {
			return
		}
		if !wait(ctx, pending.Interval) {
			return
		}
		form := url.Values{"client_id": {clientID}, "device_code": {pending.Code}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}
		var token tokenResponse
		if err := s.form(ctx, oauthBase+"/token", form, &token); err != nil {
			continue
		}
		if token.AccessToken == "" {
			continue
		}
		validation, err := s.validate(ctx, token.AccessToken)
		if err != nil {
			continue
		}
		identityID := pending.Request.IdentityID
		if identityID == "" {
			identityID = uuid.NewString()
		}
		label := pending.Request.Label
		if existing, found := s.identity(identityID); found && strings.TrimSpace(label) == "" {
			label = existing.Label
		}
		identity := identityFromValidation(identityID, label, validation, domain.TwitchConnectionDeviceCode)
		expiry := time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second)
		identity.ExpiresAt = &expiry
		if err := s.putTokens(identity.ID, tokenBundle{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken}); err != nil {
			return
		}
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		_ = s.recordIdentity(ctx, identity)
		return
	}
}
func (s *Service) identityToken(ctx context.Context, id string) (domain.TwitchIdentity, string, error) {
	identity, found := s.identity(id)
	if !found {
		return domain.TwitchIdentity{}, "", fmt.Errorf("select a connected Twitch identity")
	}
	if identity.Status != domain.TwitchIdentityConnected {
		return domain.TwitchIdentity{}, "", fmt.Errorf("twitch identity %q needs reconnection", identity.Label)
	}
	if identity.Method == domain.TwitchConnectionDeviceCode && identity.ExpiresAt != nil && time.Until(*identity.ExpiresAt) < time.Minute {
		refreshed, err := s.refreshIdentity(ctx, identity)
		if err != nil {
			return domain.TwitchIdentity{}, "", err
		}
		identity = refreshed
	}
	tokens, err := s.tokens(identity.ID)
	return identity, tokens.AccessToken, err
}

func (s *Service) identity(id string) (domain.TwitchIdentity, bool) {
	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	if id == "" {
		id = settings.DefaultBotIdentityID
	}
	for _, identity := range settings.Identities {
		if identity.ID == id {
			return identity, true
		}
	}
	return domain.TwitchIdentity{}, false
}

// refreshIdentity performs Twitch's rotating refresh-token grant before an
// expiring device-code identity is used. Both replacement values stay inside
// the vault; only refreshed public identity metadata is persisted.
func (s *Service) refreshIdentity(ctx context.Context, identity domain.TwitchIdentity) (domain.TwitchIdentity, error) {
	current, err := s.tokens(identity.ID)
	if err != nil {
		return domain.TwitchIdentity{}, fmt.Errorf("twitch identity %q needs reconnection", identity.Label)
	}
	if current.RefreshToken == "" {
		return domain.TwitchIdentity{}, fmt.Errorf("twitch identity %q needs reconnection", identity.Label)
	}
	form := url.Values{"client_id": {s.clientID()}, "grant_type": {"refresh_token"}, "refresh_token": {current.RefreshToken}}
	var token tokenResponse
	if err := s.form(ctx, oauthBase+"/token", form, &token); err != nil || token.AccessToken == "" {
		identity.Status = domain.TwitchIdentityReconnectRequired
		_ = s.recordIdentity(ctx, identity)
		if err != nil {
			return domain.TwitchIdentity{}, fmt.Errorf("refresh Twitch identity: %w", err)
		}
		return domain.TwitchIdentity{}, fmt.Errorf("refresh Twitch identity: Twitch returned no access token")
	}
	if token.RefreshToken != "" {
		current.RefreshToken = token.RefreshToken
	}
	current.AccessToken = token.AccessToken
	if err := s.putTokens(identity.ID, current); err != nil {
		return domain.TwitchIdentity{}, err
	}
	validation, err := s.validate(ctx, token.AccessToken)
	if err != nil {
		return domain.TwitchIdentity{}, err
	}
	identity.Login, identity.UserID, identity.Scopes = validation.Login, validation.UserID, normalizeScopes(validation.Scopes)
	expiresAt := time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second)
	identity.ExpiresAt, identity.Status = &expiresAt, domain.TwitchIdentityConnected
	if err := s.recordIdentity(ctx, identity); err != nil {
		return domain.TwitchIdentity{}, err
	}
	return identity, nil
}
func (s *Service) recordIdentity(ctx context.Context, identity domain.TwitchIdentity) error {
	s.mu.Lock()
	found := false
	for index := range s.settings.Identities {
		if s.settings.Identities[index].ID == identity.ID {
			s.settings.Identities[index] = identity
			found = true
		}
	}
	if !found && identity.Status != domain.TwitchIdentityRevoked {
		s.settings.Identities = append(s.settings.Identities, identity)
	}
	if identity.Status == domain.TwitchIdentityConnected && s.settings.DefaultBotIdentityID == "" {
		s.settings.DefaultBotIdentityID = identity.ID
	}
	s.mu.Unlock()
	if s.onIdentity != nil {
		return s.onIdentity(ctx, identity)
	}
	return nil
}
func (s *Service) clientID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return strings.TrimSpace(s.settings.ClientID)
}
func (s *Service) validate(ctx context.Context, token string) (tokenValidation, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, oauthBase+"/validate", nil)
	if err != nil {
		return tokenValidation{}, err
	}
	request.Header.Set("Authorization", "OAuth "+token)
	response, err := s.http.Do(request)
	if err != nil {
		return tokenValidation{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return tokenValidation{}, fmt.Errorf("validate Twitch token: %s", response.Status)
	}
	var value tokenValidation
	return value, json.NewDecoder(response.Body).Decode(&value)
}
func (s *Service) form(ctx context.Context, endpoint string, form url.Values, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := s.http.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("twitch request failed: %s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(destination)
}
func (s *Service) helix(ctx context.Context, method, path, token string, body any, destination any) error {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, helixBase+path, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Client-Id", s.clientID())
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := s.http.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("twitch Helix request failed: %s", response.Status)
	}
	if destination == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(destination)
}
func identityFromValidation(id, label string, value tokenValidation, method domain.TwitchConnectionMethod) domain.TwitchIdentity {
	label = strings.TrimSpace(label)
	if label == "" {
		label = value.Login
	}
	return domain.TwitchIdentity{ID: id, Label: label, UserID: value.UserID, Login: value.Login, Scopes: normalizeScopes(value.Scopes), Status: domain.TwitchIdentityConnected, Method: method}
}
func tokenKey(id string) string { return "twitch:" + id + ":tokens" }
func (s *Service) tokens(id string) (tokenBundle, error) {
	raw, err := s.vault.Get(tokenKey(id))
	if err != nil {
		return tokenBundle{}, err
	}
	var tokens tokenBundle
	if err := json.Unmarshal([]byte(raw), &tokens); err != nil || strings.TrimSpace(tokens.AccessToken) == "" {
		if err != nil {
			return tokenBundle{}, fmt.Errorf("read Twitch tokens: %w", err)
		}
		return tokenBundle{}, fmt.Errorf("twitch identity token is missing")
	}
	return tokens, nil
}
func (s *Service) putTokens(id string, tokens tokenBundle) error {
	if strings.TrimSpace(tokens.AccessToken) == "" {
		return fmt.Errorf("twitch access token is required")
	}
	raw, err := json.Marshal(tokens)
	if err != nil {
		return err
	}
	return s.vault.Put(tokenKey(id), string(raw))
}
func identityHasScopes(identity domain.TwitchIdentity, required []string) bool {
	granted := make(map[string]bool, len(identity.Scopes))
	for _, scope := range identity.Scopes {
		granted[scope] = true
	}
	for _, scope := range required {
		if !granted[scope] {
			return false
		}
	}
	return true
}

// resolvedSubscriptionCondition keeps channel logins at the graph boundary and
// resolves the Helix-only broadcaster user ID immediately before subscribing.
// The resolved ID is deliberately never persisted into the node configuration.
func (s *Service) resolvedSubscriptionCondition(ctx context.Context, descriptor domain.TwitchEventDescriptor, config map[string]any, identity domain.TwitchIdentity, token string) (map[string]string, error) {
	values := make(map[string]any, len(config)+1)
	for key, value := range config {
		values[key] = value
	}
	for _, field := range descriptor.Conditions {
		configKey, channelCondition := channelConfigKey(field.ID)
		if !channelCondition {
			continue
		}
		channel := stringConfig(config, configKey)
		if channel == "" && !field.Required {
			continue
		}
		id, err := s.resolveChannelID(ctx, token, channel)
		if err != nil {
			return nil, err
		}
		if field.ID == "broadcaster_user_id" {
			values["broadcasterId"] = id
		} else {
			values[field.ID] = id
		}
	}
	if descriptor.Type == "channel.raid" && stringConfig(values, "to_broadcaster_user_id") == "" && stringConfig(values, "from_broadcaster_user_id") == "" {
		return nil, fmt.Errorf("a source or target channel name is required for a raid trigger")
	}
	return subscriptionCondition(descriptor, values, identity, s.clientID())
}

func channelConfigKey(conditionID string) (string, bool) {
	switch conditionID {
	case "broadcaster_user_id":
		return "channel", true
	case "to_broadcaster_user_id":
		return "targetChannel", true
	case "from_broadcaster_user_id":
		return "sourceChannel", true
	default:
		return "", false
	}
}

// resolveChannelID translates a human-friendly Twitch channel login into the
// numeric user ID required by EventSub and the Send Chat Message Helix API.
func (s *Service) resolveChannelID(ctx context.Context, token, channel string) (string, error) {
	login := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(channel), "@"))
	if login == "" {
		return "", fmt.Errorf("channel name is required")
	}
	s.mu.RLock()
	id := s.channels[login]
	s.mu.RUnlock()
	if id != "" {
		return id, nil
	}
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := s.helix(ctx, http.MethodGet, "/users?login="+url.QueryEscape(login), token, nil, &response); err != nil {
		return "", err
	}
	if len(response.Data) == 0 || strings.TrimSpace(response.Data[0].ID) == "" {
		return "", fmt.Errorf("twitch channel %q was not found", channel)
	}
	id = strings.TrimSpace(response.Data[0].ID)
	s.mu.Lock()
	s.channels[login] = id
	s.mu.Unlock()
	return id, nil
}

func subscriptionCondition(descriptor domain.TwitchEventDescriptor, config map[string]any, identity domain.TwitchIdentity, clientID string) (map[string]string, error) {
	condition := make(map[string]string, len(descriptor.Conditions))
	for _, field := range descriptor.Conditions {
		value := ""
		switch field.ID {
		case "broadcaster_user_id":
			value = stringConfig(config, "broadcasterId")
		case "user_id", "moderator_user_id":
			value = identity.UserID
		case "client_id":
			value = strings.TrimSpace(clientID)
		default:
			value = stringConfig(config, field.ID)
		}
		if value == "" {
			if field.Required {
				return nil, fmt.Errorf("%s is required", field.Label)
			}
			continue
		}
		condition[field.ID] = value
	}
	return condition, nil
}
func normalizeScopes(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
func stringConfig(value map[string]any, key string) string {
	raw, ok := value[key]
	if !ok {
		return ""
	}
	result, _ := raw.(string)
	return strings.TrimSpace(result)
}
func nestedString(value map[string]any, parent, key string) string {
	nested, _ := value[parent].(map[string]any)
	return stringConfig(nested, key)
}
func canonicalCondition(value map[string]string) string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+value[key])
	}
	return strings.Join(parts, "&")
}
func (s *Service) markMessage(id string) bool {
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
func (s *Service) setState(state string, connected bool) {
	s.mu.Lock()
	s.status.ConnectionState = state
	s.status.Connected = connected
	s.mu.Unlock()
}
func (s *Service) setConnection(conn *websocket.Conn) {
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
}
func (s *Service) clearConnection(conn *websocket.Conn) {
	s.mu.Lock()
	if s.conn == conn {
		s.conn = nil
	}
	s.mu.Unlock()
}
func (s *Service) setError(err error) {
	s.mu.Lock()
	s.status.LastError = err.Error()
	s.mu.Unlock()
	if s.emit != nil {
		s.emit("twitch.status.error", err.Error())
	}
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

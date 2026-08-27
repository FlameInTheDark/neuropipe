// Package kvsub delivers Redis pub/sub messages to published pipeline
// bindings. It mirrors the Twitch EventSub service lifecycle: bindings come
// from the store, delivery goes through the execution queue, and every
// goroutine is owned by Start/Stop.
package kvsub

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	kvservice "github.com/FlameInTheDark/neuropipe/internal/kv"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

// BindingSource reads the KV subscribe bindings a publication created.
type BindingSource interface {
	ListTriggers(ctx context.Context, kind domain.TriggerKind) ([]domain.TriggerBinding, error)
}

// SubscriptionSource opens one pub/sub subscription on a registered KV
// connection. The kv service implements it.
type SubscriptionSource interface {
	Subscribe(ctx context.Context, databaseID string, channels, patterns []string) (kvservice.Subscription, error)
}

// ExecutionQueue starts one binding's pipeline run.
type ExecutionQueue interface {
	QueueBinding(ctx context.Context, bindingID string, input pipeline.Packet, unattended bool) (domain.Execution, error)
}

// EventSink reports lifecycle changes to the UI without carrying payloads.
type EventSink func(event string, payload any)

// reconnectBackoff caps the wait between reconnect attempts.
const reconnectBackoff = 5 * time.Second

// Service owns one receive goroutine per active (database, channel-set)
// subscription. Bindings sharing the same database and channel set share one
// subscription; each message is delivered to every matching binding.
type Service struct {
	bindings BindingSource
	source   SubscriptionSource
	runner   ExecutionQueue
	emit     EventSink

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	workers sync.WaitGroup
	current map[string]context.CancelFunc
	started bool
}

// New creates the KV subscribe delivery service.
func New(bindings BindingSource, source SubscriptionSource, runner ExecutionQueue, emit EventSink) *Service {
	return &Service{bindings: bindings, source: source, runner: runner, emit: emit, current: make(map[string]context.CancelFunc)}
}

// Start begins reconciling subscriptions under the parent context.
func (s *Service) Start(parent context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	s.ctx, s.cancel = context.WithCancel(parent)
	s.started = true
	s.mu.Unlock()
	s.Reconcile()
}

// Stop cancels every subscription and waits for the receive loops to exit.
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.started = false
	s.current = make(map[string]context.CancelFunc)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		s.workers.Wait()
	}
}

// Reconcile rebuilds the active subscription set from enabled, trusted
// bindings. Stale subscriptions stop; new ones start. It is called after
// publish, trust, enable, and connection changes.
func (s *Service) Reconcile() {
	s.mu.Lock()
	ctx := s.ctx
	if ctx == nil || !s.started {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	bindings, err := s.bindings.ListTriggers(ctx, domain.TriggerKV)
	if err != nil {
		s.emit("kvsub.status.error", map[string]any{"error": "list kv triggers failed"})
		return
	}
	wanted := make(map[string][]domain.TriggerBinding)
	for _, binding := range bindings {
		if !binding.Enabled || !binding.Trusted {
			continue
		}
		databaseID, _ := binding.Config["databaseId"].(string)
		databaseID = strings.TrimSpace(databaseID)
		if databaseID == "" {
			continue
		}
		key := subscriptionKey(databaseID, binding.Config)
		wanted[key] = append(wanted[key], binding)
	}
	// Stop subscriptions whose key disappeared.
	s.mu.Lock()
	for key, cancel := range s.current {
		if _, exists := wanted[key]; !exists {
			cancel()
			delete(s.current, key)
		}
	}
	s.mu.Unlock()
	// Start subscriptions that are not running yet.
	for key, group := range wanted {
		s.mu.Lock()
		_, running := s.current[key]
		s.mu.Unlock()
		if running {
			continue
		}
		databaseID, _ := group[0].Config["databaseId"].(string)
		subCtx, cancel := context.WithCancel(ctx)
		s.mu.Lock()
		if s.ctx == nil {
			// Stop raced us; drop the new subscription immediately.
			s.mu.Unlock()
			cancel()
			return
		}
		s.current[key] = cancel
		s.mu.Unlock()
		s.workers.Add(1)
		go s.receiveLoop(subCtx, key, databaseID, group)
	}
	s.emit("kvsub.status.updated", map[string]any{"subscriptions": len(wanted)})
}

// receiveLoop keeps one subscription alive until its context is cancelled.
// Connection errors reconnect after a capped backoff instead of failing the
// pipeline system.
func (s *Service) receiveLoop(ctx context.Context, key string, databaseID string, initial []domain.TriggerBinding) {
	defer s.workers.Done()
	defer func() {
		s.mu.Lock()
		delete(s.current, key)
		s.mu.Unlock()
	}()
	for {
		if ctx.Err() != nil {
			return
		}
		bindings := s.latestBindings(ctx, key, initial)
		channels, patterns := subscriptionTargets(bindings)
		if len(channels) == 0 && len(patterns) == 0 {
			return
		}
		subscription, err := s.source.Subscribe(ctx, databaseID, channels, patterns)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.emit("kvsub.status.error", map[string]any{"database": databaseID})
			if !sleepContext(ctx, reconnectBackoff) {
				return
			}
			continue
		}
		for {
			message, err := subscription.Receive(ctx)
			if err != nil {
				_ = subscription.Close()
				if ctx.Err() != nil {
					return
				}
				break
			}
			s.deliver(ctx, bindings, message)
		}
	}
}

// latestBindings re-reads the binding group so channel-set changes apply
// without restarting the whole service.
func (s *Service) latestBindings(ctx context.Context, key string, initial []domain.TriggerBinding) []domain.TriggerBinding {
	bindings, err := s.bindings.ListTriggers(ctx, domain.TriggerKV)
	if err != nil {
		return initial
	}
	result := make([]domain.TriggerBinding, 0)
	for _, binding := range bindings {
		if !binding.Enabled || !binding.Trusted {
			continue
		}
		databaseID, _ := binding.Config["databaseId"].(string)
		if strings.TrimSpace(databaseID) == "" {
			continue
		}
		if subscriptionKey(strings.TrimSpace(databaseID), binding.Config) == key {
			result = append(result, binding)
		}
	}
	if len(result) == 0 {
		return initial
	}
	return result
}

// deliver hands one message to every binding whose channels or patterns
// match. Errors queueing a run are reported without stopping the loop.
func (s *Service) deliver(ctx context.Context, bindings []domain.TriggerBinding, message kvservice.PubSubMessage) {
	for _, binding := range bindings {
		if !matchesBinding(binding, message) {
			continue
		}
		packet := pipeline.Packet{
			"channel":    message.Channel,
			"message":    message.Payload,
			"pattern":    message.Pattern,
			"receivedAt": time.Now().UTC().Format(time.RFC3339),
		}
		if _, err := s.runner.QueueBinding(ctx, binding.ID, packet, true); err != nil {
			s.emit("kvsub.status.error", map[string]any{"binding": binding.ID})
		}
	}
}

// matchesBinding routes one message to a binding. Pattern-delivered
// messages match only bindings whose pattern list contains the delivering
// pattern; channel messages match by exact channel name.
func matchesBinding(binding domain.TriggerBinding, message kvservice.PubSubMessage) bool {
	if message.Pattern != "" {
		for _, pattern := range splitTargets(binding.Config["patterns"]) {
			if pattern == message.Pattern {
				return true
			}
		}
		return false
	}
	channels := splitTargets(binding.Config["channels"])
	if len(channels) == 0 {
		return true
	}
	for _, channel := range channels {
		if channel == message.Channel {
			return true
		}
	}
	return false
}

// patternMatches implements the Redis glob subset used by PSUBSCRIBE.
func patternMatches(pattern, channel string) bool {
	// Fast path: exact match.
	if pattern == channel {
		return true
	}
	return globMatch(pattern, channel)
}

// globMatch evaluates a Redis-style glob with '*' and '?'.
func globMatch(pattern, subject string) bool {
	// Iterative glob matcher: pi walks the pattern, si the subject, and
	// star remembers the last '*' position for backtracking.
	var pi, si, star, mark int
	for si < len(subject) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == subject[si]) {
			pi++
			si++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			star = pi
			mark = si
			pi++
		} else if star >= 0 && star < len(pattern) {
			pi = star + 1
			mark++
			si = mark
		} else {
			return false
		}
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}

// subscriptionKey groups bindings by database and the exact channel set so
// identical bindings share one connection.
func subscriptionKey(databaseID string, config map[string]any) string {
	targets := append(splitTargets(config["channels"]), splitTargets(config["patterns"])...)
	sort.Strings(targets)
	return fmt.Sprintf("%s|%s", databaseID, strings.Join(targets, ","))
}

// subscriptionTargets returns the sorted channel and pattern lists for one
// subscription group.
func subscriptionTargets(bindings []domain.TriggerBinding) (channels, patterns []string) {
	channelSet := make(map[string]struct{})
	patternSet := make(map[string]struct{})
	for _, binding := range bindings {
		for _, channel := range splitTargets(binding.Config["channels"]) {
			channelSet[channel] = struct{}{}
		}
		for _, pattern := range splitTargets(binding.Config["patterns"]) {
			patternSet[pattern] = struct{}{}
		}
	}
	channels = sortedKeys(channelSet)
	patterns = sortedKeys(patternSet)
	return channels, patterns
}

func sortedKeys(set map[string]struct{}) []string {
	result := make([]string, 0, len(set))
	for key := range set {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

// splitTargets parses a comma-separated channel or pattern list.
func splitTargets(raw any) []string {
	text, _ := raw.(string)
	result := make([]string, 0)
	for _, item := range strings.Split(text, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// sleepContext waits for the duration or the context, whichever ends first.
func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

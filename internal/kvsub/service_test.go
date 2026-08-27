package kvsub

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	kvservice "github.com/FlameInTheDark/neuropipe/internal/kv"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

type bindingSourceStub struct {
	mu        sync.Mutex
	bindings  []domain.TriggerBinding
	listCalls int
}

func (s *bindingSourceStub) ListTriggers(_ context.Context, kind domain.TriggerKind) ([]domain.TriggerBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if kind != domain.TriggerKV {
		return nil, nil
	}
	s.listCalls++
	return s.bindings, nil
}

type subscriptionStub struct {
	messages  chan kvservice.PubSubMessage
	closeOnce sync.Once
	mu        sync.Mutex
	closed    bool
}

func newSubscriptionStub() *subscriptionStub {
	return &subscriptionStub{messages: make(chan kvservice.PubSubMessage, 8)}
}

func (s *subscriptionStub) Receive(ctx context.Context) (kvservice.PubSubMessage, error) {
	select {
	case message := <-s.messages:
		return message, nil
	case <-ctx.Done():
		return kvservice.PubSubMessage{}, ctx.Err()
	}
}

func (s *subscriptionStub) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
	})
	return nil
}

type sourceStub struct {
	mu            sync.Mutex
	subscriptions []*subscriptionStub
	channels      [][]string
	patterns      []string
}

func (s *sourceStub) Subscribe(_ context.Context, databaseID string, channels, patterns []string) (kvservice.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels = append(s.channels, channels)
	s.patterns = patterns
	stub := newSubscriptionStub()
	s.subscriptions = append(s.subscriptions, stub)
	return stub, nil
}

// eventsSubscription returns the subscription opened for the exact channel
// list containing "events".
func (s *sourceStub) eventsSubscription() *subscriptionStub {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, channels := range s.channels {
		for _, channel := range channels {
			if channel == "events" {
				return s.subscriptions[index]
			}
		}
	}
	return nil
}

type queueStub struct {
	mu      sync.Mutex
	packets []queuedDelivery
}

type queuedDelivery struct {
	bindingID string
	packet    pipeline.Packet
}

func (q *queueStub) QueueBinding(_ context.Context, bindingID string, packet pipeline.Packet, unattended bool) (domain.Execution, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !unattended {
		return domain.Execution{}, errors.New("kv triggers must queue unattended runs")
	}
	q.packets = append(q.packets, queuedDelivery{bindingID: bindingID, packet: packet})
	return domain.Execution{ID: "exec"}, nil
}

func TestDeliversMessagesToMatchingBindings(t *testing.T) {
	bindings := []domain.TriggerBinding{
		{ID: "b1", Enabled: true, Trusted: true, Config: map[string]any{"databaseId": "db-1", "channels": "events"}},
		{ID: "b2", Enabled: true, Trusted: true, Config: map[string]any{"databaseId": "db-1", "channels": "other"}},
		{ID: "b3", Enabled: false, Trusted: true, Config: map[string]any{"databaseId": "db-1", "channels": "events"}},
		{ID: "b4", Enabled: true, Trusted: false, Config: map[string]any{"databaseId": "db-1", "channels": "events"}},
	}
	source := &sourceStub{}
	queue := &queueStub{}
	service := New(&bindingSourceStub{bindings: bindings}, source, queue, func(string, any) {})
	service.Start(context.Background())
	defer service.Stop()

	// Wait for the service to open its subscriptions (one per channel set:
	// "events" and "other").
	deadline := time.Now().Add(2 * time.Second)
	var stub *subscriptionStub
	for time.Now().Before(deadline) {
		stub = source.eventsSubscription()
		if stub != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if stub == nil {
		t.Fatal("no subscription opened for the events channel")
	}

	stub.messages <- kvservice.PubSubMessage{Channel: "events", Payload: "hello"}

	// Give the deliver loop a moment to queue the run.
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		queue.mu.Lock()
		count := len(queue.packets)
		queue.mu.Unlock()
		if count == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if len(queue.packets) != 1 {
		t.Fatalf("queued packets = %#v", queue.packets)
	}
	if queue.packets[0].bindingID != "b1" || queue.packets[0].packet["message"] != "hello" || queue.packets[0].packet["channel"] != "events" {
		t.Fatalf("packet = %#v", queue.packets[0])
	}
	if _, ok := queue.packets[0].packet["receivedAt"]; !ok {
		t.Fatalf("packet missing receivedAt: %#v", queue.packets[0].packet)
	}
}

func TestGlobMatching(t *testing.T) {
	cases := []struct {
		pattern string
		channel string
		want    bool
	}{
		{"events:*", "events:signup", true},
		{"events:*", "other:signup", false},
		{"*", "anything", true},
		{"exact", "exact", true},
		{"exact", "other", false},
		{"user:?:profile", "user:1:profile", true},
		{"user:?:profile", "user:12:profile", false},
		{"a*b*c", "aXXXbYYYc", true},
		{"a*b*c", "aXXbYY", false},
	}
	for _, testCase := range cases {
		if got := patternMatches(testCase.pattern, testCase.channel); got != testCase.want {
			t.Fatalf("patternMatches(%q, %q) = %v, want %v", testCase.pattern, testCase.channel, got, testCase.want)
		}
	}
}

func TestMatchesBindingPatterns(t *testing.T) {
	binding := domain.TriggerBinding{Config: map[string]any{"patterns": "events:*"}}
	if !matchesBinding(binding, kvservice.PubSubMessage{Channel: "events:x", Pattern: "events:*"}) {
		t.Fatal("pattern message did not match its pattern binding")
	}
	if matchesBinding(binding, kvservice.PubSubMessage{Channel: "events:x", Pattern: "other:*"}) {
		t.Fatal("message matched a foreign pattern")
	}
	channelBinding := domain.TriggerBinding{Config: map[string]any{"channels": "events"}}
	if !matchesBinding(channelBinding, kvservice.PubSubMessage{Channel: "events"}) {
		t.Fatal("channel message did not match its channel binding")
	}
	if matchesBinding(channelBinding, kvservice.PubSubMessage{Channel: "nope"}) {
		t.Fatal("message matched a foreign channel")
	}
}

func TestSubscriptionTargetsMergeBindings(t *testing.T) {
	bindings := []domain.TriggerBinding{
		{Config: map[string]any{"channels": "a, b", "patterns": "p:*"}},
		{Config: map[string]any{"channels": "b, c"}},
	}
	channels, patterns := subscriptionTargets(bindings)
	if len(channels) != 3 || channels[0] != "a" || channels[1] != "b" || channels[2] != "c" {
		t.Fatalf("channels = %#v", channels)
	}
	if len(patterns) != 1 || patterns[0] != "p:*" {
		t.Fatalf("patterns = %#v", patterns)
	}
}

func TestStopJoinsGoroutines(t *testing.T) {
	source := &sourceStub{}
	queue := &queueStub{}
	service := New(&bindingSourceStub{bindings: []domain.TriggerBinding{
		{ID: "b1", Enabled: true, Trusted: true, Config: map[string]any{"databaseId": "db-1", "channels": "events"}},
	}}, source, queue, func(string, any) {})
	service.Start(context.Background())
	// Wait for the subscription to open.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		source.mu.Lock()
		count := len(source.subscriptions)
		source.mu.Unlock()
		if count == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	source.mu.Lock()
	if len(source.subscriptions) != 1 {
		source.mu.Unlock()
		t.Fatalf("subscriptions = %d", len(source.subscriptions))
	}
	stub := source.subscriptions[0]
	source.mu.Unlock()
	service.Stop()
	stub.mu.Lock()
	closed := stub.closed
	stub.mu.Unlock()
	if !closed {
		t.Fatal("Stop() left the subscription open")
	}
}

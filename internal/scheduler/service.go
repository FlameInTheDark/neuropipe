// Package scheduler runs trusted cron bindings without sharing Wails state.
package scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/execution"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/robfig/cron/v3"
)

// Service owns cron lifecycle and rebuilds entries when schedules are published or toggled.
type Service struct {
	store  *persistence.Store
	runner *execution.Service
	cron   *cron.Cron

	mu      sync.Mutex
	entries map[string]cron.EntryID
}

// New creates a scheduler using standard five-field cron syntax.
func New(store *persistence.Store, runner *execution.Service) *Service {
	return &Service{store: store, runner: runner, cron: cron.New(), entries: make(map[string]cron.EntryID)}
}

// Start loads eligible entries and starts the cron worker.
func (s *Service) Start(ctx context.Context) error {
	s.cron.Start()
	return s.Reload(ctx)
}

// Stop removes jobs and terminates the scheduler worker.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.entries {
		s.cron.Remove(entry)
	}
	s.entries = make(map[string]cron.EntryID)
	stop := s.cron.Stop()
	<-stop.Done()
}

// Reload reconciles active cron jobs from persisted bindings.
func (s *Service) Reload(ctx context.Context) error {
	bindings, err := s.store.ListTriggers(ctx, domain.TriggerCron)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.entries {
		s.cron.Remove(entry)
	}
	s.entries = make(map[string]cron.EntryID)
	for _, binding := range bindings {
		if !binding.Enabled || !binding.Trusted {
			continue
		}
		spec, err := cronSpec(binding.Cron, binding.Timezone)
		if err != nil {
			continue
		}
		bindingID := binding.ID
		entry, err := s.cron.AddFunc(spec, func() {
			_, _ = s.runner.RunBinding(context.Background(), bindingID, pipeline.Packet{"trigger": "cron", "scheduledAt": time.Now().UTC().Format(time.RFC3339)}, true)
		})
		if err != nil {
			continue
		}
		s.entries[binding.ID] = entry
		if next := s.cron.Entry(entry).Next; !next.IsZero() {
			_ = s.store.SetTriggerNextRun(ctx, binding.ID, next)
		}
	}
	return nil
}

// CronSpec validates a five-field cron expression with an optional IANA
// timezone and returns the robfig/cron specification string. It is shared
// with the remote executor's autonomous scheduler.
func CronSpec(expression, timezone string) (string, error) {
	return cronSpec(expression, timezone)
}

func cronSpec(expression, timezone string) (string, error) {
	if len(strings.Fields(expression)) != 5 {
		return "", fmt.Errorf("cron expressions must have five fields")
	}
	if timezone == "" || timezone == "Local" {
		return expression, nil
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return "", fmt.Errorf("invalid timezone: %w", err)
	}
	return "CRON_TZ=" + timezone + " " + expression, nil
}

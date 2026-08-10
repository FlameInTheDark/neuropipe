package hotkey

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

// BindingSource supplies the small persisted projection needed to reconcile
// global hotkeys. It keeps this package independent of SQLite.
type BindingSource interface {
	ListAllTriggers(context.Context) ([]domain.TriggerBinding, error)
}

// BindingRunner starts a published binding without coupling hotkey handling to
// the concrete execution service.
type BindingRunner interface {
	QueueBinding(context.Context, string, pipeline.Packet, bool) (domain.Execution, error)
}

type registration struct {
	id        uint32
	bindingID string
	shortcut  chord
}

type nativeHost interface {
	Start() error
	Replace(context.Context, []registration) error
	Events() <-chan uint32
	Stop()
}

// Service owns native registration and routes a hotkey press through the
// application-owned execution queue. It does not import Wails or persistence.
type Service struct {
	source BindingSource
	runner BindingRunner
	host   nativeHost

	mu       sync.RWMutex
	started  bool
	ctx      context.Context
	cancel   context.CancelFunc
	bindings map[uint32]registration
	workers  sync.WaitGroup
}

// New creates a platform-specific global hotkey service.
func New(source BindingSource, runner BindingRunner) *Service {
	return newService(source, runner, newNativeHost())
}

func newService(source BindingSource, runner BindingRunner, host nativeHost) *Service {
	return &Service{source: source, runner: runner, host: host, bindings: make(map[uint32]registration)}
}

// Start opens the native host, loads registered bindings, and owns the
// dispatcher until Stop or the parent context is cancelled.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	if err := s.host.Start(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.started = true
	s.workers.Add(1)
	go s.dispatch(s.ctx)
	s.mu.Unlock()

	if err := s.Reload(ctx); err != nil {
		s.Stop()
		return err
	}
	return nil
}

// Stop unregisters every global hotkey and waits for the dispatcher to exit.
func (s *Service) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	cancel := s.cancel
	s.started = false
	s.cancel = nil
	s.bindings = make(map[uint32]registration)
	s.mu.Unlock()

	cancel()
	s.host.Stop()
	s.workers.Wait()
}

// Reload replaces native registrations with every enabled configured hotkey.
func (s *Service) Reload(ctx context.Context) error {
	bindings, err := s.source.ListAllTriggers(ctx)
	if err != nil {
		return fmt.Errorf("list trigger bindings: %w", err)
	}
	registrations, err := registrationsFor(bindings)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	if err := s.host.Replace(ctx, registrations); err != nil {
		return err
	}
	s.bindings = make(map[uint32]registration, len(registrations))
	for _, registration := range registrations {
		s.bindings[registration.id] = registration
	}
	return nil
}

// Validate rejects malformed or duplicate configured hotkeys before a pipeline
// replaces its published trigger bindings.
func (s *Service) Validate(ctx context.Context, replacingPipelineID string, proposed []domain.TriggerBinding) error {
	bindings, err := s.source.ListAllTriggers(ctx)
	if err != nil {
		return fmt.Errorf("list trigger bindings: %w", err)
	}
	remaining := make([]domain.TriggerBinding, 0, len(bindings)+len(proposed))
	for _, binding := range bindings {
		if binding.PipelineID != replacingPipelineID {
			remaining = append(remaining, binding)
		}
	}
	for _, binding := range proposed {
		// Publish enables Button and Global Hotkey bindings. The caller validates
		// the prospective published state before persistence assigns that flag.
		if binding.Kind == domain.TriggerButton || binding.Kind == domain.TriggerHotkey {
			binding.Enabled = true
		}
		remaining = append(remaining, binding)
	}
	_, err = registrationsFor(remaining)
	return err
}

func (s *Service) dispatch(ctx context.Context) {
	defer s.workers.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case registrationID, open := <-s.host.Events():
			if !open {
				return
			}
			s.mu.RLock()
			registration, ok := s.bindings[registrationID]
			s.mu.RUnlock()
			if !ok {
				continue
			}
			_, _ = s.runner.QueueBinding(ctx, registration.bindingID, pipeline.Packet{
				"trigger": "hotkey",
				"hotkey":  registration.shortcut.canonical,
			}, false)
		}
	}
}

func registrationsFor(bindings []domain.TriggerBinding) ([]registration, error) {
	registrations := make([]registration, 0)
	byShortcut := make(map[string]domain.TriggerBinding)
	nextID := uint32(1)
	for _, binding := range bindings {
		if !binding.Enabled || (binding.Kind != domain.TriggerHotkey && binding.Kind != domain.TriggerButton) {
			continue
		}
		if strings.TrimSpace(binding.Hotkey) == "" {
			continue
		}
		shortcut, err := parseShortcut(binding.Hotkey)
		if err != nil {
			return nil, fmt.Errorf("global hotkey for %q: %w", binding.Label, err)
		}
		if duplicate, exists := byShortcut[shortcut.canonical]; exists {
			return nil, fmt.Errorf("global hotkey %q is already assigned to %q and %q", shortcut.canonical, duplicate.Label, binding.Label)
		}
		byShortcut[shortcut.canonical] = binding
		registrations = append(registrations, registration{id: nextID, bindingID: binding.ID, shortcut: shortcut})
		nextID++
	}
	return registrations, nil
}

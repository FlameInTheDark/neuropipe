// Package hotkey owns global shortcut registration through Wails v3's
// GlobalShortcutManager. The package no longer ships a custom Win32 message
// loop: Wails v3 already owns one and exposes a stable, cross-platform
// Register/Unregister API.
//
// The service reconciles persisted TriggerBindings into native registrations,
// routes presses through the application-owned execution queue, and validates
// proposed bindings to detect duplicates before persistence. Shortcut parsing
// stays in this package so the canonical form used for conflict detection is
// stable and independent of the Wails version.
package hotkey

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/wailsapp/wails/v3/pkg/application"
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

// registration is the in-memory record of one active global shortcut. The
// canonical accelerator string is the key Wails v3 uses for Register/Unregister.
type registration struct {
	bindingID string
	shortcut  chord
}

// Service owns native registration and routes a hotkey press through the
// application-owned execution queue. It does not import persistence directly.
type Service struct {
	source BindingSource
	runner BindingRunner
	app    *application.App

	mu       sync.RWMutex
	started  bool
	ctx      context.Context
	cancel   context.CancelFunc
	bindings map[string]registration // keyed by canonical accelerator
	workers  sync.WaitGroup
}

// New creates a global hotkey service backed by Wails v3's
// GlobalShortcutManager. The Wails app may be nil at construction time; call
// SetApp before Start so the service can register shortcuts.
func New(source BindingSource, runner BindingRunner) *Service {
	return &Service{source: source, runner: runner, bindings: make(map[string]registration)}
}

// SetApp wires the Wails v3 application that owns the GlobalShortcutManager.
// It is called from the desktop Startup hook before Start.
func (s *Service) SetApp(app *application.App) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.app = app
}

// Start loads registered bindings and begins dispatching hotkey presses. The
// supplied context is cancelled by Stop.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	if s.app == nil {
		s.mu.Unlock()
		return fmt.Errorf("global hotkey service requires a Wails application")
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.started = true
	s.mu.Unlock()

	if err := s.Reload(ctx); err != nil {
		s.Stop()
		return err
	}
	return nil
}

// Stop unregisters every global hotkey. Wails v3's GlobalShortcutManager
// owns the native cleanup, so this is a logical unregister backed by the
// manager's UnregisterAll.
func (s *Service) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	cancel := s.cancel
	s.started = false
	s.cancel = nil
	bindings := s.bindings
	s.bindings = make(map[string]registration)
	app := s.app
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.workers.Wait()
	if app != nil {
		for canonical := range bindings {
			_ = app.GlobalShortcut.Unregister(canonical)
		}
	}
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
	app := s.app
	// Unregister every previous binding before registering the new set. Wails
	// v3's GlobalShortcutManager rejects duplicate accelerators, so a clean
	// slate is required.
	for canonical := range s.bindings {
		_ = app.GlobalShortcut.Unregister(canonical)
	}
	next := make(map[string]registration, len(registrations))
	for _, registration := range registrations {
		callback := s.dispatch(registration)
		if err := app.GlobalShortcut.Register(registration.shortcut.canonical, callback); err != nil {
			// Roll back any registrations we already made for this Reload.
			for canonical := range next {
				_ = app.GlobalShortcut.Unregister(canonical)
			}
			return fmt.Errorf("register global hotkey %q: %w", registration.shortcut.canonical, err)
		}
		next[registration.shortcut.canonical] = registration
	}
	s.bindings = next
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
		if binding.Kind == domain.TriggerButton || binding.Kind == domain.TriggerHotkey {
			binding.Enabled = true
		}
		remaining = append(remaining, binding)
	}
	_, err = registrationsFor(remaining)
	return err
}

// dispatch returns the callback invoked by Wails v3 when a registered global
// shortcut fires. The callback runs on its own goroutine inside Wails, so we
// can call the runner directly without blocking the native event loop.
func (s *Service) dispatch(registration registration) func() {
	return func() {
		s.mu.RLock()
		ctx := s.ctx
		s.mu.RUnlock()
		if ctx == nil {
			return
		}
		_, _ = s.runner.QueueBinding(ctx, registration.bindingID, pipeline.Packet{
			"trigger": "hotkey",
			"hotkey":  registration.shortcut.canonical,
		}, false)
	}
}

func registrationsFor(bindings []domain.TriggerBinding) ([]registration, error) {
	registrations := make([]registration, 0)
	byShortcut := make(map[string]domain.TriggerBinding)
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
		registrations = append(registrations, registration{bindingID: binding.ID, shortcut: shortcut})
	}
	return registrations, nil
}

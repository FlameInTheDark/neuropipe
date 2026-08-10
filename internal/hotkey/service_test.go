package hotkey

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

func TestParseShortcut(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		canonical string
		wantError bool
	}{
		{name: "letter", input: "Ctrl+Alt+n", canonical: "Ctrl+Alt+N"},
		{name: "function key", input: "Shift+F12", canonical: "Shift+F12"},
		{name: "named key", input: "Meta+ArrowLeft", canonical: "Meta+ArrowLeft"},
		{name: "modifier only", input: "Ctrl", wantError: true},
		{name: "key only", input: "N", wantError: true},
		{name: "duplicate modifier", input: "Ctrl+Ctrl+N", wantError: true},
		{name: "unsupported key", input: "Ctrl+MediaPlayPause", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chord, err := parseShortcut(test.input)
			if test.wantError {
				if err == nil {
					t.Fatalf("parseShortcut(%q) succeeded", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseShortcut(%q) error = %v", test.input, err)
			}
			if chord.canonical != test.canonical {
				t.Fatalf("canonical = %q, want %q", chord.canonical, test.canonical)
			}
		})
	}
}

func TestServiceRegistersEnabledHotkeysAndDispatchesPresses(t *testing.T) {
	host := &fakeHost{events: make(chan uint32, 1)}
	source := &fakeSource{bindings: []domain.TriggerBinding{
		{ID: "hotkey", Label: "Global", Kind: domain.TriggerHotkey, Enabled: true, Hotkey: "Ctrl+Alt+N"},
		{ID: "button", Label: "Button", Kind: domain.TriggerButton, Enabled: true, Hotkey: "Shift+F12"},
		{ID: "disabled", Label: "Disabled", Kind: domain.TriggerHotkey, Enabled: false, Hotkey: "Ctrl+Alt+D"},
	}}
	runner := &fakeRunner{calls: make(chan runCall, 1)}
	service := newService(source, runner, host)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := service.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer service.Stop()

	registered := host.Registrations()
	if len(registered) != 2 {
		t.Fatalf("registered = %#v, want two hotkeys", registered)
	}
	host.events <- registered[0].id
	select {
	case call := <-runner.calls:
		if call.bindingID != registered[0].bindingID || call.unattended {
			t.Fatalf("run call = %#v", call)
		}
		if call.input["trigger"] != "hotkey" || call.input["hotkey"] != registered[0].shortcut.canonical {
			t.Fatalf("run input = %#v", call.input)
		}
	case <-time.After(time.Second):
		t.Fatal("global hotkey was not dispatched")
	}
}

func TestServiceRejectsDuplicateHotkeys(t *testing.T) {
	source := &fakeSource{bindings: []domain.TriggerBinding{
		{ID: "existing", PipelineID: "other", Label: "Existing", Kind: domain.TriggerHotkey, Enabled: true, Hotkey: "Ctrl+Alt+N"},
	}}
	service := newService(source, &fakeRunner{calls: make(chan runCall)}, &fakeHost{events: make(chan uint32)})
	err := service.Validate(context.Background(), "pipeline", []domain.TriggerBinding{
		{ID: "proposed", Label: "Proposed", Kind: domain.TriggerHotkey, Hotkey: "ctrl + alt + n"},
	})
	if err == nil {
		t.Fatal("Validate() succeeded for a duplicate shortcut")
	}
}

type fakeSource struct{ bindings []domain.TriggerBinding }

func (s *fakeSource) ListAllTriggers(context.Context) ([]domain.TriggerBinding, error) {
	return append([]domain.TriggerBinding(nil), s.bindings...), nil
}

type runCall struct {
	bindingID  string
	input      pipeline.Packet
	unattended bool
}

type fakeRunner struct{ calls chan runCall }

func (r *fakeRunner) QueueBinding(_ context.Context, bindingID string, input pipeline.Packet, unattended bool) (domain.Execution, error) {
	r.calls <- runCall{bindingID: bindingID, input: input, unattended: unattended}
	return domain.Execution{}, nil
}

type fakeHost struct {
	mu            sync.Mutex
	registrations []registration
	events        chan uint32
}

func (h *fakeHost) Start() error { return nil }

func (h *fakeHost) Replace(_ context.Context, registrations []registration) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.registrations = append([]registration(nil), registrations...)
	return nil
}

func (h *fakeHost) Events() <-chan uint32 { return h.events }
func (h *fakeHost) Stop()                 {}

func (h *fakeHost) Registrations() []registration {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]registration(nil), h.registrations...)
}

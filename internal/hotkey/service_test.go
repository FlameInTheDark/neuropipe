package hotkey

import (
	"context"
	"errors"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// stubShortcutApp is a minimal *application.App stand-in. We can't construct
// a real *application.App without running a Wails event loop, so the tests
// exercise the parser and binding-reconciliation logic instead, which is
// where the regression risk lives.
type stubShortcutApp struct {
	registered   map[string]func()
	unregistered map[string]struct{}
}

func newStubApp() *application.App { return nil }

// fakeSource returns a fixed set of trigger bindings for tests.
type fakeSource struct {
	bindings []domain.TriggerBinding
	err      error
}

func (s fakeSource) ListAllTriggers(context.Context) ([]domain.TriggerBinding, error) {
	return s.bindings, s.err
}

// fakeRunner records queued binding invocations.
type fakeRunner struct {
	queued []string
}

func (r *fakeRunner) QueueBinding(_ context.Context, id string, _ pipeline.Packet, _ bool) (domain.Execution, error) {
	r.queued = append(r.queued, id)
	return domain.Execution{ID: id}, nil
}

func TestParseShortcutCanonical(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Ctrl+Alt+N", "Alt+Ctrl+N"},
		{"Shift+Ctrl+P", "Ctrl+Shift+P"},
		{"ctrl+shift+f5", "Ctrl+Shift+F5"},
		{"Win+Down", "Win+DOWN"},
		{"CmdOrCtrl+L", "Ctrl+L"},
		{"Meta+Up", "Win+UP"},
	}
	for _, test := range tests {
		got, err := parseShortcut(test.input)
		if err != nil {
			t.Fatalf("parseShortcut(%q) error: %v", test.input, err)
		}
		if got.canonical != test.want {
			t.Errorf("parseShortcut(%q).canonical = %q; want %q", test.input, got.canonical, test.want)
		}
	}
}

func TestParseShortcutRejectsDuplicates(t *testing.T) {
	bindings := []domain.TriggerBinding{
		{ID: "a", Kind: domain.TriggerHotkey, Enabled: true, Hotkey: "Ctrl+Alt+N", Label: "Daily"},
		{ID: "b", Kind: domain.TriggerHotkey, Enabled: true, Hotkey: "Alt+Ctrl+N", Label: "Weekly"},
	}
	if _, err := registrationsFor(bindings); err == nil {
		t.Fatal("expected duplicate-hotkey error, got nil")
	}
}

func TestParseShortcutRejectsMissingModifier(t *testing.T) {
	if _, err := parseShortcut("N"); err == nil {
		t.Fatal("expected error for shortcut without modifier, got nil")
	}
}

func TestValidateReportsSourceErrors(t *testing.T) {
	service := New(fakeSource{err: errors.New("boom")}, nil)
	if err := service.Validate(context.Background(), "p1", nil); err == nil {
		t.Fatal("expected source error, got nil")
	}
}

// TestNewServiceWithoutAppConfirmsStartGuards guards the SetApp requirement.
// Without a Wails application, Start must refuse to run.
func TestNewServiceWithoutAppConfirmsStartGuards(t *testing.T) {
	service := New(fakeSource{}, &fakeRunner{})
	if err := service.Start(context.Background()); err == nil {
		t.Fatal("expected error when starting without a Wails application, got nil")
	}
}

// Compile-time assertion: stubShortcutApp is unused but documents the test
// boundary. Reference it so the compiler keeps the type.
var _ = newStubApp
var _ = new(fakeSource)
var _ = new(fakeRunner)
var _ *application.App

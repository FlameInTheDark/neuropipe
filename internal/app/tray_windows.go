// Package app contains the sole Wails-facing façade for Neuropipe. This file
// owns the Wails v3 SystemTray adapter that supersedes the previous
// getlantern/systray integration. Wails v3 ships a cross-platform
// SystemTrayManager, so the platform split is no longer required: a single
// adapter works on every supported platform.
package app

import (
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// wailsSystemTray adapts Wails v3's SystemTray to Neuropipe's SystemTray
// contract. It owns the localized menu labels, builds the native menu on
// Start, and forwards click events to the TrayController.
type wailsSystemTray struct {
	controller *TrayController
	app        *application.App
	icon       []byte

	labelsMu sync.RWMutex
	labels   TrayMenuLabels
	menuMu   sync.Mutex
	tray     *application.SystemTray
	menu     *application.Menu
	show     *application.MenuItem
	settings *application.MenuItem
	hide     *application.MenuItem
	close    *application.MenuItem

	ready     atomic.Bool
	started   atomic.Bool
	startOnce sync.Once
	stopOnce  sync.Once
	stopped   chan struct{}
}

// NewSystemTray creates the Wails v3 notification-area adapter. Wails v3 has
// a built-in SystemTrayManager, so this adapter owns only native menu concerns
// while the injected actions retain ownership of application/window behaviour.
// The app may be nil at construction time; call SetApp before Start so the
// tray can register its native surface.
func NewSystemTray(desktop *Desktop, icon []byte) SystemTray {
	tray := &wailsSystemTray{
		controller: NewTrayController(desktopTrayActions{desktop: desktop}),
		icon:       icon,
		labels:     defaultTrayMenuLabels(desktop.GetSettings().Language),
		stopped:    make(chan struct{}),
	}
	desktop.attachTrayMenu(tray)
	return tray
}

// SetApp wires the Wails v3 application that owns the SystemTrayManager. It
// is called from the desktop Startup hook before Start.
func (t *wailsSystemTray) SetApp(app *application.App) {
	t.menuMu.Lock()
	defer t.menuMu.Unlock()
	t.app = app
}

func (t *wailsSystemTray) Start() {
	t.startOnce.Do(func() {
		t.started.Store(true)
		t.menuMu.Lock()
		app := t.app
		t.menuMu.Unlock()
		if app == nil {
			// Without a Wails application the tray cannot register a native
			// surface; mark ready as false so ShouldHideOnClose stays
			// conservative.
			return
		}
		t.build(app)
	})
}

func (t *wailsSystemTray) Stop() {
	if !t.started.Load() {
		return
	}
	t.stopOnce.Do(func() {
		close(t.stopped)
		t.menuMu.Lock()
		tray := t.tray
		t.tray = nil
		t.menuMu.Unlock()
		// Wails v3 owns native cleanup at shutdown; we only release our
		// reference so the menu cannot be re-clicked after Stop.
		_ = tray
	})
}

func (t *wailsSystemTray) Ready() bool    { return t.ready.Load() }
func (t *wailsSystemTray) Quitting() bool { return t.controller.Quitting() }

func (t *wailsSystemTray) SetLabels(labels TrayMenuLabels) {
	t.labelsMu.Lock()
	t.labels = labels
	t.labelsMu.Unlock()
	t.updateMenuLabels(labels)
}

func (t *wailsSystemTray) updateMenuLabels(labels TrayMenuLabels) {
	t.menuMu.Lock()
	defer t.menuMu.Unlock()
	if t.show != nil {
		t.show.SetLabel(labels.Show)
	}
	if t.settings != nil {
		t.settings.SetLabel(labels.Settings)
	}
	if t.hide != nil {
		t.hide.SetLabel(labels.Hide)
	}
	if t.close != nil {
		t.close.SetLabel(labels.Close)
	}
}

// build constructs the native tray icon, menu, and click handlers using the
// Wails v3 SystemTrayManager.
func (t *wailsSystemTray) build(app *application.App) {
	t.labelsMu.RLock()
	labels := t.labels
	t.labelsMu.RUnlock()

	tray := app.SystemTray.New()
	tray.SetIcon(t.icon)
	tray.SetTooltip("Neuropipe")

	menu := app.NewMenu()
	show := menu.Add(labelText(labels.Show))
	show.OnClick(func(_ *application.Context) {
		t.controller.Show()
	})
	settings := menu.Add(labelText(labels.Settings))
	settings.OnClick(func(_ *application.Context) {
		t.controller.OpenSettings()
	})
	menu.AddSeparator()
	hide := menu.Add(labelText(labels.Hide))
	hide.OnClick(func(_ *application.Context) {
		t.controller.Hide()
	})
	menu.AddSeparator()
	close := menu.Add(labelText(labels.Close))
	close.OnClick(func(_ *application.Context) {
		t.controller.Quit()
	})

	t.menuMu.Lock()
	t.tray = tray
	t.menu = menu
	t.show = show
	t.settings = settings
	t.hide = hide
	t.close = close
	t.menuMu.Unlock()

	tray.SetMenu(menu)
	t.ready.Store(true)
}

// labelText returns the menu item label, falling back to a non-empty string
// so Wails v3's menu builder never receives an empty label.
func labelText(value string) string {
	if value == "" {
		return " "
	}
	return value
}

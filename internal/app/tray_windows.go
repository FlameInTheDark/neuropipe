//go:build windows

package app

import (
	"sync"
	"sync/atomic"

	"github.com/getlantern/systray"
)

type windowsSystemTray struct {
	controller *TrayController
	icon       []byte

	labelsMu sync.RWMutex
	labels   TrayMenuLabels
	menuMu   sync.RWMutex
	show     *systray.MenuItem
	settings *systray.MenuItem
	hide     *systray.MenuItem
	close    *systray.MenuItem

	ready     atomic.Bool
	started   atomic.Bool
	startOnce sync.Once
	stopOnce  sync.Once
	stopped   chan struct{}
}

// NewSystemTray creates the Windows-native notification-area adapter. Wails
// v2 has no tray API, so this adapter owns only native menu concerns while the
// injected actions retain ownership of application/window behaviour.
func NewSystemTray(desktop *Desktop, icon []byte) SystemTray {
	tray := &windowsSystemTray{
		controller: NewTrayController(desktopTrayActions{desktop: desktop}),
		icon:       icon,
		labels:     defaultTrayMenuLabels(desktop.GetSettings().Language),
		stopped:    make(chan struct{}),
	}
	desktop.attachTrayMenu(tray)
	return tray
}

func (t *windowsSystemTray) Start() {
	t.startOnce.Do(func() {
		t.started.Store(true)
		go systray.Run(t.onReady, t.onExit)
	})
}

func (t *windowsSystemTray) Stop() {
	if !t.started.Load() {
		return
	}
	t.stopOnce.Do(func() {
		close(t.stopped)
		systray.Quit()
	})
}

func (t *windowsSystemTray) Ready() bool    { return t.ready.Load() }
func (t *windowsSystemTray) Quitting() bool { return t.controller.Quitting() }

func (t *windowsSystemTray) SetLabels(labels TrayMenuLabels) {
	t.labelsMu.Lock()
	t.labels = labels
	t.labelsMu.Unlock()
	t.updateMenuLabels(labels)
}

func (t *windowsSystemTray) updateMenuLabels(labels TrayMenuLabels) {
	t.menuMu.RLock()
	defer t.menuMu.RUnlock()
	if t.show != nil {
		t.show.SetTitle(labels.Show)
	}
	if t.settings != nil {
		t.settings.SetTitle(labels.Settings)
	}
	if t.hide != nil {
		t.hide.SetTitle(labels.Hide)
	}
	if t.close != nil {
		t.close.SetTitle(labels.Close)
	}
}

func (t *windowsSystemTray) onReady() {
	t.labelsMu.RLock()
	labels := t.labels
	t.labelsMu.RUnlock()

	systray.SetIcon(t.icon)
	systray.SetTooltip("Neuropipe")
	show := systray.AddMenuItem(labels.Show, "")
	settings := systray.AddMenuItem(labels.Settings, "")
	systray.AddSeparator()
	hide := systray.AddMenuItem(labels.Hide, "")
	systray.AddSeparator()
	close := systray.AddMenuItem(labels.Close, "")

	t.menuMu.Lock()
	t.show, t.settings, t.hide, t.close = show, settings, hide, close
	t.menuMu.Unlock()
	t.labelsMu.RLock()
	labels = t.labels
	t.labelsMu.RUnlock()
	t.updateMenuLabels(labels)
	t.ready.Store(true)

	for {
		select {
		case <-show.ClickedCh:
			t.controller.Show()
		case <-settings.ClickedCh:
			t.controller.OpenSettings()
		case <-hide.ClickedCh:
			t.controller.Hide()
		case <-close.ClickedCh:
			t.controller.Quit()
		case <-t.stopped:
			return
		}
	}
}

func (t *windowsSystemTray) onExit() {
	t.ready.Store(false)
	t.stopOnce.Do(func() { close(t.stopped) })
}

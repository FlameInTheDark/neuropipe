package app

import (
	"sync/atomic"

	"github.com/FlameInTheDark/neuropipe/internal/localization"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const openSettingsEvent = "app.open.settings"

// TrayMenuLabels is the renderer-supplied, localized copy for the native tray
// menu. The fallback labels keep the menu usable during renderer startup.
type TrayMenuLabels struct {
	Show     string `json:"show"`
	Settings string `json:"settings"`
	Hide     string `json:"hide"`
	Close    string `json:"close"`
}

// TrayActions is the narrow application-facing contract used by the native
// tray adapter. It keeps the adapter independent from Wails and Desktop.
type TrayActions interface {
	Show()
	OpenSettings()
	Hide()
	Quit()
}

// TrayController owns the semantic actions of a tray menu. Native menu
// implementations forward their click events to this small, testable type.
type TrayController struct {
	actions  TrayActions
	quitting atomic.Bool
}

func NewTrayController(actions TrayActions) *TrayController {
	return &TrayController{actions: actions}
}

func (c *TrayController) Show()         { c.actions.Show() }
func (c *TrayController) OpenSettings() { c.actions.OpenSettings() }
func (c *TrayController) Hide()         { c.actions.Hide() }

// Quit records an explicit exit request so the close-to-tray lifecycle hook
// allows the application to terminate. Repeated native menu notifications are
// intentionally harmless.
func (c *TrayController) Quit() {
	if c.quitting.CompareAndSwap(false, true) {
		c.actions.Quit()
	}
}

func (c *TrayController) Quitting() bool { return c.quitting.Load() }

// SystemTray is the platform boundary used by the Wails bootstrap. The
// Windows implementation owns its native event loop; unsupported platforms
// supply a no-op implementation so close never hides an inaccessible app.
type SystemTray interface {
	Start()
	Stop()
	Ready() bool
	Quitting() bool
	SetLabels(TrayMenuLabels)
}

// ShouldHideOnClose protects users from a hidden, inaccessible application:
// close is intercepted only when the preference is enabled, the native tray is
// ready, and an explicit tray-quit action is not already in progress.
func ShouldHideOnClose(hideToTrayOnClose bool, tray SystemTray) bool {
	return hideToTrayOnClose && tray != nil && tray.Ready() && !tray.Quitting()
}

type trayLabelSink interface {
	SetLabels(TrayMenuLabels)
}

type desktopTrayActions struct{ desktop *Desktop }

func (a desktopTrayActions) Show() {
	ctx := a.desktop.context()
	wailsruntime.WindowShow(ctx)
	wailsruntime.WindowUnminimise(ctx)
}

func (a desktopTrayActions) OpenSettings() {
	a.Show()
	a.desktop.emit(openSettingsEvent, nil)
}

func (a desktopTrayActions) Hide() {
	wailsruntime.WindowHide(a.desktop.context())
}

func (a desktopTrayActions) Quit() {
	wailsruntime.Quit(a.desktop.context())
}

func defaultTrayMenuLabels(language string) TrayMenuLabels {
	switch localization.Normalize(language) {
	case localization.German:
		return TrayMenuLabels{Show: "Neuropipe anzeigen", Settings: "Einstellungen öffnen", Hide: "Neuropipe ausblenden", Close: "Neuropipe beenden"}
	case localization.French:
		return TrayMenuLabels{Show: "Afficher Neuropipe", Settings: "Ouvrir les paramètres", Hide: "Masquer Neuropipe", Close: "Fermer Neuropipe"}
	case localization.Russian:
		return TrayMenuLabels{Show: "Показать Neuropipe", Settings: "Открыть настройки", Hide: "Скрыть Neuropipe", Close: "Закрыть Neuropipe"}
	default:
		return TrayMenuLabels{Show: "Show Neuropipe", Settings: "Open Settings", Hide: "Hide Neuropipe", Close: "Close Neuropipe"}
	}
}

func normalizeTrayMenuLabels(labels TrayMenuLabels, language string) TrayMenuLabels {
	fallback := defaultTrayMenuLabels(language)
	if labels.Show == "" {
		labels.Show = fallback.Show
	}
	if labels.Settings == "" {
		labels.Settings = fallback.Settings
	}
	if labels.Hide == "" {
		labels.Hide = fallback.Hide
	}
	if labels.Close == "" {
		labels.Close = fallback.Close
	}
	return labels
}

func (d *Desktop) attachTrayMenu(sink trayLabelSink) {
	d.trayMu.Lock()
	d.trayMenu = sink
	d.trayMu.Unlock()
}

// ConfigureTrayMenu applies localized menu labels received from the renderer.
// It is safe to call before the platform tray finishes initializing.
func (d *Desktop) ConfigureTrayMenu(labels TrayMenuLabels) {
	settings := d.GetSettings()
	labels = normalizeTrayMenuLabels(labels, settings.Language)
	d.trayMu.RLock()
	sink := d.trayMenu
	d.trayMu.RUnlock()
	if sink != nil {
		sink.SetLabels(labels)
	}
}

// ShouldHideToTrayOnClose returns the current persisted close behaviour.
func (d *Desktop) ShouldHideToTrayOnClose() bool {
	d.settingsMu.RLock()
	defer d.settingsMu.RUnlock()
	return d.settings.HideToTrayOnClose
}

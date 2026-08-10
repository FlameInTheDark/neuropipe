//go:build !windows

package app

// unavailableSystemTray deliberately reports itself as unavailable. This
// keeps a configured close-to-tray preference from hiding the main window on
// platforms where this Wails v2 build has no supported native tray adapter.
type unavailableSystemTray struct{ controller *TrayController }

func NewSystemTray(desktop *Desktop, _ []byte) SystemTray {
	tray := &unavailableSystemTray{controller: NewTrayController(desktopTrayActions{desktop: desktop})}
	desktop.attachTrayMenu(tray)
	return tray
}

func (t *unavailableSystemTray) Start()                   {}
func (t *unavailableSystemTray) Stop()                    {}
func (t *unavailableSystemTray) Ready() bool              { return false }
func (t *unavailableSystemTray) Quitting() bool           { return t.controller.Quitting() }
func (t *unavailableSystemTray) SetLabels(TrayMenuLabels) {}

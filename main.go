package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/FlameInTheDark/neuropipe/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var trayIcon []byte

// appVersion is replaced by the release workflow. Development builds keep the
// default value and deliberately skip the public update check.
var appVersion = "dev"

// mainWindowName is the Name assigned to the primary WebviewWindow. The tray
// adapter looks up the window by this name to show, hide, and unminimise it
// without depending on a Wails v2 context.
const mainWindowName = "main"

// mustSub extracts a sub-filesystem from the embedded assets. It panics on
// failure because a missing frontend/dist directory is a build-time error:
// the //go:embed directive requires the directory to exist with at least one
// file. In dev mode, a placeholder index.html is committed to the repo so
// the embed always succeeds; the Wails asset server proxies to Vite for the
// real content.
func mustSub(fsys embed.FS, path string) fs.FS {
	sub, err := fs.Sub(fsys, path)
	if err != nil {
		log.Fatal(err)
	}
	return sub
}

func main() {
	desktop, err := app.New(appVersion)
	if err != nil {
		log.Fatal(err)
	}
	tray := app.NewSystemTray(desktop, trayIcon)

	appInstance := application.New(application.Options{
		Name:        "Neuropipe",
		Description: "Neuropipe local automation workspace",
		Services: []application.Service{
			application.NewService(desktop),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(mustSub(assets, "frontend/dist")),
		},
		// Frameless: the renderer owns the title bar, so the OS frame is
		// suppressed exactly like the previous Wails v2 build.
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		// ShouldQuit intercepts the OS-level quit. When the user has chosen to
		// hide to the tray instead of quitting, the main window is hidden and
		// the application keeps running. An explicit tray Close action clears
		// the controller's quitting flag, allowing the next ShouldQuit call to
		// return true.
		ShouldQuit: func() bool {
			if app.ShouldHideOnClose(desktop.ShouldHideToTrayOnClose(), tray) {
				desktop.HideMainWindow()
				return false
			}
			return true
		},
	})

	// The main window mirrors the previous Wails v2 layout: frameless, dark
	// background, and a stable name so the tray adapter can look it up.
	window := appInstance.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             mainWindowName,
		Title:            "Neuropipe",
		Width:            1440,
		Height:           920,
		MinWidth:         1080,
		MinHeight:        700,
		Frameless:        true,
		BackgroundColour: application.NewRGBA(9, 9, 11, 255),
		URL:              "/",
	})

	// Wire lifecycle hooks. The ApplicationStarted event fires after the
	// Wails event loop is ready, so Desktop.Startup can safely call into the
	// Wails v3 application's managers (Dialog, Event, GlobalShortcut, etc.).
	appInstance.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		desktop.Startup(appInstance)
		tray.Start()
	})

	// Keep the window reference alive so linters don't complain; the window
	// is registered with the WindowManager during NewWithOptions.
	_ = window

	appInstance.OnShutdown(func() {
		tray.Stop()
		desktop.Shutdown(appInstance.Context())
	})

	if err := appInstance.Run(); err != nil {
		log.Fatal(err)
	}
}

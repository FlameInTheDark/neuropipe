package main

import (
	"context"
	"embed"
	"log"

	"github.com/FlameInTheDark/neuropipe/internal/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

// appVersion is replaced by the release workflow. Development builds keep the
// default value and deliberately skip the public update check.
var appVersion = "dev"

func main() {
	desktop, err := app.New(appVersion)
	if err != nil {
		log.Fatal(err)
	}
	tray := app.NewSystemTray(desktop, trayIcon)

	if err := wails.Run(&options.App{
		Title:            "Neuropipe",
		Width:            1440,
		Height:           920,
		MinWidth:         1080,
		MinHeight:        700,
		Frameless:        true,
		BackgroundColour: &options.RGBA{R: 9, G: 9, B: 11, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup: func(ctx context.Context) {
			desktop.Startup(ctx)
			tray.Start()
		},
		OnBeforeClose: func(ctx context.Context) bool {
			if !app.ShouldHideOnClose(desktop.ShouldHideToTrayOnClose(), tray) {
				return false
			}
			wailsruntime.WindowHide(ctx)
			return true
		},
		OnShutdown: func(ctx context.Context) {
			tray.Stop()
			desktop.Shutdown(ctx)
		},
		Bind:    []interface{}{desktop},
		Windows: &windows.Options{WebviewIsTransparent: false},
	}); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"embed"
	"log"

	"github.com/FlameInTheDark/neuropipe/internal/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// appVersion is replaced by the release workflow. Development builds keep the
// default value and deliberately skip the public update check.
var appVersion = "dev"

func main() {
	desktop, err := app.New(appVersion)
	if err != nil {
		log.Fatal(err)
	}

	if err := wails.Run(&options.App{
		Title:            "Neuropipe",
		Width:            1440,
		Height:           920,
		MinWidth:         1080,
		MinHeight:        700,
		Frameless:        true,
		BackgroundColour: &options.RGBA{R: 9, G: 9, B: 11, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        desktop.Startup,
		OnShutdown:       desktop.Shutdown,
		Bind:             []interface{}{desktop},
		Windows:          &windows.Options{WebviewIsTransparent: false},
	}); err != nil {
		log.Fatal(err)
	}
}

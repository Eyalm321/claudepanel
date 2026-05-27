package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// trayIconBytes is defined per-OS in icon_{windows,darwin,linux}.go.

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "__claudebar__",
		Width:            1920,
		Height:           app.cfg.BarHeight,
		MinWidth:         400,
		MinHeight:        1,
		MaxHeight:        0,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    false,
		HideWindowOnClose: true,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnDomReady: app.domReady,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		CSSDragProperty:  "--wails-draggable",
		CSSDragValue:     "drag",
	})

	if err != nil {
		log.Fatalf("Wails error: %v", err)
	}
}

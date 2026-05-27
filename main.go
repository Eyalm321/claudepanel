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

//go:embed build/windows/icon.ico
var trayIconBytes []byte

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "__claudebar__",
		Width:            1920,
		Height:           app.cfg.BarHeight,
		MinWidth:         400,
		MinHeight:        app.cfg.BarHeight,
		MaxHeight:        app.cfg.BarHeight,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		HideWindowOnClose: true,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnDomReady: app.domReady,
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

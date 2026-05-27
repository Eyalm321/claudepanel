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
		// Title intentionally blank: with WebviewIsTransparent the title text
		// can bleed through the frame when the window loses focus.
		Title:            "",
		Width:            1920,
		Height:           app.cfg.BarHeight,
		MinWidth:         400,
		MinHeight:        1,
		MaxHeight:        0,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		HideWindowOnClose: true,
		// Match the bar's --bg colour. When the window is briefly visible
		// before the slide-down animation starts on expand, we want the
		// "pop in" frame to be dark-on-dark (window bg ≈ bar bg) so it's
		// imperceptible. The actual hide is handled by ShowWindow(SW_HIDE)
		// once the slide-up animation finishes.
		BackgroundColour: &options.RGBA{R: 0x0B, G: 0x0C, B: 0x0E, A: 255},
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

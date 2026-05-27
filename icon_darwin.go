//go:build darwin

package main

// Tray icon: getlantern/systray on macOS uses NSImage from raw bytes, which
// supports PNG directly. The .app bundle's Dock icon is generated separately
// by Wails from build/appicon.png during `wails build -platform darwin/...`.

import _ "embed"

//go:embed build/appicon.png
var trayIconBytes []byte

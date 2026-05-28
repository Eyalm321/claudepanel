//go:build windows

package main

// Tray icon: Wails v3's setIcon calls CreateIconFromResourceEx on the raw bytes.
// That API takes a single icon resource's bits, not a multi-image ICO container
// with a directory header, so embedding build/windows/icon.ico fails with
// "The operation completed successfully" (a NULL return + ERROR_SUCCESS).
// PNG goes through the same call cleanly on Vista+. The .exe's window/taskbar
// icon is still wired up separately from the ICO via the resource manifest.

import _ "embed"

//go:embed build/appicon.png
var trayIconBytes []byte

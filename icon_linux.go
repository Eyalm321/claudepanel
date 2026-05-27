//go:build linux

package main

import _ "embed"

//go:embed build/linux/icon.png
var trayIconBytes []byte

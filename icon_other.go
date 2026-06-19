//go:build !windows

package main

import _ "embed"

// On macOS/Linux the tray and notifications both use the PNG.

//go:embed assets/toast.png
var trayIconData []byte

var notifyIconData = trayIconData

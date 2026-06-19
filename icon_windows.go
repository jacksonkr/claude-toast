//go:build windows

package main

import _ "embed"

// trayIconData is an .ico (what the Windows tray expects); notifyIconData is a
// .png (what toast app-logo images want).

//go:embed assets/toast.ico
var trayIconData []byte

//go:embed assets/toast.png
var notifyIconData []byte

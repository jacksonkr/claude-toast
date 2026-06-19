//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func desktopEntryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "autostart", "claude-toast.desktop")
}

func enableAutostart() error {
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Claude Toast
Comment=Desktop notifications for Claude Code
Exec=%s tray
X-GNOME-Autostart-enabled=true
`, trayExe())

	p := desktopEntryPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(entry), 0o644)
}

func disableAutostart() error {
	if err := os.Remove(desktopEntryPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// On Linux notify-send carries the app name per call; nothing to register.
func registerBranding() error   { return nil }
func unregisterBranding() error { return nil }

func startTrayNow() error {
	return exec.Command(trayExe(), "tray").Start()
}

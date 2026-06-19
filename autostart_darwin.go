//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const launchAgentLabel = "com.claudetoast.tray"

func launchAgentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

func enableAutostart() error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key>
  <array><string>%s</string><string>tray</string></array>
  <key>RunAtLoad</key><true/>
</dict>
</plist>
`, launchAgentLabel, trayExe())

	p := launchAgentPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(plist), 0o644); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "unload", p).Run()
	return exec.Command("launchctl", "load", p).Run()
}

func disableAutostart() error {
	p := launchAgentPath()
	_ = exec.Command("launchctl", "unload", p).Run()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// macOS toasts are posted by osascript and inherit its identity; nothing to register.
func registerBranding() error   { return nil }
func unregisterBranding() error { return nil }

func startTrayNow() error {
	return exec.Command(trayExe(), "tray").Start()
}

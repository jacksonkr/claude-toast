//go:build windows

package main

import (
	"fmt"
	"os/exec"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath    = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName  = "ClaudeToast"
	aumidKeyPath  = `Software\Classes\AppUserModelId\` + appID
)

func enableAutostart() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(runValueName, fmt.Sprintf(`"%s" tray`, trayExe()))
}

func disableAutostart() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	_ = k.DeleteValue(runValueName)
	return nil
}

// registerBranding makes Windows attribute the toasts to "Claude Toast" with
// our icon, via the documented HKCU AppUserModelId registry method.
func registerBranding() error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, aumidKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetStringValue("DisplayName", "Claude Toast"); err != nil {
		return err
	}
	if icon := iconFilePath(); icon != "" {
		_ = k.SetStringValue("IconUri", icon)
	}
	return nil
}

func unregisterBranding() error {
	_ = registry.DeleteKey(registry.CURRENT_USER, aumidKeyPath)
	return nil
}

func startTrayNow() error {
	return exec.Command(trayExe(), "tray").Start()
}

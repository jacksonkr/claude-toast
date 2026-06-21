package main

import (
	"context"
	"time"

	"fyne.io/systray"
)

// runTray runs the always-on system-tray daemon (the 🍞 icon + menu). On
// Windows this is launched as the GUI-subsystem claude-toast-tray.exe so no
// console window appears.
func runTray() {
	systray.Run(onTrayReady, func() {})
}

func onTrayReady() {
	systray.SetIcon(trayIconData)
	systray.SetTitle("")
	systray.SetTooltip("Claude Toast — notifications for Claude Code")

	mPause := systray.AddMenuItemCheckbox("Pause notifications", "Mute Claude toasts", isPaused())
	systray.AddSeparator()
	mPairing := systray.AddMenuItem("Show pairing code", "Open the cross-device pairing token")
	mFolder := systray.AddMenuItem("Open config folder", "Open the claude-toast data folder")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop the tray until next login")

	// Make sure this device has a UID/relay so it runs as a node out of the box.
	ensureInitialized()

	// Listen for broadcasts from the user's other devices and show them locally.
	go broadcastListener(context.Background())

	go func() {
		for {
			select {
			case <-mPause.ClickedCh:
				if mPause.Checked() {
					if setPaused(false) == nil {
						mPause.Uncheck()
					}
				} else {
					if setPaused(true) == nil {
						mPause.Check()
					}
				}
			case <-mPairing.ClickedCh:
				showPairingCode()
			case <-mFolder.ClickedCh:
				if d, err := ensureConfigDir(); err == nil {
					_ = openPath(d)
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

// broadcastListener keeps a subscription to the broadcast topic whenever the
// device is paired with broadcast enabled, showing inbound toasts. It reloads
// config on every (re)connect so pairing while the tray is running takes effect
// without a restart.
func broadcastListener(ctx context.Context) {
	for {
		cfg, err := loadConfig()
		if err == nil && cfg.paired() && cfg.Broadcast {
			if ks, ok := keysetFor(cfg); ok {
				_ = ntfySubscribe(ctx, cfg.NtfyServer, []string{ks.broadcastTopic()}, func(m ntfyMessage) {
					if isPaused() {
						return
					}
					if n, ok := broadcastToNotification(cfg, m); ok {
						_ = showNotification(n)
					}
				})
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			// reconnect / re-check pairing
		}
	}
}

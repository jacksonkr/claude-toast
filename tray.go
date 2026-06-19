package main

import "fyne.io/systray"

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

	mTest := systray.AddMenuItem("Send test toast", "Fire a sample notification")
	mPause := systray.AddMenuItemCheckbox("Pause notifications", "Mute Claude toasts", isPaused())
	systray.AddSeparator()
	mFolder := systray.AddMenuItem("Open config folder", "Open the claude-toast data folder")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop the tray until next login")

	go func() {
		for {
			select {
			case <-mTest.ClickedCh:
				_ = showNotification(notification{
					AppName:  "Claude Toast",
					Title:    "Claude Toast",
					Lines:    []string{"Test notification", "The tray is working. 🍞"},
					IconPath: iconFilePath(),
				})
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

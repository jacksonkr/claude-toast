//go:build windows

package main

import (
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-toast/toast"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procFindWindowW     = user32.NewProc("FindWindowW")
	procIsWindowVisible = user32.NewProc("IsWindowVisible")
)

// isTrayFlyoutOpen reports whether the taskbar hidden-icons flyout ("^") is
// on screen right now. Toasts land in the same bottom-right region, so if
// the user just popped the flyout to click a tray icon and a toast fires,
// the toast will cover the icons.
//
// Two window classes cover Win10 and Win11 builds:
//   - NotifyIconOverflowWindow          classic overflow, still used in Win11
//   - TopLevelWindowForOverflowXamlIsland  Win11 XAML-hosted overflow
//
// Either being present AND visible means the flyout is open.
func isTrayFlyoutOpen() bool {
	for _, class := range []string{
		"NotifyIconOverflowWindow",
		"TopLevelWindowForOverflowXamlIsland",
	} {
		p, err := syscall.UTF16PtrFromString(class)
		if err != nil {
			continue
		}
		hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(p)), 0)
		if hwnd == 0 {
			continue
		}
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		if vis != 0 {
			return true
		}
	}
	return false
}

// waitForTrayFlyoutClose returns immediately when the flyout is closed, and
// otherwise polls until it closes or timeout elapses. Cap it well under the
// 10s hook budget so the hook never stalls the pipeline; the toast is still
// pushed after the wait so the notification reaches the Action Center
// regardless of whether the user closed the flyout in time.
func waitForTrayFlyoutClose(timeout time.Duration) {
	if !isTrayFlyoutOpen() {
		return
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if !isTrayFlyoutOpen() {
			return
		}
	}
}

func showNotification(n notification) error {
	waitForTrayFlyoutClose(4 * time.Second)

	note := toast.Notification{
		AppID:   appID,
		Title:   n.Title,
		Message: strings.Join(n.Lines, "\n"),
		Audio:   toast.Default,
	}
	if n.IconPath != "" {
		note.Icon = n.IconPath
	}
	return note.Push()
}

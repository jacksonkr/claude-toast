// Command claude-toast shows desktop notifications when Claude Code needs
// attention (a permission prompt, idle input, or a finished response), and
// runs a small system-tray daemon to control them. It is cross-platform:
// Windows, macOS, and Linux.
package main

import (
	"fmt"
	"os"
)

// appID is the Windows AppUserModelID the toasts post under. Registered with a
// DisplayName + icon at install time so notifications read as "Claude Toast".
const appID = "Claude.Toast"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		return
	}

	switch args[0] {
	case "hook":
		runHook(args[1:])
	case "tray":
		runTray()
	case "test":
		runTest()
	case "install":
		mustRun(runInstall())
	case "uninstall":
		mustRun(runUninstall())
	case "uid":
		runUID()
	case "link":
		runLink(args[1:])
	case "pair":
		runPair(args[1:])
	case "status":
		runStatus()
	case "simulate-pretooluse":
		runSimulatePreToolUse(args[1:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", args[0])
		usage()
		os.Exit(2)
	}
}

func mustRun(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runTest() {
	n := notification{
		AppName:  "Claude Toast",
		Title:    "Claude Toast",
		Lines:    []string{"Test notification", "If you can see this, it works. 🍞"},
		IconPath: iconFilePath(),
	}
	if err := showNotification(n); err != nil {
		fmt.Fprintln(os.Stderr, "could not show notification:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`claude-toast - desktop notifications for Claude Code

Usage:
  claude-toast install      Register the Claude Code hooks + tray autostart
  claude-toast uninstall    Remove the hooks, autostart, and branding
  claude-toast tray         Run the system-tray daemon (started automatically)
  claude-toast test         Fire a test notification
  claude-toast uid          Show this device's UID (for linking other devices)
  claude-toast link <uid>   Link this device to another device's UID
  claude-toast status       Show cross-device settings
  claude-toast hook --event <Notification|Stop>
                            Invoked by Claude Code; reads its JSON from stdin

  Advanced: claude-toast pair [--server <url>] for a self-hosted relay.`)
}

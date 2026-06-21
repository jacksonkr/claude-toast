# 🍞 Claude Toast

Native desktop notifications for [Claude Code](https://claude.com/claude-code) —
get a toast (with sound) the instant Claude finishes a turn or needs your input,
plus a system-tray icon to control it. **Windows, macOS, and Linux.**

## Who this is for

Power users who keep Claude Code running while they work **across multiple
monitors and multiple virtual desktops**.

If you run more than a couple of sessions, the terminal that needs you is almost
never the window you're looking at — it's on another monitor, or on a virtual
desktop you've switched away from (Windows **⊞ Win+Tab** / macOS **Mission
Control Spaces** / GNOME workspaces). You either babysit the terminals or you
lose minutes to a session that finished long ago and has been sitting idle
waiting for you.

Claude Toast fixes that. The notification is an **OS-level toast**, so it
surfaces over whatever you're doing, on whichever desktop or screen is in front
of you — you don't have to be looking at the terminal, or even on the same
virtual desktop, to know Claude is waiting. A persistent tray icon gives you
at-a-glance presence and one-click control no matter where you are.

Each toast shows, top to bottom:

1. the **project** (working directory) as the title — so you know which checkout
2. your **`/rename` session title**, if you set one
3. the **first few words of your last message** — so you know *which* session, even with several open
4. **what Claude wants** — "Finished responding", "Claude needs your permission…"

## Install

### From a release (recommended)

Download the archive for your OS from the
[Releases](https://github.com/jacksonkr/claude-toast/releases) page, unzip it
somewhere permanent, then run the installer once:

```sh
# macOS / Linux
./claude-toast install

# Windows (PowerShell)
.\claude-toast.exe install
```

`install` wires the hooks into `~/.claude/settings.json`, registers the tray to
start at login, and launches it immediately. Run `claude-toast test` to fire a
sample notification.

### From source

Requires Go 1.25+.

```sh
git clone https://github.com/jacksonkr/claude-toast
cd claude-toast

# macOS / Linux
go build -o claude-toast . && ./claude-toast install

# Windows
.\build.ps1 ; .\claude-toast.exe install
```

> **Linux** needs GTK + AppIndicator dev headers for the tray:
> `sudo apt-get install libgtk-3-dev libayatana-appindicator3-dev`

## Usage

```
claude-toast install      Register the Claude Code hooks + tray autostart
claude-toast uninstall    Remove the hooks, autostart, and branding
claude-toast tray         Run the system-tray daemon (started automatically)
claude-toast test         Fire a test notification
claude-toast hook --event <Notification|Stop>   (called by Claude Code)
```

The tray menu lets you **pause/resume** notifications and **open the config
folder**.

## How it works

Claude Code fires [hooks](https://code.claude.com/docs/en/hooks) on its
`Notification` and `Stop` events, passing context as JSON on stdin. `install`
adds a hook that runs `claude-toast hook`, which reads that JSON, finds your last
message in the session transcript, and shows a native notification:

| OS      | Notifications          | Tray              | Autostart                       |
| ------- | ---------------------- | ----------------- | ------------------------------- |
| Windows | WinRT toast (go-toast) | `fyne.io/systray` | `HKCU\…\Run`                    |
| macOS   | `osascript`            | `fyne.io/systray` | LaunchAgent                     |
| Linux   | `notify-send`          | `fyne.io/systray` | `~/.config/autostart/*.desktop` |

On Windows two binaries are built from the same source: `claude-toast.exe`
(console — for `install`/`hook`/`test`) and `claude-toast-tray.exe` (GUI
subsystem — so the autostarted tray never flashes a console window).

### A note on remote / SSH sessions

Hooks run wherever `claude` runs. If you SSH into another machine and start
Claude there, the toast fires on the remote box. Claude Toast covers sessions
whose `claude` process runs on *your* desktop — which is exactly the
multi-monitor / multi-desktop case it's built for.

## Uninstall

```sh
claude-toast uninstall
```

Removes the hooks, the autostart entry, and (on Windows) the toast branding.
Quit the running tray from its menu.

## License

MIT — see [LICENSE](LICENSE).

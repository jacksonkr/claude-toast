# CLAUDE.md

Guidance for Claude Code when working in this repo.

## What this is

**Claude Toast** — cross-platform (Windows/macOS/Linux) desktop notifications +
a system-tray daemon for Claude Code, fired from its `Notification` and `Stop`
hooks. Target user: someone running Claude Code across multiple monitors and
multiple virtual desktops who would otherwise miss a session that needs them.

It is an **alert layer only**. Jumping between sessions is done with
`claude agents` (Agent View), not by this tool.

## Stack & layout

Go, single `package main`, split per-OS by build tags. No internal packages, so
the module path is cosmetic (only in `go.mod`).

| File(s)                       | Role                                                       |
|-------------------------------|------------------------------------------------------------|
| `main.go`                     | CLI dispatch: `install`/`uninstall`/`tray`/`test`/`hook`   |
| `hook.go`                     | Parse Claude Code's stdin JSON, build the toast text       |
| `notify.go`                   | `notification` struct (the cross-OS payload)               |
| `notify_{windows,darwin,linux}.go` | `showNotification` per OS (go-toast / osascript / notify-send) |
| `tray.go`                     | `fyne.io/systray` daemon + menu                            |
| `install.go`                  | settings.json hook wiring (idempotent)                     |
| `autostart_{windows,darwin,linux}.go` | autostart + (Windows) AUMID branding              |
| `icon_{windows,other}.go`     | `//go:embed` the tray/notify icons                         |
| `paths.go`                    | config dir, pause flag, icon extraction                    |
| `tools/gen-icon.ps1`          | Dev-time icon generator → committed `assets/toast.{ico,png}` |

## Build & test (Windows)

```powershell
.\build.ps1          # builds claude-toast.exe (console) + claude-toast-tray.exe (GUI)
.\claude-toast.exe test
'{"cwd":"C:\\demo","session_id":"t1","hook_event_name":"Stop"}' | .\claude-toast.exe hook --event Stop
```

`go build` for the other OSes; CI (`.github/workflows/ci.yml`) builds + vets on
all three. Releases are native per-OS builds attached on a `v*` tag
(`release.yml`).

## Conventions / gotchas (do not re-break)

- **Two Windows binaries from one source.** `claude-toast.exe` is console
  subsystem; `claude-toast-tray.exe` is built with `-ldflags "-H windowsgui"`.
  The tray MUST be the GUI build — a console-subsystem tray launched from the
  Run key flashes/keeps a console window. Hooks use the console exe (Claude
  Code shares its console, so no flash).
- **Windows branding** is the documented HKCU `AppUserModelId` registry method
  (DisplayName + IconUri), registered at install. The old BurntToast couldn't
  do this; don't reintroduce a PowerShell/BurntToast dependency.
- **The tray needs cgo on macOS/Linux** (systray), but **not on Windows** (pure
  Go). That's why Linux CI installs `libgtk-3-dev libayatana-appindicator3-dev`
  and why we build natively per-OS instead of cross-compiling.
- **`install`/`uninstall` only touch claude-toast's own hook entries** in
  `settings.json` (matched by the command containing `claude-toast`), and
  preserve everything else. Keep them idempotent.
- **Pause** is a flag file in the config dir; the hook early-exits if present.
- Commit messages end with the `Co-Authored-By` trailer. Default branch is
  `main`. Do not push unless asked.

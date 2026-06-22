# CLAUDE.md

Guidance for Claude Code when working in this repo.

## What this is

**Claude Toast** — cross-platform (Windows/macOS/Linux) desktop notifications +
a system-tray daemon for Claude Code, fired from its `Notification` and `Stop`
hooks. Target user: someone running Claude Code across multiple monitors and
multiple virtual desktops who would otherwise miss a session that needs them.

Primarily an **alert layer**. It also has an optional **cross-device** layer
(broadcast toasts to your other computers + phone, and remote approve/deny of
permission prompts) — see below. Jumping between sessions is still done with
`claude agents` (Agent View), not by this tool.

## Stack & layout

Go, single `package main`, split per-OS by build tags. No internal packages, so
the module path is cosmetic (only in `go.mod`).

| File(s)                       | Role                                                       |
|-------------------------------|------------------------------------------------------------|
| `main.go`                     | CLI dispatch: `install`/`uninstall`/`tray`/`test`/`hook`/`uid`/`link`/`status`/`remote` |
| `hook.go`                     | Parse Claude Code's stdin JSON, build the toast text; PreToolUse decision branch |
| `notify.go`                   | `notification` struct (the cross-OS payload)               |
| `notify_{windows,darwin,linux}.go` | `showNotification` per OS (go-toast / osascript / notify-send) |
| `tray.go`                     | `fyne.io/systray` daemon + menu; broadcast listener        |
| `install.go`                  | settings.json hook wiring (idempotent); conditional PreToolUse hook |
| `autostart_{windows,darwin,linux}.go` | autostart + (Windows) AUMID branding              |
| `icon_{windows,other}.go`     | `//go:embed` the tray/notify icons                         |
| `paths.go`                    | config dir, pause flag, icon extraction                    |
| `config.go`                   | cross-device config (`config.json`, mode 0600) + load/save |
| `crypto.go`                   | HKDF topic/key derivation + NaCl secretbox envelopes       |
| `transport_ntfy.go`           | ntfy publish/subscribe (stdlib net/http only)              |
| `broadcast.go`                | publish/receive broadcast toasts; echo suppression         |
| `approve.go`                  | `decidePreToolUse` remote approve engine; `remote`/`simulate-pretooluse` |
| `pair.go`                     | `uid`/`link`/`pair`/`status` + auto-init of UID/relay      |
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

## Cross-device (do not re-break)

- **Outbound-only, never a server.** Devices only *dial out* to an ntfy relay
  (publish + subscribe); no machine opens a listening port. This is a deliberate
  security choice (an inbound service on a box that runs Claude = RCE surface).
  Don't add a listener/P2P server.
- **Linked by UID.** One pairing secret (the "UID", base64url of 32 bytes) is
  copied across devices. HKDF derives the AEAD key **and** the three topic names
  (`-bc` broadcast, `-aq` approve-request, `-ar` approve-response) from it, so the
  relay only sees opaque topics + (for approve) ciphertext. Relay defaults to
  public `ntfy.sh`; `pair --server` switches to self-hosted.
- **Broadcast is cleartext ntfy fields** (title/message) so the stock ntfy phone
  app renders it; confidentiality there rests on the unguessable topic + (ideally)
  your own server. **Approve is end-to-end encrypted** (NaCl secretbox) — the
  origin pre-seals both allow/deny responses into the ntfy action buttons, so the
  phone needs no key and a decision is nonce-bound (no forge/replay).
- **Remote approve is deny-only + allowlist.** Only allowlisted tools (default
  Read/Glob/Grep/LS) are ever sent remote; off-allowlist → `ask` (normal local
  prompt), never remote-approvable. Silence/unreachable → `deny`. The PreToolUse
  hook is wired **only when `remote_approve` is on** (with a tool-name `matcher`),
  and its `timeout` must exceed `approve_timeout_sec`.
- **PreToolUse blocks Claude** while it waits for a tap (phone push latency is
  often 10s+). It's a "stepping away" toggle, off by default — keep it that way.
- **The hook must run the console exe**, even when the tray rewrites hooks: use
  `hookExe()` (sibling `claude-toast.exe` on Windows), not `selfExe()`.
- **ntfy subscribe takes no `since`** (stream new only); `since=now` is invalid and
  silently breaks streaming receive.

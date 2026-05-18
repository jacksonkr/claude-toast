# claude-toast

Stacking Windows toast notifications for [Claude Code](https://claude.com/claude-code) sessions.

When a Claude Code session needs your input or finishes a turn, you get a
Windows toast naming **which session** and **what it's asking**. Each session
gets its own toast in the Action Center, so concurrent sessions stack instead
of overwriting each other — glance at the Action Center to see the state of
every terminal.

## What it does

- **One line: `[session] - [what Claude is asking]`.** The toast reads, e.g.,
  `Fix the login bug  -  Claude needs your permission to run Bash`, or
  `... - Finished responding` on a `Stop`. The "asking" text comes from
  Claude Code's notification message (the hook does not receive the verbatim
  chat question).
- **Stacks per session.** Each toast is keyed by session id
  (`-UniqueIdentifier`, i.e. matching Tag + Group), so concurrent sessions
  pile up separately in the Action Center. A new toast for the *same* session
  updates that session's single entry in place instead of adding a new one.
- **Names the session.** Reads the human session title from Claude Code's
  `sessions-index.json` (`summary` → `firstPrompt` → first transcript user
  message), falling back to the project folder name.
- **Plain native toast.** No `Reminder` scenario and no buttons — that hack
  replayed the entrance animation and caused a stuttering re-render on
  Windows 11. Toasts land in the Action Center via the default Windows
  behavior and stay there until you clear them.

## Navigation

This tool is the **alert** layer only. To jump to the session that needs
you, use Claude Code's built-in dashboard:

```
claude agents
```

Agent View lists every session with live status and lets you drop straight
into any one — no fragile window-focusing required. (An earlier
click-to-focus button was removed because reliably stealing focus to an
arbitrary terminal on Windows is brittle by design.)

## Requirements

- Windows 10/11
- Windows PowerShell 5.1 (preinstalled) — the hook runs under `powershell`
- [BurntToast](https://github.com/Windos/BurntToast) — installed
  automatically by `install.ps1` (CurrentUser scope)
- Claude Code

## Install

```powershell
git clone <this-repo> claude-toast
cd claude-toast
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

The installer:

1. Relaxes the **CurrentUser** execution policy to `RemoteSigned` if needed.
2. Installs the NuGet provider + BurntToast for the current user.
3. Copies `claude-toast.ps1` to `~/.claude/claude-toast/`.
4. Merges `Notification` and `Stop` hooks into `~/.claude/settings.json`
   (idempotent; backs the file up to `settings.json.bak` first).

Open a new Claude Code session (or run `/hooks` once) to load the hooks.

### Options

```powershell
# Only the "needs you" toast, not "finished"
powershell -File .\install.ps1 -Events Notification

# Custom install location
powershell -File .\install.ps1 -InstallDir 'D:\tools\claude-toast'
```

## Uninstall

```powershell
powershell -ExecutionPolicy Bypass -File .\uninstall.ps1            # remove hooks
powershell -ExecutionPolicy Bypass -File .\uninstall.ps1 -RemoveFiles  # also delete the script
```

BurntToast is left installed (other tools may use it).

## Caveats

- **Remote / SSH sessions.** Hooks run wherever `claude` runs. If you SSH
  into another machine and start Claude there, the hook fires on the remote
  box — no local Windows toast. This tool only covers sessions whose
  `claude` process runs on this Windows machine. For remote sessions, use
  Claude Code's built-in mobile push instead.
- **`Stop` fires every turn.** With the Stop hook enabled you get a
  "finished" toast after every response, per session. Install with
  `-Events Notification` if that's too noisy.
- **Readability / transparency.** Windows themes all toast text; a script
  cannot set toast text color. If text looks washed out, the usual cause is
  **Transparency effects** — the toast surface is translucent and bright
  windows behind it collapse the contrast of the OS-dimmed text. Fix:
  Settings → Personalization → Colors → **Transparency effects → Off** for a
  solid surface and consistently readable text.

## License

MIT — see [LICENSE](LICENSE).

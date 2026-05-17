# claude-toast

Persistent, stacking Windows toast notifications for [Claude Code](https://claude.com/claude-code) sessions.

When a Claude Code session needs your input or finishes a turn, you get a
Windows toast that tells you **which project and which session** — and the
toast stays put until you dismiss it. Multiple sessions stack instead of
overwriting each other, so you can glance at the Action Center and see the
state of every terminal.

## What it does

- **Stacks per session.** Each session gets its own toast (keyed by session
  id), so concurrent sessions pile up in the Action Center instead of
  replacing one another. A new toast for the *same* session updates that
  session's single entry.
- **Persists.** Uses the Windows `Reminder` scenario — toasts do not
  auto-dismiss; they stay until you close them (a `Dismiss` button is
  included).
- **Names the session.** Reads the human session title from Claude Code's
  `sessions-index.json` (falls back to the first prompt / first message).
- **Readable on any theme.** The critical line (`Claude needs you -
  <project>`) is placed on the bold, full-contrast title line, since
  Windows dims every line after the first.

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
  persistent "finished" toast after every response per session. Install
  with `-Events Notification` if that's too noisy.
- **Theme contrast.** Windows controls toast colors; this tool can't set
  text color. Critical info is forced onto the high-contrast title line,
  but secondary lines are dimmed by the OS by design.

## License

MIT — see [LICENSE](LICENSE).

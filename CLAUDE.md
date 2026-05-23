# CLAUDE.md

Guidance for Claude Code when working in this repo.

## What this is

A small Windows-only tool that fires a BurntToast notification when a Claude
Code session needs input (`Notification` hook) or finishes a turn (`Stop`
hook). It is an **alert layer only** — navigation between sessions is done
with `claude agents` (Agent View), not by this tool.

## Layout

| Path                  | Role                                                    |
|-----------------------|---------------------------------------------------------|
| `src/claude-toast.ps1`| The runtime script. **Single source of truth.**         |
| `install.ps1`         | Installs BurntToast, copies the script, wires hooks.    |
| `uninstall.ps1`       | Removes only claude-toast hook entries; leaves BurntToast.|
| `README.md`           | User-facing docs.                                        |

## Critical: two copies of the script

`src/claude-toast.ps1` is the repo copy. `install.ps1` **copies** it to
`~/.claude/claude-toast/claude-toast.ps1`, and the hooks in
`~/.claude/settings.json` invoke *that* copy — not the repo. After editing
`src/claude-toast.ps1`, the live behavior does not change until you re-copy
it (re-run `install.ps1`, or copy the file). When testing changes, always
verify which copy you are running.

## Environment gotchas (do not re-break these)

- **Windows PowerShell 5.1, not pwsh 7.** BurntToast 1.1.0 is installed under
  Windows PowerShell for the current user. Hooks and tests must invoke
  `powershell`, never `pwsh` — BurntToast is not visible from pwsh.
- **Per-session stacking** relies on `New-BurntToastNotification
  -UniqueIdentifier $sid` (sets Tag + Group to the session id). Targeted
  removal requires `-Tag X -Group X` together; `-UniqueIdentifier` alone on
  `Remove-BTNotification` clears *all* toasts, and `-Tag` alone is a no-op.
- **No `Reminder` scenario, no buttons, no image rendering.** All were tried
  and removed: `Reminder` replays the entrance animation and stutters on
  Win11; the image-hero approach was rejected; the click-to-focus button was
  abandoned (stealing focus to an arbitrary terminal on Windows is brittle).
  Do not reintroduce these without explicit user request.
- **Click-to-dismiss is a `Protocol` activation, not `Background`.** The
  toast XML is built with `activationType="protocol"
  launch="claude-toast-noop:"`. The installer registers that URI to a
  windowless `wscript.exe noop.vbs`. Do NOT switch to `Background` or
  `Foreground` activation — both routes go through BurntToast/Toolkit's COM
  activator and spawn a visible `powershell.exe` on click (verified
  empirically). The "no-op via OS protocol handler" pattern is the only way
  found to get true click-to-dismiss without a console flash on this stack.
  `New-BurntToastNotification` does not expose activation type, so the
  script intentionally uses the lower-level `New-BTContent` +
  `Submit-BTNotification` pipeline.
- **Toast text color is 100% OS-themed.** A script cannot set it. Washed-out
  text is caused by Windows **Transparency effects** (translucent surface),
  not by the script. The fix is a user setting, documented in the README —
  do not try to "fix" contrast in code.

## Testing a change

```powershell
# Run the repo script directly with a synthetic hook payload:
'{"session_id":"test-123","cwd":"C:\\tmp\\demo","message":"Claude needs your permission to run Bash"}' |
  powershell -NoProfile -ExecutionPolicy Bypass -File .\src\claude-toast.ps1 -Event Notification

# Clear test toasts afterward:
powershell -NoProfile -Command "Remove-BTNotification -Tag test-123 -Group test-123"
```

The script reads the hook JSON from **stdin**; `-Event` is the only param.

## Conventions

- Keep it dependency-light: BurntToast + built-in PowerShell only.
- `install.ps1` / `uninstall.ps1` must stay **idempotent** and must only
  touch claude-toast's own hook entries in `settings.json` (always back up to
  `settings.json.bak` first).
- Commit messages: end with the `Co-Authored-By` trailer. Branch off `main`
  before committing if on the default branch. Do not push unless asked.

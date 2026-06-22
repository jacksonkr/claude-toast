# 🍞 Claude Toast

Native desktop notifications for [Claude Code](https://claude.com/claude-code) —
get a toast (with sound) the instant Claude finishes a turn or needs your input,
plus a system-tray icon to control it. **Windows, macOS, and Linux.**

Optionally, [mirror toasts to your phone and other computers](#cross-device-phone--other-computers-optional)
and **approve permission prompts remotely** so a run keeps going while you're away.

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

> **macOS** needs the Xcode Command Line Tools — the tray (`fyne.io/systray`) is
> built with cgo. If `go build` complains about a missing compiler, run
> `xcode-select --install` and retry. (A full Xcode install also satisfies this.)
>
> **Linux** needs GTK + AppIndicator dev headers for the tray:
> `sudo apt-get install libgtk-3-dev libayatana-appindicator3-dev`

## Usage

```
claude-toast install      Register the Claude Code hooks + tray autostart
claude-toast uninstall    Remove the hooks, autostart, and branding
claude-toast tray         Run the system-tray daemon (started automatically)
claude-toast test         Fire a test notification
claude-toast uid          Show this device's UID (to link other devices)
claude-toast link <uid>   Link this device to another device's UID
claude-toast status       Show cross-device settings
claude-toast remote <on|off>   Enable/disable remote approve of permission prompts
claude-toast hook --event <Notification|Stop>   (called by Claude Code)
```

The tray menu lets you **pause/resume** notifications, toggle **remote approve**,
**show the pairing code**, and **open the config folder**.

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

## Cross-device: phone + other computers (optional)

Claude Toast can also mirror toasts to your **other computers and your phone**,
and even let you **approve/deny permission prompts remotely** so a run keeps
going while you're away from that machine.

It works through [ntfy](https://ntfy.sh): your devices link into one group by a
shared **UID**, and each device only ever makes **outbound** connections to the
ntfy relay — *no machine opens a listening port*. The default relay is the public
`ntfy.sh` (zero setup; topic names are unguessable). You can self-host with
`claude-toast pair --server https://your-ntfy`.

### Link your devices

```sh
claude-toast uid                 # on the first computer: prints its UID
claude-toast link <uid>          # on every other computer: join that UID
```

On your **phone**, install the [ntfy app](https://ntfy.sh), point it at the same
server (`https://ntfy.sh`), and subscribe to the topics printed by
`claude-toast status`:

- `…-bc` — toasts (the viewer feed)
- `…-aq` — Allow/Deny prompts (only needed for remote approve)

Phones are **viewers**: they receive toasts (and can tap Allow/Deny), but never
run Claude.

> **Naming:** each toast is suffixed with the sending device's name (`@laptop`).
> On macOS this is taken from your **Computer Name** (System Settings → General →
> About), falling back to the LocalHostName, then the system hostname; on
> Windows/Linux it's the system hostname. The name is cached on first run — to
> change it later, update the OS name and re-derive (see Troubleshooting).

### Groups & the fingerprint

A **group** is just the set of devices that share one **UID**. Everything else is
derived from that UID locally, with no negotiation over the network:

- The same UID deterministically yields the same encryption key **and** the same
  ntfy topic names on every device — that's what puts them in one group.
- `claude-toast status` prints a **group fp** — a short checksum (the first 6 hex
  of SHA-256) of the UID. It's computed offline, so two devices showing the *same*
  fp are guaranteed to share the *same* UID. Use it to confirm a link worked
  without ever printing the secret itself.

**Switching groups:**

```sh
# Join a different existing group (replaces this device's UID, fresh identity):
claude-toast link <that-groups-uid>
claude-toast pair --join <token>            # same, for a custom-relay token

# Start a brand-new group (mints a fresh random UID nobody else has yet):
claude-toast pair --server https://ntfy.sh --force
claude-toast uid                            # read the new UID, link it elsewhere
```

After any switch, the topics change, so on each device: **restart the tray** (it
keeps using the old topic until its connection drops — see Troubleshooting) and
**re-subscribe the phone** to the new topics shown by `claude-toast status`.

### Remote approve / deny

```sh
claude-toast remote on           # enable;  remote off to disable
```

When on, an allowlisted tool triggers an **Allow / Deny** notification on your
linked devices; tap it and Claude continues. Safety model:

- **Deny-only + allowlist** — only tools on the allowlist (default
  `Read, Glob, Grep, LS`) are ever sent for remote approval. Anything else falls
  back to Claude's normal local prompt and can **never** be remote-approved.
- **No answer ⇒ deny** — if nobody taps before the timeout, the tool is denied.
- **Encrypted, nonce-bound decision** — the Allow/Deny *decision* is encrypted
  (NaCl secretbox) and tied to a one-time nonce, so it can't be forged or replayed
  onto a different request, and your phone needs no key to answer. Two honest
  caveats: the human-readable *summary* (tool + path) is sent to the relay in
  **cleartext by default** (set `approve_summary_cleartext: false` in `config.json`
  to suppress it), and anyone who can read your request topic — including the
  operator of the public `ntfy.sh` relay — could submit the pre-sealed Allow.
  Impact is bounded (only read-only allowlisted tools are ever sent, and silence
  still denies); for stronger guarantees, self-host the relay with
  `pair --server`.

> **Practical tip:** remote approve makes Claude **wait** for your tap (and deny
> on silence) before every allowlisted tool — phone push latency alone is often
> 10+ seconds. Treat it as a **"stepping away" switch**: `remote on` when you
> leave, `remote off` (the default) when you're back at the keyboard.

## Troubleshooting

**Cross-device uses the relay, not your LAN.** Devices never talk to each other
directly — they all dial out to ntfy. Being on the same network is neither
required nor sufficient.

**A linked device doesn't receive toasts.** First confirm it's in the group:
`claude-toast status` should show `broadcast: on` and the **same** `group fp` and
`broadcast topic` as the sender. If those match but toasts still don't arrive, the
running **tray is on a stale subscription** — it only re-reads config when its
connection drops, so a tray that was already running when you linked is still
listening on the old topic. **Quit and relaunch the tray** (from its menu, or
re-run `claude-toast install`).

**Test the relay path directly.** Subscribe with `curl` and fire a toast from
another device:

```sh
curl -s "https://ntfy.sh/<your-bc-topic>/json?poll=1&since=10m"
```

- A line appears → the relay reaches this machine; the problem is the tray (above)
  or OS notification settings (below).
- Nothing appears → the network is blocking the long-lived subscribe stream
  (corporate proxy, firewall, or a proxy that buffers streaming HTTP). Allow the
  tray's outbound HTTPS, or self-host ntfy with `pair --server`.

**Phone gets toasts but a computer doesn't.** Expected difference: the ntfy phone
app uses push delivery, while the desktop tray holds a raw streaming HTTP
connection that some networks block. See the `curl` test above.

**No banner appears at all (local `test` included).** Check OS notification
permissions: macOS → System Settings → Notifications (allow the script runner);
Windows → Settings → System → Notifications (allow "Claude Toast"). Turn off
**Do Not Disturb / Focus** while testing.

**Wrong / stale device name in the `@name` suffix.** The name is cached in
`config.json` on first run. To refresh it, set the OS name you want, then clear the
field and let it re-derive — `claude-toast status` rewrites it:

```sh
# macOS config: ~/Library/Application Support/claude-toast/config.json
# delete the "device_name" value (set it to ""), then:
claude-toast status
```

(Re-linking with `claude-toast link <uid>` also resets the local identity and
re-derives the name.)

## Uninstall

```sh
claude-toast uninstall
```

Removes the hooks, the autostart entry, and (on Windows) the toast branding.
Quit the running tray from its menu.

## License

MIT — see [LICENSE](LICENSE).

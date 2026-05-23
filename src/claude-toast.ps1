param([string]$Event = "Notification")

$ErrorActionPreference = "SilentlyContinue"

# Claude Code passes hook context as JSON on stdin
$raw = [Console]::In.ReadToEnd()
$data = $null
if ($raw) { $data = $raw | ConvertFrom-Json }

$cwd = if ($data.cwd) { $data.cwd } else { (Get-Location).Path }
$project = Split-Path $cwd -Leaf
$sid = if ($data.session_id) { $data.session_id } else { "unknown" }
$transcript = if ($data.transcript_path) { $data.transcript_path } elseif ($data.transcriptPath) { $data.transcriptPath } else { $null }

# Resolve the label that goes before " - " in the toast. Priority:
#   1. The MOST RECENT user-typed message in the live transcript. We want
#      the toast to reflect what the session is currently about, not the
#      first prompt frozen at session start.
#   2. The sessions-index.json `summary` (Claude Code's auto-title) - used
#      only when no transcript is available.
#   3. `firstPrompt` from sessions-index.json - last resort before the
#      project folder name.
function Get-RecentPrompt($transcript) {
    if (-not $transcript -or -not (Test-Path $transcript)) { return $null }

    # Walk a set of transcript lines and remember the latest real user
    # prompt. tool_result entries also have type=user but their content is
    # an array of objects, so we filter by $c -is [string].
    $scan = {
        param($lines)
        $latest = $null
        foreach ($line in $lines) {
            if ($line -notmatch '"type":"user"') { continue }
            try { $obj = $line | ConvertFrom-Json -ErrorAction Stop } catch { continue }
            if ($obj.isMeta -or $obj.isSidechain) { continue }
            $c = $obj.message.content
            if ($c -is [string] -and $c.Trim()) { $latest = $c }
        }
        return $latest
    }

    # Fast path: the latest user message is almost always within the last
    # few hundred lines. Fall back to the whole file only if the tail has
    # no user prompts (long tool-use streaks).
    $hit = & $scan (Get-Content $transcript -Tail 500 -ErrorAction SilentlyContinue)
    if (-not $hit) { $hit = & $scan (Get-Content $transcript -ErrorAction SilentlyContinue) }
    if (-not $hit) { return $null }

    $clean = $hit.Trim()
    # Slash commands come through as <command-name>/foo</command-name> ... -
    # extract just the command name so the toast reads "/compact" not raw XML.
    if ($clean -match '<command-name>([^<]+)</command-name>') { $clean = $matches[1].Trim() }
    $clean = ($clean -replace '\s+', ' ').Trim()
    return $clean.Substring(0, [Math]::Min(60, $clean.Length))
}

function Get-IndexedTitle($sid, $transcript) {
    $tryIndex = {
        param($ip)
        if (-not (Test-Path $ip)) { return $null }
        try { $idx = Get-Content $ip -Raw -ErrorAction Stop | ConvertFrom-Json } catch { return $null }
        $e = $idx.entries | Where-Object { $_.sessionId -eq $sid } | Select-Object -First 1
        if (-not $e) { return $null }
        if ($e.summary) { return $e.summary }
        if ($e.firstPrompt -and $e.firstPrompt -ne 'No prompt') {
            return $e.firstPrompt.Substring(0, [Math]::Min(60, $e.firstPrompt.Length))
        }
        return $null
    }
    if ($transcript) {
        $adjacent = Join-Path (Split-Path $transcript -Parent) 'sessions-index.json'
        $n = & $tryIndex $adjacent
        if ($n) { return $n }
    }
    $root = Join-Path $env:USERPROFILE ".claude\projects"
    foreach ($ip in (Get-ChildItem $root -Recurse -Filter 'sessions-index.json' -ErrorAction SilentlyContinue).FullName) {
        $n = & $tryIndex $ip
        if ($n) { return $n }
    }
    return $null
}

$sessionName = Get-RecentPrompt $transcript
if (-not $sessionName) { $sessionName = Get-IndexedTitle $sid $transcript }

# "what Claude is asking" comes from Claude Code's notification message
# (e.g. "Claude needs your permission to run Bash"); the hook does not pass
# the verbatim chat question.
switch ($Event) {
    "Notification" { $ask = if ($data.message) { $data.message } else { "Waiting for your input" } }
    "Stop"         { $ask = "Finished responding" }
    default        { $ask = $Event }
}

# Format: [label] - [what Claude is asking]. "Label" is the most recent
# user prompt in the session (truncated), with index/title and finally the
# project folder as fallbacks.
$window = if ($sessionName) { $sessionName } else { $project }
$lines  = @("$window  -  $ask")

Import-Module BurntToast -ErrorAction SilentlyContinue

# Plain native toast. No "Reminder" scenario (it replays the entrance
# animation and causes a stuttering flash on Windows 11). UniqueIdentifier =
# session id, so distinct sessions STACK in the Action Center instead of
# replacing each other; a new toast for the SAME session updates that
# session's single entry.
#
# Click-to-dismiss: ActivationType=Protocol routes the click through the OS
# URI handler instead of BurntToast/Toolkit's registered COM activator (which
# would launch a fresh powershell.exe to "handle" the click, since our hook
# process is long-gone). The installer registers `claude-toast-noop:` to a
# windowless wscript stub that exits immediately -- clicking the toast fires
# that, sees nothing happen visibly, and Windows dismisses the toast.
# Background activation was tried first and still triggered the Toolkit's
# activator EXE.
# Going through the lower-level New-BTContent / Submit-BTNotification
# pipeline because New-BurntToastNotification does not expose activation.
#
# Readability note: toast text is themed entirely by Windows. With
# "Transparency effects" ON, the toast surface is translucent and bright
# content behind it collapses the contrast of the OS-dimmed secondary text.
# Turn it off (Settings > Personalization > Colors > Transparency effects)
# for a solid surface and consistently readable native text.
$text    = New-BTText -Text $lines[0]
$binding = New-BTBinding -Children $text
$visual  = New-BTVisual -BindingGeneric $binding
$content = New-BTContent -Visual $visual -ActivationType Protocol -Launch 'claude-toast-noop:'
Submit-BTNotification -Content $content -UniqueIdentifier $sid

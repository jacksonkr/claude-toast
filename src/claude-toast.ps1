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

# /rename target. Claude Code stores it as the `name` field in a per-process
# file at ~/.claude/sessions/<pid>.json. The auto-generated summary in
# sessions-index.json is DIFFERENT and lives elsewhere -- we deliberately do
# not fall back to it here, because the user only wants a session-name line
# when they explicitly set one.
function Get-RenamedTitle($sid) {
    $dir = Join-Path $env:USERPROFILE '.claude\sessions'
    if (-not (Test-Path $dir)) { return $null }
    foreach ($f in Get-ChildItem $dir -Filter '*.json' -ErrorAction SilentlyContinue) {
        try { $o = Get-Content $f.FullName -Raw -ErrorAction Stop | ConvertFrom-Json } catch { continue }
        if ($o.sessionId -eq $sid -and $o.name) { return ([string]$o.name).Trim() }
    }
    return $null
}

# Most recent user-typed message in the live transcript, cleaned but NOT
# truncated -- the caller decides how to slice it (first-N-words for the
# toast, etc.). tool_result rows have type=user but content is an array, so
# we keep only string-typed content. Slash-command echoes get reduced to
# just the command name so the toast does not show raw <command-name> XML.
function Get-LastUserMessage($transcript) {
    if (-not $transcript -or -not (Test-Path $transcript)) { return $null }

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
    # few hundred lines. Fall back to the whole file only if a long tool-use
    # streak pushed the latest prompt out of the tail.
    $hit = & $scan (Get-Content $transcript -Tail 500 -ErrorAction SilentlyContinue)
    if (-not $hit) { $hit = & $scan (Get-Content $transcript -ErrorAction SilentlyContinue) }
    if (-not $hit) { return $null }

    $clean = $hit.Trim()
    if ($clean -match '<command-name>([^<]+)</command-name>') { $clean = $matches[1].Trim() }
    return ($clean -replace '\s+', ' ').Trim()
}

function Get-FirstWords($s, $n) {
    if (-not $s) { return $null }
    $w = @(($s -split '\s+') | Where-Object { $_ -ne '' })
    if (-not $w.Count) { return $null }
    return (($w | Select-Object -First $n) -join ' ')
}

$titleLine = Get-RenamedTitle $sid                          # /rename target, may be $null
$lastMsg   = Get-LastUserMessage $transcript                # may be $null
$prompt5w  = Get-FirstWords $lastMsg 5                      # first 5 words of last user message

# "what Claude is asking" comes from Claude Code's notification message
# (e.g. "Claude needs your permission to run Bash"); the hook does not pass
# the verbatim chat question.
switch ($Event) {
    "Notification" { $ask = if ($data.message) { $data.message } else { "Waiting for your input" } }
    "Stop"         { $ask = "Finished responding" }
    default        { $ask = $Event }
}

# Toast layout (top to bottom):
#   1. Renamed session title (only if /rename was used)
#   2. First 5 words of the last user message
#   3. Claude's current question / status ($ask)
# Lines 1 and 2 are skipped when the inputs are absent. The toast always
# ends with $ask so there is always at least one line.
$textLines = @()
if ($titleLine) { $textLines += $titleLine }
if ($prompt5w)  { $textLines += $prompt5w  }
$textLines += $ask

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
$texts   = @($textLines | ForEach-Object { New-BTText -Text $_ })
$binding = New-BTBinding -Children $texts
$visual  = New-BTVisual -BindingGeneric $binding
$content = New-BTContent -Visual $visual -ActivationType Protocol -Launch 'claude-toast-noop:'
Submit-BTNotification -Content $content -UniqueIdentifier $sid

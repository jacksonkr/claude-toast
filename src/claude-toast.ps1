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

# Resolve the human session name. Claude Code stores it as the "summary"
# field in sessions-index.json (one per project). Fall back to the first
# prompt, then the first user message in the transcript.
function Get-SessionName($sid, $transcript) {
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
    if ($transcript -and (Test-Path $transcript)) {
        $line = Get-Content $transcript -ErrorAction SilentlyContinue |
                Where-Object { $_ -match '"type":"user"' } | Select-Object -First 1
        if ($line) {
            try {
                $c = ($line | ConvertFrom-Json).message.content
                if ($c -is [string] -and $c) {
                    return $c.Substring(0, [Math]::Min(60, $c.Length))
                }
            } catch {}
        }
    }
    return $null
}

$sessionName = Get-SessionName $sid $transcript

switch ($Event) {
    "Notification" {
        $title = "Claude needs you"
        $body  = if ($data.message) { $data.message } else { "Waiting for input or permission" }
    }
    "Stop" {
        $title = "Claude finished"
        $body  = "Response complete"
    }
    default {
        $title = "Claude Code"
        $body  = $Event
    }
}

# Toast text color is 100% OS-themed -- the XML has no color attribute, so
# nothing here can set it. Windows renders ONLY the first text element at
# full contrast; later lines are dimmed by the OS by design. So pack all
# identifying info (action + project + session) into the first line and
# keep a single supporting line.
$head = "$title  -  $project"
if ($sessionName) { $head = "$head  -  $sessionName" }
$lines = @($head, $body)

Import-Module BurntToast -ErrorAction SilentlyContinue

# Plain native toast. No "Reminder" scenario (it replays the entrance
# animation and causes a stuttering flash on Windows 11). UniqueIdentifier =
# session id, so distinct sessions STACK in the Action Center instead of
# replacing each other; a new toast for the SAME session updates that
# session's single entry.
#
# Readability note: toast text is themed entirely by Windows. With
# "Transparency effects" ON, the toast surface is translucent and bright
# content behind it collapses the contrast of the OS-dimmed secondary text.
# Turn it off (Settings > Personalization > Colors > Transparency effects)
# for a solid surface and consistently readable native text.
New-BurntToastNotification -Text $lines -UniqueIdentifier $sid

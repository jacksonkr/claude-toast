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

# Line 1 is the only line Windows renders at full contrast (bold title);
# every later line is dimmed by the OS. Put the critical signal (action +
# project) on line 1 so it's readable on any theme.
$lines = @("$title  -  $project")
if ($sessionName) { $lines += $sessionName }
$lines += $body

Import-Module BurntToast -ErrorAction SilentlyContinue

# A system Dismiss button (no custom activation, nothing fragile) keeps the
# Reminder scenario valid and gives an explicit close action.
$dismiss = New-BTButton -Dismiss -Content "Dismiss"

# UniqueIdentifier = session id. Distinct sessions get distinct ids, so
# their toasts STACK in the Action Center instead of replacing each other.
# A new toast for the SAME session updates that session's single entry.
# Scenario "Reminder" makes the toast persist on screen until the user
# dismisses it (no auto-timeout).
try {
    $bt      = $lines | ForEach-Object { New-BTText -Text $_ }
    $binding = New-BTBinding -Children $bt
    $visual  = New-BTVisual -BindingGeneric $binding
    $action  = New-BTAction -Buttons $dismiss
    $content = New-BTContent -Visual $visual -Actions $action -Scenario Reminder
    Submit-BTNotification -Content $content -UniqueIdentifier $sid
}
catch {
    # Fallback if the builder pipeline is unavailable for any reason.
    New-BurntToastNotification -Text $lines -UniqueIdentifier $sid
}

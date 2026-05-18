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

# Windows themes ALL toast <text>, and the XML has no color attribute, so
# on some themes the text is unreadable and nothing in the toast markup can
# fix it. Workaround: render the message as an IMAGE (not subject to text
# theming) and show it as the toast's hero image with a fixed dark
# background + white text -> guaranteed contrast on any Windows theme.
function New-ToastImage {
    param([string]$Head, [string]$Body, [string]$Path)
    Add-Type -AssemblyName System.Drawing
    $w = 364; $h = 180
    $bmp = New-Object System.Drawing.Bitmap $w, $h
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode     = 'AntiAlias'
    $g.TextRenderingHint = 'ClearTypeGridFit'
    $g.Clear([System.Drawing.Color]::FromArgb(255, 28, 28, 30))
    # accent bar
    $accent = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::FromArgb(255, 96, 165, 250))
    $g.FillRectangle($accent, 0, 0, 6, $h)

    $white = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::White)
    $sub   = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::FromArgb(255, 210, 210, 214))
    $fHead = New-Object System.Drawing.Font 'Segoe UI', 13, ([System.Drawing.FontStyle]::Bold)
    $fBody = New-Object System.Drawing.Font 'Segoe UI', 11, ([System.Drawing.FontStyle]::Regular)
    $fmt   = New-Object System.Drawing.StringFormat
    $fmt.Trimming   = [System.Drawing.StringTrimming]::EllipsisCharacter
    $rectHead = New-Object System.Drawing.RectangleF 18, 16, ($w-32), 86
    $rectBody = New-Object System.Drawing.RectangleF 18, 104, ($w-32), 64
    $g.DrawString($Head, $fHead, $white, $rectHead, $fmt)
    $g.DrawString($Body, $fBody, $sub,   $rectBody, $fmt)

    $g.Dispose()
    $dir = Split-Path $Path -Parent
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    $bmp.Save($Path, [System.Drawing.Imaging.ImageFormat]::Png)
    $bmp.Dispose()
}

# Plain standard toast (no "Reminder" scenario -- that replays the entrance
# animation and causes a stuttering flash on Windows 11). UniqueIdentifier =
# session id, so distinct sessions STACK in the Action Center instead of
# replacing each other.
$imgOk = $false
$imgPath = Join-Path $env:TEMP ("claude-toast\img-{0}.png" -f ($sid -replace '[^\w.-]', '_'))
try { New-ToastImage -Head $head -Body $body -Path $imgPath; $imgOk = Test-Path $imgPath } catch { $imgOk = $false }

if ($imgOk) {
    # $lines still passed as -Text so the Action Center entry and screen
    # readers have real text; the hero image is the readable banner.
    New-BurntToastNotification -Text $lines -HeroImage $imgPath -UniqueIdentifier $sid
} else {
    New-BurntToastNotification -Text $lines -UniqueIdentifier $sid
}

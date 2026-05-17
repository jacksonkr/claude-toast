<#
.SYNOPSIS
    Installs claude-toast: persistent, stacking Windows toast notifications
    for Claude Code sessions.

.DESCRIPTION
    - Ensures BurntToast (and the NuGet provider) is installed for the
      current user under Windows PowerShell.
    - Relaxes the per-user execution policy to RemoteSigned if needed.
    - Copies claude-toast.ps1 to the install directory.
    - Idempotently merges Notification and Stop hooks into
      ~/.claude/settings.json (backs the file up first).

.PARAMETER InstallDir
    Where the runtime script is copied. Default: ~/.claude/claude-toast.

.PARAMETER Events
    Which hook events to wire up. Default: Notification and Stop.

.PARAMETER TimeoutSeconds
    Per-hook timeout. Default: 10.
#>
[CmdletBinding()]
param(
    [string]   $InstallDir     = (Join-Path $env:USERPROFILE ".claude\claude-toast"),
    [ValidateSet('Notification', 'Stop')]
    [string[]] $Events          = @('Notification', 'Stop'),
    [int]      $TimeoutSeconds  = 10
)

$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$src  = Join-Path $here 'src\claude-toast.ps1'
if (-not (Test-Path $src)) { throw "Cannot find src\claude-toast.ps1 next to this installer." }

function ConvertTo-HashtableDeep {
    param($obj)
    if ($null -eq $obj) { return $null }
    if ($obj -is [System.Collections.IEnumerable] -and $obj -isnot [string]) {
        return @($obj | ForEach-Object { ConvertTo-HashtableDeep $_ })
    }
    if ($obj -is [psobject] -and $obj.PSObject.Properties.Name.Count) {
        $h = [ordered]@{}
        foreach ($p in $obj.PSObject.Properties) { $h[$p.Name] = ConvertTo-HashtableDeep $p.Value }
        return $h
    }
    return $obj
}

Write-Host "Installing claude-toast..." -ForegroundColor Cyan

# 1. Per-user execution policy ------------------------------------------------
$cur = Get-ExecutionPolicy -Scope CurrentUser
if ($cur -in @('Restricted', 'AllSigned', 'Undefined')) {
    Write-Host "  Setting CurrentUser execution policy -> RemoteSigned"
    Set-ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
}

# 2. BurntToast under Windows PowerShell (the host the hook uses) -------------
$btPresent = powershell -NoProfile -Command "if (Get-Module -ListAvailable -Name BurntToast) {'yes'} else {'no'}"
if ($btPresent -ne 'yes') {
    Write-Host "  Installing BurntToast (CurrentUser, Windows PowerShell)..."
    powershell -NoProfile -Command @'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Install-PackageProvider -Name NuGet -MinimumVersion 2.8.5.201 -Scope CurrentUser -Force | Out-Null
Set-PSRepository -Name PSGallery -InstallationPolicy Trusted
Install-Module -Name BurntToast -Scope CurrentUser -Force -AllowClobber
'@
} else {
    Write-Host "  BurntToast already present."
}

# 3. Copy the runtime script -------------------------------------------------
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$scriptPath = Join-Path $InstallDir 'claude-toast.ps1'
Copy-Item $src $scriptPath -Force
Write-Host "  Script -> $scriptPath"

# 4. Merge hooks into ~/.claude/settings.json --------------------------------
$settingsPath = Join-Path $env:USERPROFILE '.claude\settings.json'
if (Test-Path $settingsPath) {
    Copy-Item $settingsPath "$settingsPath.bak" -Force
    $rawJson  = Get-Content $settingsPath -Raw
    $settings = if ($rawJson.Trim()) { ConvertTo-HashtableDeep ($rawJson | ConvertFrom-Json) } else { [ordered]@{} }
} else {
    New-Item -ItemType Directory -Force -Path (Split-Path $settingsPath) | Out-Null
    $settings = [ordered]@{}
}
if ($settings -isnot [System.Collections.IDictionary]) { $settings = [ordered]@{} }
if (-not $settings.Contains('hooks') -or $settings['hooks'] -isnot [System.Collections.IDictionary]) {
    $settings['hooks'] = [ordered]@{}
}

foreach ($evt in $Events) {
    $existing = @()
    if ($settings['hooks'].Contains($evt)) {
        # Drop any prior claude-toast entry so re-running is idempotent.
        $existing = @($settings['hooks'][$evt] | Where-Object {
            $cmd = ($_.hooks | ForEach-Object { $_.command }) -join ' '
            $cmd -notmatch 'claude-toast\.ps1'
        })
    }
    $entry = [ordered]@{
        hooks = @(
            [ordered]@{
                type    = 'command'
                command = "powershell -NoProfile -ExecutionPolicy Bypass -File `"$scriptPath`" -Event $evt"
                async   = $true
                timeout = $TimeoutSeconds
            }
        )
    }
    $settings['hooks'][$evt] = @($existing + $entry)
    Write-Host "  Hook wired: $evt"
}

($settings | ConvertTo-Json -Depth 20) | Set-Content -Path $settingsPath -Encoding UTF8

Write-Host ""
Write-Host "Done." -ForegroundColor Green
Write-Host "Open a NEW Claude Code session (or run /hooks once) to load the hooks."
Write-Host "Navigate between sessions with:  claude agents"

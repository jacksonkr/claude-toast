<#
.SYNOPSIS
    Removes claude-toast hooks (and optionally the installed script).

.DESCRIPTION
    - Removes any Notification/Stop hook entries that reference
      claude-toast.ps1 from ~/.claude/settings.json (backs it up first).
    - With -RemoveFiles, also deletes the install directory.
    - Leaves the shared BurntToast module alone (other tools may use it).

.PARAMETER InstallDir
    The install directory to remove when -RemoveFiles is set.
    Default: ~/.claude/claude-toast.

.PARAMETER RemoveFiles
    Also delete the installed script directory.
#>
[CmdletBinding()]
param(
    [string] $InstallDir  = (Join-Path $env:USERPROFILE ".claude\claude-toast"),
    [switch] $RemoveFiles
)

$ErrorActionPreference = 'Stop'

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

Write-Host "Uninstalling claude-toast..." -ForegroundColor Cyan

$settingsPath = Join-Path $env:USERPROFILE '.claude\settings.json'
if (Test-Path $settingsPath) {
    Copy-Item $settingsPath "$settingsPath.bak" -Force
    $rawJson = Get-Content $settingsPath -Raw
    if ($rawJson.Trim()) {
        $settings = ConvertTo-HashtableDeep ($rawJson | ConvertFrom-Json)
        if ($settings['hooks'] -is [System.Collections.IDictionary]) {
            foreach ($evt in @($settings['hooks'].Keys)) {
                $kept = @($settings['hooks'][$evt] | Where-Object {
                    $cmd = ($_.hooks | ForEach-Object { $_.command }) -join ' '
                    $cmd -notmatch 'claude-toast\.ps1'
                })
                if ($kept.Count) { $settings['hooks'][$evt] = $kept }
                else             { $settings['hooks'].Remove($evt) }
            }
            if ($settings['hooks'].Count -eq 0) { $settings.Remove('hooks') }
        }
        ($settings | ConvertTo-Json -Depth 20) | Set-Content -Path $settingsPath -Encoding UTF8
        Write-Host "  Hooks removed from settings.json (backup: settings.json.bak)"
    }
} else {
    Write-Host "  No settings.json found - nothing to unhook."
}

# Remove the click-to-dismiss protocol registration (always - it's ours).
$protoRoot = 'HKCU:\Software\Classes\claude-toast-noop'
if (Test-Path $protoRoot) {
    Remove-Item $protoRoot -Recurse -Force
    Write-Host "  Protocol removed: claude-toast-noop:"
}

if ($RemoveFiles -and (Test-Path $InstallDir)) {
    Remove-Item $InstallDir -Recurse -Force
    Write-Host "  Removed $InstallDir"
}

Write-Host ""
Write-Host "Done." -ForegroundColor Green
Write-Host "BurntToast was left installed. Remove it manually if unused:"
Write-Host "  powershell -Command `"Uninstall-Module BurntToast`""

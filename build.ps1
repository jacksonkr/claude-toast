# Local Windows build helper. Produces both binaries:
#   claude-toast.exe       - console subsystem (install / hook / test)
#   claude-toast-tray.exe  - GUI subsystem (the autostarted tray, no console)
$ErrorActionPreference = "Stop"
Push-Location $PSScriptRoot
try {
    Write-Host "Building claude-toast.exe (console)..."
    go build -o claude-toast.exe .
    Write-Host "Building claude-toast-tray.exe (GUI subsystem)..."
    go build -ldflags "-H windowsgui" -o claude-toast-tray.exe .
    Write-Host "Done. Run: .\claude-toast.exe install"
} finally {
    Pop-Location
}

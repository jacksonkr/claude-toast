# Generates assets/toast.png (256x256) and assets/toast.ico (multi-size).
# Dev-time tool only: the produced PNG/ICO are committed and are the build's
# source of truth. Run on Windows (needs System.Drawing / GDI+):
#   powershell -NoProfile -ExecutionPolicy Bypass -File tools/gen-icon.ps1
Add-Type -AssemblyName System.Drawing

$outDir = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\assets'))

function New-ToastBitmap([int]$size) {
    $bmp = New-Object System.Drawing.Bitmap($size, $size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $g.Clear([System.Drawing.Color]::Transparent)

    $pad    = $size * 0.07
    $left   = $pad
    $right  = $size - $pad
    $top    = $size * 0.17
    $bottom = $size - $pad
    $w      = $right - $left
    $rTop   = $w * 0.46   # big radius -> domed bread top
    $rBot   = $w * 0.13   # small radius -> squared base

    $path = New-Object System.Drawing.Drawing2D.GraphicsPath
    $path.AddArc($left,            $top,              2*$rTop, 2*$rTop, 180, 90)
    $path.AddArc($right - 2*$rTop, $top,              2*$rTop, 2*$rTop, 270, 90)
    $path.AddArc($right - 2*$rBot, $bottom - 2*$rBot, 2*$rBot, 2*$rBot,   0, 90)
    $path.AddArc($left,            $bottom - 2*$rBot, 2*$rBot, 2*$rBot,  90, 90)
    $path.CloseFigure()

    $rect  = New-Object System.Drawing.RectangleF($left, $top, $w, ($bottom - $top))
    $brush = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
        $rect,
        [System.Drawing.Color]::FromArgb(255, 240, 200, 120),
        [System.Drawing.Color]::FromArgb(255, 214, 150, 64), 90)
    $g.FillPath($brush, $path)

    $penW = [Math]::Max(1.0, $size * 0.05)
    $pen  = New-Object System.Drawing.Pen([System.Drawing.Color]::FromArgb(255, 122, 74, 30), $penW)
    $pen.LineJoin = [System.Drawing.Drawing2D.LineJoin]::Round
    $g.DrawPath($pen, $path)

    if ($size -ge 48) {
        $inset = $size * 0.18
        $il = $left + $inset;  $ir = $right - $inset
        $it = $top + $inset * 1.1; $ib = $bottom - $inset
        $iw = $ir - $il; $irTop = $iw * 0.5; $irBot = $iw * 0.18
        $ip = New-Object System.Drawing.Drawing2D.GraphicsPath
        $ip.AddArc($il,            $it,            2*$irTop, 2*$irTop, 180, 90)
        $ip.AddArc($ir - 2*$irTop, $it,            2*$irTop, 2*$irTop, 270, 90)
        $ip.AddArc($ir - 2*$irBot, $ib - 2*$irBot, 2*$irBot, 2*$irBot,   0, 90)
        $ip.AddArc($il,            $ib - 2*$irBot, 2*$irBot, 2*$irBot,  90, 90)
        $ip.CloseFigure()
        $ib2 = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(55, 150, 90, 30))
        $g.FillPath($ib2, $ip)
        $ib2.Dispose(); $ip.Dispose()
    }

    $brush.Dispose(); $pen.Dispose(); $path.Dispose(); $g.Dispose()
    return $bmp
}

# PNG (used for the macOS/Linux tray icon and the toast app-logo)
$png = New-ToastBitmap 256
$png.Save((Join-Path $outDir 'toast.png'), [System.Drawing.Imaging.ImageFormat]::Png)
$png.Dispose()

# Multi-size ICO (PNG-compressed entries; Windows tray + .exe icon)
$sizes = 16, 24, 32, 48, 64, 128, 256
$entries = foreach ($s in $sizes) {
    $b = New-ToastBitmap $s
    $ms = New-Object System.IO.MemoryStream
    $b.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
    [pscustomobject]@{ size = $s; bytes = $ms.ToArray() }
    $b.Dispose(); $ms.Dispose()
}

$fs = [System.IO.File]::Create((Join-Path $outDir 'toast.ico'))
$bw = New-Object System.IO.BinaryWriter($fs)
$bw.Write([UInt16]0); $bw.Write([UInt16]1); $bw.Write([UInt16]$entries.Count)
$offset = 6 + 16 * $entries.Count
foreach ($e in $entries) {
    $dim = if ($e.size -ge 256) { 0 } else { $e.size }
    $bw.Write([Byte]$dim); $bw.Write([Byte]$dim); $bw.Write([Byte]0); $bw.Write([Byte]0)
    $bw.Write([UInt16]1); $bw.Write([UInt16]32)
    $bw.Write([UInt32]$e.bytes.Length); $bw.Write([UInt32]$offset)
    $offset += $e.bytes.Length
}
foreach ($e in $entries) { $bw.Write($e.bytes) }
$bw.Flush(); $bw.Close(); $fs.Close()

Write-Host "Wrote toast.png and toast.ico to $outDir"

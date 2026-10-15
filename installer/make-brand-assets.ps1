# Draws the brand assets the build ships: the application/installer icon
# (flowlight.ico) and the two installer wizard images. All three are drawn
# here from geometry, so they can be regenerated instead of edited by hand:
# a graphite tile carrying the amber "flowing light" mark (three tapered
# streaks, the same shape as the client's BrandMark geometry).
#
#   powershell -NoProfile -ExecutionPolicy Bypass -File installer\make-brand-assets.ps1
param([string]$Out = $PSScriptRoot)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$graphiteTop = [Drawing.Color]::FromArgb(255, 0x2A, 0x27, 0x33)
$graphiteBottom = [Drawing.Color]::FromArgb(255, 0x12, 0x11, 0x16)
$amberLight = [Drawing.Color]::FromArgb(255, 0xF7, 0xCB, 0x72)
$amber = [Drawing.Color]::FromArgb(255, 0xE6, 0xAE, 0x45)

function New-Canvas([int]$w, [int]$h) {
  $bmp = [Drawing.Bitmap]::new($w, $h, [Drawing.Imaging.PixelFormat]::Format32bppArgb)
  $g = [Drawing.Graphics]::FromImage($bmp)
  $g.SmoothingMode = [Drawing.Drawing2D.SmoothingMode]::AntiAlias
  $g.PixelOffsetMode = [Drawing.Drawing2D.PixelOffsetMode]::HighQuality
  $g.TextRenderingHint = [Drawing.Text.TextRenderingHint]::AntiAliasGridFit
  return @($bmp, $g)
}

function Rounded([float]$x, [float]$y, [float]$w, [float]$h, [float]$r) {
  $p = [Drawing.Drawing2D.GraphicsPath]::new()
  $d = 2 * $r
  $p.AddArc($x, $y, $d, $d, 180, 90); $p.AddArc($x + $w - $d, $y, $d, $d, 270, 90)
  $p.AddArc($x + $w - $d, $y + $h - $d, $d, $d, 0, 90); $p.AddArc($x, $y + $h - $d, $d, $d, 90, 90)
  $p.CloseFigure(); return $p
}

# The mark in a unit box: three streaks, each tapering at both ends of its
# lower edge. Scaled into (x, y, size) and filled amber.
function Draw-Mark($g, [float]$x, [float]$y, [float]$size) {
  $bars = @(
    @(0.06, 0.16, 0.70, 0.62, 0.20, 0.15),
    @(0.24, 0.43, 0.94, 0.86, 0.47, 0.15),
    @(0.06, 0.70, 0.56, 0.48, 0.74, 0.15)
  )
  $brush = [Drawing.Drawing2D.LinearGradientBrush]::new([Drawing.PointF]::new($x, $y), [Drawing.PointF]::new($x + $size, $y + $size), $amberLight, $amber)
  foreach ($b in $bars) {
    $x0 = $x + $size * $b[0]; $top = $y + $size * $b[1]; $x1 = $x + $size * $b[2]
    $bottom = $top + $size * $b[5]
    $pts = [Drawing.PointF[]]@(
      [Drawing.PointF]::new($x0, $top), [Drawing.PointF]::new($x1, $top),
      [Drawing.PointF]::new($x + $size * $b[3], $bottom), [Drawing.PointF]::new($x0 + $size * 0.1, $bottom))
    $g.FillPolygon($brush, $pts)
  }
  $brush.Dispose()
}

function Draw-Tile($g, [int]$size) {
  $path = Rounded 0 0 $size $size ([Math]::Max(2, $size * 0.22))
  $fill = [Drawing.Drawing2D.LinearGradientBrush]::new([Drawing.PointF]::new(0, 0), [Drawing.PointF]::new(0, $size), $graphiteTop, $graphiteBottom)
  $g.FillPath($fill, $path); $fill.Dispose(); $path.Dispose()
  $inset = $size * 0.2
  Draw-Mark $g $inset ($inset + $size * 0.02) ($size - 2 * $inset)
}

# A diagonal light streak, transparent at both ends.
function Draw-Streak($g, [float]$x, [float]$y, [float]$len, [float]$thick, [int]$alpha) {
  $rect = [Drawing.RectangleF]::new($x, $y, $len, $thick)
  $brush = [Drawing.Drawing2D.LinearGradientBrush]::new($rect, [Drawing.Color]::Transparent, [Drawing.Color]::Transparent, 0.0)
  $blend = [Drawing.Drawing2D.ColorBlend]::new(3)
  $blend.Colors = [Drawing.Color[]]@([Drawing.Color]::FromArgb(0, $amberLight), [Drawing.Color]::FromArgb($alpha, $amberLight), [Drawing.Color]::FromArgb(0, $amberLight))
  $blend.Positions = [single[]]@(0, 0.72, 1)
  $brush.InterpolationColors = $blend
  $g.FillRectangle($brush, $rect); $brush.Dispose()
}

function Save-Bmp24($bmp, [string]$path) {
  $flat = [Drawing.Bitmap]::new($bmp.Width, $bmp.Height, [Drawing.Imaging.PixelFormat]::Format24bppRgb)
  $g = [Drawing.Graphics]::FromImage($flat); $g.DrawImage($bmp, 0, 0, $bmp.Width, $bmp.Height); $g.Dispose()
  $flat.Save($path, [Drawing.Imaging.ImageFormat]::Bmp); $flat.Dispose()
}

# --- flowlight.ico: 32-bit DIB entries up to 64 px, a PNG entry at 256 px.
$sizes = 16, 20, 24, 32, 40, 48, 64, 256
$images = foreach ($s in $sizes) {
  $bmp, $g = New-Canvas $s $s
  Draw-Tile $g $s
  $g.Dispose()
  if ($s -eq 256) {
    $ms = [IO.MemoryStream]::new(); $bmp.Save($ms, [Drawing.Imaging.ImageFormat]::Png); $bytes = $ms.ToArray(); $ms.Dispose()
  } else {
    $data = $bmp.LockBits([Drawing.Rectangle]::new(0, 0, $s, $s), [Drawing.Imaging.ImageLockMode]::ReadOnly, [Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $pixels = [byte[]]::new($s * $s * 4)
    [Runtime.InteropServices.Marshal]::Copy($data.Scan0, $pixels, 0, $pixels.Length)
    $bmp.UnlockBits($data)
    $mask = [int]([Math]::Ceiling($s / 32.0) * 4) * $s
    $ms = [IO.MemoryStream]::new(); $w = [IO.BinaryWriter]::new($ms)
    $w.Write([int]40); $w.Write([int]$s); $w.Write([int]($s * 2)); $w.Write([int16]1); $w.Write([int16]32)
    $w.Write([int]0); $w.Write([int]($pixels.Length + $mask)); $w.Write([int]0); $w.Write([int]0); $w.Write([int]0); $w.Write([int]0)
    for ($row = $s - 1; $row -ge 0; $row--) { $w.Write($pixels, $row * $s * 4, $s * 4) }
    $w.Write([byte[]]::new($mask))
    $w.Flush(); $bytes = $ms.ToArray(); $w.Dispose()
  }
  $bmp.Dispose()
  , @($s, $bytes)
}
$ico = [IO.MemoryStream]::new(); $w = [IO.BinaryWriter]::new($ico)
$w.Write([int16]0); $w.Write([int16]1); $w.Write([int16]$images.Count)
$offset = 6 + 16 * $images.Count
foreach ($img in $images) {
  $s = $img[0]; $len = $img[1].Length
  $w.Write([byte]($(if ($s -ge 256) { 0 } else { $s }))); $w.Write([byte]($(if ($s -ge 256) { 0 } else { $s })))
  $w.Write([byte]0); $w.Write([byte]0); $w.Write([int16]1); $w.Write([int16]32); $w.Write([int]$len); $w.Write([int]$offset)
  $offset += $len
}
foreach ($img in $images) { $w.Write($img[1]) }
$w.Flush(); [IO.File]::WriteAllBytes((Join-Path $Out 'flowlight.ico'), $ico.ToArray()); $w.Dispose()

# --- wizard-small.bmp: the tile on the wizard's light header, 55 x 55.
$bmp, $g = New-Canvas 55 55
$g.Clear([Drawing.Color]::White)
$tile, $tg = New-Canvas 47 47; Draw-Tile $tg 47; $tg.Dispose()
$g.DrawImage($tile, 4, 4, 47, 47); $tile.Dispose(); $g.Dispose()
Save-Bmp24 $bmp (Join-Path $Out 'wizard-small.bmp'); $bmp.Dispose()

# --- wizard-image.bmp: the side banner, 164 x 314. Graphite with a warm glow,
# amber streaks, the mark and the product name.
$bmp, $g = New-Canvas 164 314
$bg = [Drawing.Drawing2D.LinearGradientBrush]::new([Drawing.PointF]::new(0, 0), [Drawing.PointF]::new(164, 314), $graphiteTop, $graphiteBottom)
$g.FillRectangle($bg, 0, 0, 164, 314); $bg.Dispose()
$glowPath = [Drawing.Drawing2D.GraphicsPath]::new(); $glowPath.AddEllipse(-40, -60, 260, 240)
$glow = [Drawing.Drawing2D.PathGradientBrush]::new($glowPath)
$glow.CenterColor = [Drawing.Color]::FromArgb(70, $amber); $glow.SurroundColors = [Drawing.Color[]]@([Drawing.Color]::FromArgb(0, $amber))
$g.FillPath($glow, $glowPath); $glow.Dispose(); $glowPath.Dispose()
$g.TranslateTransform(82, 157); $g.RotateTransform(-22); $g.TranslateTransform(-82, -157)
$rand = [Random]::new(7)
for ($i = 0; $i -lt 11; $i++) {
  Draw-Streak $g ($rand.Next(-90, 150)) ($rand.Next(-40, 360)) ($rand.Next(60, 150)) ([float](1 + $rand.NextDouble() * 3)) ($rand.Next(60, 200))
}
$g.ResetTransform()
Draw-Mark $g 22 34 40
$font = [Drawing.Font]::new('Microsoft YaHei UI', 15, [Drawing.FontStyle]::Bold, [Drawing.GraphicsUnit]::Pixel)
$g.DrawString('流光加速器', $font, [Drawing.Brushes]::White, 20, 86); $font.Dispose()
$small = [Drawing.Font]::new('Microsoft YaHei UI', 10, [Drawing.FontStyle]::Regular, [Drawing.GraphicsUnit]::Pixel)
$g.DrawString('FLOWLIGHT ACCELERATOR', $small, [Drawing.SolidBrush]::new([Drawing.Color]::FromArgb(170, 255, 255, 255)), 21, 110); $small.Dispose()
$g.Dispose()
Save-Bmp24 $bmp (Join-Path $Out 'wizard-image.bmp'); $bmp.Dispose()
Write-Output "wrote flowlight.ico, wizard-small.bmp, wizard-image.bmp to $Out"

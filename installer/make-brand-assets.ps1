# Draws the brand assets the build ships: the application/installer icon
# (flowlight.ico) and the installer wizard's art (art\: backgrounds, the mark
# tile and the call-to-action buttons). All are drawn here from geometry, so
# they can be regenerated instead of edited by hand: a graphite tile carrying
# the amber "flowing light" mark (three tapered streaks, the same shape as the
# client's BrandMark geometry).
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

Write-Output "wrote flowlight.ico to $Out"

# --- art\: the installer's dark wizard (Inno Setup 6.7 background images).
# Backgrounds keep Inno's 497:360 image area at the sizes it picks between
# by DPI; the left part stays dark and quiet for the page text. Drawn in a
# 994 x 720 design space and scaled.
$art = Join-Path $Out 'art'
New-Item -ItemType Directory -Force $art | Out-Null
$deep = [Drawing.Color]::FromArgb(255, 0x0D, 0x0C, 0x11)
$violet = [Drawing.Color]::FromArgb(255, 0x5B, 0x4A, 0x8A)

function Save-Png($bmp, [string]$name) { $bmp.Save((Join-Path $art $name), [Drawing.Imaging.ImageFormat]::Png) }

function Fill-Glow($g, [float]$cx, [float]$cy, [float]$rx, [float]$ry, $color, [int]$alpha) {
  $path = [Drawing.Drawing2D.GraphicsPath]::new(); $path.AddEllipse($cx - $rx, $cy - $ry, 2 * $rx, 2 * $ry)
  $brush = [Drawing.Drawing2D.PathGradientBrush]::new($path)
  $brush.CenterColor = [Drawing.Color]::FromArgb($alpha, $color); $brush.SurroundColors = [Drawing.Color[]]@([Drawing.Color]::FromArgb(0, $color))
  $g.FillPath($brush, $path); $brush.Dispose(); $path.Dispose()
}

function Draw-Back([int]$w, [int]$h, [bool]$hero) {
  $bmp, $g = New-Canvas $w $h
  $g.ScaleTransform($w / 994.0, $h / 720.0)
  $base = [Drawing.Drawing2D.LinearGradientBrush]::new([Drawing.PointF]::new(0, 0), [Drawing.PointF]::new(994, 720), [Drawing.Color]::FromArgb(255, 0x1C, 0x1A, 0x22), $deep)
  $g.FillRectangle($base, 0, 0, 994, 720); $base.Dispose()
  if ($hero) {
    Fill-Glow $g 800 170 430 340 $amber 78
    Fill-Glow $g 1000 740 400 300 $violet 60
  } else {
    Fill-Glow $g 960 700 420 300 $amber 46
    Fill-Glow $g 1000 40 360 240 $violet 34
  }
  $g.TranslateTransform(497, 360); $g.RotateTransform(-22); $g.TranslateTransform(-497, -360)
  $rand = [Random]::new($(if ($hero) { 11 } else { 23 }))
  $count = $(if ($hero) { 14 } else { 7 })
  for ($i = 0; $i -lt $count; $i++) {
    $x = $rand.Next($(if ($hero) { 420 } else { 600 }), 1150)
    $y = $rand.Next($(if ($hero) { -40 } else { 380 }), 820)
    Draw-Streak $g $x $y ($rand.Next(150, 400)) ([float](1.5 + $rand.NextDouble() * 4.5)) ($rand.Next($(if ($hero) { 70 } else { 40 }), $(if ($hero) { 210 } else { 120 })))
  }
  $g.ResetTransform(); $g.ScaleTransform($w / 994.0, $h / 720.0)
  # Darken the bottom, where the checkbox and the buttons sit.
  $fade = [Drawing.Drawing2D.LinearGradientBrush]::new([Drawing.PointF]::new(0, 520), [Drawing.PointF]::new(0, 720), [Drawing.Color]::FromArgb(0, $deep), [Drawing.Color]::FromArgb(190, $deep))
  $g.FillRectangle($fade, 0, 520, 994, 200); $fade.Dispose()
  $g.Dispose()
  return $bmp
}

foreach ($size in @(@(596, 432), @(796, 576), @(994, 720), @(1193, 864), @(1272, 922))) {
  $bmp = Draw-Back $size[0] $size[1] $true; Save-Png $bmp ("back-hero-{0}.png" -f $size[0]); $bmp.Dispose()
  $bmp = Draw-Back $size[0] $size[1] $false; Save-Png $bmp ("back-page-{0}.png" -f $size[0]); $bmp.Dispose()
}

# The mark tile shown beside the product name, at 100 / 150 / 200 % for 56 px.
foreach ($scale in 100, 150, 200) {
  $s = [int](56 * $scale / 100)
  $bmp, $g = New-Canvas $s $s; Draw-Tile $g $s; $g.Dispose()
  Save-Png $bmp ("mark-{0}.png" -f $scale); $bmp.Dispose()
}

# Amber call-to-action buttons, 200 x 46 at 100 %, label in the client's
# on-amber ink. Inno's own buttons cannot take the brand colour.
$ink = [Drawing.Color]::FromArgb(255, 0x1C, 0x13, 0x00)
$labels = [ordered]@{ install = '立即安装'; upgrade = '立即升级'; reinstall = '重新安装'; start = '开始使用' }
foreach ($key in $labels.Keys) {
  foreach ($scale in 100, 150, 200) {
    $f = $scale / 100.0
    $w = [int](200 * $f); $h = [int](46 * $f)
    $bmp, $g = New-Canvas $w $h
    $shape = Rounded 0.5 0.5 ($w - 1) ($h - 1) (8 * $f)
    $fill = [Drawing.Drawing2D.LinearGradientBrush]::new([Drawing.PointF]::new(0, 0), [Drawing.PointF]::new(0, $h), [Drawing.Color]::FromArgb(255, 0xF3, 0xC4, 0x66), [Drawing.Color]::FromArgb(255, 0xE0, 0xA3, 0x3A))
    $g.FillPath($fill, $shape); $fill.Dispose(); $shape.Dispose()
    $font = [Drawing.Font]::new('Microsoft YaHei UI', [float](16 * $f), [Drawing.FontStyle]::Bold, [Drawing.GraphicsUnit]::Pixel)
    $format = [Drawing.StringFormat]::new(); $format.Alignment = 'Center'; $format.LineAlignment = 'Center'
    $text = [Drawing.SolidBrush]::new($ink)
    $g.DrawString($labels[$key], $font, $text, [Drawing.RectangleF]::new(0, 0, $w, $h + 1 * $f), $format)
    $text.Dispose(); $format.Dispose(); $font.Dispose(); $g.Dispose()
    Save-Png $bmp ("button-{0}-{1}.png" -f $key, $scale); $bmp.Dispose()
  }
}
Write-Output "wrote installer art to $art"

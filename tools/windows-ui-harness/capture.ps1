# Captures the client window to PNG at a given size. Usage:
#   capture.ps1 -Width 1120 -Height 750 -Out shot.png [-Wait 2]
param([int]$Width = 1120, [int]$Height = 750, [string]$Out = 'shot.png', [int]$Wait = 2)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
Add-Type -Namespace W -Name N -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hwnd);
[DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr hwnd, int x, int y, int width, int height, bool repaint);
[DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hwnd, out RECT rect);
[StructLayout(LayoutKind.Sequential)] public struct RECT { public int L, T, R, B; }
'@
# A self-contained WPF app takes a few seconds to show its first window.
$h = [IntPtr]::Zero
for ($i = 0; $i -lt 40 -and $h -eq [IntPtr]::Zero; $i++) {
  $p = Get-Process FlowLight.Client -ErrorAction SilentlyContinue | Where-Object MainWindowHandle -ne 0 | Select-Object -First 1
  if ($p) { $h = $p.MainWindowHandle } else { Start-Sleep -Milliseconds 250 }
}
if ($h -eq [IntPtr]::Zero) { throw 'client window never appeared' }
[W.N]::MoveWindow($h, 40, 40, $Width, $Height, $true) | Out-Null
[W.N]::SetForegroundWindow($h) | Out-Null
Start-Sleep -Seconds $Wait
$r = New-Object -TypeName 'W.N+RECT'
[W.N]::GetWindowRect($h, [ref]$r) | Out-Null
$bmp = New-Object Drawing.Bitmap(($r.R - $r.L), ($r.B - $r.T))
$g = [Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($r.L, $r.T, 0, 0, $bmp.Size)
$bmp.Save($Out, [Drawing.Imaging.ImageFormat]::Png)
"saved $Out ($($bmp.Width)x$($bmp.Height))"

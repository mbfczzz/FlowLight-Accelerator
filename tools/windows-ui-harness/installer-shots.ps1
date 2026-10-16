<#
.SYNOPSIS
Walks the preview installer page by page and screenshots each one.

The preview build (/DPreview) needs no administrator rights and installs
nothing real, so this can be run freely. Inno's wizard window belongs to the
"*.tmp" child process, not the launcher, and the Chinese pages use
owner-drawn art buttons, so pages are advanced by clicking the button's
centre from its bounding rectangle. One PNG per page.

  powershell -NoProfile -ExecutionPolicy Bypass -File installer-shots.ps1
#>
param(
  [string]$Exe = "D:\mbfczzzz\claude\FlowLight-Accelerator\artifacts\installer-preview\FlowLight-Setup-Preview.exe",
  [string]$Out = "$env:TEMP\fl-installer-shots",
  [string]$ExtraArgs = "",
  [int]$InstallWait = 20
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -Namespace W -Name N -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hwnd);
[DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
[DllImport("user32.dll")] public static extern void mouse_event(uint f, uint dx, uint dy, uint d, IntPtr e);
[DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hwnd, out RECT rect);
[StructLayout(LayoutKind.Sequential)] public struct RECT { public int L, T, R, B; }
'@
New-Item -ItemType Directory -Force $Out | Out-Null

$MOUSEEVENTF_LEFTDOWN = 0x0002
$MOUSEEVENTF_LEFTUP = 0x0004

function Shot([string]$name, [IntPtr]$hwnd) {
  [W.N]::SetForegroundWindow($hwnd) | Out-Null
  Start-Sleep -Milliseconds 1000
  $r = New-Object -TypeName 'W.N+RECT'
  [W.N]::GetWindowRect($hwnd, [ref]$r) | Out-Null
  $w = $r.R - $r.L; $h = $r.B - $r.T
  $bmp = New-Object Drawing.Bitmap($w, $h)
  $g = [Drawing.Graphics]::FromImage($bmp)
  $g.CopyFromScreen($r.L, $r.T, 0, 0, $bmp.Size)
  $bmp.Save((Join-Path $Out "$name.png"), [Drawing.Imaging.ImageFormat]::Png)
  "shot $name (${w}x${h})"
  $g.Dispose(); $bmp.Dispose()
}

function Buttons($root) {
  $cond = New-Object System.Windows.Automation.PropertyCondition(
    [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
    [System.Windows.Automation.ControlType]::Button)
  $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond)
}

function Click-Button($button, [IntPtr]$hwnd) {
  $r = $button.Current.BoundingRectangle
  if ($r.Width -lt 4 -or $r.Height -lt 4) { return $false }
  $x = [int]($r.Left + $r.Width / 2)
  $y = [int]($r.Top + $r.Height / 2)
  [W.N]::SetForegroundWindow($hwnd) | Out-Null
  Start-Sleep -Milliseconds 400
  [W.N]::SetCursorPos($x, $y) | Out-Null
  Start-Sleep -Milliseconds 200
  [W.N]::mouse_event($MOUSEEVENTF_LEFTDOWN, 0, 0, 0, [IntPtr]::Zero)
  [W.N]::mouse_event($MOUSEEVENTF_LEFTUP, 0, 0, 0, [IntPtr]::Zero)
  Start-Sleep -Milliseconds 1500
  return $true
}

$startArgs = @()
if ($ExtraArgs) { $startArgs = $ExtraArgs.Split(' ') }
if ($startArgs.Count -gt 0) { Start-Process -FilePath $Exe -ArgumentList $startArgs | Out-Null }
else { Start-Process -FilePath $Exe | Out-Null }

# Inno's real window is the "*.tmp" child of the launcher.
$hwnd = [IntPtr]::Zero
for ($i = 0; $i -lt 250 -and $hwnd -eq [IntPtr]::Zero; $i++) {
  Start-Sleep -Milliseconds 400
  $p = Get-Process | Where-Object {
    $_.ProcessName -like 'FlowLight-Setup-Preview*' -and $_.MainWindowHandle -ne 0
  } | Select-Object -First 1
  if ($p) { $hwnd = $p.MainWindowHandle }
}
if ($hwnd -eq [IntPtr]::Zero) { throw 'the installer window never appeared' }

$root = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)
Shot "1-welcome" $hwnd

$advance = @('立即安装', '立即升级', '重新安装', 'Install', 'Upgrade', 'Reinstall', '下一步', 'Next')
for ($page = 2; $page -le 5; $page++) {
  $clicked = $false
  foreach ($b in Buttons $root) {
    if ($advance -contains $b.Current.Name -and (Click-Button $b $hwnd)) {
      "clicked '$($b.Current.Name)' for page $page"
      $clicked = $true
      break
    }
  }
  if (-not $clicked) { "no button found for page $page"; break }
  if ($page -eq 4) { Start-Sleep -Seconds $InstallWait } else { Start-Sleep -Seconds 2 }
  Shot "$page" $hwnd
}

foreach ($p in Get-Process | Where-Object { $_.ProcessName -like 'FlowLight-Setup-Preview*' }) {
  Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
}
"done; shots in $Out"

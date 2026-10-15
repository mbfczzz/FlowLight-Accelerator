# Exploratory pass over the client as a user would see it. Screenshots every
# state into -Ui. Windows PowerShell 5.1 (UIAutomationClient).
param([Parameter(Mandatory)][string]$Exe, [Parameter(Mandatory)][string]$Ui, [Parameter(Mandatory)][string]$Harness)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
Add-Type -Namespace W -Name N -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hwnd);
[DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
[DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr hwnd, int x, int y, int width, int height, bool repaint);
[DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hwnd, out RECT rect);
[StructLayout(LayoutKind.Sequential)] public struct RECT { public int L, T, R, B; }
'@
$log = Join-Path $Ui 'explore.log'
Remove-Item $log -ErrorAction SilentlyContinue
function Log($m) { Add-Content -LiteralPath $log -Encoding UTF8 -Value ((Get-Date -Format HH:mm:ss.fff) + ' ' + $m) }
$AE = [System.Windows.Automation.AutomationElement]
$CT = [System.Windows.Automation.ControlType]
$script:fake = $null

function Start-Fake { $script:fake = Start-Process powershell -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File',"$Harness\fake-service.ps1" -WindowStyle Hidden -PassThru; Start-Sleep -Seconds 3; Log 'fake service started' }
function Stop-Fake { if ($script:fake) { Stop-Process -Id $script:fake.Id -Force -ErrorAction SilentlyContinue; $script:fake = $null; Log 'fake service stopped' } }
function Hwnd { for ($i = 0; $i -lt 60; $i++) { $p = Get-Process FlowLight.Client -ErrorAction SilentlyContinue | Where-Object MainWindowHandle -ne 0 | Select-Object -First 1; if ($p) { return $p.MainWindowHandle }; Start-Sleep -Milliseconds 250 }; throw 'no window' }
function Launch { $env:FLOWLIGHT_DEV_PIPE = '1'; $env:FLOWLIGHT_PIPE_NAME = 'FlowLight.Accelerator.dev'; Start-Process $Exe | Out-Null; return $AE::FromHandle((Hwnd)) }
function Shot($name, $w = 1120, $h = 750, [switch]$NoMove) {
  $hwnd = Hwnd
  if (-not $NoMove) { [W.N]::MoveWindow($hwnd, 40, 40, $w, $h, $true) | Out-Null }
  [W.N]::SetForegroundWindow($hwnd) | Out-Null; Start-Sleep -Milliseconds 900
  $r = New-Object -TypeName 'W.N+RECT'; [W.N]::GetWindowRect($hwnd, [ref]$r) | Out-Null
  $bmp = New-Object Drawing.Bitmap(($r.R - $r.L), ($r.B - $r.T)); $g = [Drawing.Graphics]::FromImage($bmp)
  $g.CopyFromScreen($r.L, $r.T, 0, 0, $bmp.Size); $bmp.Save((Join-Path $Ui "$name.png"), [Drawing.Imaging.ImageFormat]::Png); $g.Dispose(); $bmp.Dispose()
  Log "shot $name ($($r.R - $r.L)x$($r.B - $r.T))"
}
function Find($root, $type, $name) {
  $cond = New-Object System.Windows.Automation.PropertyCondition($AE::ControlTypeProperty, $type)
  if ($name) { $cond = New-Object System.Windows.Automation.AndCondition($cond, (New-Object System.Windows.Automation.PropertyCondition($AE::NameProperty, $name))) }
  for ($i = 0; $i -lt 40; $i++) { $el = $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond); if ($el) { return $el }; Start-Sleep -Milliseconds 250 }
  throw "element not found: $type $name"
}
function FindAll($root, $type) { $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, (New-Object System.Windows.Automation.PropertyCondition($AE::ControlTypeProperty, $type))) }
function Click($root, $name) { (Find $root $CT::Button $name).GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke(); Log "clicked $name" }
function Type-Into($root, $name, $text) { (Find $root $CT::Edit $name).GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).SetValue($text) }
function Text-ById($root, $id) {
  $cond = New-Object System.Windows.Automation.PropertyCondition($AE::AutomationIdProperty, $id)
  $el = $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond)
  if ($el) { return $el.Current.Name } else { return '(none)' }
}
function ById($root, $id) {
  $cond = New-Object System.Windows.Automation.PropertyCondition($AE::AutomationIdProperty, $id)
  for ($i = 0; $i -lt 40; $i++) { $el = $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond); if ($el) { return $el }; Start-Sleep -Milliseconds 250 }
  throw "element not found: $id"
}
function Items($root, $id) { (ById $root $id).FindAll([System.Windows.Automation.TreeScope]::Children, (New-Object System.Windows.Automation.PropertyCondition($AE::ControlTypeProperty, $CT::ListItem))) }
function Pick($el) { $el.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern).Select() }
function Nav($root, $id) { Pick (ById $root $id); Start-Sleep -Milliseconds 800; Log "nav $id" }
function Dial($root) { Text-ById $root 'ActionButton' }
# Another app can steal the foreground mid-run (it happened); never send keys
# until the client really is in front.
function Focus-Client {
  $h = Hwnd
  for ($i = 0; $i -lt 10; $i++) {
    [W.N]::SetForegroundWindow($h) | Out-Null; Start-Sleep -Milliseconds 200
    if ([W.N]::GetForegroundWindow() -eq $h) { return $h }
  }
  Log 'WARN: client not in foreground'; return $h
}
function Keys($k) { Focus-Client | Out-Null; [System.Windows.Forms.SendKeys]::SendWait($k); Start-Sleep -Milliseconds 600 }
function Buttons($root) { (FindAll $root $CT::Button | ForEach-Object { $_.Current.Name + $(if (-not $_.Current.IsEnabled) { '(灰)' }) }) -join ' | ' }
# Account actions live in the menu behind the avatar at the bottom of the rail.
function Menu-Click($root, $name) { Click $root '账号'; Start-Sleep -Milliseconds 500; Click $root $name }

try {
  Get-Process FlowLight.Client -ErrorAction SilentlyContinue | Stop-Process -Force
  Start-Fake
  $root = Launch; Start-Sleep -Seconds 2

  # 1. Wrong password, then Enter to submit the right one; lands on the library.
  Type-Into $root '账号' 'tester'; Type-Into $root '密码' 'wrong'
  Click $root '登录'; Start-Sleep -Seconds 2; Shot 'e01-wrong-password'
  Type-Into $root '密码' 'pw'; (Find $root $CT::Edit '密码').SetFocus(); Keys '{ENTER}'; Start-Sleep -Seconds 3
  Shot 'e02-library'
  Log ("buttons: " + (Buttons $root))

  # 2. Open the first game; region tiles and line rows as a player sees them.
  Pick (Items $root 'GameList')[0]; Nav $root 'NavGame'; Start-Sleep -Seconds 1
  Shot 'e03-game-page'
  $lines = Items $root 'LineList'
  Log ("lines: " + (($lines | ForEach-Object { $_.Current.Name + $(if (-not $_.Current.IsEnabled) { '(不可选)' }) }) -join ' | '))
  Pick (Items $root 'RegionList')[1]; Start-Sleep -Seconds 2
  Pick (Items $root 'LineList')[2]; Start-Sleep -Seconds 1
  Shot 'e04-other-region-slow-line'
  Log ("summary: " + (Text-ById $root 'SummaryRegion') + " / " + (Text-ById $root 'SummaryLine'))
  Pick (Items $root 'RegionList')[0]; Start-Sleep -Seconds 2

  # 3. Search narrows the library; clearing it brings everything back.
  Nav $root 'NavLibrary'
  Type-Into $root '搜索游戏' '测试'; Start-Sleep -Seconds 1; Shot 'e05-search'
  Log ("cards while searching: " + (Items $root 'GameList').Count)
  Type-Into $root '搜索游戏' 'zzz'; Start-Sleep -Seconds 1; Shot 'e06-search-empty'
  Type-Into $root '搜索游戏' ''; Start-Sleep -Seconds 1

  # 4. Start refused by the service: the reason sits under the dial.
  Nav $root 'NavGame'
  New-Item -ItemType File -Path (Join-Path $Harness 'fail-start.flag') -Force | Out-Null
  (ById $root 'ActionButton').GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke(); Start-Sleep -Seconds 2; Shot 'e07-start-refused'
  Log ("refused: status=" + (Text-ById $root 'ConnectionState') + " inline=" + (Text-ById $root 'InlineError'))
  Remove-Item (Join-Path $Harness 'fail-start.flag') -Force
  # The refused line comes back after the line list's own refresh.
  Click $root '刷新线路'; Start-Sleep -Seconds 2
  Log ("after line refresh: " + ((Items $root 'LineList' | ForEach-Object { $_.Current.Name + $(if (-not $_.Current.IsEnabled) { '(不可选)' }) }) -join ' | '))

  # 5. Normal start: connecting, then the live dashboard with a few probes.
  (ById $root 'ActionButton').GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke(); Start-Sleep -Milliseconds 1500; Shot 'e08-connecting'
  Log ("connecting: dial=" + (Dial $root))
  Start-Sleep -Seconds 18; Shot 'e09-active'
  Log ("active: dial=" + (Dial $root) + " latency=" + (Text-ById $root 'StatLatency'))
  Nav $root 'NavLibrary'; Start-Sleep -Seconds 1; Shot 'e10-library-while-running'
  Click $root '查看'; Start-Sleep -Seconds 1
  Click $root '停止加速'; Start-Sleep -Milliseconds 1500; Shot 'e11-just-stopped'

  # 6. The account menu, diagnostics, and switching account.
  Click $root '账号'; Start-Sleep -Milliseconds 800; Shot 'e12-account-menu'
  Click $root '连接诊断'; Start-Sleep -Seconds 1; Shot 'e13-diagnostics'
  Log ("toast: " + (Text-ById $root 'Toast'))
  Menu-Click $root '切换账号'; Start-Sleep -Seconds 1; Shot 'e14-switch-account'; Keys '{ESC}'

  # 6b. Sign out: the login dialog must come back on its own.
  Menu-Click $root '退出登录'; Start-Sleep -Seconds 3; Shot 'e15-logged-out'
  Log ("after logout: " + (Buttons $root))
  Type-Into $root '密码' 'pw'; (Find $root $CT::Edit '密码').SetFocus(); Keys '{ENTER}'; Start-Sleep -Seconds 3

  # 7. Big and small windows while signed in, library and game page.
  Shot 'e16-min-library' 900 650
  Pick (Items $root 'GameList')[0]; Nav $root 'NavGame'; Shot 'e17-min-game' 900 650
  $root.GetCurrentPattern([System.Windows.Automation.WindowPattern]::Pattern).SetWindowVisualState([System.Windows.Automation.WindowVisualState]::Maximized)
  Start-Sleep -Seconds 1; Shot 'e18-max-game' -NoMove
  Nav $root 'NavLibrary'; Shot 'e19-max-library' -NoMove
  $root.GetCurrentPattern([System.Windows.Automation.WindowPattern]::Pattern).SetWindowVisualState([System.Windows.Automation.WindowVisualState]::Normal)

  # 8. Service disappears mid-use, then comes back (signed out again).
  Stop-Fake; Start-Sleep -Seconds 5; Shot 'e20-service-gone'
  Log ("service gone: " + (Buttons $root))
  Start-Fake; Start-Sleep -Seconds 5; Shot 'e21-service-back'
  Log ("service back: " + (Buttons $root))
  Log 'done'
} catch {
  Log ('FAILED: ' + $_.Exception.Message + ' at line ' + $_.InvocationInfo.ScriptLineNumber)
  try { Shot 'e99-at-failure' } catch { }
  exit 1
} finally {
  Get-Process FlowLight.Client -ErrorAction SilentlyContinue | Stop-Process -Force
  Stop-Fake
  Remove-Item (Join-Path $Harness 'fail-start.flag') -Force -ErrorAction SilentlyContinue
}

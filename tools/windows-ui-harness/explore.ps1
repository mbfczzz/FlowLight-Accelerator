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
# Account actions live in the menu behind the avatar; open it first.
function Menu-Click($root, $name) { Click $root '账号菜单'; Start-Sleep -Milliseconds 500; Click $root $name }

try {
  Get-Process FlowLight.Client -ErrorAction SilentlyContinue | Stop-Process -Force
  Start-Fake
  $root = Launch; Start-Sleep -Seconds 2

  # 1. Wrong password, then Enter to submit the right one.
  Type-Into $root '账号' 'tester'; Type-Into $root '密码' 'wrong'
  Click $root '登录'; Start-Sleep -Seconds 2; Shot 'e01-wrong-password'
  Type-Into $root '密码' 'pw'; (Find $root $CT::Edit '密码').SetFocus(); Keys '{ENTER}'; Start-Sleep -Seconds 3
  Shot 'e02-logged-in'
  Log ("buttons: " + (Buttons $root))

  # 2. Dropdowns as the user opens them; the unavailable line is listed but cannot be picked.
  $combos = FindAll $root $CT::ComboBox
  $combos[0].SetFocus(); Keys '{F4}'; Shot 'e03-region-dropdown-open'; Keys '{ESC}'
  $combos[1].SetFocus(); Keys '{F4}'; Shot 'e04-line-dropdown-open'; Keys '{ESC}'
  Focus-Client | Out-Null; $combos[1].SetFocus(); Keys '{DOWN}'; Keys '{DOWN}'; Start-Sleep -Seconds 1; Shot 'e05-after-down-twice'
  Log ("line after Down twice: " + (($combos[1].GetCurrentPattern([System.Windows.Automation.SelectionPattern]::Pattern).Current.GetSelection() | ForEach-Object { $_.Current.Name }) -join ','))
  Keys '{UP}'; Keys '{UP}'

  # 3. Search narrows the library; clearing it brings everything back.
  Type-Into $root '搜索游戏' '测试'; Start-Sleep -Seconds 1; Shot 'e06-search'
  Log ("rows while searching: " + (FindAll $root $CT::ListItem).Count)
  Type-Into $root '搜索游戏' ''; Start-Sleep -Seconds 1
  (FindAll $root $CT::ListItem)[0].GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern).Select(); Start-Sleep -Seconds 1

  # 4. Start refused by the service: the reason sits under the button.
  New-Item -ItemType File -Path (Join-Path $Harness 'fail-start.flag') -Force | Out-Null
  Click $root '开始加速'; Start-Sleep -Seconds 2; Shot 'e07-start-refused'
  Log ("refused: status=" + (Text-ById $root 'ConnectionState') + " inline=" + (Text-ById $root 'InlineError'))
  Remove-Item (Join-Path $Harness 'fail-start.flag') -Force

  # 5. Normal start, stop feedback.
  Click $root '开始加速'; Start-Sleep -Seconds 9; Shot 'e08-active'
  Click $root '停止加速'; Start-Sleep -Milliseconds 1500; Shot 'e09-just-stopped'

  # 6. The account menu, diagnostics, and switching account.
  Click $root '账号菜单'; Start-Sleep -Milliseconds 800; Shot 'e10-account-menu'
  Click $root '连接诊断'; Start-Sleep -Seconds 1; Shot 'e10b-diagnostics'
  Log ("toast: " + (Text-ById $root 'Toast'))
  Menu-Click $root '切换账号'; Start-Sleep -Seconds 1; Shot 'e11-switch-account'; Keys '{ESC}'

  # 6b. Sign out: the login dialog must come back on its own.
  Menu-Click $root '退出登录'; Start-Sleep -Seconds 3; Shot 'e16-logged-out'
  Log ("after logout: " + (Buttons $root))
  Type-Into $root '密码' 'pw'; (Find $root $CT::Edit '密码').SetFocus(); Keys '{ENTER}'; Start-Sleep -Seconds 3

  # 7. Big and small windows while signed in.
  Shot 'e12-min-size' 900 650
  $root.GetCurrentPattern([System.Windows.Automation.WindowPattern]::Pattern).SetWindowVisualState([System.Windows.Automation.WindowVisualState]::Maximized)
  Start-Sleep -Seconds 1; Shot 'e13-maximized' -NoMove
  $root.GetCurrentPattern([System.Windows.Automation.WindowPattern]::Pattern).SetWindowVisualState([System.Windows.Automation.WindowVisualState]::Normal)

  # 8. Service disappears mid-use, then comes back (signed out again).
  Stop-Fake; Start-Sleep -Seconds 5; Shot 'e14-service-gone'
  Log ("service gone: " + (Buttons $root))
  Start-Fake; Start-Sleep -Seconds 5; Shot 'e15-service-back'
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

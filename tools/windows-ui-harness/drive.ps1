# Drives the client through the signed-in flow with UI Automation against the
# fake service and screenshots each step. Run with Windows PowerShell 5.1
# (UIAutomationClient is a .NET Framework assembly).
param([Parameter(Mandatory)][string]$Exe, [Parameter(Mandatory)][string]$Ui, [string]$PipeName = 'FlowLight.Accelerator.dev')
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
$log = Join-Path $Ui 'drive.log'
Remove-Item $log -ErrorAction SilentlyContinue
function Log($m) { Add-Content -LiteralPath $log -Encoding UTF8 -Value ((Get-Date -Format HH:mm:ss.fff) + ' ' + $m) }
function Shot($name) { & "$PSScriptRoot\capture.ps1" -Width 1120 -Height 750 -Out (Join-Path $Ui "$name.png") -Wait 1 | Out-Null; Log "shot $name" }
$AE = [System.Windows.Automation.AutomationElement]
$CT = [System.Windows.Automation.ControlType]

function Launch {
  $env:FLOWLIGHT_DEV_PIPE = '1'; $env:FLOWLIGHT_PIPE_NAME = $PipeName
  # Framework-dependent builds need DOTNET_ROOT to point at a .NET 10 runtime;
  # set it before running when not using the self-contained publish output.
  Start-Process $Exe | Out-Null
  for ($i = 0; $i -lt 60; $i++) {
    $p = Get-Process FlowLight.Client -ErrorAction SilentlyContinue | Where-Object MainWindowHandle -ne 0 | Select-Object -First 1
    if ($p) { return $AE::FromHandle($p.MainWindowHandle) }
    Start-Sleep -Milliseconds 250
  }
  throw 'client window never appeared'
}
function Find($root, $type, $name) {
  $cond = New-Object System.Windows.Automation.PropertyCondition($AE::ControlTypeProperty, $type)
  if ($name) { $cond = New-Object System.Windows.Automation.AndCondition($cond, (New-Object System.Windows.Automation.PropertyCondition($AE::NameProperty, $name))) }
  for ($i = 0; $i -lt 40; $i++) {
    $el = $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond)
    if ($el) { return $el }
    Start-Sleep -Milliseconds 250
  }
  throw "element not found: $type $name"
}
function FindAll($root, $type) {
  $cond = New-Object System.Windows.Automation.PropertyCondition($AE::ControlTypeProperty, $type)
  $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond)
}
function Invoke-Button($root, $name) { (Find $root $CT::Button $name).GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke(); Log "clicked $name" }
function Set-Text($root, $name, $text) { (Find $root $CT::Edit $name).GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).SetValue($text); Log "typed into $name" }
function Select-Item($el) { $el.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern).Select() }
# Elements by x:Name (WPF exposes it as the AutomationId).
function ById($root, $id) {
  $cond = New-Object System.Windows.Automation.PropertyCondition($AE::AutomationIdProperty, $id)
  for ($i = 0; $i -lt 40; $i++) { $el = $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond); if ($el) { return $el }; Start-Sleep -Milliseconds 250 }
  throw "element not found: $id"
}
function Text-ById($root, $id) {
  $cond = New-Object System.Windows.Automation.PropertyCondition($AE::AutomationIdProperty, $id)
  $el = $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond)
  if ($el) { return $el.Current.Name } else { return '(none)' }
}
function Items($root, $id) { (ById $root $id).FindAll([System.Windows.Automation.TreeScope]::Children, (New-Object System.Windows.Automation.PropertyCondition($AE::ControlTypeProperty, $CT::ListItem))) }
# The dial reports its state in its name: 开始加速 / 正在连接 / 加速中 / 正在停止.
function Dial($root) { Text-ById $root 'ActionButton' }

try {
Get-Process FlowLight.Client -ErrorAction SilentlyContinue | Stop-Process -Force
$root = Launch
Start-Sleep -Seconds 2
Shot 'd1-login-prompt'

# Sign in with the fake credentials.
Set-Text $root '账号' 'tester'
Set-Text $root '密码' 'pw'
Invoke-Button $root '登录'
Start-Sleep -Seconds 3
Shot 'd2-library'
$games = Items $root 'GameList'
Log ("library cards: " + $games.Count + " / dial: " + (Dial $root))

# Open the second game the way the rail does, then pick its second region.
Select-Item $games[1]
Start-Sleep -Milliseconds 600
Select-Item (ById $root 'NavGame')
Start-Sleep -Seconds 1
$regions = Items $root 'RegionList'
Log ("region tiles: " + $regions.Count)
Select-Item $regions[1]
Start-Sleep -Seconds 2
$lines = Items $root 'LineList'
Log ("line rows: " + $lines.Count + " / enabled: " + (($lines | Where-Object { $_.Current.IsEnabled }) | Measure-Object).Count)
Shot 'd3-game-page'

(ById $root 'ActionButton').GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke(); Log 'clicked dial'
Start-Sleep -Milliseconds 1200
Shot 'd4-connecting'
Log ("during connect: dial=" + (Dial $root) + " status=" + (Text-ById $root 'ConnectionState'))
Start-Sleep -Seconds 14
Shot 'd5-active'
Log ("when active: dial=" + (Dial $root) + " status=" + (Text-ById $root 'ConnectionState') + " latency=" + (Text-ById $root 'StatLatency'))

# Kill the client while the fake session stays ACTIVE; a fresh client must
# open the running game's page with its region and line.
Get-Process FlowLight.Client -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1
$root = Launch
Start-Sleep -Seconds 6
Shot 'd6-restored'
Log ("after restart: dial=" + (Dial $root) + " status=" + (Text-ById $root 'ConnectionState'))

Invoke-Button $root '停止加速'
Start-Sleep -Seconds 3
Shot 'd7-stopped'
Log ("after stop: dial=" + (Dial $root) + " toast=" + (Text-ById $root 'Toast'))

# Close button hides to the tray; the process must stay alive, and a relaunch
# must wake it instead of starting a second instance.
$root.GetCurrentPattern([System.Windows.Automation.WindowPattern]::Pattern).Close()
Start-Sleep -Seconds 2
$p = Get-Process FlowLight.Client -ErrorAction SilentlyContinue
Log ("after close: processes=" + @($p).Count + " visible-window=" + (($p | Where-Object MainWindowHandle -ne 0 | Measure-Object).Count))
Start-Process $Exe | Out-Null
Start-Sleep -Seconds 4
$p = Get-Process FlowLight.Client -ErrorAction SilentlyContinue
Log ("after relaunch: processes=" + @($p).Count + " visible-window=" + (($p | Where-Object MainWindowHandle -ne 0 | Measure-Object).Count))
Shot 'd8-woken'
Get-Process FlowLight.Client -ErrorAction SilentlyContinue | Stop-Process -Force
Log 'done'
} catch { Log ('FAILED: ' + $_.Exception.Message + ' at line ' + $_.InvocationInfo.ScriptLineNumber); exit 1 } finally { Get-Process FlowLight.Client -ErrorAction SilentlyContinue | Stop-Process -Force }

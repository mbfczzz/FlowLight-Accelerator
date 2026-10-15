# Fake FlowLight local service for UI verification. Speaks the client's
# one-line-JSON-per-connection pipe protocol and walks a session through
# PREPARING -> CONNECTING -> ACTIVE on a timer. Run the client with
# FLOWLIGHT_DEV_PIPE=1 so it skips the service identity check.
param([int]$Games = 6, [int]$ConnectSeconds = 3, [string]$PipeName = 'FlowLight.Accelerator.dev')
$ErrorActionPreference = 'Stop'
$utf8 = [Text.UTF8Encoding]::new($false)
$pipeName = $PipeName
$logFile = Join-Path $PSScriptRoot 'fake.log'
function Log([string]$m) { Add-Content -LiteralPath $logFile -Encoding UTF8 -Value ((Get-Date -Format HH:mm:ss.fff) + ' ' + $m) }

$state = @{ logged = $false; user = $null; session = 'IDLE'; since = $null; active = $null; game = $null; region = $null; line = $null }

function New-Pipe {
  [IO.Pipes.NamedPipeServerStream]::new($pipeName, [IO.Pipes.PipeDirection]::InOut, 8,
    [IO.Pipes.PipeTransmissionMode]::Byte, [IO.Pipes.PipeOptions]::Asynchronous, 65536, 65536)
}

function Advance {
  if ($state.session -in 'PREPARING','CONNECTING') {
    $age = ((Get-Date) - $state.since).TotalSeconds
    if ($age -gt $ConnectSeconds * 2) { $state.session = 'ACTIVE'; $state.active = (Get-Date).ToUniversalTime() }
    elseif ($age -gt $ConnectSeconds) { $state.session = 'CONNECTING' }
  }
}

function Status {
  Advance
  $probe = $null
  $activeSince = $null
  if ($state.session -eq 'ACTIVE') {
    # A new probe every 4 seconds (the real service probes about every 30)
    # so the chart has a few points within a short harness run.
    $bucket = [Math]::Floor(((Get-Date).ToUniversalTime() - $state.active).TotalSeconds / 4)
    $rtt = [Math]::Round(38 + 7 * [Math]::Sin($bucket * 0.9) + ($bucket % 3), 1)
    $observed = $state.active.AddSeconds($bucket * 4).ToString('o')
    $probe = @{ tcp = $true; udp = $true; rtt_ms = $rtt; jitter_ms = 2 + ($bucket % 4); loss = $(if ($bucket % 5 -eq 4) { 0.2 } else { 0 }); samples = 3; exit_ip = '203.0.113.7'; observed_at = $observed }
    $activeSince = $state.active.ToString('o')
  }
  @{
    state = $state.session; error = ''; session_id = $(if ($state.session -ne 'IDLE') { 'sess-' + $state.since.Ticks } else { $null })
    probe = $probe; lease_until = $null; active_since = $activeSince; control_url = 'https://accel.example:8443'
    logged_in = $state.logged; username = $state.user; core_available = $true
    game_id = $state.game; region_id = $state.region; line_id = $state.line
  }
}

function Handle($cmd, $a) {
  switch ($cmd) {
    'status' { return Status }
    'login'  {
      if ($a.password -ne 'pw') { throw '账号或密码错误' }
      $state.logged = $true; $state.user = $a.username
      return @{ id = 'u1'; username = $a.username; role = 'player' }
    }
    'games' {
      if (-not $state.logged) { throw '请先登录' }
      $list = @(@{ id = 'valorant'; name = 'VALORANT'; regions = @('kr','jp','sg'); published_version = 3 },
                @{ id = 'lol'; name = '英雄联盟'; regions = @('kr','tw'); published_version = 2 })
      for ($i = 3; $i -le $Games; $i++) { $list += @{ id = "g$i"; name = "测试游戏 $i"; regions = @('kr'); published_version = 1 } }
      # A game whose rules are still a draft: no regions, nothing to start.
      $list += @{ id = 'draft'; name = '待发布游戏'; regions = @(); published_version = 0 }
      return $list
    }
    'lines' {
      if (-not $state.logged) { throw '请先登录' }
      return @(@{ id = "$($a.region_id)-01"; name = "$($a.region_id.ToUpper()) 直连-01"; available = $true;  capacity = 50; active_sessions = 12; probe = @{ rtt_ms = 38 } },
               @{ id = "$($a.region_id)-02"; name = "$($a.region_id.ToUpper()) 中转-02"; available = $true;  capacity = 40; active_sessions = 27; probe = @{ rtt_ms = 71 } },
               @{ id = "$($a.region_id)-04"; name = "$($a.region_id.ToUpper()) 海外-04"; available = $true;  capacity = 20; active_sessions = 18; probe = @{ rtt_ms = 142 } },
               @{ id = "$($a.region_id)-03"; name = "$($a.region_id.ToUpper()) 备用-03"; available = $false; capacity = 30; active_sessions = 0; probe = @{} })
    }
    'start' {
      if (Test-Path (Join-Path $PSScriptRoot 'fail-start.flag')) { throw '线路容量已满，请选择其他线路' }
      if ($state.session -ne 'IDLE') { throw '已有加速会话在运行' }
      $state.session = 'PREPARING'; $state.since = Get-Date
      $state.game = $a.game_id; $state.region = $a.region_id; $state.line = $a.line_id
      return @{ state = 'PREPARING' }
    }
    'stop'  { $state.session = 'IDLE'; $state.active = $null; $state.game = $null; $state.region = $null; $state.line = $null; return @{ state = 'IDLE' } }
    'logout' { $state.logged = $false; $state.user = $null; return @{ ok = $true } }
    default { throw '不支持的操作' }
  }
}

$current = New-Pipe
Log "fake service listening on \\.\pipe\$pipeName (login password: pw)"
while ($true) {
  # Keep a second instance open while serving so the pipe name never vanishes
  # between connections; the client treats a missing name as "not running".
  $next = New-Pipe
  try {
    $current.WaitForConnection()
    $reader = [IO.StreamReader]::new($current, $utf8, $false, 4096, $true)
    $writer = [IO.StreamWriter]::new($current, $utf8, 4096, $true); $writer.AutoFlush = $true
    $line = $reader.ReadLine()
    $req = $line | ConvertFrom-Json
    try {
      $data = Handle $req.command $req.args
      $reply = @{ success = $true; data = $data } | ConvertTo-Json -Depth 6 -Compress
    } catch {
      $reply = @{ success = $false; data = $null; error = $_.Exception.Message } | ConvertTo-Json -Compress
    }
    $writer.WriteLine($reply)
    Log ("{0,-7} -> {1}" -f $req.command, $reply.Substring(0, [Math]::Min(90, $reply.Length)))
  } catch { Log "pipe error: $($_.Exception.Message) at $($_.InvocationInfo.ScriptLineNumber)" } finally { try { $current.Dispose() } catch { } }
  $current = $next
}

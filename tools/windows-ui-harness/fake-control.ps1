# A stand-in CONTROL SERVICE, so the real Windows service's start-retry path
# can be driven: it answers the session endpoints the way the control service
# does, refusing the first N creates with a retryable 409 and then succeeding.
#
#   powershell -NoProfile -ExecutionPolicy Bypass -File fake-control.ps1 -Port 5199 -Refuse 2
param([int]$Port = 5199, [int]$Refuse = 2)
$ErrorActionPreference = 'Stop'

$state = @{ refuses = $Refuse; creates = 0; sessions = @{} }

$listener = [System.Net.HttpListener]::new()
$listener.Prefixes.Add("http://127.0.0.1:$Port/")
$listener.Start()
Write-Host "fake control service on http://127.0.0.1:$Port (refusing the first $Refuse starts)"

function Send($ctx, $code, $json) {
  $bytes = [Text.Encoding]::UTF8.GetBytes($json)
  $ctx.Response.StatusCode = $code
  $ctx.Response.ContentType = 'application/json'
  $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length)
  $ctx.Response.Close()
}

while ($true) {
  $ctx = $listener.GetContext()
  $path = $ctx.Request.Url.AbsolutePath
  $method = $ctx.Request.HttpMethod
  try {
    if ($path -eq '/api/node/v1/register') {
      Send $ctx 200 '{"token":"t"}'; continue
    }
    if ($path -match '^/api/node/v1/jobs') {
      Send $ctx 200 '[]'; continue
    }
    if ($path -eq '/api/node/v1/heartbeat') {
      Send $ctx 200 '{"server_time":"2026-01-01T00:00:00Z","leases":{}}'; continue
    }

    # --- the client API the local service calls ---
    if ($path -eq '/api/client/v1/auth/login') {
      Send $ctx 200 '{"access_token":"a","refresh_token":"r","user":{"id":"u1","username":"tester"}}'; continue
    }
    if ($path -eq '/api/client/v1/devices') {
      Send $ctx 200 '{"id":"d1"}'; continue
    }
    if ($path -eq '/api/client/v1/games') {
      Send $ctx 200 '[{"id":"valorant","name":"VALORANT","regions":["kr"],"published_version":1}]'; continue
    }
    if ($path -eq '/api/client/v1/lines') {
      Send $ctx 200 '[{"id":"kr-01","name":"KR 直连-01","available":true,"capacity":9,"active_sessions":1,"probe":{"rtt_ms":38}}]'; continue
    }
    if ($path -eq '/api/client/v1/sessions' -and $method -eq 'POST') {
      # The interesting part: refuse like the control service does while the
      # previous session is still closing, and count it.
      if ($state.refuses -gt 0) {
        $state.refuses--
        Write-Host "  refusing a create (retryable); $($state.refuses) left"
        Send $ctx 409 '{"error":"上一个加速会话正在结束，请稍候重试","code":"session-closing"}'; continue
      }
      $state.creates++
      $id = "session-$($state.creates)"
      $state.sessions[$id] = 'PREPARING'
      Write-Host "  CREATE #$($state.creates) -> $id (accepted)"
      Send $ctx 202 "{`"id`":`"$id`",`"state`":`"PREPARING`",`"lease_until`":`"2099-01-01T00:00:00Z`"}"; continue
    }
    if ($path -match '^/api/client/v1/sessions/([^/]+)$') {
      $id = $Matches[1]
      Send $ctx 200 "{`"session`":{`"id`":`"$id`",`"state`":`"PREPARING`",`"lease_until`":`"2099-01-01T00:00:00Z`"}}"; continue
    }
    if ($path -match '/stop$') {
      Send $ctx 200 '{}'; continue
    }
    Send $ctx 404 '{"error":"not found"}'
  } catch {
    Write-Host "error: $($_.Exception.Message)"
    try { $ctx.Response.StatusCode = 500; $ctx.Response.Close() } catch { }
  }
}

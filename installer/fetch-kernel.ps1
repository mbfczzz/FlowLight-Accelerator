<#
.SYNOPSIS
Downloads the pinned Mihomo kernel named in kernel.json, verifies the archive
and the executable against the pinned SHA-256 values, and places the kernel at
artifacts/windows/core/mihomo.exe for the installer build.
#>
[CmdletBinding()]
param(
 [string]$Destination = (Join-Path (Split-Path $PSScriptRoot -Parent) 'artifacts/windows/core')
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$pin = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'kernel.json') -Raw | ConvertFrom-Json
$url = "https://github.com/$($pin.project)/releases/download/$($pin.version)/$($pin.asset)"
# The download is untrusted until both checksums match; keep it in its own directory.
$work = Join-Path ([IO.Path]::GetTempPath()) ('flowlight-kernel-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $work | Out-Null
try {
 $zip = Join-Path $work $pin.asset
 Write-Host "Downloading $url"
 Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
 $zipHash = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLowerInvariant()
 if ($zipHash -ne $pin.zip_sha256.ToLowerInvariant()) { throw "Archive checksum mismatch: $zipHash" }
 $extracted = Join-Path $work 'extracted'
 Expand-Archive -LiteralPath $zip -DestinationPath $extracted -Force
 $exe = Join-Path $extracted $pin.exe_in_zip
 if (-not (Test-Path -LiteralPath $exe)) { throw "Archive does not contain $($pin.exe_in_zip)" }
 $exeHash = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant()
 if ($exeHash -ne $pin.exe_sha256.ToLowerInvariant()) { throw "Executable checksum mismatch: $exeHash" }
 New-Item -ItemType Directory -Path $Destination -Force | Out-Null
 Copy-Item -LiteralPath $exe -Destination (Join-Path $Destination 'mihomo.exe') -Force
 Write-Host "Kernel $($pin.version) ready, sha256 $exeHash"
 if ($env:GITHUB_OUTPUT) { "core_sha256=$exeHash" | Add-Content -LiteralPath $env:GITHUB_OUTPUT }
} finally {
 Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
}

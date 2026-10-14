<#
.SYNOPSIS
Manual installation from build output, for machines without the packaged
installer. Copies the client, service and verified kernel into Program Files,
then hands over to installer/configure-service.ps1 for the service and data
directory. Prefer the Setup.exe from a release for end users.
#>
param(
 [Parameter(Mandatory=$true)][string]$ControlUrl,
 [Parameter(Mandatory=$true)][string]$MihomoPath,
 [Parameter(Mandatory=$true)][ValidatePattern('^[a-fA-F0-9]{64}$')][string]$MihomoSha256,
 [string]$Artifacts = (Join-Path (Split-Path $PSScriptRoot -Parent) 'artifacts/windows')
)
$ErrorActionPreference = 'Stop'
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
if (-not ([Security.Principal.WindowsPrincipal]::new($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Run installation as administrator' }
if (([Uri]$ControlUrl).Scheme -ne 'https') { throw 'Production control URL must use HTTPS' }
$hash = (Get-FileHash -LiteralPath $MihomoPath -Algorithm SHA256).Hash
if ($hash -ne $MihomoSha256) { throw 'Kernel checksum mismatch' }
$configure = Join-Path (Split-Path $PSScriptRoot -Parent) 'installer/configure-service.ps1'
& $configure -Stop
# The client only talks to a service under this exact path.
$installRoot = Join-Path $env:ProgramFiles 'FlowLight Accelerator'
New-Item -ItemType Directory -Path $installRoot,(Join-Path $installRoot 'core') -Force | Out-Null
Copy-Item -LiteralPath (Join-Path $Artifacts 'client') -Destination $installRoot -Recurse -Force
Copy-Item -LiteralPath (Join-Path $Artifacts 'service') -Destination $installRoot -Recurse -Force
Copy-Item -LiteralPath $configure -Destination (Join-Path $installRoot 'service') -Force
Copy-Item -LiteralPath $MihomoPath -Destination (Join-Path $installRoot 'core/mihomo.exe') -Force
& $configure -Install -InstallRoot $installRoot -ControlUrl $ControlUrl -CoreSha256 $MihomoSha256
if ($LASTEXITCODE -ne 0) { throw 'Service configuration failed' }
Write-Host ('Installed. Launch ' + (Join-Path $installRoot 'client/FlowLight.Client.exe'))

param(
 [string]$Runtime = 'win-x64',
 # Stamped into the executables so they match the installer version.
 [string]$Version = '0.1.0'
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$artifactRoot = Join-Path $projectRoot 'artifacts/windows'
dotnet publish (Join-Path $projectRoot 'apps/windows-client/FlowLight.Client.csproj') -c Release -r $Runtime --self-contained true "/p:Version=$Version" -o (Join-Path $artifactRoot 'client')
if ($LASTEXITCODE -ne 0) { throw 'Client build failed' }
dotnet publish (Join-Path $projectRoot 'apps/windows-service/FlowLight.Service.csproj') -c Release -r $Runtime --self-contained true "/p:Version=$Version" -o (Join-Path $artifactRoot 'service')
if ($LASTEXITCODE -ne 0) { throw 'Service build failed' }
Write-Host "Windows artifacts: $artifactRoot"

param([Parameter(Mandatory=$true)][string]$AdminPassword)
$ErrorActionPreference = 'Stop'
if ($AdminPassword.Length -lt 14) { throw 'Choose an administrator password with at least 14 characters' }
$env:ADMIN_PASSWORD = $AdminPassword
$env:ADMIN_ORIGIN = 'http://localhost:5173'
$env:DATA_DIR = Join-Path (Split-Path $PSScriptRoot -Parent) 'runtime/control'
Set-Location (Split-Path $PSScriptRoot -Parent)
go run ./apps/control-api

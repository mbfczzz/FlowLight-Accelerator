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
$existing = Get-Service -Name FlowLightAccelerator -ErrorAction SilentlyContinue
if ($existing -and $existing.Status -ne 'Stopped') { throw 'Stop acceleration and the service before updating it' }
$hash = (Get-FileHash -LiteralPath $MihomoPath -Algorithm SHA256).Hash
if ($hash -ne $MihomoSha256) { throw 'Kernel checksum mismatch' }
$installRoot = Join-Path $env:ProgramFiles 'FlowLight Accelerator'
$dataRoot = Join-Path $env:ProgramData 'FlowLightAccelerator'
# Standard users may create folders under ProgramData. A folder they created
# first stays theirs after the ACL reset and would let them rewrite
# service.json for the SYSTEM service, so only trust one owned by SYSTEM or
# Administrators. Never delete it here: it could be a junction elsewhere.
$adminsSid = [Security.Principal.SecurityIdentifier]::new([Security.Principal.WellKnownSidType]::BuiltinAdministratorsSid,$null)
$systemSid = [Security.Principal.SecurityIdentifier]::new([Security.Principal.WellKnownSidType]::LocalSystemSid,$null)
if (Test-Path -LiteralPath $dataRoot) {
 $item = Get-Item -LiteralPath $dataRoot -Force
 $owner = (Get-Acl -LiteralPath $dataRoot).GetOwner([Security.Principal.SecurityIdentifier])
 if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -or ($owner -ne $adminsSid -and $owner -ne $systemSid)) { throw "Untrusted existing $dataRoot (owner $owner). Inspect and remove it as an administrator, then retry." }
}
New-Item -ItemType Directory -Path $installRoot,$dataRoot,(Join-Path $installRoot 'core') -Force | Out-Null
$acl = [Security.AccessControl.DirectorySecurity]::new()
$acl.SetOwner($adminsSid)
$acl.SetAccessRuleProtection($true,$false)
foreach ($sidType in @([Security.Principal.WellKnownSidType]::LocalSystemSid,[Security.Principal.WellKnownSidType]::BuiltinAdministratorsSid)) {
 $sid = [Security.Principal.SecurityIdentifier]::new($sidType,$null)
 $acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($sid,'FullControl','ContainerInherit,ObjectInherit','None','Allow'))
}
Set-Acl -LiteralPath $dataRoot -AclObject $acl
Copy-Item -LiteralPath (Join-Path $Artifacts 'client') -Destination $installRoot -Recurse -Force
Copy-Item -LiteralPath (Join-Path $Artifacts 'service') -Destination $installRoot -Recurse -Force
$coreTarget = Join-Path $installRoot 'core/mihomo.exe'
Copy-Item -LiteralPath $MihomoPath -Destination $coreTarget -Force
@{control_url=$ControlUrl;core_path=$coreTarget;core_sha256=$MihomoSha256.ToLowerInvariant();data_dir=(Join-Path $dataRoot 'runtime');allow_local_http=$false} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $dataRoot 'service.json') -Encoding utf8
$serviceExe = Join-Path $installRoot 'service/FlowLight.Service.exe'
if (-not $existing) { New-Service -Name FlowLightAccelerator -BinaryPathName ('"' + $serviceExe + '"') -DisplayName 'FlowLight Accelerator Network Service' -StartupType Automatic | Out-Null }
Start-Service FlowLightAccelerator
Write-Host ('Installed. Launch ' + (Join-Path $installRoot 'client/FlowLight.Client.exe'))

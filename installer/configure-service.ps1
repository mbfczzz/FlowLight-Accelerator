<#
.SYNOPSIS
Registers, stops or removes the FlowLight Accelerator Windows service once the
program files are in place. The installer runs it after copying files, before
an upgrade and during uninstall; scripts/install-windows-service.ps1 uses it
for manual installs. Compatible with Windows PowerShell 5.1.

Stopping also closes the client and any kernel left running from the
install folder, so their files can be replaced or removed.

Exit codes tell the installer which step failed: 10 kernel file missing or
altered, 11 old service did not stop, 12 untrusted data folder, 13 service
could not be registered, 14 service did not start, 1 anything else.
#>
[CmdletBinding(DefaultParameterSetName = 'Install')]
param(
 [Parameter(ParameterSetName = 'Install', Mandatory = $true)][switch]$Install,
 [Parameter(ParameterSetName = 'Stop', Mandatory = $true)][switch]$Stop,
 [Parameter(ParameterSetName = 'Uninstall', Mandatory = $true)][switch]$Uninstall,
 [string]$InstallRoot = (Split-Path -Parent $PSScriptRoot),
 [Parameter(ParameterSetName = 'Install')][string]$ControlUrl,
 [Parameter(ParameterSetName = 'Install')][string]$CoreSha256,
 [string]$LogPath
)
$ErrorActionPreference = 'Stop'
$ServiceName = 'FlowLightAccelerator'
$dataRoot = Join-Path $env:ProgramData 'FlowLightAccelerator'

function Write-Log([string]$Message) {
 if ($LogPath) {
  try { Add-Content -LiteralPath $LogPath -Value ('{0} {1}' -f (Get-Date -Format s), $Message) -Encoding utf8 } catch { }
 }
 Write-Host $Message
}
function Assert-Administrator {
 $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
 $principal = [Security.Principal.WindowsPrincipal]::new($identity)
 if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Run as administrator' }
}
$script:Stage = 1
function Stop-FlowLightService {
 $script:Stage = 11
 $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
 if ($service -and $service.Status -ne 'Stopped') {
  Write-Log 'Stopping service'
  Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
  try { $service.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(25)) } catch { throw 'Service did not stop' }
 }
 # The client (often in the tray) and a kernel the service left behind
 # hold files the installer replaces. Only processes from this folder.
 $root = [IO.Path]::GetFullPath($InstallRoot).TrimEnd('\') + '\'
 foreach ($p in Get-CimInstance Win32_Process -Filter "Name = 'FlowLight.Client.exe' OR Name = 'mihomo.exe'") {
  if ($p.ExecutablePath -and $p.ExecutablePath.StartsWith($root, [StringComparison]::OrdinalIgnoreCase)) {
   Write-Log ('Closing ' + $p.ExecutablePath)
   Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
  }
 }
 $script:Stage = 1
}

# Standard users may create folders under ProgramData. A folder they created
# first would stay theirs after the ACL reset and let them rewrite service.json
# for the SYSTEM service, so only a folder owned by SYSTEM or Administrators is
# trusted. It is never deleted here: it could be a junction elsewhere.
function Initialize-DataRoot {
 $adminsSid = [Security.Principal.SecurityIdentifier]::new([Security.Principal.WellKnownSidType]::BuiltinAdministratorsSid, $null)
 $systemSid = [Security.Principal.SecurityIdentifier]::new([Security.Principal.WellKnownSidType]::LocalSystemSid, $null)
 if (Test-Path -LiteralPath $dataRoot) {
  $item = Get-Item -LiteralPath $dataRoot -Force
  $owner = (Get-Acl -LiteralPath $dataRoot).GetOwner([Security.Principal.SecurityIdentifier])
  if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -or ($owner -ne $adminsSid -and $owner -ne $systemSid)) {
   throw "Untrusted existing $dataRoot (owner $owner). Inspect and remove it as an administrator, then retry."
  }
 }
 New-Item -ItemType Directory -Path $dataRoot -Force | Out-Null
 $acl = [Security.AccessControl.DirectorySecurity]::new()
 $acl.SetOwner($adminsSid)
 $acl.SetAccessRuleProtection($true, $false)
 foreach ($sid in @($systemSid, $adminsSid)) {
  $acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($sid, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow'))
 }
 Set-Acl -LiteralPath $dataRoot -AclObject $acl
}

try {
 Assert-Administrator
 if ($Stop) {
  Stop-FlowLightService
  exit 0
 }
 if ($Uninstall) {
  Stop-FlowLightService
  if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
   Write-Log 'Removing service registration'
   & sc.exe delete $ServiceName | Out-Null
  }
  if (Test-Path -LiteralPath $dataRoot) {
   $item = Get-Item -LiteralPath $dataRoot -Force
   if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) {
    # Remove the link itself, never what it points to.
    [IO.Directory]::Delete($dataRoot)
   } else {
    Remove-Item -LiteralPath $dataRoot -Recurse -Force
   }
  }
  Write-Log 'Service removed'
  exit 0
 }

 if (-not $ControlUrl) { throw 'ControlUrl is required' }
 if (([Uri]$ControlUrl).Scheme -ne 'https') { throw 'Control URL must use HTTPS' }
 if ($CoreSha256 -notmatch '^[a-fA-F0-9]{64}$') { throw 'CoreSha256 must be 64 hexadecimal characters' }
 $coreTarget = Join-Path $InstallRoot 'core\mihomo.exe'
 $serviceExe = Join-Path $InstallRoot 'service\FlowLight.Service.exe'
 $script:Stage = 10
 foreach ($required in @($coreTarget, $serviceExe)) {
  if (-not (Test-Path -LiteralPath $required)) { throw "Missing $required" }
 }
 $hash = (Get-FileHash -LiteralPath $coreTarget -Algorithm SHA256).Hash
 if ($hash -ne $CoreSha256) { throw 'Kernel checksum mismatch' }

 Stop-FlowLightService
 $script:Stage = 12
 Initialize-DataRoot
 $script:Stage = 1
 $config = @{
  control_url = $ControlUrl
  core_path = $coreTarget
  core_sha256 = $CoreSha256.ToLowerInvariant()
  data_dir = (Join-Path $dataRoot 'runtime')
  allow_local_http = $false
 }
 $config | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $dataRoot 'service.json') -Encoding utf8
 Write-Log "Wrote service configuration for $ControlUrl"

 $binary = '"' + $serviceExe + '"'
 $script:Stage = 13
 if (-not (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue)) {
  Write-Log 'Registering service'
  New-Service -Name $ServiceName -BinaryPathName $binary -DisplayName 'FlowLight Accelerator Network Service' `
   -Description 'Manages the FlowLight Accelerator tunnel and network kernel.' -StartupType Automatic | Out-Null
 } else {
  & sc.exe config $ServiceName binPath= $binary start= auto | Out-Null
 }
 # Restart automatically after a crash; the Job Object has already reclaimed the kernel.
 & sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null
 $script:Stage = 14
 Start-Service -Name $ServiceName
 (Get-Service -Name $ServiceName).WaitForStatus('Running', [TimeSpan]::FromSeconds(20))
 Write-Log 'Service running'
 exit 0
} catch {
 Write-Log ('ERROR (' + $script:Stage + '): ' + $_.Exception.Message)
 exit $script:Stage
}

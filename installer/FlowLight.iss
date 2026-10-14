; FlowLight Accelerator Windows installer (Inno Setup 6.3 or later).
; Compiled by .github/workflows/release.yml:
;   ISCC.exe /DAppVersion=0.1.0 /DFileVersion=0.1.0.0 /DCoreSha256=<sha256 of core\mihomo.exe> installer\FlowLight.iss
; The payload directory is produced by scripts\build-windows.ps1 (client, service)
; and installer\fetch-kernel.ps1 (core\mihomo.exe).

#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif
#ifndef FileVersion
  #define FileVersion "0.1.0.0"
#endif
#ifndef Payload
  #define Payload SourcePath + "\..\artifacts\windows"
#endif
#ifndef CoreSha256
  #error Define CoreSha256 with the SHA-256 of core\mihomo.exe in the payload
#endif
#ifndef DefaultControlUrl
  #define DefaultControlUrl ""
#endif
; Echoed in the compiler log so every build proves which address was baked in.
#pragma message "DefaultControlUrl=" + DefaultControlUrl
#define AppName "FlowLight Accelerator"
#define AppNameZh "流光加速器"
#define ServiceName "FlowLightAccelerator"
#define ClientExe "client\FlowLight.Client.exe"

[Setup]
AppId={{7F2B6C0D-3A1E-4C7B-9E55-2A9F0C4D1B61}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppNameZh} {#AppName} {#AppVersion}
AppPublisher=FlowLight
AppPublisherURL=https://github.com/mbfczzz/FlowLight-Accelerator
VersionInfoVersion={#FileVersion}
VersionInfoProductTextVersion={#AppVersion}
; The client verifies that the service runs from exactly this path, so the
; directory is fixed.
DefaultDirName={autopf}\{#AppName}
DisableDirPage=yes
UsePreviousAppDir=no
DisableProgramGroupPage=yes
DefaultGroupName={#AppNameZh}
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.17763
OutputDir={#SourcePath}\..\artifacts\installer
OutputBaseFilename=FlowLight-Accelerator-Setup-{#AppVersion}
SetupIconFile={#SourcePath}\flowlight.ico
; Inno shows these in the wizard; it requires 24-bit BMPs at exactly
; 164x314 (side banner) and 55x55 (top-right corner).
WizardImageFile={#SourcePath}\wizard-image.bmp
WizardSmallImageFile={#SourcePath}\wizard-small.bmp
UninstallDisplayIcon={app}\{#ClientExe}
UninstallDisplayName={#AppNameZh} {#AppName}
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
RestartApplications=no
SetupLogging=yes
ShowLanguageDialog=auto

[Languages]
Name: "zh"; MessagesFile: "ChineseSimplified.isl"
Name: "en"; MessagesFile: "compiler:Default.isl"

[CustomMessages]
zh.ControlPageCaption=控制服务地址
zh.ControlPageDescription=客户端将连接运营方提供的控制服务。
zh.ControlPageSubCaption=请输入运营方提供的控制服务地址，必须以 https:// 开头。
zh.ControlPagePrompt=控制服务地址：
zh.ControlUrlInvalid=控制服务地址必须以 https:// 开头，且不含空格或引号。
zh.ServiceConfigureFailed=本地服务配置失败，程序文件已安装但加速不可用。详情见：%1
zh.LaunchClient=启动流光加速器
en.ControlPageCaption=Control service
en.ControlPageDescription=The client connects to the control service run by your operator.
en.ControlPageSubCaption=Enter the control service address from your operator. It must start with https://.
en.ControlPagePrompt=Control service address:
en.ControlUrlInvalid=The control service address must start with https:// and contain no spaces or quotes.
en.ServiceConfigureFailed=The local service could not be configured. Files are installed but acceleration is unavailable. Details: %1
en.LaunchClient=Launch FlowLight Accelerator

[Files]
Source: "{#Payload}\client\*"; DestDir: "{app}\client"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#Payload}\service\*"; DestDir: "{app}\service"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#Payload}\core\mihomo.exe"; DestDir: "{app}\core"; Flags: ignoreversion
Source: "configure-service.ps1"; DestDir: "{app}\service"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#AppNameZh}"; Filename: "{app}\{#ClientExe}"
Name: "{autodesktop}\{#AppNameZh}"; Filename: "{app}\{#ClientExe}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Run]
Filename: "{app}\{#ClientExe}"; Description: "{cm:LaunchClient}"; Flags: nowait postinstall skipifsilent runasoriginaluser

[UninstallRun]
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File ""{app}\service\configure-service.ps1"" -Uninstall -LogPath ""{app}\service\uninstall.log"""; Flags: runhidden waituntilterminated; RunOnceId: "RemoveService"

[Code]
var
  ControlPage: TInputQueryWizardPage;

const
  ServiceName = '{#ServiceName}';

function PowerShell(): String;
begin
  Result := ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe');
end;

function RunPowerShell(const Args: String; var ResultCode: Integer): Boolean;
begin
  Log('powershell ' + Args);
  Result := Exec(PowerShell(), '-NoProfile -NonInteractive -ExecutionPolicy Bypass ' + Args, '',
    SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Log('powershell exit code ' + IntToStr(ResultCode));
end;

function ControlUrl(): String;
begin
  Result := Trim(ControlPage.Values[0]);
end;

function ValidControlUrl(const Url: String): Boolean;
begin
  Result := (Length(Url) > 8) and (CompareText(Copy(Url, 1, 8), 'https://') = 0)
    and (Pos('"', Url) = 0) and (Pos(' ', Url) = 0);
end;

procedure InitializeWizard;
var
  Initial: String;
begin
  ControlPage := CreateInputQueryPage(wpWelcome, CustomMessage('ControlPageCaption'),
    CustomMessage('ControlPageDescription'), CustomMessage('ControlPageSubCaption'));
  ControlPage.Add(CustomMessage('ControlPagePrompt'), False);
  { Silent installs pass /CONTROLURL=...; upgrades keep the previous address. }
  Initial := ExpandConstant('{param:CONTROLURL|}');
  if Initial = '' then
    Initial := GetPreviousData('ControlUrl', '{#DefaultControlUrl}');
  ControlPage.Values[0] := Initial;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if (CurPageID = ControlPage.ID) and not ValidControlUrl(ControlUrl()) then
  begin
    MsgBox(CustomMessage('ControlUrlInvalid'), mbError, MB_OK);
    Result := False;
  end;
end;

procedure RegisterPreviousData(PreviousDataKey: Integer);
begin
  SetPreviousData(PreviousDataKey, 'ControlUrl', ControlUrl());
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  Code: Integer;
begin
  Result := '';
  if not ValidControlUrl(ControlUrl()) then
  begin
    Result := CustomMessage('ControlUrlInvalid');
    exit;
  end;
  { An upgrade replaces files the running service holds open. }
  RunPowerShell('-Command "$s = Get-Service -Name ' + ServiceName + ' -ErrorAction SilentlyContinue; '
    + 'if ($s -and $s.Status -ne ''Stopped'') { Stop-Service -Name ' + ServiceName + ' -Force; '
    + '$s.WaitForStatus(''Stopped'', [TimeSpan]::FromSeconds(25)) }"', Code);
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  Code: Integer;
  LogFile: String;
begin
  if CurStep = ssPostInstall then
  begin
    LogFile := ExpandConstant('{app}\service\install.log');
    if not RunPowerShell('-File "' + ExpandConstant('{app}\service\configure-service.ps1') + '"'
        + ' -Install -InstallRoot "' + ExpandConstant('{app}') + '"'
        + ' -ControlUrl "' + ControlUrl() + '"'
        + ' -CoreSha256 {#CoreSha256}'
        + ' -LogPath "' + LogFile + '"', Code) or (Code <> 0) then
      MsgBox(FmtMessage(CustomMessage('ServiceConfigureFailed'), [LogFile]), mbCriticalError, MB_OK);
  end;
end;

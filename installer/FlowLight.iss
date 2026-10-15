; FlowLight Accelerator Windows installer (Inno Setup 6.7 or later).
; Compiled by .github/workflows/release.yml:
;   ISCC.exe /DAppVersion=0.1.0 /DFileVersion=0.1.0.0 /DCoreSha256=<sha256 of core\mihomo.exe> installer\FlowLight.iss
; The payload directory is produced by scripts\build-windows.ps1 (client, service)
; and installer\fetch-kernel.ps1 (core\mihomo.exe). installer\art is drawn by
; installer\make-brand-assets.ps1.
;
; /DPreview builds a look-alike that needs no administrator rights, installs
; its payload into %LOCALAPPDATA%\FlowLightInstallerPreview, registers nothing
; and only pretends to configure the service, for checking the wizard's
; screens. /PREVIEWUPGRADE=<version> and /PREVIEWFAIL=<code> on its command
; line show the upgrade and failure states.

; Background art, the dark style and hidebevels need Inno Setup 6.7.
#if Ver < EncodeVer(6, 7, 0)
  #error Inno Setup 6.7 or later is required
#endif

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
#ifdef Preview
PrivilegesRequired=lowest
DefaultDirName={localappdata}\FlowLightInstallerPreview
Uninstallable=no
OutputDir={#SourcePath}\..\artifacts\installer-preview
OutputBaseFilename=FlowLight-Setup-Preview
#else
; The client verifies that the service runs from exactly this path, so the
; directory is fixed.
DefaultDirName={autopf}\{#AppName}
PrivilegesRequired=admin
OutputDir={#SourcePath}\..\artifacts\installer
OutputBaseFilename=FlowLight-Accelerator-Setup-{#AppVersion}
#endif
DisableDirPage=yes
UsePreviousAppDir=no
DisableProgramGroupPage=yes
DisableWelcomePage=no
DisableReadyPage=yes
DefaultGroupName={#AppNameZh}
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.17763
SetupIconFile={#SourcePath}\flowlight.ico
UninstallDisplayIcon={app}\{#ClientExe}
UninstallDisplayName={#AppNameZh} {#AppName}
Compression=lzma2/max
SolidCompression=yes
; The client's look: graphite with amber light, no divider lines, a dark
; title bar. The background art is switched per page in [Code]; the colour
; turns the custom background on and shows under it.
WizardStyle=modern dark includetitlebar hidebevels
WizardBackColor=#131217
WizardImageFile=
WizardSmallImageFile=
; [Code] closes the client and the kernel before files are replaced, so the
; Restart Manager page (and its "could not close" warning) never appears.
CloseApplications=no
RestartApplications=no
SetupLogging=yes
ShowLanguageDialog=auto

[Languages]
Name: "zh"; MessagesFile: "ChineseSimplified.isl"
Name: "en"; MessagesFile: "compiler:Default.isl"

[LangOptions]
zh.DialogFontName=Microsoft YaHei UI
zh.WelcomeFontName=Microsoft YaHei UI

[Messages]
zh.SetupWindowTitle=流光加速器 安装程序
zh.UninstallAppTitle=卸载流光加速器
en.SetupWindowTitle=FlowLight Accelerator Setup

[CustomMessages]
zh.Tagline=FLOWLIGHT ACCELERATOR
zh.WelcomeHeading=为游戏而生的加速器
zh.WelcomeBody=选好游戏和区服，一键连上更快的线路；延迟、丢包和线路负载随时看得见。
zh.InstallTo=将安装到 %1
zh.UpgradeFrom=已安装 v%1，将升级到 v%2，账号和设置都会保留
zh.ReinstallSame=已安装 v%1，将重新安装以修复程序文件
zh.DesktopIcon=创建桌面快捷方式
zh.CustomServer=自定义服务器地址
zh.AdminNote=安装需要管理员权限，并会注册“流光加速器网络服务”。如果安全软件询问，请选择允许。
zh.ButtonInstall=立即安装
zh.ButtonUpgrade=立即升级
zh.ButtonReinstall=重新安装
zh.ButtonStart=开始使用
zh.InstallingTitle=正在安装流光加速器
zh.UpgradingTitle=正在升级流光加速器
zh.InstallingNote=通常不到一分钟，请稍候
zh.StatusService=正在配置并启动加速服务…
zh.DoneTitle=安装完成
zh.UpgradedTitle=升级完成
zh.DoneBody=打开流光加速器，用运营方提供的账号登录，选好游戏和区服就可以开始加速。
zh.FailedTitle=已安装，但加速服务没有启动
zh.LogAt=安装记录：%1
zh.ControlPageCaption=自定义服务器地址
zh.ControlPageDescription=一般不需要修改。只有运营方给了单独的地址时才填写。
zh.ControlPageSubCaption=控制服务地址必须以 https:// 开头。
zh.ControlPagePrompt=控制服务地址：
zh.ControlUrlInvalid=控制服务地址必须以 https:// 开头，且不含空格或引号。
zh.LaunchClient=立即启动流光加速器
zh.ErrorKernel=加速内核文件缺失或被改动，通常是安全软件把它当成了风险文件。请在安全软件里恢复它，并把安装目录 %1 加入信任区，然后重新运行安装程序。
zh.ErrorStop=旧版本的加速服务没有及时停止。请重启电脑后重新运行安装程序。
zh.ErrorDataFolder=文件夹 %1 的权限不安全。请删除这个文件夹后重新运行安装程序。
zh.ErrorRegister=无法注册流光加速器网络服务，可能被安全软件拦截。请在安全软件中允许后重新运行安装程序。
zh.ErrorStart=加速服务没有启动，可能被安全软件拦截。请在安全软件中允许 FlowLight.Service.exe，然后重新运行安装程序。
zh.ErrorPowerShell=无法运行 Windows PowerShell 来配置加速服务。请确认系统没有禁用 PowerShell 后重新运行安装程序。
zh.ErrorOther=配置加速服务时出错。请重新运行安装程序；仍然失败时，把安装记录发给客服。
en.Tagline=FLOWLIGHT ACCELERATOR
en.WelcomeHeading=Made for gaming
en.WelcomeBody=Pick a game and a region and connect through a faster route; latency, loss and line load are always in view.
en.InstallTo=Installs to %1
en.UpgradeFrom=Version %1 is installed and will be upgraded to %2; your account and settings stay.
en.ReinstallSame=Version %1 is installed and will be reinstalled to repair it.
en.DesktopIcon=Create a desktop shortcut
en.CustomServer=Custom server address
en.AdminNote=Setup needs administrator rights and registers the FlowLight network service. Allow it if your security software asks.
en.ButtonInstall=Install
en.ButtonUpgrade=Upgrade
en.ButtonReinstall=Reinstall
en.ButtonStart=Get started
en.InstallingTitle=Installing FlowLight Accelerator
en.UpgradingTitle=Upgrading FlowLight Accelerator
en.InstallingNote=This usually takes less than a minute
en.StatusService=Configuring and starting the network service…
en.DoneTitle=Installed
en.UpgradedTitle=Upgraded
en.DoneBody=Open FlowLight Accelerator, sign in with the account from your operator, pick a game and a region, and start.
en.FailedTitle=Installed, but the network service did not start
en.LogAt=Setup log: %1
en.ControlPageCaption=Custom server address
en.ControlPageDescription=Only change this if your operator gave you a separate address.
en.ControlPageSubCaption=The control service address must start with https://.
en.ControlPagePrompt=Control service address:
en.ControlUrlInvalid=The control service address must start with https:// and contain no spaces or quotes.
en.LaunchClient=Launch FlowLight Accelerator now
en.ErrorKernel=The network kernel is missing or altered, usually because security software flagged it. Restore it there, trust "%1", then run Setup again.
en.ErrorStop=The previous network service did not stop in time. Restart the computer and run Setup again.
en.ErrorDataFolder=The folder "%1" has unsafe permissions. Delete it and run Setup again.
en.ErrorRegister=The network service could not be registered, possibly blocked by security software. Allow it and run Setup again.
en.ErrorStart=The network service did not start, possibly blocked by security software. Allow FlowLight.Service.exe and run Setup again.
en.ErrorPowerShell=Windows PowerShell could not be run to configure the network service. Make sure it is not disabled and run Setup again.
en.ErrorOther=Configuring the network service failed. Run Setup again; if it still fails, send the setup log to support.

[Files]
Source: "{#Payload}\client\*"; DestDir: "{app}\client"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#Payload}\service\*"; DestDir: "{app}\service"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#Payload}\core\mihomo.exe"; DestDir: "{app}\core"; Flags: ignoreversion
Source: "configure-service.ps1"; DestDir: "{app}\service"; Flags: ignoreversion
; Wizard art, extracted to {tmp} at startup.
Source: "art\*.png"; Flags: dontcopy noencryption

#ifndef Preview
[Icons]
Name: "{autoprograms}\{#AppNameZh}"; Filename: "{app}\{#ClientExe}"
Name: "{autodesktop}\{#AppNameZh}"; Filename: "{app}\{#ClientExe}"; Check: WantDesktopIcon
#endif

[Run]
Filename: "{app}\{#ClientExe}"; Description: "{cm:LaunchClient}"; Flags: nowait postinstall skipifsilent runasoriginaluser; Check: ServiceReady
; An update started from the client runs silently with /LAUNCH=1 and reopens it.
Filename: "{app}\{#ClientExe}"; Flags: nowait runasoriginaluser; Check: LaunchAfterUpdate

#ifndef Preview
[UninstallRun]
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File ""{app}\service\configure-service.ps1"" -Uninstall -LogPath ""{app}\service\uninstall.log"""; Flags: runhidden waituntilterminated; RunOnceId: "RemoveService"
#endif

[Code]
const
  ServiceName = '{#ServiceName}';
  UninstallKey = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\{7F2B6C0D-3A1E-4C7B-9E55-2A9F0C4D1B61}_is1';

var
  ControlPage: TInputQueryWizardPage;
  { The custom server page is shown only when asked for (or when no valid
    address was baked in). }
  CustomServer: Boolean;
  { Version already installed, '' on a first install. }
  InstalledVersion: String;
  { Why the service is not running after install, '' when it is. }
  ServiceError: String;
  HeroImages, PageImages: array of TGraphic;
  StartButton, FinishButton: TBitmapButton;
  DesktopCheck: TNewCheckBox;
  InfoText, FinishTitle, FinishBody, PercentText, InstallTitle: TNewStaticText;
  ProgressTrack, ProgressFill: TPanel;

{ ---------- helpers ---------- }

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

function LaunchAfterUpdate(): Boolean;
begin
  Result := WizardSilent() and (ExpandConstant('{param:LAUNCH|0}') = '1');
end;

function ServiceReady(): Boolean;
begin
  Result := ServiceError = '';
end;

function WantDesktopIcon(): Boolean;
begin
  Result := DesktopCheck.Checked;
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

{ The artwork is drawn in Chinese; other languages use Inno's own buttons. }
function ArtButtons(): Boolean;
begin
  Result := ActiveLanguage() = 'zh';
end;

{ 100, 150 or 200: the art scale closest above the display scale. }
function ArtScale(): String;
begin
  if ScaleX(100) > 155 then
    Result := '200'
  else if ScaleX(100) > 105 then
    Result := '150'
  else
    Result := '100';
end;

function ArtFile(const Name: String): String;
begin
  Result := ExpandConstant('{tmp}\' + Name);
  if not FileExists(Result) then
    ExtractTemporaryFile(Name);
end;

function Upgrading(): Boolean;
begin
  Result := (InstalledVersion <> '') and (InstalledVersion <> '{#AppVersion}');
end;

function ActionCaption(): String;
begin
  if InstalledVersion = '' then
    Result := CustomMessage('ButtonInstall')
  else if Upgrading() then
    Result := CustomMessage('ButtonUpgrade')
  else
    Result := CustomMessage('ButtonReinstall');
end;

function ActionArt(): String;
begin
  if InstalledVersion = '' then
    Result := 'install'
  else if Upgrading() then
    Result := 'upgrade'
  else
    Result := 'reinstall';
end;

function NewText(Parent: TWinControl; const Caption: String; X, Y, Size: Integer;
  Bold: Boolean; const Color: String): TNewStaticText;
begin
  Result := TNewStaticText.Create(WizardForm);
  Result.Parent := Parent;
  Result.Left := X;
  Result.Top := Y;
  Result.Font.Name := 'Microsoft YaHei UI';
  Result.Font.Size := Size;
  if Bold then
    Result.Font.Style := [fsBold];
  Result.StyleElements := Result.StyleElements - [seFont];
  Result.Font.Color := StrToColor(Color);
  Result.Caption := Caption;
end;

function NewWrappedText(Parent: TWinControl; const Caption: String; X, Y, Width, Size: Integer;
  const Color: String): TNewStaticText;
begin
  Result := NewText(Parent, Caption, X, Y, Size, False, Color);
  Result.AutoSize := False;
  Result.WordWrap := True;
  Result.Width := Width;
  Result.AdjustHeight();
end;

function NewArtButton(Parent: TWinControl; const Art, Caption: String; X, Y: Integer): TBitmapButton;
begin
  Result := TBitmapButton.Create(WizardForm);
  Result.Parent := Parent;
  { The image has a 2 pixel focus margin on each side. }
  Result.SetBounds(X - 2, Y - 2, ScaleX(200) + 4, ScaleY(46) + 4);
  Result.Stretch := True;
  Result.PngImage.LoadFromFile(ArtFile('button-' + Art + '-' + ArtScale() + '.png'));
  Result.Caption := Caption;
  Result.Cursor := crHand;
end;

procedure NewMark(Parent: TWinControl; X, Y: Integer);
var
  Mark: TBitmapImage;
begin
  Mark := TBitmapImage.Create(WizardForm);
  Mark.Parent := Parent;
  Mark.SetBounds(X, Y, ScaleX(44), ScaleY(44));
  Mark.Stretch := True;
  Mark.BackColor := clNone;
  Mark.PngImage.LoadFromFile(ArtFile('mark-' + ArtScale() + '.png'));
end;

procedure LoadBackImages();
var
  I: Integer;
  Width: String;
  Png: TPngImage;
begin
  SetArrayLength(HeroImages, 5);
  SetArrayLength(PageImages, 5);
  for I := 0 to 4 do
  begin
    case I of
      0: Width := '596';
      1: Width := '796';
      2: Width := '994';
      3: Width := '1193';
    else
      Width := '1272';
    end;
    Png := TPngImage.Create();
    Png.LoadFromFile(ArtFile('back-hero-' + Width + '.png'));
    HeroImages[I] := Png;
    Png := TPngImage.Create();
    Png.LoadFromFile(ArtFile('back-page-' + Width + '.png'));
    PageImages[I] := Png;
  end;
end;

{ ---------- the start page ---------- }

procedure ClickNext(Sender: TObject);
begin
  WizardForm.NextButton.OnClick(WizardForm.NextButton);
end;

procedure CustomServerClick(Sender: TObject);
begin
  CustomServer := True;
  ClickNext(Sender);
end;

procedure BuildWelcomePage();
var
  Page: TNewNotebookPage;
  Left, Top: Integer;
  Text: TNewStaticText;
  Info: String;
begin
  Page := WizardForm.WelcomePage;
  WizardForm.WizardBitmapImage.Visible := False;
  WizardForm.WelcomeLabel1.Visible := False;
  WizardForm.WelcomeLabel2.Visible := False;
  Left := ScaleX(36);

  NewMark(Page, Left, ScaleY(30));
  NewText(Page, '{#AppNameZh}', Left + ScaleX(56), ScaleY(29), 15, True, '#FFFFFF');
  NewText(Page, CustomMessage('Tagline') + '  ·  v{#AppVersion}', Left + ScaleX(57), ScaleY(56), 8, False, '#8F889C');

  Text := NewText(Page, CustomMessage('WelcomeHeading'), Left, ScaleY(116), 21, True, '#F3F0F6');
  Text := NewWrappedText(Page, CustomMessage('WelcomeBody'), Left, Text.Top + Text.Height + ScaleY(10),
    ScaleX(330), 10, '#B3ADBE');

  if InstalledVersion = '' then
    Info := FmtMessage(CustomMessage('InstallTo'), [WizardDirValue()])
  else if Upgrading() then
    Info := FmtMessage(CustomMessage('UpgradeFrom'), [InstalledVersion, '{#AppVersion}'])
  else
    Info := FmtMessage(CustomMessage('ReinstallSame'), [InstalledVersion]);
  InfoText := NewWrappedText(Page, Info, Left, Text.Top + Text.Height + ScaleY(18), ScaleX(380), 9, '#8F889C');
  if InstalledVersion <> '' then
    InfoText.Font.Color := StrToColor('#E6AE45');

  Top := Page.Height - ScaleY(112);
  if ArtButtons() then
  begin
    StartButton := NewArtButton(Page, ActionArt(), ActionCaption(), Left, Top);
    StartButton.OnClick := @ClickNext;
  end;

  DesktopCheck := TNewCheckBox.Create(WizardForm);
  DesktopCheck.Parent := Page;
  DesktopCheck.Caption := CustomMessage('DesktopIcon');
  DesktopCheck.SetBounds(Left, Top + ScaleY(62), ScaleX(170), ScaleY(20));
  { Upgrades keep the shortcut as the player left it. }
  DesktopCheck.Checked := (InstalledVersion = '')
    or FileExists(ExpandConstant('{autodesktop}\{#AppNameZh}.lnk'));

  Text := NewText(Page, CustomMessage('CustomServer'), Left + ScaleX(168), Top + ScaleY(63), 9, False, '#8F889C');
  Text.Font.Style := [fsUnderline];
  Text.Cursor := crHand;
  Text.OnClick := @CustomServerClick;

  NewWrappedText(Page, CustomMessage('AdminNote'), Left, Top + ScaleY(90), Page.Width - 2 * Left, 8, '#7F798B');
end;

{ ---------- the installing page: an amber bar like the client's ---------- }

procedure BuildInstallingPage();
var
  Page: TNewNotebookPage;
  Gauge: TNewProgressBar;
  Note: TNewStaticText;
begin
  { The same large title as the start and finish pages, then the status
    line and the bar. }
  Page := WizardForm.InstallingPage;
  Gauge := WizardForm.ProgressGauge;
  InstallTitle := NewText(Page, CustomMessage('InstallingTitle'), Gauge.Left, ScaleY(14), 21, True, '#F3F0F6');
  Note := NewText(Page, CustomMessage('InstallingNote'), Gauge.Left, InstallTitle.Top + InstallTitle.Height + ScaleY(8), 10, False, '#B3ADBE');
  WizardForm.FilenameLabel.Visible := False;
  WizardForm.StatusLabel.Top := Note.Top + Note.Height + ScaleY(54);
  WizardForm.StatusLabel.StyleElements := WizardForm.StatusLabel.StyleElements - [seFont];
  WizardForm.StatusLabel.Font.Color := StrToColor('#8F889C');
  Gauge.Top := WizardForm.StatusLabel.Top + WizardForm.StatusLabel.Height + ScaleY(12);

  ProgressTrack := TPanel.Create(WizardForm);
  ProgressTrack.Parent := WizardForm.InstallingPage;
  ProgressTrack.BevelOuter := bvNone;
  ProgressTrack.StyleElements := [];
  ProgressTrack.Color := StrToColor('#2B2833');
  ProgressTrack.Caption := '';
  ProgressTrack.SetBounds(Gauge.Left, Gauge.Top + ScaleY(4), Gauge.Width - ScaleX(52), ScaleY(8));

  ProgressFill := TPanel.Create(WizardForm);
  ProgressFill.Parent := ProgressTrack;
  ProgressFill.BevelOuter := bvNone;
  ProgressFill.StyleElements := [];
  ProgressFill.Color := StrToColor('#E6AE45');
  ProgressFill.Caption := '';
  ProgressFill.SetBounds(0, 0, 0, ProgressTrack.Height);

  PercentText := NewText(WizardForm.InstallingPage, '0%', ProgressTrack.Left + ProgressTrack.Width + ScaleX(12),
    Gauge.Top - ScaleY(1), 10, True, '#F3F0F6');
end;

procedure ShowProgress(Percent: Integer);
begin
  if Percent < 0 then
    Percent := 0;
  if Percent > 100 then
    Percent := 100;
  ProgressFill.Width := ProgressTrack.ClientWidth * Percent div 100;
  PercentText.Caption := IntToStr(Percent) + '%';
end;

procedure CurInstallProgressChanged(CurProgress, MaxProgress: Integer);
begin
  if MaxProgress > 0 then
    ShowProgress(Round(100.0 * CurProgress / MaxProgress));
end;

{ ---------- the finish page ---------- }

procedure BuildFinishedPage();
var
  Page: TNewNotebookPage;
  Left: Integer;
begin
  Page := WizardForm.FinishedPage;
  WizardForm.WizardBitmapImage2.Visible := False;
  WizardForm.FinishedHeadingLabel.Visible := False;
  WizardForm.FinishedLabel.Visible := False;
  Left := ScaleX(36);

  NewMark(Page, Left, ScaleY(30));
  NewText(Page, '{#AppNameZh}', Left + ScaleX(56), ScaleY(29), 15, True, '#FFFFFF');
  NewText(Page, CustomMessage('Tagline') + '  ·  v{#AppVersion}', Left + ScaleX(57), ScaleY(56), 8, False, '#8F889C');

  FinishTitle := NewText(Page, '', Left, ScaleY(116), 21, True, '#F3F0F6');
  FinishBody := NewWrappedText(Page, '', Left, ScaleY(160), ScaleX(430), 10, '#B3ADBE');

  if ArtButtons() then
  begin
    FinishButton := NewArtButton(Page, 'start', CustomMessage('ButtonStart'), Left, Page.Height - ScaleY(112));
    FinishButton.OnClick := @ClickNext;
  end;
end;

procedure ShowFinished();
var
  Top: Integer;
begin
  if ServiceError = '' then
  begin
    if Upgrading() then
      FinishTitle.Caption := CustomMessage('UpgradedTitle')
    else
      FinishTitle.Caption := CustomMessage('DoneTitle');
    FinishBody.Caption := CustomMessage('DoneBody');
    FinishBody.Font.Color := StrToColor('#B3ADBE');
  end
  else
  begin
    FinishTitle.Caption := CustomMessage('FailedTitle');
    FinishTitle.Font.Color := StrToColor('#F0A25A');
    FinishBody.Caption := ServiceError;
    FinishBody.Font.Color := StrToColor('#E3DEEA');
  end;
  FinishBody.AdjustHeight();

  { The launch option sits under the call to action. }
  Top := WizardForm.FinishedPage.Height - ScaleY(112);
  WizardForm.RunList.Left := ScaleX(34);
  WizardForm.RunList.Top := Top + ScaleY(60);
  WizardForm.RunList.Width := ScaleX(320);
  WizardForm.RunList.Height := ScaleY(24);

  if ArtButtons() and (ServiceError = '') then
  begin
    FinishButton.Visible := True;
    WizardForm.NextButton.Visible := False;
  end
  else
  begin
    if ArtButtons() then
      FinishButton.Visible := False;
    WizardForm.NextButton.Visible := True;
  end;
end;

{ ---------- the service ---------- }

function ServiceErrorText(Code: Integer): String;
begin
  case Code of
    10: Result := FmtMessage(CustomMessage('ErrorKernel'), [ExpandConstant('{app}')]);
    11: Result := CustomMessage('ErrorStop');
    12: Result := FmtMessage(CustomMessage('ErrorDataFolder'), [ExpandConstant('{commonappdata}\FlowLightAccelerator')]);
    13: Result := CustomMessage('ErrorRegister');
    14: Result := CustomMessage('ErrorStart');
  else
    Result := CustomMessage('ErrorOther');
  end;
end;

procedure ConfigureService();
var
  Code: Integer;
  Ran: Boolean;
  LogFile: String;
begin
  LogFile := ExpandConstant('{app}\service\install.log');
#ifdef Preview
  Ran := Exec(ExpandConstant('{sys}\ping.exe'), '-n 4 127.0.0.1', '', SW_HIDE, ewWaitUntilTerminated, Code);
  Code := StrToIntDef(ExpandConstant('{param:PREVIEWFAIL|0}'), 0);
#else
  Ran := RunPowerShell('-File "' + ExpandConstant('{app}\service\configure-service.ps1') + '"'
    + ' -Install -InstallRoot "' + ExpandConstant('{app}') + '"'
    + ' -ControlUrl "' + ControlUrl() + '"'
    + ' -CoreSha256 {#CoreSha256}'
    + ' -LogPath "' + LogFile + '"', Code);
#endif
  if not Ran then
    ServiceError := CustomMessage('ErrorPowerShell')
  else if Code <> 0 then
    ServiceError := ServiceErrorText(Code);
  if ServiceError <> '' then
  begin
    ServiceError := ServiceError + #13#10#13#10 + FmtMessage(CustomMessage('LogAt'), [LogFile]);
    Log('service not ready: ' + ServiceError);
    if WizardSilent() then
      SuppressibleMsgBox(ServiceError, mbError, MB_OK, IDOK);
  end;
end;

{ ---------- wizard events ---------- }

function InitializeSetup(): Boolean;
begin
#ifdef Preview
  InstalledVersion := ExpandConstant('{param:PREVIEWUPGRADE|}');
#else
  if not RegQueryStringValue(HKLM64, UninstallKey, 'DisplayVersion', InstalledVersion) then
    InstalledVersion := '';
#endif
  Result := True;
end;

procedure InitializeWizard();
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

  LoadBackImages();
  BuildWelcomePage();
  BuildInstallingPage();
  BuildFinishedPage();
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := (PageID = ControlPage.ID) and not CustomServer and ValidControlUrl(ControlUrl());
end;

procedure CurPageChanged(CurPageID: Integer);
begin
  if (CurPageID = wpWelcome) or (CurPageID = wpFinished) then
    WizardSetBackImage(HeroImages, True, True, 255)
  else
    WizardSetBackImage(PageImages, True, True, 255);

  { The start page is one screen with its own call to action; the title
    bar's close button cancels. }
  WizardForm.CancelButton.Visible := (CurPageID <> wpFinished) and ((CurPageID <> wpWelcome) or not ArtButtons());
  { Inner pages carry their own titles; the header band stays empty there. }
  WizardForm.PageNameLabel.Visible := CurPageID <> wpInstalling;
  WizardForm.PageDescriptionLabel.Visible := CurPageID <> wpInstalling;
  if CurPageID = wpWelcome then
  begin
    WizardForm.NextButton.Caption := ActionCaption();
    WizardForm.NextButton.Visible := not ArtButtons();
  end
  else if CurPageID = ControlPage.ID then
  begin
    WizardForm.NextButton.Caption := ActionCaption();
    WizardForm.NextButton.Visible := True;
  end
  else if CurPageID = wpInstalling then
  begin
    WizardForm.ProgressGauge.Visible := False;
    if Upgrading() then
      InstallTitle.Caption := CustomMessage('UpgradingTitle')
    else
      InstallTitle.Caption := CustomMessage('InstallingTitle');
    ShowProgress(0);
  end
  else if CurPageID = wpFinished then
    ShowFinished();
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
#ifndef Preview
  { An upgrade replaces files the service, the kernel and the client hold
    open; this setup's own script stops all three. }
  ExtractTemporaryFile('configure-service.ps1');
  if RunPowerShell('-File "' + ExpandConstant('{tmp}\configure-service.ps1') + '" -Stop'
      + ' -InstallRoot "' + ExpandConstant('{app}') + '"', Code) and (Code = 11) then
    Result := CustomMessage('ErrorStop');
#endif
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    WizardForm.StatusLabel.Caption := CustomMessage('StatusService');
    ShowProgress(100);
    ConfigureService();
  end;
end;

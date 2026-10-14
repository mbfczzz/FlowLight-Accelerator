# Windows 客户端 UI 验证脚手架

不依赖真实控制服务和真实本地服务，把客户端从登录到加速、重启恢复、托盘唤醒跑一遍并逐步截图。

- `fake-service.ps1`：替身本地服务。按客户端的命名管道协议应答 status / login / games / lines / start / stop / logout，
  会话按计时器走 PREPARING → CONNECTING → ACTIVE。默认管道名 `FlowLight.Accelerator.dev`，可与真服务共存。登录密码 `pw`。
- `drive.ps1`：UI Automation 驱动。输入账号密码、选第二个游戏和第二个区服、开始加速、等待连上、杀掉客户端再重开验证恢复选中、
  停止、关闭缩托盘、再次启动验证单实例唤醒；每步截图到 `-Ui` 目录，日志写 `drive.log`。
- `capture.ps1`：把客户端窗口调整到指定尺寸并截图。

运行（Windows PowerShell 5.1，UIAutomationClient 是 .NET Framework 程序集）：

    $ui = "$env:TEMP\fl-ui"; New-Item -ItemType Directory -Force $ui | Out-Null
    Start-Process powershell -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File','.\fake-service.ps1' -WindowStyle Hidden
    powershell -NoProfile -ExecutionPolicy Bypass -File .\drive.ps1 -Exe ..\..\artifacts\windows\client\FlowLight.Client.exe -Ui $ui

客户端通过 `FLOWLIGHT_DEV_PIPE=1` 跳过服务身份校验、`FLOWLIGHT_PIPE_NAME` 指向替身管道，驱动脚本会自动设置。
框架依赖的构建输出需要先设置 `DOTNET_ROOT` 指向 .NET 10 运行时；自包含发布产物不需要。

脚本必须保持 UTF-8 带 BOM：Windows PowerShell 5.1 读取无 BOM 文件按系统代码页解析，中文字面量会损坏。

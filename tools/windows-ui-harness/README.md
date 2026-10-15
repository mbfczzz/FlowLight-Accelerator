# Windows 客户端 UI 验证脚手架

不依赖真实控制服务和真实本地服务，把客户端从登录到加速、重启恢复、托盘唤醒跑一遍并逐步截图。

- `fake-service.ps1`：替身本地服务。按客户端的命名管道协议应答 status / login / games / lines / start / stop / logout，
  会话按计时器走 PREPARING → CONNECTING → ACTIVE；线路带容量与在线数，连上后每 4 秒给出一次带 observed_at 的新检测值，
  并报告 active_since，便于检查走势图和已加速时长。默认管道名 `FlowLight.Accelerator.dev`，可与真服务共存。登录密码 `pw`。
- `drive.ps1`：UI Automation 驱动。登录、在游戏库选第二款游戏并经左侧“加速”进入、选第二个区服、按圆形加速按钮、等待连上、杀掉客户端再重开验证自动回到正在加速的游戏页、停止、关闭缩托盘、再次启动验证单实例唤醒；每步截图到 `-Ui` 目录，日志写 `drive.log`。
- `explore.ps1`：探索式走查。输错密码、游戏页、切换区服与线路、搜索与搜索无结果、服务拒绝开始、连接中与加速中、加速时的游戏库、账号菜单、复制诊断、切换账号、退出登录、900×650 与最大化、本地服务中途消失再恢复；每个状态截图，供人工逐张审阅。
  元素按 x:Name 定位（WPF 把它作为 AutomationId）：GameList / RegionList / LineList / NavLibrary / NavGame / ActionButton，状态行、内联错误、提示分别是 ConnectionState / InlineError / Toast；圆形按钮的名称随状态变为 开始加速 / 正在连接 / 加速中 / 正在停止；账号相关操作先点头像按钮“账号”再点菜单项。
- `capture.ps1`：把客户端窗口调整到指定尺寸并截图。

运行（Windows PowerShell 5.1，UIAutomationClient 是 .NET Framework 程序集）：

    $ui = "$env:TEMP\fl-ui"; New-Item -ItemType Directory -Force $ui | Out-Null
    Start-Process powershell -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File','.\fake-service.ps1' -WindowStyle Hidden
    powershell -NoProfile -ExecutionPolicy Bypass -File .\drive.ps1 -Exe ..\..\artifacts\windows\client\FlowLight.Client.exe -Ui $ui
    # explore.ps1 自己启动并重启替身服务；先结束手动启动的那个，否则两个实例轮流应答，客户端会在已登录/未登录之间跳变。
    Get-CimInstance Win32_Process | Where-Object CommandLine -like '*fake-service.ps1*' | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }
    powershell -NoProfile -ExecutionPolicy Bypass -File .\explore.ps1 -Exe ..\..\artifacts\windows\client\FlowLight.Client.exe -Ui $ui -Harness (Get-Location)

客户端通过 `FLOWLIGHT_DEV_PIPE=1` 跳过服务身份校验、`FLOWLIGHT_PIPE_NAME` 指向替身管道，驱动脚本会自动设置。
框架依赖的构建输出需要先设置 `DOTNET_ROOT` 指向 .NET 10 运行时；自包含发布产物不需要。

脚本必须保持 UTF-8 带 BOM：Windows PowerShell 5.1 读取无 BOM 文件按系统代码页解析，中文字面量会损坏。

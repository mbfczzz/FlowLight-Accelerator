# FlowLight Accelerator · 流光加速器

独立的游戏加速平台首版工程。控制服务与节点 Agent 使用 **Go**；Windows 客户端与本地服务使用 **C# / .NET 10 / WPF**；管理后台使用 **React / TypeScript / Vite**。

客户端深色，管理后台浅色。首批游戏为 **VALORANT、英雄联盟**。内置规则是待核验草稿，未配置区服与真实节点时不会生成可用线路。

## 工程组成

| 目录 | 内容 |
|---|---|
| apps/control-api | 账号、权限、设备、游戏规则、线路发布、会话、节点任务 |
| apps/admin-web | 管理后台：深色侧栏＋浅色工作区，与客户端同一套流光标志与琥珀色；上线准备清单、资源列表、侧边编辑抽屉、游戏封面上传、发布、排空、版本切换、审计 |
| apps/windows-client | 深色 WPF：登录、游戏库、选线、开始/停止、托盘、诊断、检查并安装更新 |
| apps/windows-service | Windows Service：命名管道、会话与内核生命周期 |
| agents/node-agent | 一次性注册、mTLS、任务领取、真实转发进程、租约回收 |
| apps/probe-server | 受控 TCP / UDP 探测端点 |
| packages/contracts | Go 配置与任务契约 |
| packages/windows-core | C# API、IPC、Mihomo 配置与 TCP / UDP 探测 |
| packages/api-contracts | OpenAPI 契约 |
| deploy | 容器、systemd、节点配置、监控示例 |

## 运行链路

1. 配置游戏区服，发布不可变规则版本。
2. 登记节点，生成一次性引导凭据；Agent 用自己的 P-256 密钥申请节点证书。
3. 配置端点与有序线路。托管 SS2022 落地和 Realm 中转使用独立会话端口。
4. 发布线路创建验证会话。真实进程执行与探测成功后才更新已发布版本，失败保留旧版本。
5. 创建用户会话时，在 SQL 事务内检查幂等键、权限、设备并发、线路容量和落地所有权。
6. 节点准备完成后，本地服务验证内核 SHA256、测试配置、启动 TUN 并探测回包。
7. 运行中续租和复测；停止时撤销节点进程并回收端口。

MySQL 保存权威状态，开发可用 SQLite。首版不依赖 Redis：原先的租约辅助索引只写不读，已移除；多实例部署需要共享登录限流时再引入。

## 本地开发

需要 Go 1.26、Node.js 24、.NET 10 SDK。

    .\scripts\dev.ps1 -AdminPassword 'replace-with-your-own-long-password'

另一个终端：

    cd apps/admin-web
    npm ci
    npm run dev

后台为 http://localhost:5173，API 为 127.0.0.1:8090。首次账号为 admin，密码取启动配置（至少 14 个字符）；已有账号不会被启动参数覆盖，之后在后台“用户与权限”里改密码。开发数据库、加密密钥和节点 CA 位于 runtime/control。

## 发布与镜像

推送到 main 或打 v* 标签时，`.github/workflows/release.yml` 会：

- 在 Windows 构建自包含的客户端与本地服务，按 installer/kernel.json 下载并双重校验固定版本的 Mihomo 内核，用 Inno Setup 生成 `FlowLight-Accelerator-Setup-<版本>.exe`。main 分支构建作为工作流产物（Actions 页面下载）；v* 标签构建附加到 GitHub Release。
- 构建并推送三个镜像到 GitHub Packages：`ghcr.io/mbfczzz/flowlight-control`、`flowlight-admin`、`flowlight-probe`。main 分支为 `latest`，标签为版本号。

镜像默认私有：在仓库 Packages 设置里改为 public，或在服务器上 `docker login ghcr.io`（PAT 需 read:packages）。

安装程序固定安装到 `C:\Program Files\FlowLight Accelerator`（客户端只信任这个路径下的服务）。控制服务地址已内置，用户无需填写：工作流默认写入 `https://accel.tcmzhan.com:8443`，在仓库变量里设置 `CONTROL_URL` 可覆盖（换域名时不必改代码）。安装向导是与客户端一致的深色界面：首屏一个“立即安装 / 立即升级”按钮和“创建桌面快捷方式”，需要改成自建服务时点“自定义服务器地址”；静默安装用 `Setup.exe /VERYSILENT`，需要指定别的地址时加 `/CONTROLURL=https://...`。升级时直接运行新版安装程序，它会先停止服务并关闭正在运行的客户端和内核，再替换文件；加速服务没能启动时，完成页会写明原因（多为安全软件拦截）和处理办法。编译需要 Inno Setup 6.7 或更高版本；`ISCC /DPreview ...` 可编出无需管理员权限、只装到 `%LOCALAPPDATA%\FlowLightInstallerPreview` 的预览版用来检查界面。卸载会停止并删除服务及 ProgramData 下的配置。安装程序未签名，SmartScreen 会提示“未知发布者”，正式分发前需要代码签名。

### 客户端自动更新（v0.1.6 起）

1. 打 v* 标签，等 release 工作流完成。GitHub Release 的说明里会写出安装包文件名和 SHA256。
2. 在管理后台“客户端版本”里新增：版本号（如 0.1.6）、下载地址（Release 页安装包的链接）、SHA256，打开“发布此版本”。需要强制升级时再打开“必须更新”。
3. 客户端在连上本地服务时和之后每 4 小时检查一次，也可以在头像菜单里点“检查更新”。有新版本时游戏库顶部出现提示，点“立即更新”后下载并核对 SHA256，确认管理员权限后静默安装，装完自动打开新客户端；控制服务地址沿用原来的。

检查接口 `/api/client/v1/releases/latest` 不需要登录，旧到无法登录的客户端也能找到更新。v0.1.5 及更早的客户端没有这个功能，需要手动装一次 v0.1.6。下载地址必须是 https；信任来自控制服务和 SHA256 校验，安装包本身没有代码签名。

## 服务器部署（host 网络，复用已有 MySQL）

`deploy/server/docker-compose.yml` 是面向生产服务器的 compose：三个容器都用 host 网络，连接本机 MySQL，不使用 Redis。按文件头部注释建库、放证书、开防火墙、填值，然后：

    mkdir -p /opt/flowlight && cp deploy/server/docker-compose.yml /opt/flowlight/
    cd /opt/flowlight && docker compose up -d

端口约定：控制服务 TLS 监听 `0.0.0.0:8443`（节点 Agent 和 Windows 客户端直连，节点要做 mTLS，所以不能放在外层反向代理后面）；管理后台 nginx 只监听 `127.0.0.1:8088`，由你现有的反向代理把后台域名转发过来；探测端点 `9443/tcp`、`9081/udp`。证书用 `deploy/server/issue-certs.sh` 签发：每个域名一张，存放在 `/root/acc/cert/<域名>/`，已授权给容器内的 uid 10001，续期钩子会刷新副本、重启 control-api 与 probe 并重载 nginx。

Windows 客户端安装时填写的控制服务地址即 `https://控制服务域名:8443`。

## 容器部署（开发 / 单机）

复制 .env.example 为 .env，设置独立的数据库密码、管理员密码和 32 字节 base64 加密密钥。

    docker compose up -d --build

基础 compose 只在回环地址开放接口。生产环境使用真实 TLS 证书并合并覆盖文件：

    docker compose -f docker-compose.yml -f deploy/docker-compose.production.yml up -d --build

从仓库根目录 tls/ 挂载证书。CONTROL_HOST 为证书域名，ADMIN_ORIGIN 为实际后台 HTTPS 地址；后台放在自己的 HTTPS 入口后。节点直接连接控制服务 TLS 监听端口，保留客户端证书。

生产模式要求 TLS 和节点 mTLS。MySQL、ENCRYPTION_KEY 与控制服务 /data/pki 必须一起备份。

其他控制服务环境变量：

- TRUSTED_PROXIES：可信反向代理网段（逗号分隔 CIDR）。只有来自这些地址的 X-Real-IP 会被当作真实客户端地址，用于登录限流。compose 中固定为后台 nginx 容器地址。后台若还放在另一层 HTTPS 入口后，需在 nginx 中用 set_real_ip_from 配置该入口，否则所有后台登录共用一个限流额度。
- METRICS_TOKEN：Prometheus 抓取 /metrics 用的长期只读凭据，见 deploy/prometheus.example.yml。

登录限流：每个客户端地址每分钟 30 次；每个账号 15 分钟内失败 10 次后暂停该账号登录，成功登录清零。接口错误中，校验问题返回 400 和中文原因（说明是哪一项、规则是什么），数据库等内部错误统一返回 500，详情只写入服务日志；节点 Agent 与健康检查接口的错误保持英文，供日志排查。账号密码：玩家至少 8 位，管理与运维账号至少 14 位，最长 72 字节；把玩家改为管理或运维角色时必须同时设置新密码；改密码后该账号已登录的设备需要重新登录。账号名会去掉首尾空格，不能包含空格，最长 100 个字符。后台表单在提交前按同样的规则检查；列表里的操作人、用户、节点、端点、线路和游戏显示名称而不是编号，搜索也匹配显示出来的中文状态和名称。管理列表接口支持 limit（默认 200，最大 1000）与 offset 分页。

## 节点部署

一键接入：在后台“节点服务器”里添加服务器，点这一行的“接入命令”，把弹窗里的命令用 root 在这台服务器上运行：

    curl -fsSL https://<控制服务>/dist/node/install.sh | sudo bash -s -- --control https://<控制服务> --node <节点标识> --secret <一次性凭据>

脚本由控制服务提供，节点程序也从控制服务下载（控制服务镜像构建时编译 Agent，并按 deploy/node/fetch-tools.sh 下载固定版本、校验过 SHA256 的 shadowsocks-rust 与 Realm，x86_64 和 arm64 各一份），所以节点服务器只需要能访问控制服务的 8443 端口。脚本会：校验下载的文件、创建 flowlight-node 账号、安装到 /opt/flowlight/bin、写 /etc/flowlight/node.json（含各程序的 SHA256）和 systemd 服务、启动并等待注册成功；失败时打印最近的日志和常见原因。命令里的凭据 15 分钟内有效、只能用一次。之后用不带 --secret 的同一命令再运行一次即可更新节点程序，注册信息保留；要把服务器重新注册（例如换成另一个节点），重新生成接入命令并在末尾加 --reenroll。控制服务需要设置 PUBLIC_URL（对外的 https 地址），否则弹窗里要手动填一次控制服务地址。CI 的 node-install 任务在带 systemd 的 Ubuntu 上实际运行这条命令，确认节点注册并发出心跳，再以不带凭据的方式运行一次验证升级。

手动部署：

    go build -trimpath -o flowlight-node ./agents/node-agent

将 Agent 与校验过的 ssserver、sslocal、realm 放入 /opt/flowlight/bin/。参考 deploy/node-agent.example.json，填写各可执行文件的 SHA256，把配置写入仅节点服务账号可读的 /etc/flowlight/node.json。

使用 deploy/flowlight-node.service 运行。systemd 的 control-group 和 Linux 父进程死亡信号防止 Agent 退出后遗留转发进程。托管监听使用 1024 以上端口，Agent 可运行在独立非 root 账号下。

节点配置中的 control_ca 会把控制服务固定到该 CA，不再信任系统根证书；控制服务使用公共 CA 证书时不要设置它。

引导凭据有效期 15 分钟，注册后失效。节点证书和令牌保存于其数据目录。证书有效期 3 个月，Agent 在剩余 30 天内自动换新密钥续期，新证书首次使用后旧证书失效。证书已过期的节点无法续期，需要重新注册：备份并移开旧 identity.json，在后台生成新的引导凭据。

转发进程意外退出时，Agent 按退避重启 3 次；仍失败则上报 runtime_failed，控制服务停止对应会话。Agent 正常重启会保留运行记录，租约仍有效的会话在下一次心跳后恢复。Realm 中转节点不接收会话密钥。

## 配置第一条线路

- 为 VALORANT 或英雄联盟填写实际区服标识，核对进程、路径后发布规则。
- 注册节点并等待心跳，建立 SS2022 落地端点及端口池。
- 需要中转时添加 Realm 端点，按入口到落地顺序组合线路。
- 部署受控探测端点，配置 PROBE_URL、PROBE_UDP。HTTP 探测返回实际对端 ip；UDP 回传带随机数的小包。
  节点验证由入口节点自己连接自己的公网入口，再经整条线路到探测端点，测的是“入口到落地再到探测端点”这一段，不含玩家到入口的距离；玩家侧延迟以客户端探测为准。入口节点所在网络不支持回环访问自身公网地址（hairpin NAT）时验证会失败；设置 PROBE_NODE_ID 后，所有线路改由该在线 Agent 发起验证。
  延迟取一条已建立连接上 3 次请求的中位数（首个握手请求不计时）；UDP 发 5 个包，至少回来 4 个才算通过，并上报丢包率。
  探测服务优先用 -tls-cert / -tls-key 直接提供 HTTPS。若放在反向代理后，必须用 -trusted-proxies 指定代理网段，并由代理设置 X-Forwarded-For，否则读到的是代理地址，固定出口线路无法通过验证。
- 发布线路并查看任务。完整路径的 TCP / UDP 探测通过后才对客户端开放。

导入线路默认关闭。设置 ALLOW_IMPORTED=true 可使用 imported 或 hybrid 模式；纯导入线路还需要 PROBE_NODE_ID 指向在线探测 Agent。外部静态凭据不能由本平台撤销，相关会话标记 external-credentials。混合线路可撤销平台入口的转发进程。

用户可通过 API 登记自带 SS2022 落地，运营人员为其配置只允许该用户的线路并完成验证。独享与固定 IP 分别配置。

## Windows

终端用户使用 Release 里的安装程序。手动安装（开发机、未签名环境）：

    .\scripts\build-windows.ps1
    .\installer\fetch-kernel.ps1

管理员终端安装本地服务，核心文件必须来自核验过的发布包：

    .\scripts\install-windows-service.ps1 -ControlUrl 'https://control.example.com:8443' -MihomoPath '.\artifacts\windows\core\mihomo.exe' -MihomoSha256 (Get-Content .\installer\kernel.json | ConvertFrom-Json).exe_sha256

服务注册、数据目录 ACL 和卸载都由 installer/configure-service.ps1 处理，安装程序和手动脚本共用同一份逻辑。固定安装于 Program Files 下的 FlowLight Accelerator。客户端验证命名管道对端进程位置；本地服务按 Windows SID 隔离账号，服务配置目录仅 SYSTEM 和管理员可读写。

客户端行为：连上本地服务但未登录时自动弹出登录框，记住上次账号（只需输密码）和上次加速的游戏、区服、线路（保存在 %LocalAppData%\FlowLightAccelerator\client.json，不含密码）；重复启动只会唤醒已有窗口；点关闭缩到托盘并提示一次，托盘菜单可直接停止加速；客户端重启或第二次打开时按本地服务上报的会话恢复选中项；一个按钮在"开始加速"与"停止加速"之间切换，状态行显示正在分配线路/正在连接/已连接，延迟按 ≤60/≤120 ms 着色。登录框注明账号由运营方开通，客户端不提供自助注册。界面参考 UU、雷神这类加速器，图文为主：左侧图标导航（游戏库、加速、账号头像）；游戏库是封面卡片（每款游戏按名称生成专属配色与流光图案，不附带任何游戏官方美术），上次加速的游戏排第一并标注；进入游戏后是横幅封面、区服卡片（区服代码色块）、线路列表（信号格、历史检测延迟、负载高低、推荐标记，不可用线路置灰不可选）和一个圆形加速按钮；连接中按钮外圈有流光转动，连上后按钮显示当前延迟，左侧换成实时面板：延迟、抖动、丢包、已加速时长，以及本次会话每次隧道检测的延迟走势图（约 30 秒一次）和出口 IP。加速期间游戏库顶部显示正在加速的横幅，其他游戏变暗，点击会提示先停止。账号切换、退出登录、复制连接诊断和退出程序收在头像菜单；短暂反馈在右下角提示；未登录、本地服务未运行、游戏列表为空等情况在中间配图说明并给出下一步。控制服务与本地服务的错误原因（账号密码错误、线路满员、节点离线、会话被运营方结束等）都以中文显示，未知原因原样透传，完整信息可在“连接诊断”里复制。

客户端通过服务控制管理器校验命名管道对端就是注册在 Program Files 下的 FlowLightAccelerator 服务，普通用户权限即可完成校验。

开发调试可用 FLOWLIGHT_SERVICE_CONFIG 指向独立配置。allow_local_http 仅放行回环地址；调试客户端用 FLOWLIGHT_DEV_PIPE=1 跳过服务身份校验，再加 FLOWLIGHT_PIPE_NAME 可让客户端连接另一个管道名的替身服务（真服务不必停），生产安装无需这些变量。

关闭窗口进入托盘，加速继续；“退出流光加速器”确认会话已停止后才退出。拥有会话的 Windows 用户注销后，本地服务在 10 秒内停止该会话，其他用户随后可以使用。诊断不包含密码与隧道密钥。内核、驱动、正式签名证书不随源码分发。

内核最近 8 KB 输出附在启动失败等错误信息中（隧道密码已遮蔽），并写入数据目录下的 kernel.log（256 KB 轮换一次）。内核监听就绪最多等待 20 秒。准备阶段的控制服务短暂故障会自动重试，直到 3 分钟准备期结束；请求超时按失败报告，不再被当作用户主动停止。

## 验证

    go test ./...
    go vet ./...
    npm ci --prefix apps/admin-web
    npm run build --prefix apps/admin-web
    dotnet run --project tests/windows-core/FlowLight.Core.Checks.csproj -c Release
    dotnet build apps/windows-service/FlowLight.Service.csproj -c Release
    dotnet build apps/windows-client/FlowLight.Client.csproj -c Release

设置 FLOWLIGHT_NATIVE_TEST_DIR 可运行真实 SS2022 + Realm 测试，验证 TCP、UDP、出口与过期入口关闭。测试不开启 TUN，不修改本机路由或 DNS。

当前交付不等于 VALORANT / LOL 已通过真实对局验收，须按 [验收清单](docs/acceptance.md) 测试实际线路、反作弊共存、休眠与切网。当前托管进程监听 IPv4，不支持的能力会阻止发布。

分钟计费、支付、无感切线、自研 WFP 驱动、游戏专属测延和自动安装更新属于后续阶段。客户端版本清单提供发布与查询接口，首版通过构建、安装脚本交付。

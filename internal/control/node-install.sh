#!/usr/bin/env bash
# FlowLight node installer, served by the control service at
# /dist/node/install.sh together with the node programs. The admin's 接入命令
# dialog shows the line that runs it on a node server:
#
#   curl -fsSL https://<control>/dist/node/install.sh | sudo bash -s -- \
#     --control https://<control> --node <node id> --secret <one-time secret>
#
# It downloads the agent, ssserver, sslocal and realm for this CPU, checks
# them, creates the flowlight-node account, writes /etc/flowlight/node.json
# and the systemd unit, starts the agent and waits for it to register.
# Running it again upgrades the programs and keeps the registration; add
# --reenroll (with a new secret) to register the server again.
set -euo pipefail

CONTROL="" NODE="" SECRET="" DEV_HTTP=0 REENROLL=0
BIN=/opt/flowlight/bin
ETC=/etc/flowlight
STATE=/var/lib/flowlight-node
UNIT=/etc/systemd/system/flowlight-node.service

say() { printf '\033[1;33m==>\033[0m %s\n' "$*"; }
ok() { printf '\033[1;32m完成：\033[0m%s\n' "$*"; }
fail() { printf '\033[1;31m失败：\033[0m%s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --control) CONTROL="${2:-}"; shift 2 ;;
    --node) NODE="${2:-}"; shift 2 ;;
    --secret) SECRET="${2:-}"; shift 2 ;;
    --reenroll) REENROLL=1; shift ;;
    # Testing only: a control service on this machine over plain HTTP.
    --dev-http) DEV_HTTP=1; shift ;;
    *) fail "不认识的参数：$1" ;;
  esac
done

[ "$(id -u)" = 0 ] || fail "请用 root 运行（在 bash 前加 sudo）"
command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ] || fail "这台服务器没有使用 systemd，无法安装节点服务"
for c in curl sha256sum install useradd; do
  command -v "$c" >/dev/null 2>&1 || fail "缺少命令 $c，请先安装后重试"
done
case "$(uname -m)" in
  x86_64 | amd64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *) fail "不支持的 CPU 架构 $(uname -m)，节点只支持 x86_64 和 arm64" ;;
esac

CONTROL="${CONTROL%/}"
case "$CONTROL" in
  https://?*) ;;
  http://127.0.0.1:* | http://localhost:*) [ "$DEV_HTTP" = 1 ] || fail "控制服务地址必须以 https:// 开头" ;;
  "") fail "缺少 --control（控制服务地址）" ;;
  *) fail "控制服务地址必须以 https:// 开头：$CONTROL" ;;
esac
[[ "$NODE" =~ ^[A-Za-z0-9_-]{1,64}$ ]] || fail "缺少或无效的 --node（节点标识，在后台复制）"
[ -z "$SECRET" ] || [[ "$SECRET" =~ ^[A-Za-z0-9_+/=-]{16,256}$ ]] || fail "无效的 --secret（一次性引导凭据）"

# A server that registered before keeps its identity unless told otherwise.
registered=0
if [ -f "$STATE/identity.json" ] && [ "$REENROLL" = 0 ]; then
  previous=$(sed -n 's/.*"node_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ETC/node.json" 2>/dev/null | head -n 1)
  if [ -n "$previous" ] && [ "$previous" != "$NODE" ]; then
    fail "这台服务器已注册为节点 $previous。要改成 $NODE，请在后台为它生成新的接入命令，并在命令末尾加 --reenroll"
  fi
  registered=1
fi
[ "$registered" = 1 ] || [ -n "$SECRET" ] || fail "缺少 --secret（一次性引导凭据）"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
base="$CONTROL/dist/node/$ARCH"
say "从 $CONTROL 下载节点程序（$ARCH）"
for f in SHA256SUMS flowlight-node ssserver sslocal realm; do
  curl -fsS --retry 3 --connect-timeout 10 -o "$tmp/$f" "$base/$f" \
    || fail "无法下载 $f。请确认这台服务器能访问 $CONTROL（防火墙、安全组放行该端口），然后重试"
done
(cd "$tmp" && sha256sum -c --quiet SHA256SUMS) || fail "下载的文件校验不一致，请重试"

say "创建运行账号和目录"
if ! id flowlight-node >/dev/null 2>&1; then
  getent group flowlight-node >/dev/null 2>&1 || groupadd --system flowlight-node
  nologin=$(command -v nologin || echo /usr/sbin/nologin)
  useradd --system --gid flowlight-node --no-create-home --home-dir "$STATE" --shell "$nologin" flowlight-node
fi
install -d -m 755 "$BIN" "$ETC"

if systemctl is-active --quiet flowlight-node; then
  say "停止正在运行的节点程序"
  systemctl stop flowlight-node
fi
for f in flowlight-node ssserver sslocal realm; do
  install -m 755 "$tmp/$f" "$BIN/$f"
done
sum() { sha256sum "$BIN/$1" | cut -d ' ' -f 1; }

if [ "$REENROLL" = 1 ] && [ -f "$STATE/identity.json" ]; then
  backup="$STATE/identity.json.$(date +%Y%m%d%H%M%S)"
  mv "$STATE/identity.json" "$backup"
  say "已把旧的注册信息移到 $backup"
fi

say "写入配置"
secret_line=""
[ "$registered" = 1 ] || secret_line="\"bootstrap_secret\": \"$SECRET\","
dev_flag=false
[ "$DEV_HTTP" = 1 ] && dev_flag=true
umask 027
cat > "$ETC/node.json.new" <<EOF
{
 "control_url": "$CONTROL",
 "node_id": "$NODE",
 $secret_line
 "data_dir": "$STATE",
 "allow_http_local_dev": $dev_flag,
 "binaries": {
  "ssserver": "$BIN/ssserver",
  "sslocal": "$BIN/sslocal",
  "realm": "$BIN/realm",
  "sha256": {
   "ss2022": "$(sum ssserver)",
   "sslocal": "$(sum sslocal)",
   "realm": "$(sum realm)"
  }
 }
}
EOF
chown root:flowlight-node "$ETC/node.json.new"
chmod 640 "$ETC/node.json.new"
mv "$ETC/node.json.new" "$ETC/node.json"
umask 022

# The same unit as deploy/flowlight-node.service.
cat > "$UNIT" <<'EOF'
[Unit]
Description=FlowLight Accelerator Node Agent
Wants=network-online.target
After=network-online.target
[Service]
Type=simple
User=flowlight-node
Group=flowlight-node
ExecStart=/opt/flowlight/bin/flowlight-node -config /etc/flowlight/node.json
Restart=on-failure
RestartSec=3
KillMode=control-group
TimeoutStopSec=15
StateDirectory=flowlight-node
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/flowlight-node
LimitNOFILE=65536
[Install]
WantedBy=multi-user.target
EOF

say "启动节点程序"
systemctl daemon-reload
systemctl enable flowlight-node >/dev/null 2>&1
systemctl restart flowlight-node

# Registered means the agent wrote its identity and is still running a
# few seconds later (a refused secret makes it exit and restart).
up=0
for _ in $(seq 1 30); do
  if [ -f "$STATE/identity.json" ] && systemctl is-active --quiet flowlight-node; then
    sleep 3
    systemctl is-active --quiet flowlight-node && up=1
    break
  fi
  sleep 1
done
if [ "$up" != 1 ]; then
  journalctl -u flowlight-node -n 15 --no-pager >&2 || true
  fail "节点程序没有注册成功。常见原因：一次性凭据已过期或已用过（15 分钟有效，请在后台重新生成接入命令）；这台服务器连不上控制服务。上面是最近的日志"
fi

if [ "$registered" = 1 ]; then
  ok "节点程序已更新并重新启动，注册信息保持不变"
else
  ok "节点 $NODE 已注册并启动，开机会自动运行"
fi
cat <<EOF
  回到后台“节点服务器”，这台服务器的心跳会在几秒内变成“刚刚”。
  下一步在“服务端点”里为它开 SS2022 落地或 Realm 中转，并在云安全组和系统防火墙里
  放行端点的端口范围（TCP 和 UDP 都要放行）。
  查看状态：systemctl status flowlight-node    查看日志：journalctl -u flowlight-node -f
EOF

#!/usr/bin/env bash
# 为 FlowLight Accelerator 的域名签发 Let's Encrypt 证书，并按
#   <CERT_ROOT>/<域名>/fullchain.pem
#   <CERT_ROOT>/<域名>/privkey.pem
# 的布局存放。续期后自动刷新这些副本，并重启用到证书的容器、重载 nginx。
#
# 在服务器上以 root 运行，先改下面的变量（或通过环境变量传入）：
#   CONTROL_DOMAIN=accel.example.com ADMIN_DOMAIN=admin.example.com EMAIL=me@example.com bash issue-certs.sh
#
# 前提：域名已解析到本机，80 端口公网可达，nginx 正在运行（HTTP-01 验证经由 nginx）。
set -euo pipefail

CONTROL_DOMAIN="${CONTROL_DOMAIN:-accel.example.com}"      # 控制服务：客户端与节点连接 :8443
ADMIN_DOMAIN="${ADMIN_DOMAIN:-accel-admin.example.com}"    # 管理后台：你的 nginx 转发到 127.0.0.1:8088
PROBE_DOMAIN="${PROBE_DOMAIN:-$CONTROL_DOMAIN}"             # 探测端点；与控制服务同域名时保持默认
EMAIL="${EMAIL:-admin@example.com}"                         # Let's Encrypt 到期提醒邮箱
CERT_ROOT="${CERT_ROOT:-/root/acc/cert}"
COMPOSE_DIR="${COMPOSE_DIR:-/opt/flowlight}"
CONTAINER_UID=10001                                          # 容器内进程的 uid，证书副本要对它可读

[ "$(id -u)" -eq 0 ] || { echo "请以 root 运行" >&2; exit 1; }
for name in CONTROL_DOMAIN ADMIN_DOMAIN EMAIL; do
  case "${!name}" in *example.com*) echo "请先设置 $name" >&2; exit 1;; esac
done

if ! command -v certbot >/dev/null 2>&1; then
  if command -v apt-get >/dev/null 2>&1; then apt-get update -q && apt-get install -y -q certbot python3-certbot-nginx
  elif command -v dnf >/dev/null 2>&1; then dnf install -y certbot python3-certbot-nginx
  elif command -v yum >/dev/null 2>&1; then yum install -y certbot python3-certbot-nginx
  else echo "请先安装 certbot 和 nginx 插件" >&2; exit 1
  fi
fi

# 续期钩子：certbot 每次签发/续期后调用，RENEWED_LINEAGE=/etc/letsencrypt/live/<域名>
HOOK=/etc/letsencrypt/renewal-hooks/deploy/acc-cert.sh
mkdir -p "$(dirname "$HOOK")"
cat > "$HOOK" <<EOF
#!/bin/sh
# 由 issue-certs.sh 安装。把续期后的证书复制到 $CERT_ROOT/<域名>/ 并重启相关服务。
set -eu
domain=\$(basename "\$RENEWED_LINEAGE")
target="$CERT_ROOT/\$domain"
mkdir -p "\$target"
install -o $CONTAINER_UID -g $CONTAINER_UID -m 0644 "\$RENEWED_LINEAGE/fullchain.pem" "\$target/fullchain.pem"
install -o $CONTAINER_UID -g $CONTAINER_UID -m 0640 "\$RENEWED_LINEAGE/privkey.pem" "\$target/privkey.pem"
case "\$domain" in
  $CONTROL_DOMAIN|$PROBE_DOMAIN)
    if [ -f "$COMPOSE_DIR/docker-compose.yml" ]; then
      docker compose -f "$COMPOSE_DIR/docker-compose.yml" restart control-api probe || true
    fi ;;
esac
if nginx -t >/dev/null 2>&1; then nginx -s reload || true; fi
EOF
chmod +x "$HOOK"

for domain in $(printf '%s\n' "$CONTROL_DOMAIN" "$ADMIN_DOMAIN" "$PROBE_DOMAIN" | sort -u); do
  echo "==> 签发 $domain"
  certbot certonly --nginx -d "$domain" --non-interactive --agree-tos -m "$EMAIL" --keep-until-expiring
  RENEWED_LINEAGE="/etc/letsencrypt/live/$domain" sh "$HOOK"
done

echo
echo "证书已就位（续期由 certbot 定时任务自动完成）："
find "$CERT_ROOT" -maxdepth 2 -name '*.pem' -printf '%M %u:%g %p\n' | sort

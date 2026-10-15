#!/bin/sh
# Puts the forwarding programs a node runs into <out>/<arch>/: ssserver and
# sslocal (shadowsocks-rust) and realm, statically linked (musl) so they run
# on any Linux. Versions and archive checksums are pinned; a download that
# does not match stops the build. Used by deploy/control.Dockerfile (the
# control service hands these to nodes) and by CI.
#
#   sh deploy/node/fetch-tools.sh <out>
set -eu
out=${1:?usage: fetch-tools.sh <out>}
SS=v1.25.0
REALM=v2.9.6
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fetch() { # url sha256 file
  curl -fsSL --retry 3 -o "$tmp/$3" "$1"
  echo "$2  $tmp/$3" | sha256sum -c -
}

for arch in amd64 arm64; do
  case $arch in
    amd64)
      triple=x86_64-unknown-linux-musl
      ss_sum=8439bf43c324b0fc273e663d0b1f8926fd8f666cbd1e0fd59b35096f0e778e92
      realm_sum=b1cc335547bea8bb2a88178bef12ec7f2363e36200e7ea1d4e1e67627929bf65 ;;
    arm64)
      triple=aarch64-unknown-linux-musl
      ss_sum=2c24915b56a05effdab5898848bacfbcb625bc277b18febafc7a3aa45f87f3b0
      realm_sum=f4c0318dd86854da483dcb7645b4f39cae2cc3f91c688fef969d53220b949488 ;;
  esac
  mkdir -p "$out/$arch"
  fetch "https://github.com/shadowsocks/shadowsocks-rust/releases/download/$SS/shadowsocks-$SS.$triple.tar.xz" "$ss_sum" ss.tar.xz
  tar -xJf "$tmp/ss.tar.xz" -C "$out/$arch" ssserver sslocal
  fetch "https://github.com/zhboner/realm/releases/download/$REALM/realm-$triple.tar.gz" "$realm_sum" realm.tar.gz
  tar -xzf "$tmp/realm.tar.gz" -C "$out/$arch" realm
  chmod 755 "$out/$arch/ssserver" "$out/$arch/sslocal" "$out/$arch/realm"
done
echo "node tools in $out: shadowsocks-rust $SS, realm $REALM"

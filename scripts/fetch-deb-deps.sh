#!/usr/bin/env bash
# 预下载 .deb 内捆绑的第三方资产（goreleaser nfpm 打包前执行）：
#   dist/deps/<arch>/mihomo            mihomo 内核（MIT，随包附 LICENSE）
#   dist/deps/rules/geosite.dat        域名规则（MetaCubeX/meta-rules-dat，GPL-3.0）
#   dist/deps/rules/country.mmdb       IP 库（DB-IP Country Lite，CC BY 4.0，可再分发）
#   dist/deps/LICENSE.mihomo           mihomo 上游许可证
#
# 刻意不捆绑 geoip.metadb（GeoLite2 衍生物，MaxMind EULA 含 30 天新鲜度义务，
# 不适合冻结进安装包；运行时由用户在线更新则不受影响）。
set -euo pipefail

ARCHES=(${DEB_ARCHES:-amd64 arm64 armv7})
DEPS=packaging/deps
mkdir -p "${DEPS}/rules"

fetch() { curl -fL --retry 3 --retry-delay 2 -o "$2" "$1"; }

echo "==> mihomo 最新版本号"
TAG="$(curl -fsSL https://api.github.com/repos/MetaCubeX/mihomo/releases/latest | grep -oE '"tag_name":\s*"[^"]+"' | cut -d'"' -f4)"
echo "    ${TAG}"

for arch in "${ARCHES[@]}"; do
  mkdir -p "${DEPS}/${arch}"
  if [[ -x "${DEPS}/${arch}/mihomo" ]]; then
    echo "==> ${arch}: 已存在，跳过"
    continue
  fi
  asset_arch="${arch}"
  if [[ "${arch}" == "arm" ]]; then
    asset_arch="armv7"
  fi
  echo "==> 下载 mihomo ${TAG} (${arch})"
  fetch "https://github.com/MetaCubeX/mihomo/releases/download/${TAG}/mihomo-linux-${asset_arch}-${TAG}.gz" \
    "${DEPS}/${arch}/mihomo.gz"
  gunzip -f "${DEPS}/${arch}/mihomo.gz"
  chmod 0755 "${DEPS}/${arch}/mihomo"
done

if [[ ! -s "${DEPS}/rules/geosite.dat" ]]; then
  echo "==> 下载 geosite.dat"
  fetch "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat" \
    "${DEPS}/rules/geosite.dat"
fi

if [[ ! -s "${DEPS}/rules/country.mmdb" ]]; then
  echo "==> 下载 DB-IP Country Lite（CC BY 4.0）"
  ok=""
  for offset in 0 1 2; do
    ym="$(date -u -d "-${offset} month" +%Y-%m)"
    url="https://download.db-ip.com/free/dbip-country-lite-${ym}.mmdb.gz"
    if fetch "${url}" "${DEPS}/rules/country.mmdb.gz" 2>/dev/null; then
      gunzip -f "${DEPS}/rules/country.mmdb.gz"
      echo "    使用 ${ym} 版本"
      ok=1
      break
    fi
  done
  [[ -n "${ok}" ]] || { echo "DB-IP Country Lite 下载失败"; exit 1; }
fi

if [[ ! -s "${DEPS}/LICENSE.mihomo" ]]; then
  echo "==> 下载 mihomo LICENSE"
  fetch "https://raw.githubusercontent.com/MetaCubeX/mihomo/Meta/LICENSE" "${DEPS}/LICENSE.mihomo"
fi

echo "==> 完成"
ls -lh "${DEPS}"/*/mihomo "${DEPS}/rules/" | sed 's/^/    /'

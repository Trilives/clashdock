#!/usr/bin/env bash
# clashdock 便携包「离线友好」更新脚本（无需 root，只改解压目录内的文件）。
#
# 便携模式面向拿不到 root 的环境（如实验室机房）：本脚本刻意不碰任何系统路径，
# 只就地更新便携包解压目录里的三样东西——
#   1. clashdock 本体（TUI 二进制，解压目录根下的 ./clashdock）
#   2. mihomo 内核（deps/mihomo）
#   3. 基础规则集（deps/rules/geosite.dat + deps/rules/country.mmdb）
#
# 下载源与 .deb/便携包打包脚本一致（scripts/fetch-deb-deps.sh、internal/selfupdate）：
#   clashdock  -> GitHub Trilives/clashdock 发行版（校验 checksums.txt 的 SHA-256）
#   mihomo     -> GitHub MetaCubeX/mihomo 发行版
#   geosite    -> GitHub MetaCubeX/meta-rules-dat（latest）
#   country    -> DB-IP Country Lite（CC BY 4.0）
#
# 刻意不下载 Web UI：便携包本就不含面板，要图形界面请用完整版（install.sh）或在线面板。
#
# 可选环境变量：
#   GITHUB_MIRROR  给 github.com / raw.githubusercontent.com 链接套加速前缀（不套 API）
#   GITHUB_TOKEN   访问 GitHub API 的令牌（提高限流额度，可选）
#   DOWNLOAD_PROXY / https_proxy  走代理下载（curl 原生识别 https_proxy）
#
# 用法：
#   ./tool/update.sh              # 交互菜单
#   ./tool/update.sh clashdock    # 只更新 TUI 本体
#   ./tool/update.sh kernel       # 只更新内核
#   ./tool/update.sh rules        # 只更新规则集
#   ./tool/update.sh all          # 三者都更新
#   PREVIEW=1 ./tool/update.sh clashdock   # clashdock 跟随预览版渠道
set -euo pipefail

TOOL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$TOOL_DIR/.." && pwd)" # 便携包解压根目录
DEPS_DIR="$ROOT_DIR/deps"
RULES_DIR="$DEPS_DIR/rules"

CLASHDOCK_REPO="Trilives/clashdock"
MIHOMO_REPO="MetaCubeX/mihomo"
GEOSITE_URL="https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat"

GITHUB_MIRROR="${GITHUB_MIRROR:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-${GH_TOKEN:-}}"
if [ -n "${DOWNLOAD_PROXY:-}" ] && [ -z "${https_proxy:-}" ]; then
	export https_proxy="$DOWNLOAD_PROXY" http_proxy="$DOWNLOAD_PROXY"
fi

die() { echo "错误：$*" >&2; exit 1; }
info() { echo "==> $*"; }

command -v curl >/dev/null 2>&1 || die "需要 curl（便携环境请先装 curl）。"
command -v gzip >/dev/null 2>&1 || die "需要 gzip/gunzip。"

# detect_arch 把 uname -m 映射成发行版资产用的架构名（amd64/arm64/armv7）。
detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo amd64 ;;
	aarch64 | arm64) echo arm64 ;;
	armv7l | armv7 | armhf) echo armv7 ;;
	*) die "不支持的架构：$(uname -m)" ;;
	esac
}
ARCH="$(detect_arch)"

# mirror 对 github.com / raw 链接套加速前缀；api.github.com 不套（多数镜像不代理 API）。
mirror() {
	local url="$1"
	if [ -n "$GITHUB_MIRROR" ] && { [[ "$url" == https://github.com/* ]] || [[ "$url" == https://raw.githubusercontent.com/* ]]; }; then
		echo "${GITHUB_MIRROR%/}/$url"
	else
		echo "$url"
	fi
}

# fetch 下载到指定文件（自动套镜像、重试）。
fetch() { curl -fL --retry 3 --retry-delay 2 -o "$2" "$(mirror "$1")"; }

# api_get 拉取 GitHub API JSON（带可选 token，不套镜像）。
api_get() {
	if [ -n "$GITHUB_TOKEN" ]; then
		curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" "$1"
	else
		curl -fsSL "$1"
	fi
}

# json_str 从一段 JSON 里抓第一个 "key": "value" 的 value（够用的轻量解析，免依赖 jq）。
json_str() { grep -oE "\"$2\"[[:space:]]*:[[:space:]]*\"[^\"]+\"" | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]+)".*/\1/'; }

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

# ---- clashdock 本体 ------------------------------------------------------
update_clashdock() {
	local api="https://api.github.com/repos/$CLASHDOCK_REPO/releases/latest"
	[ "${PREVIEW:-}" = "1" ] && api="https://api.github.com/repos/$CLASHDOCK_REPO/releases?per_page=1"
	info "查询 clashdock 最新版本…"
	local tag ver
	tag="$(api_get "$api" | json_str x tag_name)"
	[ -n "$tag" ] || die "未取到 clashdock 版本号（API 限流？可设 GITHUB_TOKEN）。"
	ver="${tag#v}"
	local asset="clashdock_${ver}_linux_${ARCH}.tar.gz"
	local base="https://github.com/$CLASHDOCK_REPO/releases/download/$tag"

	info "下载 $asset"
	fetch "$base/$asset" "$TMP_DIR/$asset"
	info "校验 SHA-256（checksums.txt）"
	fetch "$base/checksums.txt" "$TMP_DIR/checksums.txt"
	local want got
	want="$(awk -v f="$asset" '$2==f{print $1}' "$TMP_DIR/checksums.txt")"
	[ -n "$want" ] || die "checksums.txt 缺少 $asset 的记录。"
	got="$(sha256sum "$TMP_DIR/$asset" | awk '{print $1}')"
	[ "$want" = "$got" ] || die "校验失败：$asset 摘要不匹配（want $want got $got）。"

	tar -xzf "$TMP_DIR/$asset" -C "$TMP_DIR"
	local inner="$TMP_DIR/clashdock_${ver}_linux_${ARCH}/clashdock"
	[ -f "$inner" ] || inner="$(find "$TMP_DIR" -type f -name clashdock ! -path "$TMP_DIR/clashdock" | head -n1)"
	[ -f "$inner" ] || die "压缩包里没找到 clashdock 二进制。"

	chmod 0755 "$inner"
	# 用 rename 覆盖，避免覆盖正在运行的自身时报 ETXTBSY（旧 inode 由运行中进程持有，退出后释放）。
	mv -f "$inner" "$ROOT_DIR/clashdock"
	echo "clashdock 已更新到 $ver（若正在运行，请退出后重新 ./clashdock 生效）。"
}

# ---- mihomo 内核 ---------------------------------------------------------
update_kernel() {
	info "查询 mihomo 最新版本…"
	local tag
	tag="$(api_get "https://api.github.com/repos/$MIHOMO_REPO/releases/latest" | json_str x tag_name)"
	[ -n "$tag" ] || die "未取到 mihomo 版本号（API 限流？可设 GITHUB_TOKEN）。"
	local url="https://github.com/$MIHOMO_REPO/releases/download/$tag/mihomo-linux-${ARCH}-${tag}.gz"
	info "下载 mihomo $tag ($ARCH)"
	fetch "$url" "$TMP_DIR/mihomo.gz"
	gzip -dc "$TMP_DIR/mihomo.gz" >"$TMP_DIR/mihomo"
	chmod 0755 "$TMP_DIR/mihomo"
	mkdir -p "$DEPS_DIR"
	mv -f "$TMP_DIR/mihomo" "$DEPS_DIR/mihomo"
	echo "mihomo 内核已更新到 $tag。重启 clashdock（菜单「重启内核」或退出重进）后生效。"
}

# ---- 规则集 --------------------------------------------------------------
update_rules() {
	mkdir -p "$RULES_DIR"
	info "下载 geosite.dat"
	fetch "$GEOSITE_URL" "$TMP_DIR/geosite.dat"
	mv -f "$TMP_DIR/geosite.dat" "$RULES_DIR/geosite.dat"

	info "下载 country.mmdb（DB-IP Country Lite，尝试近几个月版本）"
	local ok=""
	for offset in 0 1 2; do
		local ym
		ym="$(date -u -d "-${offset} month" +%Y-%m 2>/dev/null || date -u -v-"${offset}"m +%Y-%m)"
		if fetch "https://download.db-ip.com/free/dbip-country-lite-${ym}.mmdb.gz" "$TMP_DIR/country.mmdb.gz" 2>/dev/null; then
			gzip -dc "$TMP_DIR/country.mmdb.gz" >"$TMP_DIR/country.mmdb"
			mv -f "$TMP_DIR/country.mmdb" "$RULES_DIR/country.mmdb"
			echo "    使用 ${ym} 版本"
			ok=1
			break
		fi
	done
	[ -n "$ok" ] || die "country.mmdb 下载失败。"
	echo "规则集已更新。重启 clashdock 后生效。"
}

run_target() {
	case "$1" in
	clashdock | tui | self) update_clashdock ;;
	kernel | mihomo | core) update_kernel ;;
	rules | rule | geo | ruleset) update_rules ;;
	all) update_clashdock; update_kernel; update_rules ;;
	*) die "未知目标：$1（可用 clashdock|kernel|rules|all）" ;;
	esac
}

if [ "$#" -gt 0 ]; then
	run_target "$1"
	exit 0
fi

echo "clashdock 便携更新（架构 $ARCH，仅改解压目录，无需 root）"
echo "  1) 更新 clashdock 本体（TUI 二进制）"
echo "  2) 更新 mihomo 内核"
echo "  3) 更新规则集（geosite + country）"
echo "  4) 全部更新"
echo "  0) 退出"
printf "请选择 [0-4]: "
read -r choice
case "${choice:-0}" in
1) run_target clashdock ;;
2) run_target kernel ;;
3) run_target rules ;;
4) run_target all ;;
0 | "") echo "已取消。" ;;
*) die "无效选择：$choice" ;;
esac

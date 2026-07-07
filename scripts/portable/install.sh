#!/usr/bin/env bash
# clashdock 便携包安装脚本。
#
# 把便携包内的 clashdock 本体 + mihomo 内核 + 基础规则装入系统路径，落点与
# `sudo apt install clashdock_*.deb` 完全一致（见 .goreleaser.yaml 的 nfpm 契约）：
#
#   clashdock            -> /usr/bin/clashdock
#   deps/mihomo          -> /usr/libexec/clashdock/mihomo
#   deps/rules/*.dat|mmdb-> /usr/share/clashdock/ruleset/
#   deps/copyright       -> /usr/share/doc/clashdock/copyright
#   deps/LICENSE.mihomo  -> /usr/share/doc/clashdock/LICENSE.mihomo
#
# 装完即可离线初始化（clashdock 不会在初始化阶段下载内核；内核更新由用户在
# 「运行时管理 → 更新内核」显式触发）。归属与许可见 deps/copyright、deps/LICENSE.mihomo。
set -euo pipefail

# 脚本所在目录即便携包解压根目录。
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# DESTDIR：可选安装前缀（打包/测试用，遵循 GNU 约定）。默认空=装进真实系统路径。
DESTDIR="${DESTDIR:-}"

# 写真实系统路径需要 root（装进 DESTDIR 前缀时不需要）：非 root 时用 sudo 重执行。
if [ -z "$DESTDIR" ] && [ "$(id -u)" -ne 0 ]; then
	if command -v sudo >/dev/null 2>&1; then
		echo "需要 root 权限写入系统路径，正在通过 sudo 重新执行…"
		exec sudo -- "$0" "$@"
	fi
	echo "错误：本脚本需要 root 权限（写入 /usr/bin、/usr/libexec 等）。请用 root 运行或先安装 sudo。" >&2
	exit 1
fi

BIN_SRC="$SCRIPT_DIR/clashdock"
CORE_SRC="$SCRIPT_DIR/deps/mihomo"
RULES_SRC="$SCRIPT_DIR/deps/rules"

# 校验必备文件齐全（便携包可能被裁剪或下载不完整）。
missing=0
for f in "$BIN_SRC" "$CORE_SRC" "$RULES_SRC/geosite.dat" "$RULES_SRC/country.mmdb"; do
	if [ ! -f "$f" ]; then
		echo "错误：缺少文件 $f（便携包不完整？）" >&2
		missing=1
	fi
done
[ "$missing" -eq 0 ] || exit 1

install -D -m 0755 "$BIN_SRC" "$DESTDIR/usr/bin/clashdock"
install -D -m 0755 "$CORE_SRC" "$DESTDIR/usr/libexec/clashdock/mihomo"
install -D -m 0644 "$RULES_SRC/geosite.dat" "$DESTDIR/usr/share/clashdock/ruleset/geosite.dat"
install -D -m 0644 "$RULES_SRC/country.mmdb" "$DESTDIR/usr/share/clashdock/ruleset/country.mmdb"
[ -f "$SCRIPT_DIR/deps/copyright" ] && install -D -m 0644 "$SCRIPT_DIR/deps/copyright" "$DESTDIR/usr/share/doc/clashdock/copyright"
[ -f "$SCRIPT_DIR/deps/LICENSE.mihomo" ] && install -D -m 0644 "$SCRIPT_DIR/deps/LICENSE.mihomo" "$DESTDIR/usr/share/doc/clashdock/LICENSE.mihomo"

echo "clashdock 已安装到 ${DESTDIR}/usr/bin/clashdock，内核与基础规则已就位（离线可初始化）。"
echo "运行 clashdock 开始；卸载系统文件可执行同目录的 uninstall.sh。"

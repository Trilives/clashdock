#!/usr/bin/env bash
# clashdock 便携包卸载脚本：移除 install.sh 装入系统路径的文件，落点与 apt 卸载一致。
#
# 只清理系统文件，不动 /var/lib/clashdock 状态数据（订阅、内核缓存、customize.json 等）。
# 如需彻底清理服务单元、防火墙规则与状态数据，请先在 clashdock 里执行「卸载」流程，
# 再运行本脚本。
set -euo pipefail

# DESTDIR：与 install.sh 对称的可选前缀（打包/测试用）。默认空=真实系统路径。
DESTDIR="${DESTDIR:-}"

if [ -z "$DESTDIR" ] && [ "$(id -u)" -ne 0 ]; then
	if command -v sudo >/dev/null 2>&1; then
		echo "需要 root 权限，正在通过 sudo 重新执行…"
		exec sudo -- "$0" "$@"
	fi
	echo "错误：本脚本需要 root 权限。请用 root 运行或先安装 sudo。" >&2
	exit 1
fi

rm -f "$DESTDIR/usr/bin/clashdock"
rm -rf "$DESTDIR/usr/libexec/clashdock"
rm -rf "$DESTDIR/usr/share/clashdock"
rm -rf "$DESTDIR/usr/share/doc/clashdock"

echo "已移除 clashdock 系统文件。状态数据 /var/lib/clashdock 未删除。"

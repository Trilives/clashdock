#!/usr/bin/env bash
# clashdock 运行时目录迁移脚本：v0.1.x -> v0.2.0+
#
# v0.2.0 起服务运行时目录从 /etc/mihomo 改名为 /var/lib/clashdock-runtime，
# 且不再兼容旧路径（新版本的卸载/同步逻辑完全不认识 /etc/mihomo）。
# 本脚本只负责清理旧版本残留：停止/删除旧 systemd 单元、旧 NetworkManager
# 钩子、旧运行时目录。
#
# 状态目录 /var/lib/clashdock（订阅、customize.json、内核缓存等）不受影响，
# 清理完成后请重新运行新版本 clashdock 完成初始化——现有订阅与自定义配置
# 会被自动复用，只是重新注册一次服务（因为运行时目录布局是全新生成的，
# 单纯"同步"不会重建二进制/geo/UI，必须走一次完整初始化）。
#
# 用法：sudo bash migrate-runtime-dir.sh

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "请用 root 运行：sudo bash $0" >&2
  exit 1
fi

OLD_RUNTIME_DIR="/etc/mihomo"
UNITS=(
  mihomo.service
  mihomo-watchdog.service
  mihomo-watchdog.timer
  mihomo-update.service
  mihomo-update.timer
)
NM_HOOK="/etc/NetworkManager/dispatcher.d/90-mihomo-restart"

echo "==> 停止并禁用旧 systemd 单元…"
for u in "${UNITS[@]}"; do
  systemctl stop "$u" >/dev/null 2>&1 || true
  systemctl disable "$u" >/dev/null 2>&1 || true
  rm -f "/etc/systemd/system/$u"
done

if [ -f "$NM_HOOK" ]; then
  echo "==> 移除 NetworkManager 钩子 $NM_HOOK"
  rm -f "$NM_HOOK"
fi

if [ -d "$OLD_RUNTIME_DIR" ]; then
  echo "==> 清理旧运行时目录 $OLD_RUNTIME_DIR"
  rm -rf "$OLD_RUNTIME_DIR"
else
  echo "==> 未发现旧运行时目录 $OLD_RUNTIME_DIR，跳过"
fi

systemctl daemon-reload

cat <<'EOF'

完成。旧版本的服务单元与运行时残留已清理。

订阅与自定义配置保存在 /var/lib/clashdock，未受影响。
接下来请安装/运行新版本 clashdock，并完成一次初始化——它会在新路径
/var/lib/clashdock-runtime 下重新生成运行时并注册服务，现有订阅/配置
会被自动复用，无需重新添加。
EOF

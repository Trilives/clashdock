# T-010 完整模式（系统服务）实机验证

> 行数上限：300

## 概要

- 任务：T-010，见 [done.md](../done.md) 2026-10-18
- 日期：2026-10-18
- 负责人：AI 助手（Claude Code，Claude Opus 5.5）
- 范围：人工提供的测试机（局域网，Ubuntu 24.04）；代码改动仅 `flows/initflow.go` 回退修复

## 实施步骤

1. 记录测试机基线：clashdock 0.1.2（.deb）、`/var/lib/clashdock` 遗留种子数据、ufw 规则、ip rule / 路由、bashrc、Linger。
2. 打包备份 `/var/lib/clashdock`；临时加入免密 sudoers 片段，使普通用户运行的 clashdock 能在非 TTY 下调用 sudo。
3. 用 HEAD 构建的二进制跑 `init` → 固定节点 → `update` → `uninstall`，逐项核对。
4. 恢复：从备份还原 `/var/lib/clashdock`，删除 sudoers 片段与临时文件，按基线逐项核对。

## 关键决策

| 决策 | 备选方案 | 选择理由 | 是否经人工确认 |
|---|---|---|---|
| 在人工提供的测试机上验证 | 本机 / 容器 | 本机运行 sing-box，完整模式安装会删除 `sing-box.service` 并抢占 7890 / 9090 | 是（人工指定测试机） |
| 普通用户 + 临时免密 sudo | 直接以 root 运行 | 贴近真实用法（clashdock 自行调用 sudo）；片段测试后删除 | 是（人工授权完成任务并清理） |
| 下载经本机代理（`DOWNLOAD_PROXY`，仅本次进程） | 测试机直连 | 测试机直连 GitHub 约 130 KB/s 且超时；不写入测试机配置 | 否 |

## 改动内容

- 代码：`flows.undoServiceInstall`——sudo 授权失败时系统侧无残留则跳过卸载，避免误报「回退失败」；补测试。
- 文档：结果补写到 [main-group-node-selection.tdd.md](../../docs/testing/main-group-node-selection.tdd.md)。

## 验证

- 详见 [main-group-node-selection.tdd.md「完整模式实机验证」](../../docs/testing/main-group-node-selection.tdd.md)：4 个场景全部 PASS。
- 恢复核对：`/var/lib/clashdock` 与备份一致；无 `/var/lib/clashdock-runtime`、mihomo 单元、NM 钩子；ufw 规则、
  ip rule 与路由同基线；sudoers 片段已删除；`.deb` 安装的 clashdock 未受影响。

## 遗留问题

- [x] `sysd.Install` 无条件删除名为 `sing-box` 的系统服务，已由 T-011 改为初始化时先询问（见 [2026-10-18-singbox-conflict.md](2026-10-18-singbox-conflict.md)）。

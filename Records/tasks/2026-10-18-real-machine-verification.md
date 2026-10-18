# T-005 / T-006 用户模式实机验证

> 行数上限：300

## 概要

- 任务：T-005 主选择组节点选择实机验证；T-006 用户模式未实机验证的路径，见 [done.md](../done.md) 2026-10-18
- 日期：2026-10-18
- 负责人：AI 助手（Claude Code，Claude Opus 5.5）
- 范围：本机 systemd 用户实例 + 真实 mihomo；不改代码

## 实施步骤

1. 用 HEAD 源码构建标为 0.2.6 的二进制，与 `packaging/deps` 的内核和规则组成模拟便携包。
2. 从模拟包启动：安装本体、交接、端口改为 17890 / 19090、导入本地 YAML 订阅、部署用户服务并开启 linger。
3. 按 T-005 场景走节点切换 / 固定节点；按 T-006 场景跑 `clashdock update` 与稳定渠道自更新。
4. 经 `clashdock uninstall` 全量卸载，`loginctl disable-linger` 恢复 linger，核对无残留。

## 关键决策

| 决策 | 备选方案 | 选择理由 | 是否经人工确认 |
|---|---|---|---|
| 只测用户模式 | 同时测完整模式 | 本机 sudo 需要密码；完整模式还会改动路由 / 防火墙 | 否（环境限制），缺口登记为 T-010 |
| 订阅用不可达的占位节点 | 真实机场订阅 | 不使用个人订阅凭据；API 切换与持久化链路不依赖节点可用 | 否 |
| 测试进程经环境变量使用 `gh auth token` | 等待匿名限额恢复 | 匿名 GitHub API 限额已用尽；Token 只附在 API 请求上且不写盘 | 是（人工授权自行决定） |
| 测试中开启 linger，结束后关闭 | 只测拒绝分支 | 验收要求成功分支；原状态为关闭，已恢复 | 是（同上） |

## 改动内容

- 文档：结果补写到两份 TDD 记录；本记录；ideas.md / done.md。
- 代码：无。

## 验证

- T-005 详见 [main-group-node-selection.tdd.md「实机验证」](../../docs/testing/main-group-node-selection.tdd.md)：5 个场景全部 PASS。
- T-006 详见 [user-mode-service.tdd.md「补充实机验证」](../../docs/testing/user-mode-service.tdd.md)：4 个场景全部 PASS。
- 恢复核对：`Linger=no`；`~/.local/bin/clashdock`、`~/.local/share/clashdock`、用户单元均不存在；
  无残留进程；17890 / 19090 端口释放；本机原有 7890 / 9090 监听未受影响。

## 遗留问题

- [x] 完整模式（系统服务）的固定节点重启链路与 `clashdock update` 已于 2026-10-18 在测试机验证（T-010，见 [2026-10-18-full-mode-verification.md](2026-10-18-full-mode-verification.md)）。
- 未发现缺陷。

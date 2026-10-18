# T-011 完整模式安装不再无条件删除 sing-box 服务

> 行数上限：300

## 概要

- 任务：T-011，见 [done.md](../done.md) 2026-10-18
- 日期：2026-10-18
- 负责人：AI 助手（Claude Code，Claude Opus 5.5）
- 范围：`internal/sysd`（conflict.go、service.go）、`internal/flows`（singbox.go、initflow.go）、i18n

## 实施步骤

1. 去掉 `sysd.Install` 中对 `sing-box` 单元的 `removeUnit`，改为仍在运行时提示。
2. 初始化注册服务前询问是否停用已安装的 sing-box（默认否），同意才 stop + disable。
3. 拒绝且 sing-box 正在运行时检查代理 / 控制器端口，被占用则报错并回退初始化。
4. 单元测试 + 测试机实机验证（以占用端口的替身 `sing-box.service` 模拟）。

## 关键决策

| 决策 | 备选方案 | 选择理由 | 是否经人工确认 |
|---|---|---|---|
| 安装前询问 | 只删除本工具创建的单元 | 人工选择「先询问」 | 是 |
| 同意后 stop + disable，不删除单元文件 | 沿用旧行为删除单元文件 | 可用 `systemctl enable --now sing-box` 恢复，不破坏用户部署 | 否 |
| 默认否 | 默认是 | 防误触停掉用户自己的服务 | 否 |
| 拒绝且端口被占用时中止初始化 | 继续安装 | 继续安装会让 mihomo 启动失败且不易察觉 | 否 |
| 非交互路径（`clashdock update` / 定时器）只提示 | 同样询问 | 无人值守时无法询问，且不应停用用户服务 | 否 |

## 改动内容

- `sysd.ConflictingServiceState` / `StopConflictingService` / `warnIfConflictRunning`；`Install` 不再碰 sing-box。
- `flows.resolveSingBoxConflict`（依赖可注入）在 init 中先于 `sysd.Install` 调用。
- 测试：`sysd/conflict_test.go`、`flows/singbox_test.go`（不存在、同意、拒绝且端口占用、拒绝且端口空闲、
  已安装未运行、提示取消）。

## 验证

单元测试：`go test -race ./...` 通过。实机（测试机 Ubuntu 24.04，替身 `sing-box.service` 为监听
127.0.0.1:7890 的 python http.server；下载走本机预下载后上传的本地镜像 `github_mirror`）：

| 场景 | 结果 |
|---|---|
| init 时拒绝停用（替身占用 7890） | PASS：明确报端口 [7890] 被占用并回退初始化；sing-box 仍 active / enabled，单元文件保留，未注册 mihomo |
| init 时同意停用 | PASS：sing-box inactive / disabled，单元文件保留；mihomo 注册并监听 7890 / 9090 |
| sing-box 改用 17777 并重新启用后执行 `clashdock update` | PASS：完整 `Install` 只提示 sing-box 仍在运行；sing-box PID、启用状态与单元文件哈希均不变；mihomo 更新到 v1.19.32 |

恢复核对：替身单元、本地镜像、sudoers 片段与临时目录已删除；`/var/lib/clashdock` 从备份还原；
ip rule、ufw、`.deb` 安装的 clashdock 同基线。

## 遗留问题

- 无。

# 主选择组节点选择 TDD 证据

> 行数上限：300

## 来源与用户场景

- 来源：[Records/ideas.md](../../Records/ideas.md) 2026-08-21 待办。
- 用户在切换或固定节点时，只看到主选择组的当前状态，不被其它策略组干扰。
- 自动识别失败时，用户可输入组名或关键词继续；直接回车会取消且不改配置。

## RED / GREEN

| 阶段 | 命令 | 结果 | 证据 |
|---|---|---|---|
| RED | `env GOCACHE=/tmp/clashdock-go-build-root go test ./internal/flows` | FAIL | 新测试引用的 `resolveMainGroup`、`currentNodeSummary` 尚未实现 |
| GREEN | `env GOCACHE=/tmp/clashdock-go-build-root go test ./internal/flows` | PASS | 新增及既有 flows 测试全部通过 |

## 测试保证

| 保证 | 测试类型 | 位置 | 结果 |
|---|---|---|---|
| 关键词未命中时不会猜测成员最多的 `select` 组 | 单元 | `internal/flows/nodeselect_test.go` | PASS |
| 有效新关键词首插、持久化并立即识别主选择组 | 集成 | `internal/flows/nodeselect_test.go` | PASS |
| 空输入、输入取消或无效关键词不会改写配置 | 集成 | `internal/flows/nodeselect_test.go` | PASS |
| 当前状态摘要只包含已识别主选择组 | 单元 | `internal/flows/nodeselect_test.go` | PASS |
| API 不可用时仅展示主选择组的配置首选和非运行时状态 | 单元 | `internal/flows/nodeselect_test.go` | PASS |

## 覆盖率与已知缺口

- `pickGroup`：84.2%；`resolveMainGroup`：89.3%；`currentNodeSummary`：100%。
- 全仓语句覆盖率为 30.7%，其中 `internal/flows` 为 12.4%；大量终端、服务与网络编排路径仍缺少自动化测试，未达到 ECC 的全仓 80% 目标。
- 按用户安排，本轮不执行真实 mihomo / systemd 环境的实机测试；待预览包下载后另行验证交互、API 热切换和固定节点重启链路。

## 检查点

- RED：`d9fbf24`、`267d87b`
- GREEN：`419ae38`

## 实机验证（2026-10-18，T-005）

环境：本机 systemd 用户实例 + 真实 mihomo v1.19.27，用户模式部署（本机无免密 sudo，完整模式未测）；
本地 YAML 订阅含 `Airport Main`（不命中默认关键词）、`Streaming`、`Auto` 三个分组，节点为不可达的
socks5 占位。以非 TTY 回退喂答案驱动，结束后经 `clashdock uninstall` 清理。

| 场景 | 结果 |
|---|---|
| 未识别主选择组时直接回车 | PASS：提示无法切换，`customize.json` 哈希不变 |
| 输入不命中任何 select 组的关键词 | PASS：报错，`customize.json` 哈希不变 |
| 输入有效关键词 `Airport` | PASS：关键词插到 `main_group_keywords` 首位并立即识别；摘要只显示 `Airport Main` 的当前运行节点 |
| 节点切换（热切换） | PASS：Clash API `now` 变为所选节点；生效配置与运行时配置哈希不变 |
| 固定节点 | PASS：API 实时切换；生效配置、运行时配置、订阅配置首成员均为所选节点；用户服务重启后 `now` 保持所选节点 |

未覆盖：完整模式（系统服务，`sysd.SyncAndRestart`）的固定节点重启链路，已登记 T-010。

## 完整模式实机验证（2026-10-18，T-010）

环境：测试机 Ubuntu 24.04（systemd 255，ufw 启用），HEAD 构建的 clashdock 以普通用户运行、特权操作经
sudo（测试期间临时免密，结束后已撤销）；mihomo v1.19.27 预置在状态目录，订阅为本地 YAML（主组
`节点选择` 命中默认关键词，节点为不可达占位）。以非 TTY 回退经 FIFO 逐步喂答案，结束后经
`clashdock uninstall` 全量卸载并恢复测试机原状。

| 场景 | 结果 |
|---|---|
| `clashdock init`（TUN 关、局域网关、本地 YAML）注册并启动 `mihomo.service` | PASS：运行时配置校验通过，服务 active；网络自愈与每周更新定时器按选择安装 |
| 固定节点（init 末尾「切换 / 固定节点」→ 新加坡 01） | PASS：经 `sysd.SyncAndRestart` 同步并重启；生效配置、订阅配置、`/var/lib/clashdock-runtime/mihomo.yaml` 首成员均为所选节点；API `now` 为所选节点，再次手动重启后仍保持 |
| `clashdock update`（下载后完整 `Install`） | PASS：内核 v1.19.27 → v1.19.32、geo 与 Web UI 更新；服务主进程即运行时内核且为 v1.19.32，`/ui/` 返回 200，固定节点仍保持 |
| `clashdock uninstall`（全选） | PASS：系统单元、伴生定时器、NM 钩子、运行时目录与状态目录全部移除 |

顺带发现：sudo 授权失败时 init 回退会尝试卸载尚未注册的服务并误报「回退失败」，已修复（无残留时跳过卸载）。

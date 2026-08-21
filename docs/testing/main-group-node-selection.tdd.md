# 主选择组节点选择 TDD 证据

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

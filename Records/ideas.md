# 开发想法

> 行数上限：300

## 约定

- 本文件是**活动任务列表**的唯一入口；条目格式、状态与完成流程见 [WORKFLOW.md](../docs/WORKFLOW.md)。
- 完成的事项移至 [done.md](./done.md)。
- 架构设计遵循 [ARCHITECTURE.md](../docs/ARCHITECTURE.md)。
- 开发约束遵循 [CONSTRAINTS.md](../docs/CONSTRAINTS.md)；文件规模与模块边界遵循 [MODULARITY.md](../docs/MODULARITY.md)。
- 完成后要测试清楚。
- 待办清空时，按 [MAINTENANCE.md](../docs/MAINTENANCE.md) 做一次清理，然后提交所有应纳入版本控制的文件。

## 待办

按优先级排列；T-001、T-007 已完成，见 [done.md](./done.md) 2026-10-18。

### T-002 拆分 `internal/kernel/kernel.go`

- 优先级：P1
- 状态：待开始
- 来源：657 行，超过 MODULARITY.md 普通 `.go` 文件 500 行软上限；该包没有测试文件
- 验收标准：
  - 拆分后每个文件不超过 500 行（目标 200-400 行），按职责命名
  - 行为不变；为抽出的 helper 补小范围单元测试
  - `gofmt -l cmd internal` 无输出，`go vet ./...`、`go test ./...` 通过
- 记录：不需要（单包重构）

### T-003 拆分 `internal/flows/nodeselect.go`

- 优先级：P1
- 状态：待开始
- 来源：544 行，超过 MODULARITY.md 流程文件 400 行软上限；`pickNode` 97 行超过函数 80 行软上限
- 验收标准：
  - 拆分后每个文件不超过 400 行（目标 150-300 行），`pickNode` 不超过 80 行
  - 行为不变；`internal/flows/nodeselect_test.go` 现有测试全部通过
  - `gofmt`、`go vet ./...`、`go test ./...` 通过
- 记录：不需要（单包重构）

### T-008 为 push / PR 增加测试 CI

- 优先级：P1
- 状态：待开始
- 来源：[MAINTENANCE.md §3](../docs/MAINTENANCE.md#3-长期改进方向)：`release.yml` 只在 `v*` tag 时打包，不跑测试
- 验收标准：
  - 新工作流在推送到 `main` 与 PR 时运行 `gofmt -l cmd internal`（有输出即失败）、`go vet ./...`、`go test ./...`
  - Go 版本取自 `go.mod`（与 `release.yml` 一致）
  - 不改变现有 `release.yml` 的发布行为；在 GitHub 上首次运行通过
- 记录：不需要

### T-004 拆分 `internal/selfupdate/selfupdate.go`

- 优先级：P2
- 状态：待开始
- 来源：526 行，超过 500 行软上限；`Update` 99 行超过函数 80 行软上限
- 验收标准：
  - 拆分后每个文件不超过 500 行，`Update` 不超过 80 行
  - 行为不变；`internal/selfupdate/selfupdate_test.go` 现有测试全部通过
  - `gofmt`、`go vet ./...`、`go test ./...` 通过
- 记录：不需要（单包重构）

### T-005 主选择组节点选择的实机验证

- 优先级：P2
- 状态：待开始（记录中写明「待预览包下载后另行验证」，此后没有完成记录，按未完成处理）
- 来源：[main-group-node-selection.tdd.md](../docs/testing/main-group-node-selection.tdd.md)「覆盖率与已知缺口」
- 验收标准：
  - 在真实 mihomo / systemd 环境验证交互、Clash API 热切换和固定节点重启链路
  - 结果补写到上述 TDD 记录
- 记录：需要（实机验证）

### T-006 用户模式未实机验证的路径

- 优先级：P2
- 状态：待开始
- 来源：[user-mode-service.tdd.md](../docs/testing/user-mode-service.tdd.md)「覆盖率与已知缺口」
- 验收标准：
  - `loginctl enable-linger` 成功分支实机验证（依赖 polkit 允许）
  - 用户模式下 `clashdock update` 与 clashdock 自更新的联网路径实机验证
  - 结果补写到上述 TDD 记录
- 记录：需要（实机验证）

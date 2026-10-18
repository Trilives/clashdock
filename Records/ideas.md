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

### T-001 确认项目管理文档中剩余的「待人工确认」项

- 优先级：待人工确认
- 状态：阻塞（等待人工决定）
- 来源：[CONSTRAINTS.md](../docs/CONSTRAINTS.md)、[MAINTENANCE.md](../docs/MAINTENANCE.md)
- 验收标准：
  - 周期性清理的时间表已确定（MAINTENANCE.md §1）
  - 文档语言要求已明确（CONSTRAINTS.md §6）
  - 上述文档中对应的「待人工确认」已替换为确定值
- 记录：需要（修改工程约束）
- 进展：2026-10-18 各类文件行数上限已确定（见 CONSTRAINTS.md §4–§5），ARCHITECTURE.md 已拆分。

### T-002 拆分 `internal/kernel/kernel.go`

- 优先级：待人工确认
- 状态：待开始
- 来源：657 行，超过 MODULARITY.md 普通 `.go` 文件 500 行软上限
- 验收标准：
  - 拆分后每个文件不超过 500 行（目标 200-400 行），按职责命名
  - 行为不变；为抽出的 helper 补小范围单元测试（该包目前无测试文件）
  - `gofmt -l cmd internal` 无输出，`go vet ./...`、`go test ./...` 通过
- 记录：不需要（单包重构）

### T-003 拆分 `internal/flows/nodeselect.go`

- 优先级：待人工确认
- 状态：待开始
- 来源：544 行，超过 MODULARITY.md 流程文件 400 行软上限；`pickNode` 97 行超过函数 80 行软上限
- 验收标准：
  - 拆分后每个文件不超过 400 行（目标 150-300 行），`pickNode` 不超过 80 行
  - 行为不变；`internal/flows/nodeselect_test.go` 现有测试全部通过
  - `gofmt`、`go vet ./...`、`go test ./...` 通过
- 记录：不需要（单包重构）

### T-004 拆分 `internal/selfupdate/selfupdate.go`

- 优先级：待人工确认
- 状态：待开始
- 来源：526 行，超过 500 行软上限；`Update` 99 行超过函数 80 行软上限
- 验收标准：
  - 拆分后每个文件不超过 500 行，`Update` 不超过 80 行
  - 行为不变；`internal/selfupdate/selfupdate_test.go` 现有测试全部通过
  - `gofmt`、`go vet ./...`、`go test ./...` 通过
- 记录：不需要（单包重构）

### T-005 主选择组节点选择的实机验证

- 优先级：待人工确认
- 状态：待人工确认（记录中写明「待预览包下载后另行验证」，未见完成记录）
- 来源：[main-group-node-selection.tdd.md](../docs/testing/main-group-node-selection.tdd.md)「覆盖率与已知缺口」
- 验收标准：
  - 在真实 mihomo / systemd 环境验证交互、Clash API 热切换和固定节点重启链路
  - 结果补写到上述 TDD 记录
- 记录：需要（实机验证）

### T-006 用户模式未实机验证的路径

- 优先级：待人工确认
- 状态：待开始
- 来源：[user-mode-service.tdd.md](../docs/testing/user-mode-service.tdd.md)「覆盖率与已知缺口」
- 验收标准：
  - `loginctl enable-linger` 成功分支实机验证（依赖 polkit 允许）
  - 用户模式下 `clashdock update` 与 clashdock 自更新的联网路径实机验证
  - 结果补写到上述 TDD 记录
- 记录：需要（实机验证）

### T-007 处理清理候选

- 优先级：待人工确认
- 状态：阻塞（每项需人工批准）
- 来源：[MAINTENANCE.md §2](../docs/MAINTENANCE.md#2-清理候选2026-10-18-盘点)
- 验收标准：
  - 每个候选都有人工结论（执行 / 保留），执行的项已完成且可经 Git 回滚
  - 涉及 MODULARITY.md 的修改已先经人工讨论
  - MAINTENANCE.md 清单同步更新
- 记录：不需要（逐项在 done.md 记一行）

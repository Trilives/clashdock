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

按优先级排列；T-001 ~ T-003、T-007 已完成，见 [done.md](./done.md) 2026-10-18。

### T-008 为 push / PR 增加测试 CI

- 优先级：P1
- 状态：待验证（`.github/workflows/test.yml` 已提交并在本地按相同命令通过；需推送到 GitHub 后确认首次运行通过）
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

### T-009 用户模式按需 sudo 授权开启 TUN

- 优先级：P3
- 状态：待开始
- 来源：用户需求（2026-10-18）：用户模式开启 TUN 无需完整安装，直接请求 sudo 授权，并允许误触时取消
- 验收标准：
  - 用户模式下开启 TUN 时，无需执行完整安装或切换为完整模式；按需提示用户输入 sudo 密码以取得所需权限
  - 授权提示支持取消；取消或授权失败时不启用 TUN，不残留提权操作的部分改动，保持原有配置与代理服务可用
  - 特权操作统一经 `internal/execx` 的 sudo 子进程执行，不保存或记录用户密码
  - 覆盖授权成功、取消、授权失败的测试，并实机验证用户模式开启与关闭 TUN 的流程
- 记录：需要（改变用户可见行为，涉及提权与实机验证；实现时建立任务记录）

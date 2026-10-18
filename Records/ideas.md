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

按优先级排列；T-001 ~ T-010 已完成，见 [done.md](./done.md) 2026-10-18。

### T-011 完整模式安装不再无条件删除 sing-box 服务

- 优先级：P1
- 状态：待开始（处理方式需先与人工确认：安装前询问，还是只删除由本工具创建的单元）
- 来源：[2026-10-18-full-mode-verification.md](tasks/2026-10-18-full-mode-verification.md)「遗留问题」；`internal/sysd/service.go` 的 `Install` 调用 `removeUnit(conflictingName)`
- 验收标准：
  - 机器上已有用户自行部署的 `sing-box.service` 时，完整模式安装不会在未经确认的情况下停止、禁用或删除它
  - 端口冲突时给出明确提示，而不是静默接管
  - 有覆盖「存在 / 不存在 sing-box 单元」两种情况的测试
- 记录：需要（改变用户可见行为）

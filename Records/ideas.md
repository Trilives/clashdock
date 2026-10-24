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

按优先级排列；T-001 ~ T-019 已完成，见 [done.md](./done.md) 2026-10-18 与 2026-10-24。

### T-020 系统模式非 root mihomo 下 `PROCESS-NAME` 只能识别服务账户自己的进程

- 优先级：待人工确认
- 状态：待开始
- 来源：T-016 实机验证（[记录](tasks/2026-10-24-nonroot-mihomo.md)）。非 root 进程读不到其他用户的
  `/proc/<pid>/fd`，mihomo 无法把连接对应到进程：`tun_exclude_process` 生成的 `PROCESS-NAME,sshd,DIRECT`
  等规则在系统模式下不再命中（本机实测：同 uid 的 curl 命中、root 的 curl 未命中）。mars 上 root 版本
  同样从未记录到进程名，SSH 在 TUN 下正常，影响面待确认。
- 可选方向（需人工取舍）：
  - 保持现状，在初始化表单与文档中说明，并引导改用按 uid 排除（`tun_exclude_uids`，由内核按 uid 排除）；
  - 给服务加 `CAP_SYS_PTRACE`（可识别进程，但该能力可调试任意进程，等同放回 root，抵消 T-016）；
  - 增加「以 root 运行内核」的显式选项，默认非 root。
- 验收标准：待人工确认
- 记录：待定

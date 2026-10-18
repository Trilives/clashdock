# AI 助手指引

> 行数上限：300

本文件只做导航，内容以下列文档为准，不在此重复。

## 开始任何任务前必读

1. [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)：模块职责、依赖方向；新增模块须先提案并经人工批准（§12）。
2. [docs/CONSTRAINTS.md](docs/CONSTRAINTS.md)：所需工具、各类文件行数上限、Markdown 行数声明、项目约束。
3. [docs/MODULARITY.md](docs/MODULARITY.md)：文件 / 函数规模、拆分规则、包边界。
4. [docs/WORKFLOW.md](docs/WORKFLOW.md)：任务认领、状态更新、重大任务记录、清空任务后的清理与提交。

## 任务入口

- 活动任务：[Records/ideas.md](Records/ideas.md)
- 改动记录：[Records/done.md](Records/done.md)；重大任务记录模板：[Records/tasks/TEMPLATE.md](Records/tasks/TEMPLATE.md)
- 长期维护与清理候选：[docs/MAINTENANCE.md](docs/MAINTENANCE.md)

## 必须先征得人工同意的事项

- 修改工程约束（CONSTRAINTS.md / MODULARITY.md）
- 新增模块
- 删除任何文件
- 把文档中标为「待人工确认」的事项改成确定值

## 常用命令

`make build` / `make test` / `make vet` / `make fmt`，详见 `Makefile` 与 CONSTRAINTS.md §2。

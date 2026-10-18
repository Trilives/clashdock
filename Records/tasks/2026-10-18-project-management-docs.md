# T-001 / T-007 项目管理文档与首次清理

> 行数上限：300

## 概要

- 任务：建立项目管理文档（T-001 确认待定项，T-007 处理清理候选），见 [done.md](../done.md) 2026-10-18
- 日期：开始 2026-10-18 / 完成 2026-10-18
- 负责人：AI 助手（Claude Code，Claude Opus 5.5），人工确认决策
- 范围：`docs/`、`Records/`、`README.md`、`AGENTS.md`、`CLAUDE.md`、`.gitignore`、`scripts/`，
  以及 `internal/tui/tui.go`、`internal/flows/common.go`、`internal/sysd/service.go` 的注释

## 实施步骤

1. 盘点仓库：已有 `docs/ARCHITECTURE.md`、`docs/MODULARITY.md`、`Records/ideas.md`（任务入口）、
   `Records/done.md`（改动记录）、`docs/testing/*.tdd.md`（TDD 证据）；无独立配置文件需求
   （运行时配置已在 `customize.json`）。
2. 新增 CONSTRAINTS / WORKFLOW / MAINTENANCE / USERMODE、任务记录模板与 AI 助手入口；所有 Markdown 声明行数上限。
3. 人工确认行数上限后，拆分 ARCHITECTURE §6.5 与 README 的数据目录 / 目录结构，使其低于 90% 阈值。
4. 人工确认其余事项后：确定清理周期、文档语言、任务优先级，执行清理候选。

## 关键决策

| 决策 | 备选方案 | 选择理由 | 是否经人工确认 |
|---|---|---|---|
| 沿用 `Records/ideas.md` 作为任务入口 | 新建 TODO 文件或目录 | 仓库已有且在用，避免两个入口 | 是 |
| 行数上限：README 200、其它 Markdown 300、Shell 200、YAML 300、模板 / Makefile 100；golden 数据、go.sum、法律文本豁免；接近 = 90% | 统一单一上限 | 现有文件均可满足，Go 文件沿用 MODULARITY | 是 |
| ARCHITECTURE 拆分而非标为单篇长文例外 | 例外 | 拆出的用户模式章节自成一体，拆分成本低 | 是 |
| 清理周期：稳定版发布前 / 任务清空时 / 满 30 天 | 固定日历 | 与发布节奏绑定，避免遗忘 | 是 |
| 文档语言：中文；旧英文记录保留 | 统一翻译 | 记录属于历史，不在清理范围 | 是 |
| 保留「对应 Python 版」注释 | 全部删除 | 部分注释说明旧数据兼容语义 | 是 |
| 删除 `scripts/migrate-runtime-dir.sh` | 保留 | v0.2.0 一次性迁移、未随发行包分发；可从 tag `v0.3.0` 取回 | 是 |
| 不删除未跟踪的本地产物（`dist/`、`state/` 等） | 删除 | 不在 Git 中、不可回滚 | 是 |

## 改动内容

- 文档：新增 `docs/CONSTRAINTS.md`、`docs/WORKFLOW.md`、`docs/MAINTENANCE.md`、`docs/USERMODE.md`、
  `Records/tasks/TEMPLATE.md`、`AGENTS.md`、`CLAUDE.md`；更新 ARCHITECTURE（§12 新增模块流程、§6.5 摘要化）、
  MODULARITY（包边界、命名示例）、COMMANDS（`clashdock user` 精简）、README（项目管理索引）、ideas.md、done.md。
- 代码注释：`tui` / `flows` 的「四类提示」更正为五类；`sysd` 注释改为指向 Git 历史中的迁移脚本。
- 配置 / 打包：`.gitignore` 删除 Python 相关条目（被忽略文件集合不变）；删除 `scripts/migrate-runtime-dir.sh`。
- 提交：`678e882`（文档）及其后的清理提交。

## 验证

| 检查 | 命令或方式 | 结果 |
|---|---|---|
| 格式 | `gofmt -l cmd internal` | 无输出 |
| 静态检查 | `go vet ./...` | 通过 |
| 测试 | `go test ./...` | 通过 |
| 构建 | `make build` | 通过 |
| 行数声明 | `grep -L '^> 行数上限' $(git ls-files '*.md')` | 无缺失 |
| 行数阈值 | 所有 Markdown 低于各自上限的 90% | 通过（README 170/200，ARCHITECTURE 约 266/300） |
| 链接 | 脚本检查所有 Markdown 相对链接目标存在 | 无失效 |
| 忽略规则 | `git status --ignored` 前后对比 | 被忽略文件集合不变 |

## 遗留问题

- [ ] 文件拆分 T-002 ~ T-004、测试 CI T-008、实机验证 T-005 / T-006（已登记在 Records/ideas.md）
- [ ] ARCHITECTURE.md 接近 270 行阈值，下次扩写前需继续拆分（见 MAINTENANCE.md §3）

# 长期维护

> 行数上限：300

记录周期性清理与长期改进方向。具体可执行的任务登记在 [Records/ideas.md](../Records/ideas.md)；
本文只维护清单与规则。

## 1. 清理规则

- **周期**：待人工确认。另外，每当活动任务列表清空时必须做一次（见 [WORKFLOW.md §3](WORKFLOW.md#3-贡献者流程)）。
- **范围**：重复或过时的文件与内容。任务记录（`Records/done.md`、`Records/tasks/`、
  `docs/testing/`）属于历史，不在清理范围内。
- **去重**：同一信息出现在多处时，只保留一处权威说明，其余位置改成一句话加链接。
- **删除**：先在第 2 节列出候选与理由，**经人工批准**后再删；只能删除已纳入 Git 历史、
  可回滚的文件。未被 Git 跟踪的本地文件（如 `dist/`、`state/`）删除后无法通过 Git 恢复，
  必须单独征得同意。
- **约束文档**（CONSTRAINTS.md / MODULARITY.md）的改动先与人工讨论。

## 2. 清理候选（2026-10-18 盘点）

状态均为「待人工确认」，未执行任何删除。

### 2.1 重复内容

| 内容 | 出现位置 | 建议的权威位置 | 理由 |
|---|---|---|---|
| 包 / 目录结构说明 | ARCHITECTURE §9、MODULARITY「包边界」（README 已于 2026-10-18 改为链接） | ARCHITECTURE §9 写职责，MODULARITY 只写依赖规则 | 两处各自维护，已出现不一致（见 2.2） |
| 数据目录与运行时目录 | ARCHITECTURE §3、USERMODE.md、COMMANDS「环境变量」（README 已改为摘要 + 链接） | ARCHITECTURE §3 / USERMODE.md | 路径变更时需同步改多处 |
| 用户模式说明 | README「方式二」与「功能一览」、COMMANDS `clashdock user` 一行、USERMODE.md | 面向用户：README；设计：USERMODE.md；COMMANDS 只留一句并链接 | COMMANDS 表格单元格过长，与 README 重复 |
| 菜单结构 | README「预览」、ARCHITECTURE §4、COMMANDS `clashdock` 一行 | README（用户视角）+ ARCHITECTURE §4（实现） | 菜单调整时易漏改 |
| 发布说明 | `.goreleaser.yaml` 内嵌英文发布说明、`Records/done.md` | 两者受众不同，可保留；发布前核对一致即可 | 仅需核对，不必合并 |

### 2.2 过时内容

| 位置 | 问题 | 理由 / 证据 |
|---|---|---|
| MODULARITY「包边界」 | `sysd` 仍写「服务/面板」；未列出 `usermode`、`runtimesvc`、`i18n`、`clashapi`、`selfupdate`、`fetchx` 等 | ARCHITECTURE §3 写明已无独立面板单元，§6.5 已定义 `usermode` / `runtimesvc` 的依赖方向。属约束文档，需人工讨论 |
| MODULARITY「文件命名」 | 示例 `init_resources.go` 不存在 | `internal/flows/` 下没有该文件 |
| `.gitignore` | Python 字节码、pytest / mypy / ruff 缓存、`tests/.gen_check.json`、虚拟环境等条目 | 仓库已无 Python 源码与 `tests/` 目录；条目无害，可精简 |
| `scripts/migrate-runtime-dir.sh` | v0.1.x → v0.2.0 的一次性迁移脚本 | 未打进发行包（`.goreleaser.yaml` 不引用）；但 `internal/sysd/service.go` 注释仍引用它，删除前需一并改注释 |
| 源码注释「对应 Python 版…」 | `internal/flows/common.go`、`internal/tui/tui.go`、`internal/jsonx/jsonx.go`、`internal/config/config.go` | Python 版已不在仓库，对照说明对新读者意义有限 |
| ARCHITECTURE §4 | 「（初始化用它，见第 4 节）」在第 4 节内自引用 | 疑似章节调整后的残留 |

### 2.3 本地未跟踪产物（不在 Git 中，删除不可回滚）

| 路径 | 说明 |
|---|---|
| `dist/` | v0.2.3 的 GoReleaser 产物，已被 `.gitignore` 忽略 |
| `clashdock` | 本地构建的二进制，已忽略 |
| `state/` | 本地运行数据（可能含订阅与密钥），已忽略，**不要**提交或随意删除 |
| `.claude/`、`.agents/`、`.codex/` | 空目录，Git 不跟踪 |

## 3. 长期改进方向

| 方向 | 现状（证据） | 说明 |
|---|---|---|
| 测试覆盖率 | 全仓约 30%（`docs/testing/` 记录 29.9%–30.7%），目标 80% | `kernel`、`firewall`、`proxyenv`、`jsonx`、`configfile` 无测试文件；`flows` 约 12%–14% |
| CI 运行测试 | `.github/workflows/release.yml` 只在推送 `v*` tag 时打包发布，不跑 `go test` | 可考虑为 push / PR 增加测试工作流（需人工决定） |
| 超过软上限的文件 | `internal/kernel/kernel.go` 657 行、`internal/flows/nodeselect.go` 544 行、`internal/selfupdate/selfupdate.go` 526 行 | 已登记拆分任务 |
| 超过 80 行软上限的函数（均未超 120 行硬上限） | `flows.Init` 112、`selfupdate.Update` 99、`flows.pickNode` 97、`subscription.Apply` 91、`sysd.Install` 88、`kernel.extract` 87、`flows.editList` 86、`subscription.ApplyOverlay` 85 | 修改这些函数时顺带拆分；`i18n/strings_flows_usermode.go` 的 `init`（97 行）是翻译表，属 MODULARITY 允许的集中表例外 |
| 文档长度 | ARCHITECTURE.md 已从 315 行拆到约 266 行（§6.5 移至 USERMODE.md），接近 270 行阈值 | 再扩写前先按章节继续拆分（候选：§4 交互契约、§8 自更新） |
| 实机验证缺口 | 见 ideas.md 中的实机验证任务 | linger 成功分支、联网更新 / 自更新、主选择组实机验证 |

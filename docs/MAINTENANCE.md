# 长期维护

> 行数上限：300

记录周期性清理与长期改进方向。具体可执行的任务登记在 [Records/ideas.md](../Records/ideas.md)；
本文只维护清单与规则。

## 1. 清理规则

- **周期**：满足任一条件即做一次——
  - 发布稳定版（推送不带预发布后缀的 `v*` tag）之前；
  - 活动任务列表清空时（见 [WORKFLOW.md §3](WORKFLOW.md#3-贡献者流程)）；
  - 距上次清理已满 30 天。
- **范围**：重复或过时的文件与内容。任务记录（`Records/done.md`、`Records/tasks/`、
  `docs/testing/`）属于历史，不在清理范围内。
- **去重**：同一信息出现在多处时，只保留一处权威说明，其余位置改成一句话加链接。
  面向不同读者（用户视角 vs 实现视角）的描述可以并存，但不得互相矛盾。
- **删除**：先在第 2 节列出候选与理由，**经人工批准**后再删；只能删除已纳入 Git 历史、
  可回滚的文件。未被 Git 跟踪的本地文件（如 `dist/`、`state/`）删除后无法通过 Git 恢复，不在清理范围内。
- **约束文档**（CONSTRAINTS.md / MODULARITY.md）的改动先与人工讨论。
- 每次清理在 done.md 记一行，并更新第 2 节。

## 2. 清理候选

当前没有待处理的候选。新发现的候选按下表格式列在这里，处理后移入 2.1。

| 内容 / 位置 | 问题 | 理由 / 证据 | 建议处理 |
|---|---|---|---|

v0.4.0 发布前复查（2026-10-24，人工要求「清理无效文件」）：结果见 2.1；各类文件均在声明上限内，
Markdown 链接有效；贴近上限的文件与函数见第 3 节。

### 2.1 最近一次清理结果（2026-10-24，人工要求清理无效文件）

| 候选 | 结论 | 处理 |
|---|---|---|
| `docs/Pictures/portable-mode.png`（v0.2.4 便携模式界面，与现行用户模式不符） | 已删除 | README 预览改用按现行代码渲染的 `user-mode.png`；旧图可从 tag `v0.3.1` 取回 |
| i18n 中 17 条代码已不再引用的英文翻译（旧初始化问答、旧确认提示等） | 已删除 | `coverage_test` 仍通过 |
| `config.EnsureExists`（无调用方，仅测试引用） | 已删除 | 连同其测试；`config.go` 降到 488 行 |
| `subscription.FromClashYAML`、`txn.Transaction.TrackPath`、`tui.FormResult.Choice`（无生产调用方） | 保留 | 体量小且有测试或属于成组的 API；见第 3 节 |
| ARCHITECTURE.md 285 行超过 270 行阈值 | 已拆分 | §8 自更新拆到 [SELFUPDATE.md](SELFUPDATE.md)，原位保留摘要 |
| 本地未跟踪产物 `dist/`、`clashdock`、`state/`、空目录 `.agents/` `.claude/` `.codex/` | 不处理 | 同上次：不在 Git 中；`state/` 可能含订阅与密钥 |

### 2.2 上一次清理结果（2026-10-18，人工已批准）

| 候选 | 结论 | 处理 |
|---|---|---|
| 包 / 目录结构说明重复（README、ARCHITECTURE §9、MODULARITY） | 已处理 | README 改为链接；MODULARITY「包边界」只保留带约束的边界并链接 §9 |
| 数据目录说明重复（README、ARCHITECTURE §3、USERMODE、COMMANDS） | 已处理 | README 改为摘要 + 链接；COMMANDS 的环境变量表是变量参考，保留 |
| 用户模式说明重复（README、COMMANDS、USERMODE） | 已处理 | COMMANDS `clashdock user` 一行缩为一句并链接 |
| 菜单结构（README「预览」、ARCHITECTURE §4） | 保留 | 用户视角与实现视角，读者不同 |
| 发布说明（`.goreleaser.yaml`、done.md） | 保留 | 英文发布说明与中文改动记录受众不同；发布前核对一致 |
| MODULARITY「包边界」过时（面板单元、缺 `usermode` / `runtimesvc` / `i18n`） | 已处理 | 删去「面板」，补充三个包的边界与依赖规则 |
| MODULARITY 命名示例 `init_resources.go` 不存在 | 已处理 | 改为实际存在的 `initform.go` / `initform_test.go` |
| `.gitignore` 的 Python / 虚拟环境 / `tests/` 条目 | 已处理 | 删除；被忽略的文件集合不变 |
| `scripts/migrate-runtime-dir.sh`（v0.1.x → v0.2.0 一次性迁移） | 已删除 | 可从 tag `v0.3.0` 取回；`internal/sysd/service.go` 注释同步改为指向 Git 历史 |
| 源码注释「对应 Python 版…」 | 保留 | 部分注释承载旧数据兼容语义（如 `meta.json` 字段、`customize.json` 写盘顺序），删除会丢信息 |
| `tui` / `flows` 注释称「四类提示」 | 已处理 | 实际有五类（含 Form），已更正 |
| ARCHITECTURE §4 自引用「见第 4 节」 | 已处理 | 改为「见本节下文」 |
| 本地未跟踪产物 `dist/`、`clashdock`、`state/`、空目录 `.claude/` 等 | 不处理 | 不在 Git 中、删除不可回滚；`state/` 可能含订阅与密钥，不得提交 |

## 3. 长期改进方向

| 方向 | 现状（证据） | 说明 |
|---|---|---|
| 测试覆盖率 | 全仓 42.1%（2026-10-24，含真实内核用例），目标 80%；`paths` 74.3%、`usermode` 72.1%、`kernel` 61.7%、`selfupdate` 57.9% | `firewall`、`jsonx` 仍无测试文件；`fetchx` 9.0%、`cmd/clashdock` 12.8%、`flows` 21.8%、`sysd` 23.4% 最低。每个任务至少为改动部分补测试 |
| CI 运行测试 | `.github/workflows/test.yml`（push 到 `main` / PR 时跑 gofmt、vet、race 测试），2026-10-18 首次运行通过 | Actions 提示 Node 20 弃用，`actions/checkout` / `setup-go` 有新大版本时升级 |
| 超过软上限的文件 | 无；贴近软上限：`flows/modify.go` 384 / 400、`config/config.go` 488 / 500（v0.4.0 发布前复查） | 每次清理时用 MODULARITY.md「修改流程」的 `wc -l` 复查；再往这两个文件加内容前先拆分 |
| 超过 80 行软上限的函数（均未超 120 行硬上限） | `flows.Init` 111、`subscription.Apply` 91、`flows.editList` 86、`subscription.ApplyOverlay` 85（`sysd.Install` 已于 T-014 / T-016 抽出 `deployRuntime` 后降到 80 行以内） | 修改这些函数时顺带拆分；`i18n/strings_flows_usermode.go` 的 `init`（115 行）是翻译表，属 MODULARITY 允许的集中表例外 |
| 无生产调用方的代码 | `subscription.FromClashYAML`、`txn.Transaction.TrackPath`、`tui.FormResult.Choice`（`deadcode` 2026-10-24） | 下次修改所在文件时评估删除（连同只为它们存在的测试） |
| 文档长度 | ARCHITECTURE.md 257 行（2026-10-24 §8 拆到 SELFUPDATE.md） | 再扩写前先按章节继续拆分（候选：§4 交互契约，约 70 行） |
| 实机验证 | 2026-10-18 用户模式（T-005 / T-006）、完整模式（T-010）、用户模式按需 TUN（T-009）；2026-10-24 起改在 mars 验证完整模式特权边界与非 root 内核（T-012 ~ T-016） | 改动服务 / 提权 / 网络路径时按对应记录的场景复验 |

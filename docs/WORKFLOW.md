# 任务流程

> 行数上限：300

本文约定人类贡献者与 AI 助手如何认领、跟踪、完成与记录任务。沿用仓库已有的 `Records/`
体系，不另建任务系统。

## 1. 任务入口

| 位置 | 用途 |
|---|---|
| [Records/ideas.md](../Records/ideas.md) | **活动任务列表**（唯一入口）：待开始 / 进行中 / 阻塞的任务 |
| [Records/done.md](../Records/done.md) | 按日期的改动记录；完成的任务从 ideas.md 移到这里（沿用原约定） |
| `Records/tasks/` | 重大任务的详细记录（见第 4 节），模板 [TEMPLATE.md](../Records/tasks/TEMPLATE.md) |
| [docs/testing/](testing/) | 既有的 TDD 证据记录，作为对应任务的验证部分保留并被链接 |

## 2. 任务条目格式

每个任务在 ideas.md「待办」下占一个三级标题，字段齐全：

```markdown
### T-NNN 任务标题

- 优先级：P0 / P1 / P2 / P3 / 待人工确认
- 状态：待开始 / 进行中 / 阻塞（原因） / 已完成 / 已归档（未验收）
- 来源：需求出处或文档链接
- 验收标准：
  - 可检验的条件 1（未知时写「待人工确认」）
- 记录：Records/tasks/… 或「不需要」
```

- 优先级含义：P0 阻断使用或发布；P1 本版本应完成；P2 有空再做；P3 远期需求，按需排期。具体任务的优先级不确定时
  写「待人工确认」，不要自行拍板。
- 编号递增，不复用；已完成任务移入 done.md 时保留编号便于追溯。
- 「已归档（未验收）」：不再继续推进、但验收标准没有全部满足的任务（如需求取消、被其他任务取代、
  实机条件长期不具备）。同样移入 done.md，条目以「归档（未验收）」开头，写明未满足的验收项与原因，
  不得写成「完成」；之后若重新需要，另开新编号并链接旧条目。

## 3. 贡献者流程

**开始前**

1. 读 [ARCHITECTURE.md](ARCHITECTURE.md)、[CONSTRAINTS.md](CONSTRAINTS.md)、
   [MODULARITY.md](MODULARITY.md)。
2. 在 ideas.md 找到任务（没有就先登记），把状态改为「进行中」。

**进行中**

3. 遵守约束；改动让文件接近或超过其声明上限时，同一任务内先拆分。
4. 需要修改工程约束（CONSTRAINTS.md / MODULARITY.md）时，**先停下来与人工讨论**，同意后再改。
5. 需要新增模块时，先走第 6 节提案流程。

**完成后**

6. 按 MODULARITY.md「修改流程」跑检查，结果写进验收说明。
7. 把任务从 ideas.md 移到 done.md 当日条目，一句话说明改动；重大任务附记录链接。
8. 重大任务按第 4 节写记录。

**活动任务列表清空时**（ideas.md「待办」只剩「暂无」）

9. 按 [MAINTENANCE.md](MAINTENANCE.md) 做一次清理（不包括任务记录：`Records/done.md`、
   `Records/tasks/`、`docs/testing/`）。清理只产出候选清单；删除文件须人工批准。
10. 检查 `git status`：应纳入版本控制的文件全部提交；本地产物（构建结果、`state/`、`dist/`、
    `.env` 等）若未被忽略，先更新 `.gitignore` 再提交。

## 4. 重大任务记录

**何时记录**：满足任一条件即为重大任务——

- 跨两个及以上包，或改变用户可见行为、数据目录 / 服务布局；
- 新增模块，或修改工程约束；
- 发布版本（含预览版）；
- 需要实机验证（真实 systemd / mihomo / 网络）。

**如何记录**：

- 复制 [Records/tasks/TEMPLATE.md](../Records/tasks/TEMPLATE.md) 为
  `Records/tasks/YYYY-MM-DD-<slug>.md`，随任务推进填写。
- 已有 `docs/testing/*.tdd.md` 证据时，记录的「验证」部分链接过去，不复制内容。
- done.md 的对应日期条目附上记录链接。

**既有记录**（早于本流程，保留原位）：

| 日期 | 任务 | 记录 |
|---|---|---|
| 2026-08-21 | 初始化表单 Enter、配置同步链路、节点状态区分（v0.2.6-beta.2） | [ideas-preview-0.2.6-beta.2.tdd.md](testing/ideas-preview-0.2.6-beta.2.tdd.md) |
| 2026-08-22 | 主选择组节点选择 | [main-group-node-selection.tdd.md](testing/main-group-node-selection.tdd.md) |
| 2026-09-30 | 用户模式（免 root 用户级服务，v0.3.0） | [user-mode-service.tdd.md](testing/user-mode-service.tdd.md) |

## 5. 文件拆分

任何文件（含任务列表与记录）接近或超过其声明上限时拆分。ideas.md 过长时，把「阻塞」或
低优先级任务拆到 `Records/ideas-<主题>.md`，并在 ideas.md 保留链接。

## 6. 新增模块提案

新增模块必须**先提案、经人工批准后再实现**（规则见 ARCHITECTURE §12）。提案写在
`docs/proposals/<模块名>.md`（首次需要时创建该目录），内容：

```markdown
# 提案：<模块名>

> 行数上限：300

- 状态：草稿 / 待批准 / 已批准（批准人，日期） / 已拒绝
- 目的：要解决的问题，为什么现有模块不适合承担
- 职责：模块做什么、不做什么
- 依赖方向：依赖哪些包、被哪些包调用；确认不违反 MODULARITY.md「包边界」
- 对外接口：主要类型 / 函数草案
- 测试方案：单元 / 集成 / 实机验证
- 影响：需要同步修改的文档（ARCHITECTURE §9、MODULARITY 包边界等）
```

批准后在 ideas.md 登记实现任务，并在 ARCHITECTURE.md 补充模块说明。

## 7. 实机验证

- 主机：**mars**（2026-10-24 起，人工指定；访问方式与 sudo 口令在仓库根 `.env`，已被忽略，
  不得打印或提交）。mars 是在用的部署，验证后必须保持代理可用。本机运行 sing-box，不做完整模式安装。
- **验证前快照**：记录版本、单元状态与 `ExecStart` / `User=`、mihomo 进程身份、状态目录与运行时
  目录的属主 / 权限清单（`find … -printf '%u:%g %m %y %p'`）、连通性探针结果；把 `/var/lib/clashdock`、
  `/var/lib/clashdock-runtime` 与 `mihomo*` 单元打包备份到 `/root/clashdock-backup-<日期>/`，并保留
  当前版本的 `.deb` 以便回退。
- **验证后比对与恢复**：与快照逐项比对，只允许出现本任务预期的差异（写进记录）；不符合预期或
  验证中途失败时，用备份与旧 `.deb` 恢复到快照状态，再复核探针。
- 交互流程用 pty 驱动（`NO_COLOR=1` 走纯文本提示），sudo 口令只经 stdin 传入；记录里注明出现了几次
  口令提示。

## 8. 发布

**发布前门禁**（全部满足才打 tag）：

- [ ] ideas.md 中计划随本版发布的任务均为「已完成」或「已归档（未验收）」，done.md 已记录；
- [ ] `gofmt -l cmd internal` 无输出、`go vet ./...` 通过、`go test -race ./...` 通过（含设置
      `MIHOMO_BIN` 的真实内核用例），测试 CI 为绿；
- [ ] 按 MAINTENANCE.md 完成稳定版发布前清理；README / ARCHITECTURE / COMMANDS 与行为一致；
- [ ] 涉及服务布局、权限或升级路径的改动，已用 goreleaser 快照包在 mars 上完成升级验证（第 7 节）。

**发布后核对**：

- [ ] release 工作流通过，产出全部架构的 tar.gz / .deb 与 `checksums.txt`；
- [ ] `/releases/latest`（稳定版）或 `/releases`（预览版）返回新版本；
- [ ] 抽查一个包的 SHA-256 与 `checksums.txt` 一致，`clashdock version` 输出正确；
- [ ] mars 从上一版本升级到正式包后服务正常；结果写入发布记录。

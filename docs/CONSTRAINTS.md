# 开发约束

> 行数上限：300

本文是 clashdock 开发约束的**入口**。文件 / 函数规模、拆分规则与包边界的权威说明在
[MODULARITY.md](MODULARITY.md)，本文只引用、不重复其数值。

**修改本文或 MODULARITY.md 之前，必须先与人工维护者讨论并取得同意**，再改文档，并在
[Records/done.md](../Records/done.md) 记录变更。

## 1. 开始任何改动前

1. 读 [ARCHITECTURE.md](ARCHITECTURE.md)（模块职责与依赖方向）。
2. 读本文与 [MODULARITY.md](MODULARITY.md)。
3. 按 [WORKFLOW.md](WORKFLOW.md) 在 [Records/ideas.md](../Records/ideas.md) 认领或登记任务。

## 2. 运行与开发所需工具

| 工具 | 用途 | 版本 / 依据 |
|---|---|---|
| Go | 构建、测试、`go vet`、`gofmt` | `go.mod` 声明 `go 1.26`；CI 用 `setup-go` 读取 `go.mod` |
| make | `make build` / `test` / `vet` / `fmt` / `release-snapshot` | 见 `Makefile` |
| GoReleaser | 打包 tar.gz 与 .deb；`make release-snapshot`、tag 触发的 CI 发布 | v2（`.github/workflows/release.yml` 用 `~> v2`） |
| bash、curl、gunzip | `scripts/*.sh`（如打包前预下载内核与规则的 `fetch-deb-deps.sh`） | 脚本 shebang 为 `#!/usr/bin/env bash` |
| tmux | TUI 逐屏验证（`send-keys` / `capture-pane`） | 见 ARCHITECTURE §10 |
| systemd（系统实例与用户实例）、sudo | 完整模式 / 用户模式的实机验证 | 见 `docs/testing/*.tdd.md` |

- **不需要 Conda / uv / Pixi 等 Python 环境工具**：仓库没有 Python 源码，依赖由 Go modules
  （`go.mod` / `go.sum`）管理。`.gitignore` 里的 Python 条目是从 Python 版移植时留下的，
  见 [MAINTENANCE.md](MAINTENANCE.md) 清理候选。
- 提交前的检查命令以 MODULARITY.md「修改流程」为准（`gofmt`、`go test ./...`、`go vet ./...`）；
  涉及并发或服务编排时，现有记录还会跑 `go test -race ./...`。

## 3. 模块化规则

- 规模红线、拆分触发条件、包边界、文件命名、例外：见 [MODULARITY.md](MODULARITY.md)。
- **新增模块**（新的 `internal/<pkg>`、`cmd/<bin>` 或独立脚本目录）必须先写提案并取得人工
  批准，才能动手实现。流程见 [ARCHITECTURE.md §12](ARCHITECTURE.md#12-新增模块流程)，
  提案模板见 [WORKFLOW.md §6](WORKFLOW.md#6-新增模块提案)。

## 4. 各类文件的行数上限

| 文件类型 | 上限 | 依据 |
|---|---|---|
| `.go` 源码、`internal/flows/*.go`、`*_test.go`、单个函数 | 见 MODULARITY.md「规模红线」 | 已有约束 |
| `README.md`（任何目录） | **200 行** | 本文 |
| 其它 Markdown（`.md`，含任务列表、记录与提案） | **300 行**（各文件顶部声明，见第 5 节） | 本文 |
| Shell 脚本（`scripts/**/*.sh`） | **200 行** | 本文 |
| YAML（`.goreleaser.yaml`、`.github/workflows/*.yml`） | **300 行** | 本文 |
| systemd 单元模板（`internal/**/assets/*.tmpl`） | **100 行** | 本文 |
| `Makefile` | **100 行** | 本文 |
| 测试数据（`internal/subscription/testdata/golden.json`） | 豁免 | 冻结的兼容性基线，拆分会改变语义，见第 6 节 |
| 工具生成文件（`go.sum`） | 豁免 | 由 `go` 工具维护，不手工编辑 |
| `LICENSE`、`packaging/copyright` | 豁免 | 法律文本，按原文保留 |
| 图片（`docs/Pictures/*.png`） | 不适用（非文本） | — |

上表中的数值均为硬上限。**接近**指达到上限的 90%（如 300 行上限对应 270 行，200 行对应 180 行）。
Go 文件另有 MODULARITY.md 的目标值与软上限，以 MODULARITY.md 为准。

## 5. Markdown 行数声明与拆分

- 每个 `.md` 文件必须在一级标题之后的第一行声明上限，格式二选一：
  - `> 行数上限：N`（N 不得超过第 4 节对应类型的上限）
  - `> 行数上限：不限（单篇长文例外：<理由>；批准：<人>，<日期>）`
- **接近或超过**上限时必须拆分：按主题拆到同目录新文件，原文件保留摘要与链接；拆分本身
  记入任务列表或 done.md。
- **单篇长文例外**只适用于已明确标注、适合保持为一篇长文的文章，标注时写明理由与批准人。
  目前**没有**此类例外。
- 已执行的拆分：`ARCHITECTURE.md` 原 §6.5 用户模式拆到 [USERMODE.md](USERMODE.md)，原位保留摘要与依赖方向。
- 自查命令：
  - 缺少声明的文件：`grep -L '行数上限' $(git ls-files '*.md')`
  - 行数：`wc -l $(git ls-files '*.md') | sort -n`

## 6. 其它项目约束

以下约束均出自现有代码与文档，权威出处见括号：

- **静态单二进制**：`CGO_ENABLED=0`，不引入运行时依赖（`Makefile`、`.goreleaser.yaml`、README）。
- **特权操作**只经 `internal/execx` 的 sudo 子进程（ARCHITECTURE §3）。
- **生效配置写盘为 JSON**（ARCHITECTURE §2）。
- **订阅数据 copy-on-write**，不得原地修改解析结果（MODULARITY「包边界」）。
- **i18n**：每条 `i18n.T("…")` 字面量都必须登记英文翻译（`internal/i18n/coverage_test.go`
  强制）；不得在包级 `var` / `const` 初始化中调用 `i18n.T`（ARCHITECTURE §4）。
- **golden 基线**：`internal/subscription/testdata/golden.json` 是冻结的兼容性基线（ARCHITECTURE §10），
  修改它属于行为变更，需人工确认。
- **测试不得触碰真实主机服务**：systemd / sudo 调用通过注入的 runner 或假 `Service` 覆盖
  （`docs/testing/*.tdd.md`）；实机验证单独进行并记录。
- **运行时配置**：用户配置统一在 `customize.json`（`internal/config` 维护默认值与字段元数据），
  环境变量见 [COMMANDS.md](COMMANDS.md#环境变量)。不另建中央配置文件。
- **提交信息**：`<type>: <description>`，type 取 feat / fix / refactor / docs / test / chore / perf / ci
  （与 git 历史一致）。
- **版本与发布**：版本号在 `VERSION`；推送 `v*` tag 触发 `release.yml`；带预发布后缀的 tag
  自动标为 GitHub prerelease（ARCHITECTURE §8）。
- **文档语言**：现有文档以中文为主（`docs/testing/` 中有一篇英文）；是否强制统一语言，待人工确认。

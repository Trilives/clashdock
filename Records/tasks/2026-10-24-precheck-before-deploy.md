# T-014 先校验、后覆盖运行时文件

> 行数上限：300

## 概要

- 任务：T-014，见 [done.md](../done.md) 2026-10-24
- 日期：2026-10-24
- 负责人：AI 助手（Claude Code，Claude Opus 5.5）
- 范围：`internal/configfile`（precheck.go）、`internal/sysd`（service.go）、`internal/usermode`（service.go）

## 实施步骤

1. 新增 `configfile.Precheck(bin, parent, cfg, assets)`：在状态目录下的私有临时目录写入配置，geo 数据
   以指向绝对路径的符号链接放入，`external-ui` 改写到校验目录内（mihomo 要求面板路径落在 `-d` 之内），
   用即将部署的内核执行 `-t`；失败返回 `*PrecheckError`（带内核输出，说明处于校验阶段、运行时未改动）。
2. `sysd.Install` 先用状态目录的内核与 geo 校验，通过后才部署运行时文件、移除旧单元；`SyncAndRestart`
   先用运行中的内核与运行时 geo 校验暂存配置，通过后才安装配置并重启。
3. 用户模式部署同样先校验。
4. 单元测试 + 真实内核用例（`MIHOMO_BIN`）。

## 关键决策

| 决策 | 备选方案 | 选择理由 | 是否经人工确认 |
|---|---|---|---|
| 校验目录放在状态目录下 | 系统 `/tmp` | 状态目录已收归 root，避免共享 `/tmp` 的竞争 | 否 |
| geo 以绝对路径符号链接放入 | 复制 | geo 文件较大；相对路径会在校验目录里悬空（已有回归测试） | 否 |
| 部署后的 root `-t` 保留 | 删除 | 额外兜底；实测 `-t` 不打开 provider / cache 文件（见 T-012 记录复审 MEDIUM-8） | 否 |

## 改动内容

- 代码：`configfile/precheck.go`；`sysd/service.go` 的 `Install` / `syncRuntime` 改为先校验；`usermode/service.go`。
- 测试：`TestPrecheckRunsInPrivateDirWithAssetsAndRewrittenUI`、`TestPrecheckLinksRelativeAssetsByAbsolutePath`、
  `TestPrecheckFailureIsPrecheckErrorWithKernelOutput`、`TestPrecheckWithRealKernel`、
  `TestInstallPrecheckFailureTouchesNothing`、`TestSyncRuntimePrecheckFailureKeepsRuntimeConfig`。

## 验证

| 检查 | 命令或方式 | 结果 |
|---|---|---|
| 格式 | `gofmt -l cmd internal` | 无输出 |
| 静态检查 | `go vet ./...` | 通过 |
| 测试 | `MIHOMO_BIN=… go test -race ./...` | 全部通过（含真实内核） |
| 实机验证 | mars：快照包上的重新部署与 `clashdock update`（完整 `Install`） | 校验通过后才部署，服务正常；现场快照与恢复见 [T-012 记录](2026-10-24-privilege-boundary.md) |

## 遗留问题

- 无。

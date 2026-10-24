# 改动记录

> 行数上限：300

## 约定

- 每次改动按日期记录。
- 重大任务在当日条目后附记录链接（何时算重大任务见 [WORKFLOW.md §4](../docs/WORKFLOW.md#4-重大任务记录)）。

## 2026-10-24

- 发布前清理（人工要求「清理无效文件」）：删除 17 条未引用的翻译、无调用方的 `config.EnsureExists`、过时的
  `docs/Pictures/portable-mode.png`（README 用户模式预览改为按现行代码渲染的 `user-mode.png`）；ARCHITECTURE
  §8 自更新拆到 `docs/SELFUPDATE.md`（285 → 257 行）；README 自更新说明同步 dpkg 升级与 SHA-256 来源。
  详见 MAINTENANCE §2.1。记录：[2026-10-24-release-v0.4.0.md](tasks/2026-10-24-release-v0.4.0.md)
- T-012 ~ T-019 来自对 sboxkit `Records/` 的借鉴（优先级经人工同意）。实机验证主机改为 mars（人工指定）。
- T-012 / T-013 / T-015：系统模式特权边界——状态目录收归 `root:<管理员私有组>`（凭据 0640、订阅目录 0750，
  首个 root 入口含 watchdog 自动收归旧部署）；主菜单按需提权（经人工要求不再每次打开都要密码，写状态的
  菜单项才 sudo 并以 `--resume` 回到原处）；watchdog / 周更入口改用稳定调用路径并修复旧单元；dpkg 管理的
  安装自更新改为校验后 `dpkg -i`。独立安全复审的 CRITICAL / HIGH / 两项 MEDIUM 已修复（`CLASHDOCK_HOME`
  不随提权传递、`WriteRoot` 经标准输入、bashrc 经校验句柄读写、自更新以 API digest 为准、单元入口须整条
  路径只有 root 可写）。mars 复现旧版漏洞并确认关闭，现场已按快照恢复设置。记录：[2026-10-24-privilege-boundary.md](tasks/2026-10-24-privilege-boundary.md)
- T-014：部署前在状态目录的私有临时目录用即将部署的内核与 geo 执行 `mihomo -t`，通过后才覆盖运行时文件 /
  重启；失败时运行时与服务不变并提示处于校验阶段。记录：[2026-10-24-precheck-before-deploy.md](tasks/2026-10-24-precheck-before-deploy.md)
- T-016：系统模式 mihomo 以系统账户 `mihomo` + `AmbientCapabilities` 运行（不用命名空间加固，兼容无特权
  LXC），运行时仍归 root，只给服务一个家目录；旧单元由主菜单询问重新部署。mars 实机验证 TUN、面板、缓存
  迁移与探针正常。新增 T-020（非 root 下 `PROCESS-NAME` 只能识别服务账户进程，待人工取舍）。记录：[2026-10-24-nonroot-mihomo.md](tasks/2026-10-24-nonroot-mihomo.md)
- T-017：内核下载按 GitHub API 资产 `digest` 校验 SHA-256（镜像只负责传输），缓存不符时重下一次，无摘要时提示后沿用原行为。
- T-018：设置 `MIHOMO_BIN` 时，代表性生成配置（golden 补丁输入含 TUN 开 / 关，及其叠加 / 地区组变体，有 / 无 geo）经真实
  内核 `-t` 校验，测试离线（`geox-url` 指向不可达地址）；测试 CI 下载 mihomo 与 geo 数据后运行。修复
  `Precheck` 相对 geo 路径链接悬空。
- T-019：WORKFLOW 增加实机验证（mars、验证前快照、验证后比对恢复）、发布前门禁与发布后核对清单、
  「已归档（未验收）」状态及 done.md 写法；TEMPLATE「验证」表增加现场快照与现场恢复 / 比对两行。

## 2026-10-18

- 发布 v0.3.1：用户模式按需 TUN（T-009）、完整模式不再静默删除 sing-box（T-011）、初始化回退误报修复与内部重构；发布前按 MAINTENANCE 复查，无新清理候选。release 工作流通过，产出 amd64 / arm64 / armv7 的 tar.gz 与 .deb 及 checksums.txt；`/releases/latest` 返回 v0.3.1，amd64 包校验和与 `clashdock version` 核对无误。记录：[2026-10-18-release-v0.3.1.md](tasks/2026-10-18-release-v0.3.1.md)
- T-011：完整模式安装不再静默删除 `sing-box.service`——初始化时先询问（默认否），同意才停止并禁用（保留单元文件）；拒绝且端口被占用时明确报错并回退；`clashdock update` 等非交互路径只提示。单元测试与测试机实机验证通过。记录：[2026-10-18-singbox-conflict.md](tasks/2026-10-18-singbox-conflict.md)
- T-008：测试 CI 推送后首次运行通过（gofmt、vet、race 测试，47s）。
- T-010：完整模式在测试机实机验证通过——固定节点经 `sysd.SyncAndRestart` 三处落盘并跨重启保持，`clashdock update` 下载后完整 `Install`、服务使用新内核；修复 sudo 授权失败时 init 回退误报「回退失败」。测试机已恢复原状。记录：[2026-10-18-full-mode-verification.md](tasks/2026-10-18-full-mode-verification.md)
- T-009：用户模式可按需开启 TUN——主菜单确认后经 sudo `setcap` 给用户目录内核授予网络能力（可取消、不保存密码、失败回退并撤销能力），关闭无需 sudo，内核更新后询问重新授权；单元测试与真实 sudo 提示下的实机验证通过。新增 T-011（完整模式安装会删除 sing-box 服务）。记录：[2026-10-18-user-mode-tun.md](tasks/2026-10-18-user-mode-tun.md)
- 新增项目管理文档：开发约束入口 `docs/CONSTRAINTS.md`、任务流程 `docs/WORKFLOW.md`、长期维护
  `docs/MAINTENANCE.md`、任务记录模板 `Records/tasks/TEMPLATE.md`，以及 AI 助手入口 `AGENTS.md`；
  ARCHITECTURE 增加新增模块流程，ideas.md 登记已知任务，各 Markdown 顶部声明行数上限。
- 经人工同意确定各类文件行数上限（README 200、其它 Markdown 300、Shell 200、YAML 300、模板与
  Makefile 100，golden 数据 / go.sum / 法律文本豁免，接近 = 90%）；ARCHITECTURE §6.5 拆到 `docs/USERMODE.md`；README 的数据目录与目录结构改为摘要 + 链接（降到 90% 阈值以下）。
- T-001：确定清理周期（稳定版发布前 / 任务清空时 / 满 30 天）、文档语言（中文）与各任务优先级；新增 T-008（测试 CI）。
- T-007：首次清理——MODULARITY 包边界与命名示例更新、COMMANDS 用户模式条目精简、`.gitignore` 去除 Python 条目、
  删除一次性迁移脚本 `scripts/migrate-runtime-dir.sh`（可从 tag `v0.3.0` 取回）、更正「四类提示」等过时注释。
- 记录：[2026-10-18-project-management-docs.md](tasks/2026-10-18-project-management-docs.md)
- T-006：用户模式 linger 成功分支、`clashdock update`、自更新 0.2.6 → 0.3.0 与卸载实机验证通过；测试后已恢复 linger 与用户目录原状。记录：[2026-10-18-real-machine-verification.md](tasks/2026-10-18-real-machine-verification.md)
- T-005：主选择组节点选择在本机 systemd 用户实例 + 真实 mihomo 上实机验证，5 个场景全部通过（完整模式缺口登记为 T-010）。
- T-004：`internal/selfupdate/selfupdate.go`（526 行）拆为 selfupdate（公开 API 与 Update 流程）、release（发行版查询）、layout（版本目录 / 符号链接 / 首次接管 / 清理）、install（校验 / 解压 / 归一化 / 试跑）；`Update` 由 99 行降到 60 行（抽出 `downloadVerified` / `installVersion`），补充对应测试，覆盖率 44.7% → 51.5%。
- T-003：`internal/flows/nodeselect.go`（544 行）拆为 nodegroups（主组识别 / 成员分类）、nodestatus（测速 / 状态标签）、nodeselect（选择与持久化）三个文件；`pickNode` 由 97 行降到 56 行（抽出 `connectRuntime` / `buildRegionMenu` / `preferredIndex`）；测试按职责拆分并补充 helper 与 `collectMembers` 测试。
- T-002：`internal/kernel/kernel.go`（657 行）按职责拆为 kernel / release / download / update / archive / seed 六个文件（均 ≤ 170 行），`extract` 拆出 tar.gz / zip 两个 helper，内核解压抽为 `installGzipBinary`；新增该包首批单元测试，覆盖率 0% → 56.7%。

## 2026-09-30

- 便携模式更名为「用户模式」，作为第二种主流运行方式：免 root，把本体装到 `~/.local/bin`、数据放
  `~/.local/share/clashdock`，维护 `systemctl --user` 用户级服务（无 systemd 用户实例时退化为后台
  进程），不再需要前台驻留；节点、订阅、更新、卸载都围绕用户服务完成，旧便携数据可一键迁移。
- 新增可配置的控制器端口 `controller_port`；用户模式部署前检测端口占用并引导修改。
- 修复「部署设置」里修改端口以字符串落盘、实际监听不变的问题；修复 Web UI 更新后未重新部署的问题。
- 非 TTY 输入不再预读 stdin；补齐 20 条遗漏的英文翻译并加入翻译覆盖测试；更正过时的「运行时管理 →
  更新」菜单指引；移除便携包里只更新包目录的 `tool/update.sh`。
- README 安装方式精简为 .deb / 用户模式 / 源码构建；包内 `install.sh` 改为用户模式一节的附注。
- 发布 v0.3.0。
- 记录：[user-mode-service.tdd.md](../docs/testing/user-mode-service.tdd.md)

## 2026-08-22

- 节点切换与固定仅摘要展示主选择组；识别失败时可补充并验证关键词，空输入安全取消。
- 记录：[main-group-node-selection.tdd.md](../docs/testing/main-group-node-selection.tdd.md)

## 2026-08-21

- 调整初始化表单的 Enter 行为，仅在提交按钮上执行初始化。
- 统一配置保存、运行时同步、校验与服务重启链路，并显式返回同步错误。
- 在节点菜单中区分当前运行节点与配置首选，支持 API 不可用时降级展示。
- 记录：[ideas-preview-0.2.6-beta.2.tdd.md](../docs/testing/ideas-preview-0.2.6-beta.2.tdd.md)

## 之前改动

- 完善便携模式的双语使用说明、日志查看及按键退出体验。
- 为完整安装模式增加内核、TUI 日志查看入口和日志路径说明。
- 将更新功能移至工具菜单，并将运行时管理改为重建服务及订阅配置的“重载服务”。
- 优化服务暂停、重启后的组网工具恢复流程。
- 将首次初始化整合为表单，支持下载代理、端口、TUN、局域网代理、订阅及分流配置。
- 在配置变更中增加“重新初始化”，确认后删除原服务并重新执行初始化。
- 优化初始化表单的默认值、订阅类型选择和操作提示。
- 增加 TUN 直连 UID、进程名配置，避免代理影响 SSH 连接。
- 让局域网代理提示与实际代理端口保持一致。
- 为便携模式增加更新与测试脚本。
- 支持在订阅配置基础上扩充 `fake-ip-filter` 字段。

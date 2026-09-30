# 架构与设计

clashdock 是部署 / 管理 mihomo（Clash.Meta）的 Go 终端应用。本文记录核心设计决策与分层。

## 1. 核心理念：直用订阅 + 最小改写

mihomo 原生解析 Clash 配置，因此**不做协议转换、不重建分流**。机场订阅的
proxies / proxy-groups / rules / providers / dns 全部原样保留，只覆写部署必需字段：

- 本地入站：统一 `mixed-port`（定制层 `proxy_port`，默认 7890；删除冲突的 port/socks-port/redir/tproxy）
- 局域网：`allow-lan` / `bind-address`（由定制层 `lan_proxy` 决定）
- 控制器与面板：`external-controller`（默认收回 127.0.0.1，端口取定制层 `controller_port`，
  默认 9090）/ `external-ui` / `secret`
  （开 LAN 面板必须设 secret，否则拒绝）
- TUN：整段由部署层覆写（`enable_tun` / stack / 排除网段）
- `profile.store-selected: true`（选组持久化）；订阅缺 dns 段时注入最小 fake-ip 默认，
  已有 dns 段则保留订阅的 `fake-ip-filter`，再按原顺序追加定制层 `fake_ip_filter`；
  完全相同的规则稳定去重，订阅规则优先，且保留其它 DNS 字段

实现见 `internal/subscription/patch.go`；可选叠加层（AI / 流媒体规则、SG/HK 地区
url-test 聚合组）见 `overlay.go` / `regiongroups.go`，默认关闭，且与订阅自带分流共存。

## 2. 配置的表示：JSON 即 YAML

生效配置 `config.yaml` 的内容是 **JSON**（JSON 是合法 YAML，mihomo 直接解析）。
入站订阅用 yaml.v3 解析成 `map[string]any`，出站一律 JSON 写盘——省掉 YAML dumper，
也使 Go / 外部工具读取运行时配置只需 `json.Unmarshal`。

## 3. 状态与运行时分离

| 位置 | 内容 |
|---|---|
| `/var/lib/clashdock`（固定工作目录，`CLASHDOCK_HOME` 可覆盖） | 状态数据：内核、UI、规则、`subscriptions/<name>/{meta.json,raw.*,config.yaml}`、`active` 指针、`customize.json`。首次使用经 sudo 创建并交回调用者属主，root 定时器与用户会话共享同一份数据 |
| `/var/lib/clashdock-runtime`（v0.2.0 前为 `/etc/mihomo`，不再兼容旧路径） | root 态自包含运行时：内核 + 配置 + geo + UI 的暂存副本（服务与用户目录解耦） |
| `/etc/systemd/system` | `mihomo` / `mihomo-watchdog` / `mihomo-update` 三组单元（Web UI 走 mihomo 内置 `:9090/ui/` 路径，不再有独立面板单元） |

普通用户运行，特权操作全部经 `sudo` 子进程（`internal/execx`），凭证会话内缓存。

`sysd.SyncAndRestart`（订阅切换/刷新、节点固定等高频操作用）只重新同步
`config.yaml`，**不会**重新拷贝内核/geo 二进制文件到运行时目录——这些文件只有
完整的 `sysd.Install` 才会重新部署。因此「更新内核」「更新 geo 数据」
（`flows.updateCoreOnly`/`updateGeoOnly`）与每周定时器（`clashdock update`）
下载新资源后都必须走完整 `Install`，否则下载「成功」但服务其实还在用旧文件。
`sysd.RecordDeployedAssets` 在每次完整 `Install` 后记录 state 侧资源文件的
指纹（mtime+size）到 `<state>/runtime-deployed.json`；`sysd.AssetsStale` 据此
判断资源是否被下载更新过但运行时还没跟上——交互式主菜单每次启动都会检查一次，
是则询问是否现在重启应用最新资源。

## 4. 交互契约：esc 保存 / ^R 回退 + 事务

TUI（`internal/tui`，Bubble Tea）提供五类阻塞式提示：Select / MultiSelect / Ask /
Confirm / **Form**。Form 是单屏表单（`form.go`）：多字段汇总到同一盒子里一次填写、
统一提交，字段可随其它字段的当前值动态显隐/改标签（初始化用它，见第 4 节），终端
过矮时纵向滚动并按分节展示。每次调用运行一个内联 `tea.Program`，因此流程层保持
命令式结构。键位契约：

- **esc → ErrSaveExit**（保存并返回上层）
- **^R → ErrCancelled**（回退并返回）；`errors.Is(ErrSaveExit, ErrCancelled)` 成立
- 数字键跳选；菜单重入时光标停在上次选中项（`Initial`）
- 非 TTY 自动回退编号列表 + 文本输入，脚本可喂答案

事务（`internal/txn`）承载回退语义：`更改配置` 会话进入时快照
config.yaml / active / customize.json / subscriptions/，esc 提交、^R 整体还原并把
运行中的服务重新对齐；`初始化` 在主服务启动前逐步登记 undo（删订阅 / 卸服务 / 撤防火墙），
服务启动成功后提交核心模块，后续资源更新 / 伴生单元安装仅回退各自尚未提交的改动。
系统类操作（更新内核 / 重启服务等）标注「※即时」，不参与回退。
**初始化不再下载内核**：mihomo 内核与基础规则由安装包提供（`.deb` 种子接管，或
便携包 `install.sh` 装入系统路径，见第 6 节），`ensureStartupResources` 只检查本地
是否就绪、不联网下载；缺失即软失败（保留本次设置与订阅，提示补齐内核后重新执行
初始化），内核更新一律由用户在「工具 → 更新 → 内核」显式触发。geo 数据与 Web UI
（出于版权/体积原因未捆绑）仍在服务启动后自动补下载，不再询问是否下载/走代理——
服务已起，优先走本机 mixed-port（`kernel.Options.SkipCore` 跳过内核，见第 5 节）。
下载失败也**不回滚**已完成的 TUN/代理/局域网设置与已添加的订阅，只警告并提示去
「工具 → 更新」补下载，重新执行初始化即可（会识别已有订阅，跳过重复添加，
直接重试服务注册）。

主菜单原「更改配置」按「改动是否需要重启服务生效」拆成两个入口：`flows.ModifyConfig`
（订阅管理 + 定制层字段分组编辑，写的是 mihomo 运行配置本身）与 `flows.ModifyRuntime`
（节点切换 / 固定节点 / 服务设置 / 网络自愈 / 更新定时器，均即时生效；「更新」内核 /
geo / clashdock 自身已移到「工具」菜单）。`serviceSettings` 第三项为「重载服务（删除
并重建）」：仅重启仍异常时（如内核 TUN 与其它 overlay 网络抢路由后状态错乱，见下）
彻底删除 systemd 单元再完整 `Install` 重建——先按本地原文重生成生效订阅 config.yaml，
再删服务与运行时、重装内核/geo/配置并启动，等价于一次干净重来。「工具」菜单
（`flows.ToolsMenu`）聚合网络测试 / 最新日志（完整模式经 journald 读 mihomo 服务日志）/
更新 / 主要文件位置（含日志文件路径）/ 信息。初始化改由单屏 `tui.Form` 一次性收集
基础设置（下载代理 / TUN / 局域网 / bashrc / 防火墙端口 / 直连 UID / fake-ip-filter /
直连进程名）与首个订阅（`flows.runInitForm`，
字段按 TUN、局域网、订阅类型动态显隐），
两者共享同一套 `modifySession` 快照 + 回退骨架。定制层不再有单独的「编辑定制层」中间层：
`config.DeploymentFields`（部署设置）与 `config.OverlayFields`（自定义分流叠加）两个字段
分组直接是 `ModifyConfig` 菜单下的平级项（`flows.EditFieldGroup`），退出即保存本组已做的
修改；若存在生效订阅，会立即用本地原文重建 `config.yaml`，同步到运行时并重启服务。
节点切换与固定节点仅摘要展示已识别主选择组的当前状态，不再列出所有策略组；节点选择
流程按 `main_group_keywords` 识别主组，未命中时允许当场输入关键词，验证能命中 `select`
组后才插到关键词首位并写回 `customize.json`，空输入或无效输入不改配置。固定节点同样先
持久化到生效配置与订阅配置，再走统一的同步、校验、重启链路；所有这些改动仍由外层
会话的文件快照负责整体回退。启动第一步（完整模式与用户模式共用）是
`flows.EnsureLanguage`：仅当 `customize.json` 里**未显式设置过**语言（`config.LanguageConfigured`
读原始文件判定，非 Load 的默认补全值）、且未用 `CLASHDOCK_LANG` 指定时，才先弹语言
选择。之后完整模式再检测主服务未注册（`sysd.IsInstalled` 判定，已停止但单元文件仍在
也算已注册）时询问是否现在初始化，而不是自动强制进入。各级菜单选项按常用程度排列（日常操作在前，
卸载类低频/破坏性操作在后）；长提示语按终端宽度自动换行（`tui.wrapText`）；菜单序号统一
按整份菜单长度决定风格（`tui.numFor`：≤20 项整份带圈数字，否则整份普通数字），不会出现
同一菜单内前面带圈、后面变阿拉伯数字的情况。

**界面语言**（`internal/i18n`）：默认英文启动，主菜单「Language / 语言」可切中文，
写回 `customize.json` 的 `language` 字段（`CLASHDOCK_LANG=en|zh` 环境变量可覆盖，
优先级更高）。源码里的中文原文本身就是翻译表的 key（`i18n.T(zh string) string`），
中文模式下原样返回，英文模式下查表翻译；地区/分组匹配关键词等「数据」而非界面文案
一律不翻译，以免破坏对真实订阅内容的匹配。翻译只在函数体内的使用点调用，绝不在包级
`var`/`const` 初始化里调用（那会在语言设置生效前把结果烤死成英文）。
`internal/i18n/coverage_test.go` 用 go/ast 扫描全仓非测试源码里的 `i18n.T("字面量")`，
任何一条没登记英文翻译都会让测试失败。

端口类字段（`config.PortFields`：`proxy_port` / `controller_port` / `bootstrap_dns_port`）在
字段编辑器里按 1-65535 整数校验并以整数落盘；订阅改写与界面展示共用 `config.ProxyPort` /
`config.ControllerPort` 解析（兼容历史上以字符串落盘的值），避免「界面显示改了、实际监听没改」。

## 5. 下载通道：直连优先 → 代理兜底（内核/geo）；直连优先 → 可选代理（订阅）

`internal/fetchx.New`（内核/geo/UI 下载默认路径）：先探测直连（google
generate_204），可达则显式绕过一切代理（避免下载被静默隧道进本地 mihomo →
机场节点）；不可达才走配置的 `download_proxy`。支持重试、Range 续传、
gzip/tar/zip 完整性校验、GitHub 镜像前缀与 API Token。

`internal/fetchx.NewOrdered`（服务已启动之后的内核/geo 更新专用，
`kernel.Options.LocalProxyFirst`）：不做直连探测，直接按给定顺序尝试——本机
mixed-port（`127.0.0.1:7890`，走已生效订阅的节点，出海更稳）→ `download_proxy`
→ 直连兜底。

订阅拉取（`internal/subscription.Fetch`）默认方向相反：新增/刷新订阅时默认
**直连**（尽力而为，不设代理，但 TUN 模式下仍可能被路由劫持——选直连时若
TUN 开且服务在跑，会额外问是否临时暂停服务保证这次真正直连，结束后自动恢复，
见 `flows.askProxyChoice`）；用户主动选择走代理时才按本机 mixed-port →
`download_proxy` 的顺序尝试。

## 6. 离线种子（.deb / 便携包）

两种分发形态都自带 mihomo 内核与基础规则（geosite.dat + DB-IP country.mmdb），装完
离线即可初始化，初始化阶段不再下载内核：

- **`.deb`**：内核 → `/usr/libexec/clashdock`，规则 → `/usr/share/clashdock/ruleset`
  （落点见 `.goreleaser.yaml` 的 nfpm 契约）。`apt install` 直接铺好。
- **便携包（`.tar.gz`）**：主用途是用户模式——解压后直接运行 `./clashdock`，内核 +
  规则接管到用户数据目录（见 6.5）。附带的 `install.sh`（`scripts/portable/install.sh`）
  是给有 root 却不便用 `.deb` 的补充路径：把内核 + 规则 + 本体二进制装入与 `.deb`
  **完全一致**的系统路径，效果等同 `sudo apt install clashdock_*.deb`（支持 `DESTDIR`
  前缀，便于打包/测试；配套 `uninstall.sh` 移除系统文件、不动状态数据）。

无论哪种，`kernel.SeedFromSystem` 都在 state 缺失对应文件时从
`/usr/libexec/clashdock` / `/usr/share/clashdock/ruleset` 复制接管为初始资源。
刻意**不捆绑** geoip.metadb（GeoLite2 衍生物受 MaxMind EULA 新鲜度义务约束，
不适合冻结进安装包）与 Web UI；两者在服务启动后在线补下载，『更新 geo 数据』
也走在线获取。归属声明见 `packaging/copyright`（便携包内为 `deps/copyright`）。

## 6.5 用户模式（`internal/usermode` + `internal/runtimesvc`）

**设计目的**：给**拿不到 root** 的用户（实验室/机房/受管终端/多人共用服务器）一条与完整模式
并列的主流运行方式——全程不提权、不碰系统路径，但同样由服务在后台常驻，clashdock 只做管理、
不需要前台驻留。凡是需要 root 的能力（TUN、防火墙、局域网放行）在用户模式一律关闭，只做本机
纯代理（`enable_tun` / `lan_proxy` / `lan_panel` 每次进入用户模式都强制为 false，确有改动时用
本地原文重建生效订阅）。

- **布局**（`usermode/layout.go`，遵循 XDG）：本体 `~/.local/bin/clashdock`；数据
  `$XDG_DATA_HOME/clashdock`（默认 `~/.local/share/clashdock`，经 `CLASHDOCK_HOME` 传给
  `paths.Detect`，并置 `paths.Paths.UserMode=true`）；运行时 `<state>/runtime`（mihomo `-d`，
  geo 放根级、按需铺 `ui/`）；用户单元 `$XDG_CONFIG_HOME/systemd/user/clashdock-mihomo.service`。
  数据固定在用户主目录而非可执行文件旁，单元引用的路径因此不会随便携包目录移动/删除而失效。
- **安装与交接**（`flows.UserModeRun`）：从解压便携包启动时，先把旧版便携模式留在包目录旁的
  `clashdock-data` 复制迁移（仅当新位置还没有订阅，旧目录原样保留）→ `kernel.SeedFrom` 从
  `deps/` 接管内核与规则 → 把本体原子安装到 `~/.local/bin`（临时文件 + 改名；已装同版本跳过，
  不同版本询问替换）→ `syscall.Exec` 交接给已安装的本体 `clashdock user` 继续。此后自更新、
  卸载都作用于已安装本体，便携包目录可删除。非 TTY 回退的行读取逐字节进行、不预读 stdin
  （`tui.readLine`），交接后的新进程与 sudo 等子进程才能拿到后续输入。
- **部署**：内核缺失时也尝试 `kernel.SeedFromSystem`（机器上装过 .deb 时可只读接管）；检测
  `proxy_port` / `controller_port` 是否已被占用（多人共用机器上默认 7890 / 9090 常被别人占着），
  占用时默认进入改端口表单；添加/选择订阅后 `usermode.Deploy`：`StageRuntime` → `mihomo -t`
  校验 → 写单元并 `enable` + `restart`。校验不过不会写单元。
- **服务后端**（`usermode.Service` 接口）：
  - `systemd`：`systemctl --user` 管理，`Restart=on-failure`，`WantedBy=default.target`；
    ExecStart 直接运行 `<state>/bin/mihomo`（内核更新走临时文件 + 改名，不会与运行中的进程
    冲突），参数加引号并转义 `%`。部署后询问是否 `loginctl enable-linger`（登出后继续运行、
    开机自启；polkit 不允许时提示请管理员执行）。日志走用户 journal，易失性 journal 时退而按
    `--user-unit` 查。
  - `detached`：连不上 systemd 用户实例（容器、无 logind 的 SSH 会话）时的退化方案——内核以
    `setsid` 新会话脱离终端运行，clashdock 退出后由 init 收养；PID 写 `runtime/mihomo.pid`，
    日志写 `runtime/mihomo.log`（每次启动截断）；存活判定要求 `/proc/<pid>/cwd` 等于运行时
    目录，防止 PID 复用误杀无关进程。不提供开机自启与异常自动重启，界面会说明。
  - 后端选择：已部署的后端优先（单元文件 / `usermode-detached` 标记），都没有时有 systemd
    用户实例就用 systemd，否则 detached。
- **服务目标分发**（`runtimesvc.For(p)`）：订阅切换/刷新、固定节点、本地文件覆盖、配置会话
  回退、内核/geo/UI 更新后的重新部署都只依赖 `Runtime` 接口（Installed / Active /
  SyncAndRestart / Redeploy / Pause / Resume），完整模式映射到 `sysd`，用户模式映射到
  `usermode`。用户模式的运行时是普通文件复制，同步重启与完整重新部署走同一条
  「重铺运行时 → 校验 → 重启」路径。
- **菜单**（`flows/usermode_menu.go`）：节点切换 / 固定节点 / 配置变更（订阅管理 +
  `config.UserModeFields` 本地设置 + 自定义分流叠加，沿用 `modifySession` 的 esc/^R 事务）/
  启停服务 / 重启服务 / 工具（网络测试、最新日志、更新、How to Use 含文件位置）/ 语言 / 卸载。
  卸载勾选移除用户服务 / 已安装本体 / 全部数据（勾数据时自动带上服务）。
- **模式判定**（`usermode.Detect`，`cmd/clashdock` 传入 `sysd.IsInstalled` 结果）：
  `CLASHDOCK_MODE=user|service` 显式覆盖（旧值 `portable` 等同 `user`）> 可执行文件旁有
  `deps/mihomo`（解压便携包）→ 用户模式 > 已注册系统服务 → 完整模式 > 已部署用户服务 →
  用户模式 > 可执行文件就是 `~/.local/bin/clashdock` 或其自更新托管版本 → 用户模式 > 兜底
  完整模式。显式入口 `clashdock user`（旧名 `run` / `--portable`）无条件走用户模式；
  `init` / `healthcheck` 只属于完整模式，即使检测到用户模式也照常走系统服务。
- **自更新**：`selfupdate` 首次接管可执行文件为 `current` 符号链接时，目录可写（如
  `~/.local/bin`）就直接原子替换，只有 `/usr/bin` 等系统路径才走 sudo。
- 便携包 `tool/nettest.sh` 保留为不依赖 clashdock 的网络自测；原 `tool/update.sh`（只更新包
  目录内文件）已移除，更新统一走「工具 → 更新」或 `clashdock update`。

依赖方向：`usermode` 是领域包，不依赖 `flows` / `sysd`；`runtimesvc` 依赖 `sysd` 与
`usermode`，被 `flows` 与 `subscription` 调用。

## 7. 自愈与守护

- NetworkManager dispatcher 钩子：真实网卡 up / 连通性变化时防抖重启（忽略 tun 自身）
- watchdog 定时器 + `clashdock healthcheck`：仅当「有上行但代理探测不通」且服务
  已运行足够久才重启，避免重启风暴
- 暂停 / 启动把伴生单元一并带上（否则 watchdog 会把刚停的主服务拉起来）

**与 overlay 组网（EasyTier / Tailscale 等）共存的路由竞争**：TUN 模式下 mihomo 的
`auto-route` + `auto-detect-interface` 会在启动瞬间接管默认路由并锁定出口物理网卡。
若 mihomo 早于 / 与 EasyTier 抢先启动，EasyTier 到公网 peer / relay 的流量（公网 IP，
不在 `route-exclude-address` 的私网白名单内）会被 TUN 劫持进代理节点，NAT 打洞看到的
源地址变成代理出口，直连隧道建不起来。**这是配置 / 启动次序问题，不是 mihomo 内核
缺陷**：单纯 systemctl restart 往往不够（旧路由 / 已锁定网卡残留），而「暂停→再启动」
或主菜单「运行时管理 → 服务设置 → 重载服务（删除并重建）」能让 mihomo 在 EasyTier
已就位后重新探测正确物理网卡、按正确优先级重铺路由，故 EasyTier 随之恢复。根治方向：
①给 mihomo 单元加 `After=easytier.service` 之类的启动次序约束；②把 EasyTier 的虚拟网段
与 peer 网段并入 `tun_route_exclude_cidrs`（默认已排除 10/8 等私网，自定义网段需另加）。

## 8. clashdock 自更新

`internal/selfupdate`：版本化目录 `<state>/clashdock-versions/<version>/clashdock` +
`current` 符号链接方案，与 `internal/kernel` 更新 mihomo 内核/UI/geo 数据完全独立
（更新的是 clashdock 自身这个程序）。流程：查询最新 GitHub release → 下载对应架构的
`clashdock_<version>_linux_<arch>.tar.gz`（amd64 / arm64 / armv7）→ 按 `checksums.txt` 校验 SHA-256 → 解压到
独立版本目录 → 归一化出 `<version>/clashdock` → 试跑新二进制（`clashdock version`）
确认能执行 → 原子重写 `current` 符号链接 → 再次试跑确认成功；启动校验失败则回退
`current` 指向。首次自更新时若正在
运行的可执行文件（一般是 apt 装的 `/usr/bin/clashdock`）还不是托管符号链接，会先把它
原样迁移进版本目录作为基线版本，再把该路径替换成指向 `current` 的符号链接（这一步
需要 root，走 `execx.RunRoot`；此后的更新只需重写 `current`，不再需要碰 `/usr/bin`）。
只保留 `current` 指向的版本、紧邻的上一个版本、以及 `last-stable` 记录的稳定版
（如果三者不同），其余版本目录清理掉。

发行包同时也是便携包，GoReleaser 配了 `wrap_in_directory: true`，因此 tarball 里
的二进制实际位于 `clashdock_<version>_linux_<arch>/clashdock`。旧 updater 只检查
`<version>/clashdock`，所以会出现「下载和校验成功，但提示 clashdock 可执行文件未找到」
的稳定版故障；自更新解压后必须把包内二进制复制/提升到托管布局。

两条更新渠道：稳定版查 GitHub `/releases/latest`（该接口天然排除 prerelease/draft）；
预览版查 `/releases` 列表第一项（不论是否标了 prerelease，即仓库里创建时间最新的
发行版）。`.goreleaser.yaml` 配了 `release.prerelease: auto`：tag 带 semver 预发布
后缀（如 `v0.1.7-beta.1`）会被 GoReleaser 自动标记为 GitHub prerelease，不占用
"Latest release"、也不会被 `/releases/latest` 返回。每次稳定渠道更新成功都会把
`current` 同时记到 `<state>/clashdock-versions/last-stable`，供切到预览版后想
回退的用户一键切回（`RollbackToStable`）。

## 9. 包结构

```
cmd/clashdock         入口：子命令 + TUI 主菜单
internal/errs         ErrSaveExit / ErrCancelled 导航哨兵
internal/execx        日志、子进程、sudo
internal/paths        状态目录解析（CLASHDOCK_HOME > MIHOMO_DEPLOY_ROOT > XDG）
internal/config       customize.json：默认值 / 读写 / 字段元数据 / 脱敏
internal/txn          事务：BackupFile / Snapshot / TrackPath / AddUndo，LIFO 回滚
internal/tui          Bubble Tea 五类提示（Select/MultiSelect/Ask/Confirm/Form）+ 非 TTY 回退
internal/fetchx       HTTP 下载通道
internal/kernel       内核 / UI / geo 下载部署 + deb 种子接管
internal/selfupdate   clashdock 自更新：版本化目录 + 原子符号链接切换
internal/subscription 订阅域：fetch / detect / b64 / patch / overlay / regiongroups / manager
internal/sysd         systemd 三组单元（模板 go:embed）
internal/usermode     用户模式：模式判定 + XDG 布局 + 运行时铺设 + 用户级服务（systemd --user / detached）
internal/runtimesvc   服务目标分发：完整模式 → sysd 系统服务，用户模式 → usermode 用户服务
internal/clashapi     Clash API：切组 / 并发测延迟
internal/firewall     ufw > firewalld > nft > iptables 探测与放行
internal/proxyenv     ~/.bashrc 代理变量标记块
internal/i18n         中英文界面文案（默认英文，源码中文原文即翻译表 key）
internal/flows        流程编排（init（单屏表单 initform）/ modifyconfig / modifyruntime（含重载服务）/
                       tools（nettest+最新日志+更新+文件位置+信息）/ uninstall /
                       nodeselect（节点切换/固定节点）/ 定制层字段分组编辑 / 自更新 /
                       usermode（用户模式安装交接 + 菜单 + 卸载））
```

## 10. 测试策略

- 纯变换层（patch / overlay / regiongroups）以 **golden 对拍**锁定行为：
  `internal/subscription/testdata/golden.json` 是重写前实现的冻结兼容性基线，
  Go 测试要求语义等价（JSON 归一化后 DeepEqual）
- DNS patch 以定向单元测试覆盖 `fake-ip-filter` 的订阅优先追加、稳定去重、空定制保留
  与输入不可变，避免代理节点依赖的过滤规则在生成运行时配置时丢失
- txn / config / paths / tui 渲染 / b64 解析为常规单元测试（`go test ./...`）
- 用户模式：`usermode` 的模式判定、XDG 布局、单元渲染（引号/`%` 转义）、systemd 调用序列
  （注入假 runner）、detached 后端生命周期（假内核脚本）、本体安装/旧数据迁移为单元测试；
  flows 侧用假 `usermode.Service` 测卸载与 How to Use，不触碰真实 systemd 用户实例
- TUI 交互用 tmux `send-keys` / `capture-pane` 逐屏对拍验证

## 11. 模块化约束

新增或修改代码时遵守 [MODULARITY.md](MODULARITY.md)：普通 Go 文件目标
200-400 行，流程文件目标 150-300 行；超过软上限时优先同包拆分，避免把交互流程、
系统操作、下载逻辑和数据转换继续堆到单个文件中。

# 用户模式设计

> 行数上限：300

本文从 [ARCHITECTURE.md §6.5](ARCHITECTURE.md#65-用户模式internalusermode--internalruntimesvc) 拆出，是用户模式（`internal/usermode` + `internal/runtimesvc`）设计的权威说明。

**设计目的**：给**拿不到 root** 的用户（实验室/机房/受管终端/多人共用服务器）一条与完整模式
并列的主流运行方式——默认不提权、不碰系统路径，但同样由服务在后台常驻，clashdock 只做管理、
不需要前台驻留。防火墙、局域网放行在用户模式一律关闭（`lan_proxy` / `lan_panel` 每次进入用户模式
都强制为 false）；TUN 默认关闭，只在用户主动开启并经 sudo 授权后打开（见下文「按需 TUN」）。
确有改动时用本地原文重建生效订阅。

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
- **按需 TUN**（`flows/usermode_tun.go` + `usermode/tuncap.go`）：主菜单「开启 TUN 模式」先确认
  （默认否，防误触）→ 内核文件缺能力时经 `execx` 的 sudo 子进程执行
  `setcap cap_net_admin,cap_net_bind_service,cap_net_raw+ep <state>/bin/mihomo`（不保存密码）→
  写 `enable_tun` 并重建 / 重启 → 等待 `mihomo` TUN 设备出现。授权取消或失败时不做任何改动；
  之后任一步失败都回退配置，并 `setcap -r` 撤销本次授予的能力（家目录 nosuid / NFS 时能力不生效，
  靠设备检测发现）。关闭 TUN 不需要 sudo，能力保留在内核文件上，再次开启不再询问。能力直接读
  `security.capability` 扩展属性判定；进入用户模式时 `enable_tun` 只在内核仍带能力时保留。内核
  更新会换掉文件、能力随之丢失，重新部署前（`redeployIfInstalled` / `clashdock update`）询问
  重新授权，拒绝或失败则关闭 TUN。卸载数据目录即删除带能力的内核文件。
- **自更新**：`selfupdate` 首次接管可执行文件为 `current` 符号链接时，目录可写（如
  `~/.local/bin`）就直接原子替换，只有 `/usr/bin` 等系统路径才走 sudo。
- 便携包 `tool/nettest.sh` 保留为不依赖 clashdock 的网络自测；原 `tool/update.sh`（只更新包
  目录内文件）已移除，更新统一走「工具 → 更新」或 `clashdock update`。

依赖方向：`usermode` 是领域包，不依赖 `flows` / `sysd`；`runtimesvc` 依赖 `sysd` 与
`usermode`，被 `flows` 与 `subscription` 调用。

# 命令参考

clashdock 的**推荐用法是直接运行 `clashdock` 进入交互式终端**（TUI）——
所有功能都能在菜单里完成，无需记忆任何子命令。

以下子命令用于**脚本化 / 无人值守**场景（定时器、CI、远程批量运维），
效果与交互式菜单中的对应项一致。

## 子命令

| 命令 | 说明 |
|---|---|
| `clashdock` | **交互式主菜单（推荐）**：运行时管理 / 配置变更 / 工具 / 暂停启动 / 语言 / 卸载。启动第一步：配置文件未设置过语言（且未用 `CLASHDOCK_LANG` 指定）时先选语言；随后服务尚未注册时询问是否现在初始化。从解压的便携包目录启动、或已部署用户服务（且未注册系统服务）时进入用户模式（见 `user`） |
| `clashdock user` | **用户模式**（免 root；旧名 `run` / `--portable` 仍可用）：从便携包目录启动时先把本体装到 `~/.local/bin/clashdock`、内核与规则接管到 `~/.local/share/clashdock`（提示迁移旧版包目录旁的 `clashdock-data`），再交接给已安装本体；未部署时检测端口占用 → 添加/选择订阅 → 注册并启动用户级服务 `clashdock-mihomo.service`（`systemctl --user`，可选开启 linger；无 systemd 用户实例时退化为后台进程）。之后进入用户模式菜单：节点切换 / 固定节点 / 配置变更 / 启停 / 重启 / 工具 / 语言 / 卸载。纯本机代理，关闭 clashdock 不影响代理 |
| `clashdock init` | 初始化（首次部署）：下载代理→TUN→局域网→添加首个订阅（clash / base64 / 本地 YAML 文件三选一）→注册服务→可选更新内核/geo/UI。主服务启动前可 ^R 整体回退；启动后，资源更新与伴生单元安装按模块独立回退。交互式主菜单检测到服务未注册时询问是否触发初始化（语言选择已前移为启动第一步，仅在未设置过时触发）；此子命令主要供脚本/无人值守场景显式调用 |
| `clashdock modify` | 配置变更会话（对应主菜单「配置变更」）：订阅管理（添加订阅三种来源 + 本地文件覆盖）/ 部署设置 / 自定义分流叠加（esc 保存，^R 回退）。节点切换/内核更新/自更新/服务设置等即时生效操作仅在交互式主菜单「运行时管理」里，无对应子命令 |
| `clashdock nettest` | 网络测试：流媒体 / 站点 / AI 延迟（TTFB）+ OpenAI / Claude 出口 IP 落地（主菜单「工具」聚合了这个 + 主要文件位置一览） |
| `clashdock pause` | 暂停主服务及全部伴生单元（watchdog / 定时器一并停止，保持开机自启） |
| `clashdock resume` | 启动主服务及全部伴生单元 |
| `clashdock update` | 非交互全量更新：内核 + geo 数据 + Web UI 强制更新 → 服务同步重启（每周定时器的执行目标） |
| `clashdock uninstall` | 勾选式卸载：服务 / 自愈 / 定时器 / 产物 / 全部状态 |
| `clashdock version` | 显示版本 |

用户模式下（见 `user`；由启动上下文自动判定），`modify` / `nettest` / `pause` / `resume` /
`update` / `uninstall` 作用于用户服务：`pause` / `resume` 停止 / 启动用户服务，`update` 更新内核 +
geo + Web UI 后重铺运行时并重启用户服务，`uninstall` 勾选移除用户服务 / 已安装本体 / 全部数据，
`modify` 只开放用户模式可用的本地设置。`init` / `healthcheck` 始终属于完整模式。

非 TTY 下（管道 / 重定向）交互提示自动回退为编号列表 + 文本输入，
子命令因此也可以在脚本里喂答案：

```bash
printf '3\ny\n' | clashdock        # 例：进入主菜单第 3 项并确认
```

## 内部子命令

由 systemd 单元调用，一般无需手动执行：

| 命令 | 说明 |
|---|---|
| `clashdock healthcheck` | `mihomo-watchdog.service` 的执行目标；有上行但本地代理探测失败时重启主服务 |

## 环境变量

| 变量 | 说明 |
|---|---|
| `CLASHDOCK_HOME` | 覆盖数据目录（完整模式默认固定为 `/var/lib/clashdock`，用户模式默认 `~/.local/share/clashdock`；主要用于测试） |
| `CLASHDOCK_MODE=user\|service` | 强制运行模式（旧值 `portable` 等同 `user`），优先于启动上下文判定 |
| `XDG_DATA_HOME` / `XDG_CONFIG_HOME` | 用户模式的数据目录 / 用户级单元目录遵循 XDG 规范 |
| `CLASHDOCK_LANG=en\|zh` | 强制界面语言，优先级高于 `customize.json` 里保存的 `language` 字段（默认 `en`，部分终端无法正常显示中文） |
| `DOWNLOAD_PROXY` | 下载代理（customize.json 未配置时的回退） |
| `GITHUB_TOKEN` / `GH_TOKEN` | GitHub API Token（提升 release 查询限额） |
| `MIHOMO_NO_PROXY=1` | 强制下载全部走直连，禁用代理兜底 |
| `NO_COLOR` | 关闭彩色输出与 TUI 盒子（自动回退纯文本） |

## systemd 单元一览

由交互流程按需安装 / 卸载：

| 单元 | 作用 |
|---|---|
| `mihomo.service` | 主服务（`/var/lib/clashdock-runtime` 自包含运行时）；Web UI 走其内置的 `:9090/ui/` 路径，不再有独立面板服务 |
| `mihomo-watchdog.timer/.service` | 网络自愈探针（有上行但代理不通才重启） |
| `mihomo-update.timer/.service` | 每周自动更新（`clashdock update`） |

用户模式只有一个用户级单元（`systemctl --user`，位于 `~/.config/systemd/user/`）：

| 单元 | 作用 |
|---|---|
| `clashdock-mihomo.service` | 用户模式主服务（`~/.local/share/clashdock/runtime` 运行时，纯本机代理）；`systemctl --user status clashdock-mihomo` 查看状态 |

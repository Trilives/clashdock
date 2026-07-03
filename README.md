# clashdock

在 Linux 上交互式部署 / 管理 **mihomo（Clash.Meta）** 的终端应用。单个静态二进制，
全流程交互完成：**初始化 / 更改配置 / 暂停启动 / 网络测试 / 卸载**。

- **直用机场订阅**：mihomo 原生吃 Clash 配置，clashdock **直接消费机场的 Clash/mihomo
  订阅**，只最小改写部署必需字段（端口 / 局域网 / 外部控制器 / TUN / 面板），机场自带的
  策略组与分流规则**全部保留**。自定义分流为**可选叠加**，默认不启用。
- **开箱即用**：.deb 包内置 mihomo 内核与基础规则文件（geosite + IP 库），
  安装后**离线即可启动**；更大的规则数据（geoip.metadb 等）可稍后在线更新。
- **单文件零依赖**：Go 编译的静态二进制，不需要 python3 / curl / 任何运行时。
- **TUI 交互**：方向键导航、反显高亮；非 TTY（管道/脚本）自动回退编号菜单。
- **随时可中止可回退**：配置类改动包在事务里，**esc 保存退出、^R 回退退出**，
  已应用的改动自动回滚。
- **按需提权**：普通用户启动，需要 root 时自动 `sudo`。

## 安装

### 方式一：.deb（推荐，内置离线种子）

从 [Releases](https://github.com/Trilives/clashdock/releases) 下载对应架构的包：

```bash
sudo dpkg -i clashdock_*_linux_amd64.deb   # 或 arm64
clashdock
```

.deb 内含：`/usr/bin/clashdock`、mihomo 内核（`/usr/libexec/clashdock/mihomo`）、
基础规则种子（`/usr/share/clashdock/ruleset/`）。首次初始化会自动接管这些种子，
无需联网即可注册并启动服务。第三方资产的许可与归属见
`/usr/share/doc/clashdock/copyright`。

### 方式二：tar.gz（仅二进制）

```bash
tar -xzf clashdock_*_linux_amd64.tar.gz
./clashdock    # 内核 / 规则 / UI 由程序内『下载』步骤在线获取
```

### 方式三：源码构建

```bash
git clone https://github.com/Trilives/clashdock.git && cd clashdock
make build && ./clashdock
```

## 使用

**推荐方式：直接运行 `clashdock` 进入交互式终端**——这正是本项目的特点：
部署、订阅、切节点、服务管理全部在方向键菜单里完成，esc 保存返回、^R 回退返回，
无需记忆任何命令。

```bash
clashdock
```

脚本化 / 无人值守场景另有一组子命令（`init` / `modify` / `nettest` /
`pause` / `resume` / `update` / `uninstall` 等），详见
[docs/COMMANDS.md](docs/COMMANDS.md)。

## 预览

```
┌─ mihomo 部署系统 ────────────────────────┐
│                                          │
│  ❯ ① 初始化（首次部署）                  │
│    ② 更改配置                            │
│    ③ 暂停服务 ⏸                          │
│    ④ 网络测试                            │
│    ⑤ 卸载所有服务                        │
│                                          │
│  ↑/↓ 选择   ⏎ 确认   esc 退出   ^R 退出  │
└──────────────────────────────────────────┘
┌─ 更改配置 ──────────────────────────────────────────────┐
│                                                         │
│  ❯ ① 订阅管理（增 / 删 / 改名 / 切换 / 刷新）           │
│    ② 编辑定制层（TUN / 局域网 / 面板 / 自定义分流 …）   │
│    ③ 切换 / 固定节点 ※即时                              │
│    ④ 更新 内核 / UI / geo 数据 ※即时                    │
│    ⑤ 服务设置（重启 / 状态）※即时                       │
│    ⑥ 独立 Web 面板（根路径直开）※即时                   │
│    ⑦ 网络自愈设置 ※即时                                 │
│    ⑧ 每周更新定时器 ※即时                               │
│                                                         │
│  ↑/↓ 选择   ⏎ 确认   esc 保存并退出   ^R 回退并退出     │
└─────────────────────────────────────────────────────────┘
```

方向键上下移动、⏎ 确认、esc 保存返回、^R 回退返回；每层菜单重入时光标停在上次选中项。
非 TTY（管道/重定向）下自动回退为编号列表 + 文本输入。

## 功能一览

| 功能 | 说明 |
|---|---|
| 订阅管理 | 多订阅增/删/改名/切换/刷新；clash 与 base64（经 subconverter）两种来源 |
| 定制层 | TUN / 局域网代理 / LAN 面板 / 密钥（脱敏展示）/ 下载代理 / GitHub 镜像与 Token 等 23 项 |
| 节点切换 | 两级菜单（地区→节点）、Clash API 并发实测延迟、热切换 + 跨重启固定首选 |
| 地区聚合组 | 可选生成 SG-Auto / HK-Auto url-test 组，插入主选择组直接选用 |
| 自定义分流叠加 | 可选 AI / 流媒体 / 直连域名规则叠加（默认关，直用机场分流） |
| systemd 集成 | 主服务 + 独立 Web 面板 + 网络自愈 watchdog + 每周更新定时器，统一暂停/启动 |
| 网络自愈 | NetworkManager 钩子 + watchdog：断网/漫游后自动恢复，防重启风暴 |
| 网络测试 | 流媒体/站点/AI 延迟（TTFB）与 OpenAI/Claude 出口 IP 落地探测 |

## 数据目录

运行期所有数据使用**固定工作目录 `/var/lib/clashdock`**（不随用户 / HOME 变化，
root 运行的定时器与用户会话看到同一份数据；首次使用自动经 sudo 创建并交回属主）。
环境变量 `CLASHDOCK_HOME` 可覆盖（主要用于测试）。运行时自包含暂存于
`/etc/mihomo`，systemd 单元名沿用 `mihomo.service` 等。

## 目录结构

```
clashdock/
├── cmd/clashdock/          # 入口：子命令分发 + TUI 主菜单
├── internal/
│   ├── tui/                # Bubble Tea 交互组件（select/multiselect/ask/confirm）
│   ├── flows/              # 初始化 / 更改配置 / 网络测试 / 卸载 / 节点切换 等流程
│   ├── subscription/       # 订阅：拉取 / 识别 / 最小改写 / 分流叠加 / 地区聚合组
│   ├── kernel/  fetchx/    # 内核·UI·geo 下载（直连优先→代理兜底）与 deb 种子接管
│   ├── sysd/               # systemd 四组单元（服务/面板/自愈/定时器，模板内嵌）
│   ├── config/  txn/  …    # 定制层存取、事务回滚、路径、防火墙、代理环境变量
├── scripts/fetch-deb-deps.sh  # 打包前预下载 mihomo 内核与规则种子
├── packaging/copyright     # .deb 第三方资产许可与归属
└── .goreleaser.yaml        # tar.gz + .deb（amd64/arm64）发布流水线
```

架构与设计细节见 [ARCHITECTURE.md](ARCHITECTURE.md)；后续改动需遵守
[docs/MODULARITY.md](docs/MODULARITY.md)，避免单个文件持续膨胀。

## 许可

clashdock 以 [MIT](LICENSE) 发布。随 .deb 分发的第三方资产：
mihomo（MIT）、geosite.dat（GPL-3.0，MetaCubeX/meta-rules-dat）、
country.mmdb（DB-IP Country Lite，CC BY 4.0 —— *IP Geolocation by
[DB-IP](https://db-ip.com)*）。详见 `packaging/copyright`。

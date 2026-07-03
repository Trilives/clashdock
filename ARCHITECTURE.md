# 架构与设计

clashdock 是部署 / 管理 mihomo（Clash.Meta）的 Go 终端应用。本文记录核心设计决策与分层。

## 1. 核心理念：直用订阅 + 最小改写

mihomo 原生解析 Clash 配置，因此**不做协议转换、不重建分流**。机场订阅的
proxies / proxy-groups / rules / providers / dns 全部原样保留，只覆写部署必需字段：

- 本地入站：统一 `mixed-port: 7890`（删除冲突的 port/socks-port/redir/tproxy）
- 局域网：`allow-lan` / `bind-address`（由定制层 `lan_proxy` 决定）
- 控制器与面板：`external-controller`（默认收回 127.0.0.1）/ `external-ui` / `secret`
  （开 LAN 面板必须设 secret，否则拒绝）
- TUN：整段由部署层覆写（`enable_tun` / stack / 排除网段）
- `profile.store-selected: true`（选组持久化）；订阅缺 dns 段才注入最小 fake-ip 默认

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
| `/etc/mihomo` | root 态自包含运行时：内核 + 配置 + geo + UI 的暂存副本（服务与用户目录解耦） |
| `/etc/systemd/system` | `mihomo` / `mihomo-webui` / `mihomo-watchdog` / `mihomo-update` 四组单元 |

普通用户运行，特权操作全部经 `sudo` 子进程（`internal/execx`），凭证会话内缓存。

## 4. 交互契约：esc 保存 / ^R 回退 + 事务

TUI（`internal/tui`，Bubble Tea）提供四类阻塞式提示：Select / MultiSelect / Ask /
Confirm。每次调用运行一个内联 `tea.Program`，因此流程层保持命令式结构。键位契约：

- **esc → ErrSaveExit**（保存并返回上层）
- **^R → ErrCancelled**（回退并返回）；`errors.Is(ErrSaveExit, ErrCancelled)` 成立
- 数字键跳选；菜单重入时光标停在上次选中项（`Initial`）
- 非 TTY 自动回退编号列表 + 文本输入，脚本可喂答案

事务（`internal/txn`）承载回退语义：`更改配置` 会话进入时快照
config.yaml / active / customize.json / subscriptions/，esc 提交、^R 整体还原并把
运行中的服务重新对齐；`初始化` 逐步登记 undo（删订阅 / 卸服务 / 撤防火墙），
任意一步取消即逆序回滚。系统类操作（更新内核 / 重启服务等）标注「※即时」，不参与回退。

## 5. 下载通道：直连优先 → 代理兜底

`internal/fetchx`：先探测直连（google generate_204），可达则显式绕过一切代理
（避免下载被静默隧道进本地 mihomo → 机场节点）；不可达才走配置的 `download_proxy`。
支持重试、Range 续传、gzip/tar/zip 完整性校验、GitHub 镜像前缀与 API Token。

## 6. 离线种子（.deb）

.deb 内置 mihomo 内核与基础规则（geosite.dat + DB-IP country.mmdb），装包即可离线
初始化：`kernel.SeedFromSystem` 在 state 缺失对应文件时从
`/usr/libexec/clashdock` / `/usr/share/clashdock/ruleset` 复制接管。
刻意**不捆绑** geoip.metadb（GeoLite2 衍生物受 MaxMind EULA 新鲜度义务约束，
不适合冻结进安装包）；『更新 geo 数据』会在线获取。归属声明见 `packaging/copyright`。

## 7. 自愈与守护

- NetworkManager dispatcher 钩子：真实网卡 up / 连通性变化时防抖重启（忽略 tun 自身）
- watchdog 定时器 + `clashdock healthcheck`：仅当「有上行但代理探测不通」且服务
  已运行足够久才重启，避免重启风暴
- 暂停 / 启动把伴生单元一并带上（否则 watchdog 会把刚停的主服务拉起来）

## 8. 包结构

```
cmd/clashdock         入口：子命令 + TUI 主菜单
internal/errs         ErrSaveExit / ErrCancelled 导航哨兵
internal/execx        日志、子进程、sudo
internal/paths        状态目录解析（CLASHDOCK_HOME > MIHOMO_DEPLOY_ROOT > XDG）
internal/config       customize.json：默认值 / 读写 / 字段元数据 / 脱敏
internal/txn          事务：BackupFile / Snapshot / TrackPath / AddUndo，LIFO 回滚
internal/tui          Bubble Tea 四类提示 + 非 TTY 回退
internal/fetchx       HTTP 下载通道
internal/kernel       内核 / UI / geo 下载部署 + deb 种子接管
internal/subscription 订阅域：fetch / detect / b64 / patch / overlay / regiongroups / manager
internal/sysd         systemd 四组单元（模板 go:embed）
internal/clashapi     Clash API：切组 / 并发测延迟
internal/firewall     ufw > firewalld > nft > iptables 探测与放行
internal/proxyenv     ~/.bashrc 代理变量标记块
internal/flows        流程编排（init / modify / nettest / uninstall / nodeselect / 定制层编辑）
```

## 9. 测试策略

- 纯变换层（patch / overlay / regiongroups）以 **golden 对拍**锁定行为：
  `internal/subscription/testdata/golden.json` 是重写前实现的冻结兼容性基线，
  Go 测试要求语义等价（JSON 归一化后 DeepEqual）
- txn / config / paths / tui 渲染 / b64 解析为常规单元测试（`go test ./...`）
- TUI 交互用 tmux `send-keys` / `capture-pane` 逐屏对拍验证

## 10. 模块化约束

新增或修改代码时遵守 [docs/MODULARITY.md](docs/MODULARITY.md)：普通 Go 文件目标
200-400 行，流程文件目标 150-300 行；超过软上限时优先同包拆分，避免把交互流程、
系统操作、下载逻辑和数据转换继续堆到单个文件中。

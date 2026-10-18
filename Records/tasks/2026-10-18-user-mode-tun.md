# T-009 用户模式按需 sudo 授权开启 TUN

> 行数上限：300

## 概要

- 任务：T-009，见 [done.md](../done.md) 2026-10-18
- 日期：2026-10-18
- 负责人：AI 助手（Claude Code，Claude Opus 5.5）
- 范围：`internal/usermode`（tuncap）、`internal/flows`（usermode_tun / usermode / usermode_menu / modify）、
  `internal/execx`（`EnsureSudoNoHint`）、`cmd/clashdock`（`clashdock update` 前重新授权）、i18n、文档

## 实施步骤

1. 选定机制：经 sudo 给用户模式内核文件授予文件能力（`setcap`），用户服务仍以普通用户运行。
2. 实现能力检测（读 `security.capability`）、开启 / 关闭 / 回退流程、内核更新后的重新授权。
3. 单元测试覆盖成功、确认取消、sudo 取消、setcap 失败、重启失败与设备未出现的回退、关闭、重新授权。
4. 在测试机上以真实 sudo 密码提示（pty）实机验证，结束后卸载并恢复。

## 关键决策

| 决策 | 备选方案 | 选择理由 | 是否经人工确认 |
|---|---|---|---|
| 文件能力（setcap）授予内核 | 改为注册系统服务；sudo 常驻进程 | 不需完整安装、服务仍为用户级；特权操作只有一条 setcap | 否（任务验收标准限定方向） |
| 关闭 TUN 不撤销能力 | 关闭时 sudo setcap -r | 关闭无需密码；再次开启不再询问；更新或卸载内核即失效 | 否 |
| 开启后检测 TUN 设备 | 只看 setcap 返回值 | nosuid / NFS 家目录上 setcap 可能成功但能力不生效 | 否 |
| 不新增包 | 新建 tun 包 | 能力检测属 usermode 领域，交互属 flows，无需新模块提案 | 否 |

## 改动内容

- 新增 `usermode/tuncap.go`、`flows/usermode_tun.go` 及测试；主菜单新增「开启 / 关闭 TUN 模式」。
- `ensurePureProxy` 只在内核带能力时保留 `enable_tun`；内核更新后重新部署前调用 `ReconcileUserTun`。
- `execx.EnsureSudoNoHint`：用户模式不提示「直接用 sudo 启动」（以 root 启动会改用 root 的数据目录）。
- 文档：USERMODE.md「按需 TUN」、ARCHITECTURE §6.5、README 用户模式说明。

## 验证

单元测试：`go test -race ./...` 通过；`usermode` 覆盖率 71.5%。实机（Ubuntu 24.04，systemd 用户实例，
mihomo v1.19.27 → v1.19.32，订阅 `MATCH,DIRECT`；pty 下真实 sudo 提示；20 分钟兜底定时器自动停服务防失联）：

| 场景 | 结果 |
|---|---|
| 选择开启后在确认处选否 | PASS：无 sudo 提示；`customize.json` / `config.yaml` 哈希不变，无能力、无 TUN 设备 |
| 确认后连续 3 次输错 sudo 密码 | PASS：提示未取得授权；配置哈希、能力、设备、服务状态均不变 |
| 确认后输入正确密码 | PASS：内核带三项能力；服务进程以普通用户运行、CapEff=0x3400；`mihomo` 设备与路由规则出现；流量经 TUN（连接类型 Tun）；SSH 不受影响 |
| 关闭 TUN | PASS：无 sudo；设备与 2022 路由表规则消失，服务继续运行 |
| 再次开启、重启 clashdock | PASS：不再询问密码；重新进入用户模式后 TUN 保持开启 |
| 「工具 → 更新 → 内核」 | PASS：新内核不带能力 → 询问重新授权 → 输入密码后重新授予，新内核 v1.19.32 下 TUN 正常 |
| 卸载（全选） | PASS：用户服务、本体、数据目录删除；无 TUN 设备与残留规则 |

恢复核对：`~/.local/bin` 只剩原有工具；无用户单元与数据目录；Linger 保持原值 yes；ip rule、ufw 同基线。

## 遗留问题

- [ ] 「开启 TUN需要管理员权限」中英文间缺空格属既有 `EnsureSudo` 拼接方式，未改。
- `EnsureSudoNoHint` 改动在实机验证之后，仅经单元测试与构建检查（只影响一条提示）。

# 用户模式（免 root 用户级服务）TDD 证据

## 来源与用户场景

- 来源：[Records/ideas.md](../../Records/ideas.md) 2026-09-30 待办：「把便携版改个名字，作为第二种
  主流运行方式。便携版不需要 sudo 权限，但维护一个用户级服务，不再需要前台驻留，所有功能围绕这个
  用户服务开展，包括更新卸载等」「检查有没有需要优化的点」。
- 便携模式更名为**用户模式**：解压便携包运行 `./clashdock` 即把本体装到 `~/.local/bin`、数据放
  `~/.local/share/clashdock`，注册 `systemctl --user` 单元 `clashdock-mihomo.service`（无 systemd
  用户实例时退化为 setsid 后台进程）；clashdock 退出不影响代理，节点 / 订阅 / 更新 / 卸载都围绕
  用户服务完成。设计见 [ARCHITECTURE.md §6.5](../ARCHITECTURE.md)。

## RED / GREEN

| 阶段 | 命令 | 结果 | 证据 |
|---|---|---|---|
| RED | `go test ./internal/subscription -run Port` | FAIL | 字符串形式的 `proxy_port` 被静默回退为 7890；`controller_port` 不生效（`patch_ports_test.go`） |
| GREEN | 同上 | PASS | 订阅改写改用 `config.ProxyPort` / `config.ControllerPort` 共享解析 |
| RED | `go test ./internal/tui -run ReadLine` | FAIL | 非 TTY 行读取器预读 stdin，exec 交接后的进程丢失后续输入（`plainline_test.go`） |
| GREEN | 同上 | PASS | 改为逐字节读到换行的 `readLine` |
| RED | `go test ./internal/i18n` | FAIL | 新增的 `coverage_test.go` 扫出 84 条未登记英文翻译（含 20 条历史遗漏） |
| GREEN | 同上 | PASS | 补齐翻译 |
| GREEN | `go test -race ./...` | PASS | 全仓 |

## 测试保证

| 保证 | 测试类型 | 位置 | 结果 |
|---|---|---|---|
| 模式判定优先级（环境变量 / 便携包 / 用户本体 / 系统服务 / 用户服务） | 单元 | `internal/usermode/detect_test.go` | PASS |
| XDG 布局（相对路径的 XDG 变量被忽略） | 单元 | `internal/usermode/detect_test.go` | PASS |
| 用户单元渲染：路径加引号，`%` / `$` 转义，无 root 专属设置 | 单元 | `internal/usermode/systemd_test.go` | PASS |
| systemd 调用序列、enable 失败不重启、立即退出报错带日志 | 单元（假 runner） | `internal/usermode/systemd_test.go` | PASS |
| stop 失败时保留单元；单元未加载时照常删除 | 单元 | `internal/usermode/systemd_test.go` | PASS |
| 校验失败不写单元；部署前铺好运行时 | 集成（假内核） | `internal/usermode/systemd_test.go` | PASS |
| detached 后端生命周期、立即退出报错、外来 PID 不误判 | 集成（假内核脚本） | `internal/usermode/detached_test.go` | PASS |
| 本体原子安装不写穿符号链接；旧便携数据迁移且不覆盖新数据 | 单元 | `internal/usermode/install_test.go` | PASS |
| 卸载拒绝删除 `/`、主目录及非 clashdock 目录 | 单元 | `internal/usermode/install_test.go` | PASS |
| 端口占用检测 | 单元 | `internal/usermode/ports_test.go` | PASS |
| 服务目标按模式分发 | 单元 | `internal/runtimesvc/runtimesvc_test.go` | PASS |
| 用户模式字段不含 TUN / 局域网等 root 或对外暴露项 | 单元 | `internal/config/config_test.go` | PASS |
| How to Use、卸载勾选联动、纯代理收敛、同版本不重装 | 单元（假 Service） | `internal/flows/usermode_test.go` | PASS |
| CLI 分发：`init` / `healthcheck` 始终走完整模式 | 单元 | `cmd/clashdock/usermode_test.go` | PASS |
| 首次自更新在用户模式下免 sudo 接管本体 | 单元 | `internal/selfupdate/selfupdate_test.go` | PASS |

## 实机验证（本机 systemd 255 用户实例 + 真实 mihomo v1.19.27）

以非 TTY 回退喂答案驱动，测试结束后均经 `clashdock uninstall` 清理，主目录恢复原状：

- 从模拟解压包 `./clashdock` 启动：安装本体 → exec 交接 → 检测到本机 7890 / 9090 已被占用并引导
  改为 17890 / 19090（两端口相同被拒并重新询问）→ 本地 YAML 订阅 → 用户单元 active，
  `curl -x 127.0.0.1:17890` 返回 200，`127.0.0.1:19090/version` 可达。
- `clashdock pause|resume`、工具 → 最新日志（用户 journal）、节点切换（Clash API）、固定节点（写盘 →
  重铺运行时 → 重启）、配置变更改端口（以整数落盘、监听随之迁移）。
- 旧便携数据迁移 + 无 systemd 用户实例（清空 `DBUS_SESSION_BUS_ADDRESS` / `XDG_RUNTIME_DIR`）：
  自动走 detached 后端，内核被 init 收养、独立会话，pause/resume 正常，卸载后无残留进程。

## 覆盖率与已知缺口

- `internal/usermode` 70.4%；`internal/flows` 14.1%（终端交互编排为主，沿用 tmux / 非 TTY 实机验证）。
- linger 开启路径依赖 polkit 策略，只做了「拒绝开启」分支的实机验证。
- `clashdock update` / 自更新需要联网拉取发行版，本轮未实机跑，重新部署路径与固定节点共用已验证的
  「重铺运行时 → 校验 → 重启」链路。

# T-012 / T-013 / T-015 系统模式特权边界

> 行数上限：300

## 概要

- 任务：T-012（P0）、T-013、T-015，见 [done.md](../done.md) 2026-10-24
- 日期：2026-10-24
- 负责人：AI 助手（Claude Code，Claude Opus 5.5）
- 范围：`internal/paths`（secure.go、paths.go）、`internal/flows`（system_state.go、nettest.go 等）、
  `cmd/clashdock`（privilege.go、main.go）、`internal/execx`（elevate.go、trust.go、log.go、execx.go）、
  `internal/sysd`（unit_repair.go、timer.go、resilience.go）、`internal/selfupdate`（deb.go 等）、
  `internal/proxyenv`、凭据写入方（config、subscription、nodeselect、import_config、usermode/runtime）

## 实施步骤

1. **状态目录收归 root**：`paths.SecureTree` 在首个 root 入口（含每 2 分钟的 watchdog）把
   `/var/lib/clashdock` 收归 `root:<管理员私有组>`——去组 / 其他写；外来属主、可写或多硬链接的文件
   换新 inode 重建；删除指向树外的符号链接与 FIFO；属组非 root 时目录带 setgid，新文件继承属组。
   凭据文件 `0640`、订阅目录 `0750`（`SecretFileMode` / `SecretDirMode`），`WriteSecretFile` 同目录
   临时文件 + rename；日志 `0640`、`O_NOFOLLOW` 打开。
2. **按需提权**：主菜单以普通用户启动并直接读设置；写状态的菜单项经 `flows.RequireRoot` 先 `sudo -v`
   （失败留在菜单），再 `exec sudo --preserve-env=… <稳定路径> --resume=<项>`，root 进程直接进入该项。
   只有状态缺失、设置不可读或未设语言时才在启动时提权；`init` / `modify` / `uninstall` / `update` /
   `pause` / `resume` 启动即提权，`nettest` 仅在设置不可读时提权，`healthcheck` 从不提权。
3. **稳定入口（T-013）**：`execx.StableExecutablePath` 记录用户调用的路径（argv[0] / PATH，不解析符号
   链接）；`sysd.RepairCompanionUnits` 把旧版本写进版本目录的 watchdog / 周更入口改回稳定路径。
4. **不再改写 dpkg 文件（T-015）**：dpkg 管理的安装自更新改为下载同版本 `.deb`、校验后 `dpkg -i`、
   试跑确认版本；已被改成符号链接的旧部署可经「工具 → 更新」重装恢复，恢复后版本目录随 root 入口
   清理；非 dpkg 安装保留版本目录机制。
5. **安全复审**（独立 security-reviewer 代理，按「攻击者 = 管理员账户内的代码 / mihomo 账户」审查全部
   改动）后修复：见下方「复审结论」。
6. 单元测试；mars 实机复现旧版漏洞、验证收归与按需提权、最终快照包升级与现场恢复。

## 关键决策

| 决策 | 备选方案 | 选择理由 | 是否经人工确认 |
|---|---|---|---|
| 按需提权（菜单项进入时才 sudo，`--resume` 回到原处） | 照搬 sboxkit 启动即整体提权 | 人工要求「不要让系统模式每次打开都要密码，只在必要时」 | 是（2026-10-24） |
| 状态属组 = 管理员私有组（组名 = 用户名、gid = 主组） | 新建 clashdock 组 / 0600 只给 root | 管理员不经 sudo 可读设置与日志，其他用户读不到；不新增系统组 | 否 |
| 首个 root 入口自动收归，watchdog 兜底 | 只在 init / 升级时迁移 | 已有部署升级后最迟 2 分钟完成，迁移失败不影响服务 | 否 |
| `CLASHDOCK_HOME` 不随提权传递 | 照传 | 复审发现：能改管理员 shell 配置即可让 root 收归 / 执行任意目录 | 否 |
| 单元入口须整条路径只有 root 可写 | 只看文件属主 | 中间跳的符号链接所在目录可写同样可被替换 | 否 |
| 自更新以 API `digest` 为准，无摘要时 `checksums.txt` 直连 GitHub | 包与校验和都经镜像 | 被篡改的镜像能给出相符的一对，而包以 root 安装 | 否 |

## 改动内容

- 代码：上列实施步骤；新增 `paths/secure.go`、`flows/system_state.go`、`cmd/clashdock/privilege.go`、
  `execx/elevate.go`、`execx/trust.go`、`sysd/unit_repair.go`、`selfupdate/deb.go`。
- 文档：README「两种运行方式」、ARCHITECTURE §3 权限模型与 §8 自更新、COMMANDS「权限」与环境变量、
  USERMODE 数据目录权限。

## 复审结论

| 编号 | 问题 | 处理 |
|---|---|---|
| CRITICAL-1 | `CLASHDOCK_HOME` 经 `--preserve-env` 进入 root，root 会收归 / 执行该目录 | 已修复：移出保留列表，补测试 |
| HIGH-2 | 未提权的 `WriteRoot` 先写调用者临时文件再 `sudo install`，可被同用户进程替换 | 已修复：内容经标准输入交给 `install -m … /dev/stdin` |
| HIGH-3 | root 代用户改 `~/.bashrc` 时读取跟随符号链接（可泄露 root 文件、在 FIFO 上阻塞） | 已修复：同一 `O_NOFOLLOW\|O_NONBLOCK` 句柄 fstat 校验后读写 |
| HIGH-4 | 自更新的 `checksums.txt` 与包同经镜像，`dpkg -i` 以 root 执行 | 已修复：优先 API `digest`，回退时校验和直连 GitHub |
| MEDIUM-5 | 写进 root 单元的入口来自 argv[0] / PATH，可能在用户可写目录 | 已修复：`execx.TrustedExecutablePath` 逐跳检查 |
| MEDIUM-6 | 收归保留旧版用户可写文件的内容（如 `bin/mihomo`） | 不修：旧版下同一攻击者已能经 watchdog 取得 root，非回退；`clashdock update` 会重下内核 |
| MEDIUM-7 | 受限 sudoers 可能拒绝 `--preserve-env` | 不修：`ALL` 规则隐含 SETENV（mars 实测通过）；仅在变量已设置时请求 |
| MEDIUM-8 | root `mihomo -t -d <运行时>` 可能跟随服务账户目录里的链接 | 实测排除：strace 显示 `-t` 不打开 provider / cache 文件 |
| LOW-9~12 | 导入配置路径、`clashdock-last-stable` 权限、过期判定、tag 字符 | 不修：均需管理员本人操作或只涉及版本号 |

## 验证

| 检查 | 命令或方式 | 结果 |
|---|---|---|
| 格式 | `gofmt -l cmd internal` | 无输出 |
| 静态检查 | `go vet ./...` | 通过 |
| 测试 | `MIHOMO_BIN=… go test -race ./...` | 全部通过 |
| 现场快照 | mars（Ubuntu 22.04，systemd 249，管理员 mars uid/gid 1000）：clashdock 0.3.1 `.deb`、mihomo 以 root 运行、状态 `mars:mars`、凭据 0644；状态 / 运行时 / 单元打包到 `/root/clashdock-backup-20261024/`，并留 0.3.1 `.deb` | 已记录 |
| 旧版漏洞复现 | 0.3.1 下以 mars 打开 `enable_log` 并预置 `clashdock.log → ~/t012-proof` | root watchdog 跟随链接创建了 root 属主文件（漏洞成立） |
| 收归 | 安装快照包后等待 watchdog（21:31:22） | `root:mars`、目录 setgid、凭据 0640、订阅 2750；陷阱链接删除、日志重建为 root 0640；mars 可读不可写，nobody 不可读 |
| 按需提权 | pty 驱动、口令只经 stdin | 打开主菜单 0 次口令；拒绝重新部署 0 次；同意 1 次并回到原处；配置变更 1 次直达；工具 → 信息 / 日志 0 次；普通用户 `clashdock nettest` 0 次 |
| 复审修复后 | 运行时管理 → 网络自愈 → 调整间隔；`clashdock update` | 各 1 次口令；单元经 stdin 写入，`ExecStart=/usr/bin/clashdock healthcheck` 通过可信路径检查；提权提示按中文显示 |
| 现场恢复 / 比对 | 与快照比对 | 预期差异：版本为快照包、mihomo 以 `mihomo` 运行（见 [T-016 记录](2026-10-24-nonroot-mihomo.md)）、状态收归 root；`customize.json` 已用验证前原文件还原（`enable_log` 回到 false），测试日志与临时文件已删除；代理 / TUN 探针同基线 |

旧版的「替换二进制 → watchdog 以 root 执行」与已复现的日志链接同源（root 信任用户属主的状态目录）；
mars 的 0.3.1 未做过自更新，`/usr/bin/clashdock` 仍归 dpkg，因此以日志链接复现，并在新版下确认 mars
对状态目录（含 `bin/mihomo`）无写权限。

## 遗留问题

- [ ] mars 仍运行快照包 `0.3.1~SNAPSHOT-5f66466`，下个正式版发布后按 WORKFLOW §8 升级核对；
      回退方式：`/root/clashdock-backup-20261024/` 中的 0.3.1 `.deb` 与状态备份。

# T-016 系统模式 mihomo 以非 root 运行

> 行数上限：300

## 概要

- 任务：T-016，见 [done.md](../done.md) 2026-10-24
- 日期：2026-10-24
- 负责人：AI 助手（Claude Code，Claude Opus 5.5）
- 范围：`internal/sysd`（service_user.go、service.go、assets/mihomo.service.tmpl）、`cmd/clashdock`（重新部署询问）

## 实施步骤

1. `ensureServiceUser`：缺失时创建系统账户 `mihomo`（同名私有组、无家目录、nologin）。
2. 运行时布局：`/var/lib/clashdock-runtime` 仍只归 root（内核、geo、面板只读；配置 `root:mihomo 0640`）；
   服务账户只拥有 `<运行时>/home`（`0700`，mihomo 的 `-d`：cache.db、provider 下载）。旧版本在运行时
   根目录的 cache.db 只在家目录还没有时搬入一次（`mv -T` + `chown -h`，不跟随链接）。
3. 单元：`User=` / `Group=mihomo`、`AmbientCapabilities` 与 `CapabilityBoundingSet` 为
   `CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW`、`NoNewPrivileges` / `RestrictSUIDSGID` /
   `RestrictRealtime` / `LockPersonality`、`UMask=0077`；`ExecStartPre` 以服务身份把 geo 链接进家目录；
   `Environment=SAFE_PATHS=<运行时>` 放行面板路径。
4. `sysd.UnitOutdated`：已装单元与当前版本会写出的不一致（如旧版 `User=root`）时，主菜单询问重新部署。
5. 面板复制改为 `cp -R --preserve=timestamps`，不把状态目录的属组与 setgid 带进运行时。

## 关键决策

| 决策 | 备选方案 | 选择理由 | 是否经人工确认 |
|---|---|---|---|
| 不用 `ProtectSystem=` / `PrivateTmp=` 等命名空间选项 | 全套加固 | 无特权 LXC 里服务会以 226/NAMESPACE 启动失败，这类 VPS 常见 | 否 |
| 运行时根目录归 root，只给服务一个家目录 | 整个运行时交给服务账户 | root 执行的 `mihomo -t` 与部署只碰 root 属主文件 | 否 |
| `SAFE_PATHS` 放行运行时目录 | 把面板复制进家目录 | mihomo 要求面板落在 `-d` 或 `SAFE_PATHS` 内（实测）；面板保持 root 只读 | 否 |
| 升级后询问重新部署，不自动重启 | 静默迁移 | 重启会短暂断网，由管理员选择时机 | 否 |

## 改动内容

- 代码：`sysd/service_user.go`（新）、`sysd/service.go`（`deployRuntime`）、单元模板、`cmd/clashdock/main.go`
  （`maybeOfferRedeploy`）。
- 测试：`sysd/service_user_test.go`（账户创建、配置权限、布局、单元渲染与禁用项、布局失败中止、cache 迁移两例）。
- 文档：ARCHITECTURE §3 权限模型、COMMANDS 单元一览。

## 验证

| 检查 | 命令或方式 | 结果 |
|---|---|---|
| 格式 / 静态检查 / 测试 | `gofmt` / `go vet` / `go test -race ./...` | 通过 |
| 现场快照 | 见 [T-012 记录](2026-10-24-privilege-boundary.md)：0.3.1 下 mihomo 以 root 运行，TUN 与局域网代理开启 | 已记录 |
| 实机验证 | mars 主菜单同意重新部署（1 次口令） | 创建 `mihomo`（uid 999 / gid 998）；进程 `mihomo:mihomo`、CapEff `0x3400`、NoNewPrivs 1；TUN 设备与 ip rule 正常；面板 200；cache.db 已迁入家目录；无错误日志 |
| 内核更新 | `clashdock update`（1 次口令） | 内核 v1.19.33 经 SHA-256 校验部署，服务仍以 `mihomo` 运行；`runtime/ui` 为 `root:root` |
| 探针 | 7890 代理与 TUN 直连各访问 github / google / cloudflare | 与基线一致（200 / 204）；mars 自带 curl 7.81 + OpenSSL 3.0.2 访问部分 HTTPS 站点报 TLS decode error，与 clashdock 无关，改用 Python 核对 |
| 现场恢复 / 比对 | 与快照比对 | 预期差异：服务身份、单元、`mihomo` 账户；其余同 T-012 记录 |

## 遗留问题

- [ ] 非 root 的 mihomo 读不到其他用户进程的 `/proc/<pid>/fd`，`PROCESS-NAME` 规则（含 `tun_exclude_process`
      生成的直连规则）只能识别以 `mihomo` 运行的进程（本机实测：同 uid 的 curl 命中，root 的 curl 未命中）。
      mars 上 root 版本同样从未记录到进程名，SSH 在 TUN 下正常；已登记 T-020 由人工决定取舍。
- [ ] 卸载不删除系统账户 `mihomo`（与常见软件包一致，避免 uid 被复用后残留文件归属错乱）。

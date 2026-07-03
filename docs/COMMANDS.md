# 命令参考

clashdock 的**推荐用法是直接运行 `clashdock` 进入交互式终端**（TUI）——
所有功能都能在菜单里完成，无需记忆任何子命令。

以下子命令用于**脚本化 / 无人值守**场景（定时器、CI、远程批量运维），
效果与交互式菜单中的对应项一致。

## 子命令

| 命令 | 说明 |
|---|---|
| `clashdock` | **交互式主菜单（推荐）**：初始化 / 更改配置 / 暂停启动 / 网络测试 / 卸载 |
| `clashdock init` | 初始化（首次部署）：下载代理→TUN→局域网→内核与规则→订阅→注册服务，全程可 ^R 回退 |
| `clashdock modify` | 更改配置会话：订阅管理 / 定制层 / 节点切换 / 更新 / 服务设置…（esc 保存，^R 回退） |
| `clashdock nettest` | 网络测试：流媒体 / 站点 / AI 延迟（TTFB）+ OpenAI / Claude 出口 IP 落地 |
| `clashdock pause` | 暂停主服务及全部伴生单元（面板 / watchdog / 定时器一并停止，保持开机自启） |
| `clashdock resume` | 启动主服务及全部伴生单元 |
| `clashdock update` | 非交互全量更新：内核 + geo 数据 + Web UI 强制更新 → 独立面板重新暂存 → 服务同步重启（每周定时器的执行目标） |
| `clashdock uninstall` | 勾选式卸载：服务 / 自愈 / 定时器 / 面板 / 产物 / 全部状态 |
| `clashdock version` | 显示版本 |

非 TTY 下（管道 / 重定向）交互提示自动回退为编号列表 + 文本输入，
子命令因此也可以在脚本里喂答案：

```bash
printf '3\ny\n' | clashdock        # 例：进入主菜单第 3 项并确认
```

## 内部子命令

由 systemd 单元调用，一般无需手动执行：

| 命令 | 说明 |
|---|---|
| `clashdock webui-serve --port 9091 --bind 127.0.0.1 --dir /etc/mihomo/webui` | 极简静态文件服务，`mihomo-webui.service` 的执行目标（面板根路径直开） |
| `clashdock healthcheck` | `mihomo-watchdog.service` 的执行目标；有上行但本地代理探测失败时重启主服务 |

## 环境变量

| 变量 | 说明 |
|---|---|
| `CLASHDOCK_HOME` | 覆盖数据目录（默认固定为 `/var/lib/clashdock`；主要用于测试） |
| `DOWNLOAD_PROXY` | 下载代理（customize.json 未配置时的回退） |
| `GITHUB_TOKEN` / `GH_TOKEN` | GitHub API Token（提升 release 查询限额） |
| `MIHOMO_NO_PROXY=1` | 强制下载全部走直连，禁用代理兜底 |
| `NO_COLOR` | 关闭彩色输出与 TUI 盒子（自动回退纯文本） |

## systemd 单元一览

由交互流程按需安装 / 卸载：

| 单元 | 作用 |
|---|---|
| `mihomo.service` | 主服务（`/etc/mihomo` 自包含运行时） |
| `mihomo-webui.service` | 独立 Web 面板（根路径直开，`clashdock webui-serve`） |
| `mihomo-watchdog.timer/.service` | 网络自愈探针（有上行但代理不通才重启） |
| `mihomo-update.timer/.service` | 每周自动更新（`clashdock update`） |

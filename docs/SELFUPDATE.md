# clashdock 自更新

> 行数上限：300

自 [ARCHITECTURE.md §8](ARCHITECTURE.md#8-clashdock-自更新) 拆出（实现视角）；用户视角的说明见 README「功能一览」。

`internal/selfupdate`，与 `internal/kernel` 更新 mihomo 内核/UI/geo 数据完全独立。
**dpkg 管理的系统模式安装**（`dpkg-query -S` 登记了可执行文件）：下载同一发行版的
`clashdock_<version>_linux_<arch>.deb`、按 GitHub API 资产 `digest` 校验（取自 api.github.com，不经镜像；
无摘要时才回退到直接从 GitHub 取的 `checksums.txt`）后交给 `dpkg -i`，试跑确认版本，
可执行文件始终归 dpkg；稳定版回退同样下载对应 `.deb`，上一个稳定版号记在 `<state>/clashdock-last-stable`。
早期自更新把 `/usr/bin/clashdock` 改成了指向版本目录的符号链接，主菜单提示可重装 `.deb` 恢复，
恢复后版本目录随 root 入口清理。内核下载另按 GitHub API 资产的 `digest`（SHA-256）校验，镜像只负责传输。

**非 dpkg 安装**（源码构建、便携包）沿用版本化目录 `<state>/clashdock-versions/<version>/clashdock` +
`current` 符号链接方案。流程：查询最新 GitHub release → 下载对应架构的
`clashdock_<version>_linux_<arch>.tar.gz`（amd64 / arm64 / armv7）→ 同上校验 SHA-256 → 解压到
独立版本目录 → 归一化出 `<version>/clashdock` → 试跑新二进制（`clashdock version`）
确认能执行 → 原子重写 `current` 符号链接 → 再次试跑确认成功；启动校验失败则回退
`current` 指向。首次自更新时若正在
运行的可执行文件（如便携包 `install.sh` 装到 `/usr/bin/clashdock`、未登记到 dpkg 的本体）还不是托管符号链接，会先把它
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

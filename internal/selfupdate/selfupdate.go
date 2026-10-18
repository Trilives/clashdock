// Package selfupdate 更新 clashdock 自身到最新发行版（区别于 kernel 包更新的
// mihomo 内核 / Web UI / geo 数据——那些是 mihomo 的依赖资源，这里更新的是
// clashdock 这个程序本身）。
//
// 版本化目录 + current 符号链接方案：
//
//	<state>/clashdock-versions/<version>/clashdock   —— 各版本二进制，各自独立
//	<state>/clashdock-versions/current                —— 指向某个 <version>/clashdock 的符号链接
//
// 首次自更新时，若当前运行的可执行文件（os.Executable，一般是 apt 安装的
// /usr/bin/clashdock）还是普通文件而非上述符号链接，会先把它原样迁移进版本
// 目录（作为已知可回退的基线版本），再把该路径本身替换为指向 current 的符号
// 链接——此后再更新只需要原子重写 current 指向，不用碰 /usr/bin 下的文件。
//
// 更新流程：下载发行包 → 校验 SHA-256 → 解压到独立版本目录 → 试跑新二进制确认
// 能正常执行 → 原子切换 current 符号链接 → 再次试跑确认成功；若启动校验失败，
// 回退 current 指向。mihomo 内核是完全独立的另一个二进制，不受影响，本流程
// 不涉及任何 systemd 单元的停止/启动。仅保留 current 指向的版本、紧邻的上一
// 个版本、以及 last-stable 记录的稳定版（如果三者不同），其余版本目录清理掉。
//
// 更新渠道（Channel）：稳定版走 GitHub /releases/latest（排除 prerelease），
// 预览版走 /releases 列表最新一项（不论是否标 prerelease）。每次稳定渠道更新
// 成功都会把 current 同时记到 <state>/clashdock-versions/last-stable，供切到
// 预览版之后想回退的用户一键切回。
package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/fetchx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
)

// Repo clashdock 自身的发行仓库（.goreleaser.yaml 里的项目/归属一致）。
const Repo = "Trilives/clashdock"

// Channel 自更新渠道：稳定版只看 GitHub 的 /releases/latest（该接口本身就
// 排除 prerelease/draft，语义上等价于"最新正式版"）；预览版看 /releases 列表
// 第一项，即仓库里创建时间最新的发行版，不论是否标了 prerelease，供想尝鲜的
// 用户提前用上还没转正的版本。
type Channel string

const (
	Stable  Channel = "stable"
	Preview Channel = "preview"
)

// LatestVersion 只查询最新版本号（不下载），供菜单展示"当前 vX，最新 vY"。
func LatestVersion(p paths.Paths, channel Channel) (string, error) {
	f, _ := kernel.NewFetcher(p)
	rel, err := fetchRelease(f, channel)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(rel.TagName, "v"), nil
}

// LastStableVersion 返回记录的"上一个稳定版"版本号（每次稳定渠道更新成功后都会
// 刷新）；尚未记录过（从未在这台机器上完成过一次稳定渠道更新）则 ok=false。
func LastStableVersion(p paths.Paths) (string, bool) {
	target, err := os.Readlink(lastStableLink(p))
	if err != nil {
		return "", false
	}
	return filepath.Base(filepath.Dir(target)), true
}

// RollbackToStable 把 current 切回 last-stable 记录的稳定版（试跑校验通过才
// 切换），供切到预览版后想回退的用户使用。返回切回的版本号。
func RollbackToStable(p paths.Paths) (string, error) {
	target, err := os.Readlink(lastStableLink(p))
	if err != nil {
		return "", fmt.Errorf("%s", i18n.T("尚未记录过稳定版，无法回退"))
	}
	if err := probeBinary(target); err != nil {
		return "", fmt.Errorf(i18n.T("记录的稳定版二进制无法正常运行：%w"), err)
	}
	if err := swapCurrentLink(p, target); err != nil {
		return "", err
	}
	if err := probeBinary(currentLink(p)); err != nil {
		return "", fmt.Errorf(i18n.T("回退后校验失败：%w"), err)
	}
	return filepath.Base(filepath.Dir(target)), nil
}

// Update 把 clashdock 自身更新到最新版本；currentVersion 是当前运行版本号
// （main.version，"dev" 视为无基线）。返回 (最新版本号, 是否已是最新, error)。
func Update(p paths.Paths, currentVersion string, channel Channel) (string, bool, error) {
	f, s := kernel.NewFetcher(p)
	rel, err := fetchRelease(f, channel)
	if err != nil {
		return "", false, err
	}
	version := strings.TrimPrefix(rel.TagName, "v")
	if version == "" {
		return "", false, fmt.Errorf("%s", i18n.T("发行版没有有效的版本号"))
	}
	if version == strings.TrimPrefix(currentVersion, "v") {
		return version, true, nil
	}

	name := assetName(version)
	archivePath := filepath.Join(p.Downloads, name)
	sumsPath := filepath.Join(p.Downloads, name+".sha256")
	defer os.Remove(archivePath)
	defer os.Remove(sumsPath)
	if err := downloadVerified(f, s, rel, name, archivePath, sumsPath); err != nil {
		return "", false, err
	}
	newBin, err := installVersion(p, version, archivePath)
	if err != nil {
		return "", false, err
	}

	if err := ensureManagedByCurrentLink(p, currentVersion); err != nil {
		return "", false, err
	}

	prevTarget, _ := os.Readlink(currentLink(p))

	if err := swapCurrentLink(p, newBin); err != nil {
		return "", false, err
	}

	// 注意：不能用 os.Executable() 来试跑"切换后的版本"——它在 Linux 上读
	// /proc/self/exe，返回的是当前进程加载时就已解析到底的真实文件（也就是
	// 切换前那个版本），并不会因为重写了 current 符号链接而改变。要验证刚
	// 切换到的新版本，必须显式执行 currentLink（符号链接，每次 exec 都会
	// 重新解析到它当前指向的文件）。
	if err := probeBinary(currentLink(p)); err != nil {
		execx.Warn(fmt.Sprintf(i18n.T("新版本启动校验失败，回退到旧版本：%v"), err))
		if prevTarget != "" {
			swapCurrentLink(p, prevTarget) //nolint:errcheck // 回退已在出错路径，尽力而为
		}
		return "", false, fmt.Errorf(i18n.T("已回退到原版本：%w"), err)
	}

	if channel == Stable {
		// 记录本次稳定版，供之后切到预览版的用户一键回退；失败不影响本次更新
		// 结果本身，下次稳定渠道更新时会自然覆盖重试。
		atomicSymlink(newBin, lastStableLink(p)) //nolint:errcheck
	}

	pruneOldVersions(p, version)
	execx.Ok(fmt.Sprintf(i18n.T("clashdock 已更新到 %s。"), version))
	return version, false, nil
}

// downloadVerified 下载本机架构的发行包与 checksums.txt 到给定路径，并校验 SHA-256。
func downloadVerified(f *fetchx.Fetcher, s kernel.Settings, rel ghRelease, name, archivePath, sumsPath string) error {
	assetURL := findAssetURL(rel, name)
	if assetURL == "" {
		return fmt.Errorf(i18n.T("未找到本机架构的发行包: %s"), name)
	}
	sumsURL := findAssetURL(rel, "checksums.txt")
	if sumsURL == "" {
		return fmt.Errorf("%s", i18n.T("发行版缺少 checksums.txt，无法校验完整性"))
	}
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return err
	}
	execx.Info(i18n.T("下载 clashdock: ") + assetURL)
	if err := f.FetchFile(kernel.Mirror(assetURL, s.GithubMirror), archivePath); err != nil {
		return err
	}
	if err := f.FetchFile(kernel.Mirror(sumsURL, s.GithubMirror), sumsPath); err != nil {
		return err
	}
	if err := verifySHA256(archivePath, sumsPath, name); err != nil {
		return err
	}
	execx.Ok(i18n.T("SHA-256 校验通过。"))
	return nil
}

// installVersion 把已校验的发行包解压到独立版本目录、归一化出 <version>/clashdock 并试跑；
// 试跑失败时删除该版本目录。返回新版本二进制路径。
func installVersion(p paths.Paths, version, archivePath string) (string, error) {
	verDir := filepath.Join(versionsDir(p), version)
	if err := os.RemoveAll(verDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		return "", err
	}
	if err := extractTarGz(archivePath, verDir); err != nil {
		return "", err
	}
	newBin, err := prepareExtractedBinary(p, version)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(newBin, 0o755); err != nil {
		return "", err
	}
	if err := probeBinary(newBin); err != nil {
		os.RemoveAll(verDir)
		return "", fmt.Errorf(i18n.T("新版本二进制无法正常运行，已放弃更新：%w"), err)
	}
	return newBin, nil
}

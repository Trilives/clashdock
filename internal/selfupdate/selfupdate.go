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
// 能正常执行 → 停止会常驻 exec 本二进制的服务（目前只有独立 Web 面板；mihomo
// 内核是完全独立的另一个二进制，不受影响）→ 原子切换 current 符号链接 → 重启
// 该服务并再次试跑确认成功；若启动校验失败，回退 current 指向并重启服务。
// 仅保留 current 指向的版本与紧邻的上一个版本，其余版本目录清理掉。
package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/fetchx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
)

// Repo clashdock 自身的发行仓库（.goreleaser.yaml 里的项目/归属一致）。
const Repo = "Trilives/clashdock"

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func fetchRelease(f *fetchx.Fetcher) (ghRelease, error) {
	var rel ghRelease
	err := f.ReadJSON("https://api.github.com/repos/"+Repo+"/releases/latest", &rel)
	return rel, err
}

func assetName(version string) string {
	return fmt.Sprintf("clashdock_%s_linux_%s.tar.gz", version, runtime.GOARCH)
}

func findAssetURL(rel ghRelease, name string) string {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

func versionsDir(p paths.Paths) string { return filepath.Join(p.State, "clashdock-versions") }
func currentLink(p paths.Paths) string { return filepath.Join(versionsDir(p), "current") }
func versionBin(p paths.Paths, version string) string {
	return filepath.Join(versionsDir(p), version, "clashdock")
}

// LatestVersion 只查询最新版本号（不下载），供菜单展示"当前 vX，最新 vY"。
func LatestVersion(p paths.Paths) (string, error) {
	f, _ := kernel.NewFetcher(p)
	rel, err := fetchRelease(f)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(rel.TagName, "v"), nil
}

// Update 把 clashdock 自身更新到最新版本；currentVersion 是当前运行版本号
// （main.version，"dev" 视为无基线）。返回 (最新版本号, 是否已是最新, error)。
func Update(p paths.Paths, currentVersion string) (string, bool, error) {
	f, s := kernel.NewFetcher(p)
	rel, err := fetchRelease(f)
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
	assetURL := findAssetURL(rel, name)
	if assetURL == "" {
		return "", false, fmt.Errorf(i18n.T("未找到本机架构的发行包: %s"), name)
	}
	sumsURL := findAssetURL(rel, "checksums.txt")
	if sumsURL == "" {
		return "", false, fmt.Errorf("%s", i18n.T("发行版缺少 checksums.txt，无法校验完整性"))
	}

	if err := os.MkdirAll(p.Downloads, 0o755); err != nil {
		return "", false, err
	}
	archivePath := filepath.Join(p.Downloads, name)
	sumsPath := filepath.Join(p.Downloads, name+".sha256")
	defer os.Remove(archivePath)
	defer os.Remove(sumsPath)

	execx.Info(i18n.T("下载 clashdock: ") + assetURL)
	if err := f.FetchFile(kernel.Mirror(assetURL, s.GithubMirror), archivePath); err != nil {
		return "", false, err
	}
	if err := f.FetchFile(kernel.Mirror(sumsURL, s.GithubMirror), sumsPath); err != nil {
		return "", false, err
	}
	if err := verifySHA256(archivePath, sumsPath, name); err != nil {
		return "", false, err
	}
	execx.Ok(i18n.T("SHA-256 校验通过。"))

	verDir := filepath.Join(versionsDir(p), version)
	if err := os.RemoveAll(verDir); err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		return "", false, err
	}
	if err := extractTarGz(archivePath, verDir); err != nil {
		return "", false, err
	}
	newBin := versionBin(p, version)
	if _, err := os.Stat(newBin); err != nil {
		return "", false, fmt.Errorf("%s", i18n.T("解压后未找到 clashdock 可执行文件"))
	}
	if err := os.Chmod(newBin, 0o755); err != nil {
		return "", false, err
	}
	if err := probeBinary(newBin); err != nil {
		os.RemoveAll(verDir)
		return "", false, fmt.Errorf(i18n.T("新版本二进制无法正常运行，已放弃更新：%w"), err)
	}

	if err := ensureManagedByCurrentLink(p, currentVersion); err != nil {
		return "", false, err
	}

	realExe, err := filepath.EvalSymlinks(exeSelf())
	if err != nil {
		realExe = exeSelf()
	}
	prevTarget, _ := os.Readlink(currentLink(p))

	stopWebUI()
	if err := swapCurrentLink(p, newBin); err != nil {
		startWebUI()
		return "", false, err
	}
	startWebUI()

	if err := probeBinary(realExe); err != nil {
		execx.Warn(fmt.Sprintf(i18n.T("新版本启动校验失败，回退到旧版本：%v"), err))
		stopWebUI()
		if prevTarget != "" {
			swapCurrentLink(p, prevTarget) //nolint:errcheck // 回退已在出错路径，尽力而为
		}
		startWebUI()
		return "", false, fmt.Errorf(i18n.T("已回退到原版本：%w"), err)
	}

	pruneOldVersions(p, version)
	execx.Ok(fmt.Sprintf(i18n.T("clashdock 已更新到 %s。"), version))
	return version, false, nil
}

func exeSelf() string {
	p, err := os.Executable()
	if err != nil {
		return ""
	}
	return p
}

// ensureManagedByCurrentLink 首次自更新迁移：若正在运行的可执行文件还不是
// current 符号链接（apt 刚装好的普通文件），把它搬进版本目录当基线版本，
// 再把该路径替换成指向 current 的符号链接。之后的更新只需重写 current。
func ensureManagedByCurrentLink(p paths.Paths, currentVersion string) error {
	if err := os.MkdirAll(versionsDir(p), 0o755); err != nil {
		return err
	}
	exe := exeSelf()
	if exe == "" {
		return fmt.Errorf("%s", i18n.T("无法定位当前运行的可执行文件"))
	}
	if target, err := os.Readlink(exe); err == nil && filepath.Clean(target) == filepath.Clean(currentLink(p)) {
		return nil // 已经是托管符号链接
	}
	baseline := strings.TrimPrefix(currentVersion, "v")
	if baseline == "" || baseline == "dev" {
		baseline = "installed"
	}
	baselineBin := versionBin(p, baseline)
	if err := os.MkdirAll(filepath.Dir(baselineBin), 0o755); err != nil {
		return err
	}
	if err := copyFileMode(exe, baselineBin, 0o755); err != nil {
		return err
	}
	if err := atomicSymlink(baselineBin, currentLink(p)); err != nil {
		return err
	}
	execx.Info(fmt.Sprintf(i18n.T("已把当前运行的可执行文件迁移为托管版本 %s。"), baseline))
	reason := i18n.T("首次自更新需要把 ") + exe + i18n.T(" 接管为指向托管版本的符号链接")
	if _, err := execx.RunRoot([]string{"ln", "-sfn", currentLink(p), exe}, reason, nil); err != nil {
		return err
	}
	return nil
}

// swapCurrentLink 原子重写 current 符号链接指向 target（versionsDir 属当前
// 用户所有，不需要 root）。
func swapCurrentLink(p paths.Paths, target string) error {
	return atomicSymlink(target, currentLink(p))
}

func atomicSymlink(target, linkPath string) error {
	tmp := linkPath + ".new"
	os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, linkPath)
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// probeBinary 试跑新二进制，确认它能正常执行（"version" 子命令，无需 root/网络）。
func probeBinary(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// stopWebUI/startWebUI 独立 Web 面板会常驻 exec 本二进制（webui-serve），
// 切换 current 前后短暂停/启，使其真正加载到新版本代码。mihomo 内核服务是
// 完全独立的另一个二进制，不受影响，无需处理。watchdog/定时器只是周期性
// 短暂 exec 一次，下次触发自然用上新版本，无需重启。
func stopWebUI()  { execx.RunRoot([]string{"systemctl", "stop", webUIUnit}, "", quietOpt) }
func startWebUI() { execx.RunRoot([]string{"systemctl", "start", webUIUnit}, "", quietOpt) }

const webUIUnit = "mihomo-webui.service"

var quietOpt = &execx.Opt{Capture: true}

// pruneOldVersions 只保留 current 指向的版本与紧邻上一个版本，其余版本目录删除。
func pruneOldVersions(p paths.Paths, keepVersion string) {
	entries, err := os.ReadDir(versionsDir(p))
	if err != nil {
		return
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "current" {
			versions = append(versions, e.Name())
		}
	}
	sort.Strings(versions)
	keep := map[string]bool{keepVersion: true}
	if idx := sort.SearchStrings(versions, keepVersion); idx > 0 {
		keep[versions[idx-1]] = true
	}
	for _, v := range versions {
		if !keep[v] {
			os.RemoveAll(filepath.Join(versionsDir(p), v))
		}
	}
}

func verifySHA256(archivePath, sumsPath, name string) error {
	sums, err := os.ReadFile(sumsPath)
	if err != nil {
		return err
	}
	var want string
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			want = strings.ToLower(fields[0])
			break
		}
	}
	if want == "" {
		return fmt.Errorf(i18n.T("checksums.txt 里没有 %s 的记录"), name)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf(i18n.T("SHA-256 校验失败：期望 %s，实际 %s"), want, got)
	}
	return nil
}

func extractTarGz(archive, outDir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	cleanOut := filepath.Clean(outDir)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(outDir, hdr.Name)
		if target != cleanOut && !strings.HasPrefix(target, cleanOut+string(os.PathSeparator)) {
			return fmt.Errorf(i18n.T("非法压缩条目路径: %s"), hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode&0o777))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}

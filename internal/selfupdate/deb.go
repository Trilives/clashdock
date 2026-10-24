package selfupdate

// dpkg 管理的系统模式安装（apt / dpkg 装的 .deb）：自更新改为下载同一发行版的 .deb、
// 按 checksums.txt 校验后交给 dpkg，可执行文件始终归 dpkg 所有，与 apt 升级不再冲突。
// 早期版本的自更新曾把 /usr/bin/clashdock 换成指向 <state>/clashdock-versions 的符号
// 链接；重新安装 .deb 时 dpkg 用普通文件替换该链接，版本目录随之废弃（retireLegacyVersions）。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/fetchx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
)

// debDeps dpkg 交互，可在测试中替换。
type debDeps struct {
	owns    func(path string) bool     // dpkg 是否登记了该路径
	install func(debPath string) error // dpkg -i
}

var deb = debDeps{owns: dpkgOwns, install: dpkgInstall}

func dpkgOwns(path string) bool {
	if _, err := exec.LookPath("dpkg-query"); err != nil {
		return false
	}
	return exec.Command("dpkg-query", "-S", path).Run() == nil
}

func dpkgInstall(debPath string) error {
	_, err := execx.RunRoot([]string{"dpkg", "-i", debPath}, i18n.T("安装 clashdock 新版本"),
		&execx.Opt{Env: map[string]string{"DEBIAN_FRONTEND": "noninteractive"}})
	return err
}

// debManagedExe 系统模式下用户调用的可执行文件由 dpkg 管理时返回其路径。
func debManagedExe(p paths.Paths) (string, bool) {
	if p.UserMode {
		return "", false
	}
	exe, err := execx.StableExecutablePath()
	if err != nil || !deb.owns(exe) {
		return "", false
	}
	return exe, true
}

func isSymlink(path string) bool {
	st, err := os.Lstat(path)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

// NeedsDebRepair dpkg 管理的可执行文件已被旧版自更新改成符号链接：下次 apt 升级会覆盖
// 链接，与版本目录各说各话。重新安装 .deb 即可恢复（见 Update）。
func NeedsDebRepair(p paths.Paths) bool {
	exe, ok := debManagedExe(p)
	return ok && isSymlink(exe)
}

func debAssetName(version string) string {
	return fmt.Sprintf("clashdock_%s_linux_%s.deb", version, releaseArch())
}

// updateDebManaged dpkg 管理时的 Update：已是最新且无需修复则跳过。
func updateDebManaged(p paths.Paths, f *fetchx.Fetcher, s kernel.Settings, rel ghRelease, version, currentVersion, exe string, channel Channel) (string, bool, error) {
	if version == strings.TrimPrefix(currentVersion, "v") && !isSymlink(exe) {
		return version, true, nil
	}
	if err := installDebRelease(p, f, s, rel, version, exe); err != nil {
		return "", false, err
	}
	if channel == Stable {
		// 失败不影响本次更新结果，下次稳定渠道更新时会覆盖重试。
		recordStableVersion(p, version) //nolint:errcheck
	}
	execx.Ok(fmt.Sprintf(i18n.T("clashdock 已更新到 %s。"), version))
	return version, false, nil
}

// installDebRelease 下载 rel 中本机架构的 .deb，校验后交给 dpkg，再试跑确认版本。
func installDebRelease(p paths.Paths, f *fetchx.Fetcher, s kernel.Settings, rel ghRelease, version, exe string) error {
	name := debAssetName(version)
	debPath := filepath.Join(p.Downloads, name)
	sumsPath := debPath + ".sha256"
	defer os.Remove(debPath)
	defer os.Remove(sumsPath)
	if err := downloadVerified(f, s, rel, name, debPath, sumsPath); err != nil {
		return err
	}
	if err := deb.install(debPath); err != nil {
		return err
	}
	if err := probeVersion(exe, version); err != nil {
		return fmt.Errorf(i18n.T("dpkg 安装后校验失败：%w"), err)
	}
	return retireLegacyVersions(p, exe)
}

// probeVersion 试跑 exe，确认输出的版本号就是刚安装的 version。
func probeVersion(exe, version string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, exe, "version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	if got := strings.Fields(string(out)); len(got) < 2 || strings.TrimPrefix(got[1], "v") != version {
		return fmt.Errorf(i18n.T("版本号不符：期望 %s，实际 %q"), version, strings.TrimSpace(string(out)))
	}
	return nil
}

// RetireLegacyVersions 可执行文件已由 dpkg 重新接管（不再是符号链接）时，清理早期自更新
// 留下的版本目录（apt 直接升级后也会剩下它）；上一个稳定版记录随之转存。
func RetireLegacyVersions(p paths.Paths) error {
	exe, ok := debManagedExe(p)
	if !ok {
		return nil
	}
	return retireLegacyVersions(p, exe)
}

func retireLegacyVersions(p paths.Paths, exe string) error {
	if isSymlink(exe) {
		return nil // 仍经版本目录解析，不能删
	}
	if _, err := os.Lstat(versionsDir(p)); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if v, ok := linkedStableVersion(p); ok {
		if err := recordStableVersion(p, v); err != nil {
			return err
		}
	}
	return os.RemoveAll(versionsDir(p))
}

// stableRecordPath dpkg 管理时记录上一个稳定版版本号的文件（没有版本目录可链接）。
func stableRecordPath(p paths.Paths) string {
	return filepath.Join(p.State, "clashdock-last-stable")
}

func recordStableVersion(p paths.Paths, version string) error {
	return os.WriteFile(stableRecordPath(p), []byte(version+"\n"), 0o644)
}

func recordedStableVersion(p paths.Paths) (string, bool) {
	raw, err := os.ReadFile(stableRecordPath(p))
	v := strings.TrimSpace(string(raw))
	return v, err == nil && v != ""
}

// rollbackDebManaged 下载记录的稳定版 .deb 交给 dpkg（降级也由 dpkg 完成）。
func rollbackDebManaged(p paths.Paths, exe string) (string, error) {
	version, ok := recordedStableVersion(p)
	if !ok {
		return "", fmt.Errorf("%s", i18n.T("尚未记录过稳定版，无法回退"))
	}
	f, s := kernel.NewFetcher(p)
	rel, err := fetchReleaseByTag(f, version)
	if err != nil {
		return "", err
	}
	if err := installDebRelease(p, f, s, rel, version, exe); err != nil {
		return "", err
	}
	return version, nil
}

package selfupdate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

func versionsDir(p paths.Paths) string    { return p.VersionsDir() }
func currentLink(p paths.Paths) string    { return filepath.Join(versionsDir(p), "current") }
func lastStableLink(p paths.Paths) string { return filepath.Join(versionsDir(p), "last-stable") }
func versionBin(p paths.Paths, version string) string {
	return filepath.Join(versionsDir(p), version, "clashdock")
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
//
// 判断"是否已迁移"不能用 os.Readlink(exe)：os.Executable() 在 Linux 上读
// /proc/self/exe，内核已经把符号链接完全解析成真实文件路径，exe 本身永远
// 不是符号链接（迁移与否都一样），Readlink 对它总会失败。真正可靠的判断是
// 看这个已解析的真实路径是否已经落在 versionsDir 下——已经在，说明早迁移
// 过了；如果这里误判成"未迁移"又重新执行一遍复制，会把正在运行的二进制
// 自身当成迁移源尝试原地覆写自己，触发 "text file busy"。
func ensureManagedByCurrentLink(p paths.Paths, currentVersion string) error {
	if err := os.MkdirAll(versionsDir(p), 0o755); err != nil {
		return err
	}
	exe := exeSelf()
	if exe == "" {
		return fmt.Errorf("%s", i18n.T("无法定位当前运行的可执行文件"))
	}
	if strings.HasPrefix(exe, versionsDir(p)+string(os.PathSeparator)) {
		return nil // 已经落在版本目录下，说明早迁移过了
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
	return linkExecutable(currentLink(p), exe, p.UserMode)
}

// linkExecutable 把可执行文件路径替换为指向 current 的符号链接。用户模式的本体在
// ~/.local/bin（当前用户可写），直接原子替换、无需 root；其余情况（/usr/bin 等）保持
// 原先经 sudo 的显式接管，不在未经确认时替换任意可写目录里的二进制。
func linkExecutable(current, exe string, userMode bool) error {
	if userMode && dirWritable(filepath.Dir(exe)) {
		return atomicSymlink(current, exe)
	}
	reason := i18n.T("首次自更新需要把 ") + exe + i18n.T(" 接管为指向托管版本的符号链接")
	_, err := execx.RunRoot([]string{"ln", "-sfn", current, exe}, reason, nil)
	return err
}

// dirWritable 探测目录是否可创建文件（access(2) 在 root / ACL 场景下不可靠，直接试写）。
func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".clashdock-wtest-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
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

// pruneOldVersions 只保留 current 指向的版本、紧邻上一个版本、以及 last-stable
// 记录的稳定版（如果三者不同的话），其余版本目录删除。
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
	if target, err := os.Readlink(lastStableLink(p)); err == nil {
		keep[filepath.Base(filepath.Dir(target))] = true
	}
	for _, v := range versions {
		if !keep[v] {
			os.RemoveAll(filepath.Join(versionsDir(p), v))
		}
	}
}

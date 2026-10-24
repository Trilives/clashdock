package execx

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Trilives/clashdock/internal/i18n"
)

// maxSymlinkHops 解析调用路径时最多跟随的符号链接层数（与内核 ELOOP 上限一致）。
const maxSymlinkHops = 40

// TrustedExecutablePath 写进以 root 运行的单元（watchdog / 周更）的入口：用户调用的稳定
// 路径，且沿途每一级目录与最终文件都只有 root 可写。否则普通用户替换它，定时器就会以
// root 执行任意代码（例如在家目录里执行了 `sudo ./clashdock init`）。
func TrustedExecutablePath() (string, error) {
	exe, err := StableExecutablePath()
	if err != nil {
		return "", err
	}
	if err := rootOnlyPath(exe, rootOnlyEntry); err != nil {
		return "", fmt.Errorf(i18n.T("拒绝把 %s 写进以 root 运行的单元（%v）；请从 root 属主的位置运行，如 /usr/bin/clashdock"), exe, err)
	}
	return exe, nil
}

// rootOnlyPath 逐级跟随 path 的符号链接：每一跳所在目录（解析后）连同全部上级目录、以及
// 最终的文件，都要通过 check。
func rootOnlyPath(path string, check func(string, fs.FileInfo) error) error {
	for range maxSymlinkHops {
		dir, err := filepath.EvalSymlinks(filepath.Dir(path))
		if err != nil {
			return err
		}
		if err := rootOnlyDirs(dir, check); err != nil {
			return err
		}
		path = filepath.Join(dir, filepath.Base(path))
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			return check(path, info)
		}
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(dir, target)
		}
		path = target
	}
	return errors.New(i18n.T("符号链接层数过多"))
}

// rootOnlyDirs 已解析的目录 dir 及其每一级上级目录都通过 check。
func rootOnlyDirs(dir string, check func(string, fs.FileInfo) error) error {
	for d := dir; ; d = filepath.Dir(d) {
		info, err := os.Lstat(d)
		if err != nil {
			return err
		}
		if err := check(d, info); err != nil {
			return err
		}
		if filepath.Dir(d) == d {
			return nil
		}
	}
}

// rootOnlyEntry 归 root，且其他人不可写；组可写只在属组也是 root 时允许。
func rootOnlyEntry(path string, info fs.FileInfo) error {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != 0 {
		return fmt.Errorf(i18n.T("%s 不归 root 所有"), path)
	}
	writable := fs.FileMode(0o002)
	if st.Gid != 0 {
		writable |= 0o020
	}
	if info.Mode().Perm()&writable != 0 {
		return fmt.Errorf(i18n.T("%s 可被非 root 用户写入"), path)
	}
	return nil
}

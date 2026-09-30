package usermode

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InstallBinary 把 src（一般是便携包里正在运行的 clashdock）装到 dst（~/.local/bin/clashdock）：
// 先写同目录临时文件再改名——dst 可能正被运行，也可能是自更新留下的托管符号链接，
// 改名只替换目录项，不会写穿到链接目标或触发 ETXTBSY。
func InstallBinary(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := replaceFile(dst, 0o755, func(w io.Writer) error {
		_, err := io.Copy(w, in)
		return err
	}); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	return nil
}

// RemoveBinary 删除用户模式安装的本体（不存在视为成功）。
func RemoveBinary(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// BinaryVersion 运行 `<bin> version` 取版本号（`clashdock v0.3.0` → `v0.3.0`）；
// 不存在或无法执行时返回空串。
func BinaryVersion(bin string) string {
	out, err := exec.Command(bin, "version").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

// RemoveState 删除用户模式数据目录。CLASHDOCK_HOME 可把数据目录指到任意位置，为防误删，
// 拒绝删除根目录、主目录及其上级，并要求目录要么是默认位置，要么确有 clashdock 数据标记。
func RemoveState(state string) error {
	clean := filepath.Clean(state)
	home := filepath.Clean(homeDir())
	if !filepath.IsAbs(clean) || clean == "/" || clean == home || isUnder(home, clean) {
		return fmt.Errorf("refusing to remove %s: not a dedicated clashdock data directory", clean)
	}
	isDefault := clean == filepath.Clean(StateDir())
	if !isDefault && !fileExists(filepath.Join(clean, "customize.json")) && !fileExists(filepath.Join(clean, "subscriptions")) {
		return fmt.Errorf("refusing to remove %s: no clashdock data found there", clean)
	}
	return os.RemoveAll(clean)
}

// OnPath 目录是否在 PATH 中（提示用户能否直接敲 clashdock）。
func OnPath(dir string) bool {
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(d) == filepath.Clean(dir) {
			return true
		}
	}
	return false
}

// migrateSkip 迁移旧数据时跳过的顶层条目：运行时目录与 PID/日志会按新布局重新生成。
var migrateSkip = map[string]bool{"runtime": true}

// NeedsMigration 旧便携数据目录有订阅，而新数据目录还没有任何订阅时才需要迁移，
// 避免覆盖用户已在新位置产生的数据。
func NeedsMigration(legacy, state string) bool {
	if filepath.Clean(legacy) == filepath.Clean(state) {
		return false
	}
	return dirHasContent(filepath.Join(legacy, "subscriptions")) &&
		!dirHasContent(filepath.Join(state, "subscriptions"))
}

// MigrateLegacy 把旧便携数据目录（订阅 / 定制层 / 内核 / geo 等）复制到新数据目录。
// 只复制不删除：旧目录原样保留，由用户确认无误后自行删除。
func MigrateLegacy(legacy, state string) error {
	entries, err := os.ReadDir(legacy)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(state, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if migrateSkip[e.Name()] {
			continue
		}
		src, dst := filepath.Join(legacy, e.Name()), filepath.Join(state, e.Name())
		if err := copyEntry(src, dst); err != nil {
			return fmt.Errorf("migrate %s: %w", e.Name(), err)
		}
	}
	return nil
}

// copyEntry 复制文件或目录（保留可执行位；符号链接跳过——旧目录里只有自更新的托管链接，
// 在新位置无意义）。
func copyEntry(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.Type()&os.ModeSymlink != 0:
			return nil
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFileMode(path, target, info.Mode().Perm())
	})
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

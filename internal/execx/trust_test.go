package execx

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var errWritable = errors.New("writable by others")

// checkUnder 只审查 root 下的条目（测试目录之外视为可信），其他人可写即拒绝。
func checkUnder(root string) func(string, fs.FileInfo) error {
	return func(path string, info fs.FileInfo) error {
		if !strings.HasPrefix(path, root) {
			return nil
		}
		if info.Mode().Perm()&0o022 != 0 {
			return errWritable
		}
		return nil
	}
}

// trustTree 测试目录里的 safe（0755）与 open（0777）两个子目录。
func trustTree(t *testing.T) (root, safe, open string) {
	t.Helper()
	root = t.TempDir()
	safe, open = filepath.Join(root, "safe"), filepath.Join(root, "open")
	for _, d := range []string{safe, open} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(open, 0o777); err != nil {
		t.Fatal(err)
	}
	return root, safe, open
}

func writeExe(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRootOnlyPathAcceptsSystemBinaries(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip(err)
	}

	if err := rootOnlyPath(sh, rootOnlyEntry); err != nil {
		t.Fatalf("rootOnlyPath(%s) = %v, want a distro binary to be trusted", sh, err)
	}
}

func TestRootOnlyPathRejectsFilesOutsideRootControl(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "clashdock")
	writeExe(t, exe)

	if err := rootOnlyPath(exe, rootOnlyEntry); err == nil {
		t.Fatalf("%s lives in a user-owned temp dir, yet was trusted for a root unit", exe)
	}
}

func TestRootOnlyPathFollowsEverySymlinkHop(t *testing.T) {
	root, safe, open := trustTree(t)
	check := checkUnder(root)
	writeExe(t, filepath.Join(safe, "clashdock"))
	writeExe(t, filepath.Join(open, "clashdock"))
	mustLink := func(target, link string) {
		t.Helper()
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	mustLink("clashdock", filepath.Join(safe, "current"))                     // 同目录相对链接
	mustLink(filepath.Join(safe, "current"), filepath.Join(safe, "entry"))    // 两跳都在安全目录
	mustLink(filepath.Join(open, "clashdock"), filepath.Join(safe, "escape")) // 最终文件在可写目录
	mustLink(filepath.Join(safe, "clashdock"), filepath.Join(open, "hop"))    // 中间一跳在可写目录
	mustLink(filepath.Join(open, "hop"), filepath.Join(safe, "via-open"))

	if err := rootOnlyPath(filepath.Join(safe, "entry"), check); err != nil {
		t.Fatalf("chain inside safe dirs rejected: %v", err)
	}
	for _, name := range []string{"escape", "via-open"} {
		if err := rootOnlyPath(filepath.Join(safe, name), check); !errors.Is(err, errWritable) {
			t.Errorf("rootOnlyPath(%s) = %v, want rejection: a hop lives in a writable dir", name, err)
		}
	}
}

func TestRootOnlyPathChecksEveryParentOfTheFile(t *testing.T) {
	root, _, open := trustTree(t)
	nested := filepath.Join(open, "bin")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(nested, "clashdock")
	writeExe(t, exe)

	if err := rootOnlyPath(exe, checkUnder(root)); !errors.Is(err, errWritable) {
		t.Fatalf("rootOnlyPath = %v, want rejection: a grandparent dir is world-writable", err)
	}
}

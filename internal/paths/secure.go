package paths

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// 含节点凭据 / Token 的状态文件与目录权限：只有属主可写，属主与属组可读。系统模式下
// 属主是 root、属组是管理员的私有组（见 flows.EnsureStateRoot），管理员不经 sudo 也能
// 查看设置、配置与日志，其他本地用户读不到；用户模式的状态目录整体为 UserStateMode。
const (
	SecretFileMode os.FileMode = 0o640
	SecretDirMode  os.FileMode = 0o750
	UserStateMode  os.FileMode = 0o700
)

// writableByOthers 组 / 其他用户写权限位。
const writableByOthers os.FileMode = 0o022

// WriteSecretFile 以同目录临时文件 + 改名写入含凭据的文件，权限固定为 SecretFileMode：
// 中途中断不会留下截断的文件，旧版本以 0644 写下的文件也随之收紧。
func WriteSecretFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(SecretFileMode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// SecureTree 把 root 下整棵目录树收归 root 属主、gid 属组，幂等，已合规的条目不做改动。
// gid 非 0 时目录带 setgid，root 之后新建的文件随之继承该属组。
//
// 旧版本把系统模式状态目录交给调用用户，而 root 运行的定时器与服务会读取、执行其中
// 的文件；该用户身份下的任意进程都可能在收归前埋下陷阱，所以这里不只是改属主：
//   - 目录：先改属主、去掉组 / 其他写权限，再列出内容，此后无法再往里放东西；
//   - 普通文件：属主不对、组 / 其他可写或有多个硬链接时，在同目录以新 inode 重建，
//     旧文件上残留的写句柄与硬链接随之失效；
//   - 符号链接：目标落在 root 之外（如指向 /etc/shadow 的日志）时删除，否则保留；
//   - 其他类型（FIFO、套接字等）一律删除，避免 root 读取时阻塞。
func SecureTree(root string, gid int) error {
	return secureTree(root, 0, gid, os.Lchown)
}

type treeOwner struct {
	uid, gid int
	chown    func(name string, uid, gid int) error
}

func secureTree(root string, uid, gid int, chown func(string, int, int) error) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	st, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("%s: not a directory", root)
	}
	o := treeOwner{uid: uid, gid: gid, chown: chown}
	// WalkDir 先回调目录本身再读取其内容，目录收归后才列出子项。
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		switch {
		case mode.IsDir():
			return o.secureDir(path, info)
		case mode.IsRegular():
			return o.secureFile(path, info)
		case mode&fs.ModeSymlink != 0:
			return o.secureLink(root, path, info)
		default:
			return os.Remove(path)
		}
	})
}

func (o treeOwner) owns(info fs.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == o.uid && int(st.Gid) == o.gid
}

func (o treeOwner) secureDir(path string, info fs.FileInfo) error {
	chowned := !o.owns(info)
	if chowned {
		if err := o.chown(path, o.uid, o.gid); err != nil {
			return err
		}
	}
	want := info.Mode().Perm() &^ writableByOthers
	if o.gid != 0 {
		want |= fs.ModeSetgid
	}
	if chowned || info.Mode()&(fs.ModePerm|fs.ModeSetgid) != want {
		return os.Chmod(path, want)
	}
	return nil
}

func (o treeOwner) secureFile(path string, info fs.FileInfo) error {
	st, ok := info.Sys().(*syscall.Stat_t)
	if ok && o.owns(info) && st.Nlink == 1 && info.Mode().Perm()&writableByOthers == 0 {
		return nil
	}
	return o.recreateFile(path, info.Mode().Perm()&^writableByOthers)
}

// recreateFile 把 path 的内容复制到同目录的新文件再改名覆盖；
// 父目录此时已收归，改名后 path 指向的只可能是新 inode。
func (o treeOwner) recreateFile(path string, perm os.FileMode) error {
	src, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".secure-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := o.chown(tmpName, o.uid, o.gid); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (o treeOwner) secureLink(root, path string, info fs.FileInfo) error {
	target, err := os.Readlink(path)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	if !within(root, filepath.Clean(target)) {
		return os.Remove(path)
	}
	if o.owns(info) {
		return nil
	}
	return o.chown(path, o.uid, o.gid)
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

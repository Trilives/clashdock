package paths

import (
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"testing"
)

// foreignUID 测试里充当「目标属主」的 uid：与当前用户不同，所有条目都视为待收归。
const foreignUID = 54321

type chownLog struct{ paths []string }

func (c *chownLog) chown(name string, uid, gid int) error {
	if uid != foreignUID || gid != foreignUID {
		return os.ErrInvalid
	}
	c.paths = append(c.paths, filepath.Base(name))
	return nil
}

func inode(t *testing.T, path string) uint64 {
	t.Helper()
	st, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return st.Sys().(*syscall.Stat_t).Ino
}

func mustWrite(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// statePaths 状态目录尚不存在，由被测代码按自己的权限创建。
func statePaths(t *testing.T) Paths {
	t.Helper()
	t.Setenv("CLASHDOCK_HOME", filepath.Join(t.TempDir(), "state"))
	return Detect()
}

func TestSecureTreeHandsEveryEntryToTargetOwner(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "customize.json"), "{}", 0o600)
	mustWrite(t, filepath.Join(root, "bin", "mihomo"), "ELF", 0o755)
	log := &chownLog{}

	if err := secureTree(root, foreignUID, foreignUID, log.chown); err != nil {
		t.Fatal(err)
	}

	sort.Strings(log.paths)
	if len(log.paths) != 4 { // root、bin/ 与两个重建文件的临时名
		t.Fatalf("chowned %v, want root, bin and both files", log.paths)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "bin", "mihomo")); string(got) != "ELF" {
		t.Fatalf("kernel content = %q after securing", got)
	}
	if st, _ := os.Stat(filepath.Join(root, "bin", "mihomo")); st.Mode().Perm() != 0o755 {
		t.Fatalf("kernel mode = %v, executable bit must survive", st.Mode().Perm())
	}
}

func TestSecureTreeRecreatesFilesSoOldWriteHandlesGoStale(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	mustWrite(t, path, "trusted", 0o644)
	before := inode(t, path)
	stale, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer stale.Close()

	if err := secureTree(root, foreignUID, foreignUID, (&chownLog{}).chown); err != nil {
		t.Fatal(err)
	}
	if _, err := stale.WriteAt([]byte("hijack!"), 0); err != nil {
		t.Fatal(err)
	}

	if inode(t, path) == before {
		t.Fatal("foreign-owned file must be replaced by a new inode")
	}
	if got, _ := os.ReadFile(path); string(got) != "trusted" {
		t.Fatalf("content = %q, a handle opened before securing must not reach the new file", got)
	}
}

func TestSecureTreeStripsGroupAndOtherWrite(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "ruleset")
	file := filepath.Join(dir, "geosite.dat")
	mustWrite(t, file, "dat", 0o666)
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}

	if err := secureTree(root, os.Getuid(), os.Getgid(), os.Lchown); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]os.FileMode{dir: 0o755, file: 0o644} {
		st, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != want {
			t.Errorf("%s mode = %v, want %v", filepath.Base(path), st.Mode().Perm(), want)
		}
	}
}

func TestSecureTreeMakesDirsPassGroupToNewFiles(t *testing.T) {
	if os.Getgid() == 0 {
		t.Skip("needs a non-zero primary group")
	}
	root := t.TempDir()
	sub := filepath.Join(root, "subscriptions")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := secureTree(root, os.Getuid(), os.Getgid(), os.Lchown); err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{root, sub} {
		st, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode()&os.ModeSetgid == 0 {
			t.Errorf("%s mode = %v: setgid missing, files root creates later would not inherit the group", dir, st.Mode())
		}
	}
}

func TestSecureTreeClearsSetgidWhenGroupIsRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o755|os.ModeSetgid); err != nil {
		t.Fatal(err)
	}
	noop := func(string, int, int) error { return nil }

	if err := secureTree(root, os.Getuid(), 0, noop); err != nil {
		t.Fatal(err)
	}

	if st, _ := os.Stat(root); st.Mode()&os.ModeSetgid != 0 {
		t.Fatalf("mode = %v: with group 0 there is nothing to inherit", st.Mode())
	}
}

func TestSecureTreeBreaksHardLinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "victim")
	mustWrite(t, outside, "victim", 0o644)
	linked := filepath.Join(root, "active")
	if err := os.Link(outside, linked); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}

	if err := secureTree(root, os.Getuid(), os.Getgid(), os.Lchown); err != nil {
		t.Fatal(err)
	}

	if inode(t, linked) == inode(t, outside) {
		t.Fatal("hard link into the tree must be replaced by an independent copy")
	}
}

func TestSecureTreeRemovesLinksEscapingRootAndKeepsInternalOnes(t *testing.T) {
	root := t.TempDir()
	versions := filepath.Join(root, "clashdock-versions")
	mustWrite(t, filepath.Join(versions, "0.3.1", "clashdock"), "bin", 0o755)
	links := map[string]string{
		filepath.Join(versions, "current"):     filepath.Join(versions, "0.3.1", "clashdock"),
		filepath.Join(versions, "last-stable"): "0.3.1/clashdock",
		filepath.Join(root, "clashdock.log"):   "/etc/shadow",
		filepath.Join(root, "config.yaml"):     "../../etc/passwd",
	}
	for link, target := range links {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}

	if err := secureTree(root, os.Getuid(), os.Getgid(), os.Lchown); err != nil {
		t.Fatal(err)
	}

	for _, kept := range []string{"current", "last-stable"} {
		if _, err := os.Lstat(filepath.Join(versions, kept)); err != nil {
			t.Errorf("internal link %s removed: %v", kept, err)
		}
	}
	for _, gone := range []string{"clashdock.log", "config.yaml"} {
		if _, err := os.Lstat(filepath.Join(root, gone)); !os.IsNotExist(err) {
			t.Errorf("escaping link %s must be removed, lstat err = %v", gone, err)
		}
	}
}

func TestSecureTreeRemovesSpecialFiles(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "customize.json")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}

	if err := secureTree(root, os.Getuid(), os.Getgid(), os.Lchown); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Lstat(fifo); !os.IsNotExist(err) {
		t.Fatalf("FIFO must be removed so root never blocks reading it, lstat err = %v", err)
	}
}

func TestSecureTreeLeavesCompliantTreeUntouched(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "subscriptions", "a", "raw.yaml")
	mustWrite(t, path, "proxies: []", 0o600)
	before := inode(t, path)

	if err := secureTree(root, os.Getuid(), os.Getgid(), os.Lchown); err != nil {
		t.Fatal(err)
	}

	if inode(t, path) != before {
		t.Fatal("compliant file must not be rewritten on every run")
	}
}

func TestSecureTreeRefusesSymlinkedRoot(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "state")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if err := secureTree(link, os.Getuid(), os.Getgid(), os.Lchown); err == nil {
		t.Fatal("a symlinked state root must be refused")
	}
}

func TestWriteSecretFileTightensExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "customize.json")
	mustWrite(t, path, "{}", 0o644) // 旧版本写下的 0644

	if err := WriteSecretFile(path, []byte(`{"github_token":"x"}`)); err != nil {
		t.Fatal(err)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != SecretFileMode {
		t.Fatalf("mode = %v, want %v: the file holds credentials", st.Mode().Perm(), SecretFileMode)
	}
	if got, _ := os.ReadFile(path); string(got) != `{"github_token":"x"}` {
		t.Fatalf("content = %q", got)
	}
}

func TestEnsureStateDirsLocksSubscriptions(t *testing.T) {
	p := statePaths(t)
	if err := os.MkdirAll(p.Subscriptions, 0o755); err != nil { // 旧版本留下的 0755
		t.Fatal(err)
	}

	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}

	st, err := os.Stat(p.Subscriptions)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != SecretDirMode {
		t.Fatalf("subscriptions mode = %v, want %v: raw subscriptions hold node credentials", st.Mode().Perm(), SecretDirMode)
	}
}

func TestEnsureStateDirsKeepsSetgidOnSubscriptions(t *testing.T) {
	p := statePaths(t)
	if err := os.MkdirAll(p.Subscriptions, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p.Subscriptions, 0o755|os.ModeSetgid); err != nil {
		t.Fatal(err)
	}

	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}

	st, err := os.Stat(p.Subscriptions)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&os.ModeSetgid == 0 || st.Mode().Perm() != SecretDirMode {
		t.Fatalf("subscriptions mode = %v, want %v with setgid kept", st.Mode(), SecretDirMode)
	}
}

func TestEnsureStateDirsLocksUserModeStateOnly(t *testing.T) {
	for _, userMode := range []bool{false, true} {
		p := statePaths(t)
		p.UserMode = userMode

		if err := p.EnsureStateDirs(); err != nil {
			t.Fatal(err)
		}

		st, err := os.Stat(p.State)
		if err != nil {
			t.Fatal(err)
		}
		perm := st.Mode().Perm()
		if userMode && perm != UserStateMode {
			t.Errorf("user-mode state mode = %v, want %v", perm, UserStateMode)
		}
		// 系统模式的状态目录须对所有人可进入：/usr/bin/clashdock 可能经其中的版本目录解析。
		if !userMode && perm&0o005 != 0o005 {
			t.Errorf("system-mode state mode = %v, others must be able to traverse it", perm)
		}
	}
}

package flows

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/paths"
)

// stubSecureTree 替换真实的 root 收归（测试不以 root 运行，无法 chown），记录调用。
func stubSecureTree(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	orig := secureTree
	secureTree = func(root string, gid int) error {
		calls = append(calls, root)
		return nil
	}
	t.Cleanup(func() { secureTree = orig })
	return &calls
}

// stubAccounts 替换 /etc/passwd、/etc/group 查询：users 为 uid → 用户，groups 为 gid → 组名。
func stubAccounts(t *testing.T, users map[string]user.User, groups map[string]string) {
	t.Helper()
	origUser, origGroup := lookupUser, lookupGroup
	lookupUser = func(uid string) (*user.User, error) {
		if u, ok := users[uid]; ok {
			return &u, nil
		}
		return nil, user.UnknownUserIdError(0)
	}
	lookupGroup = func(gid string) (*user.Group, error) {
		if name, ok := groups[gid]; ok {
			return &user.Group{Gid: gid, Name: name}, nil
		}
		return nil, errors.New("unknown group")
	}
	t.Cleanup(func() { lookupUser, lookupGroup = origUser, origGroup })
}

func systemStatePaths(t *testing.T) paths.Paths {
	t.Helper()
	t.Setenv("CLASHDOCK_HOME", filepath.Join(t.TempDir(), "state"))
	return paths.Detect()
}

func TestEnsureSystemStateRefusesNonRoot(t *testing.T) {
	p := systemStatePaths(t)
	calls := stubSecureTree(t)

	if err := ensureSystemState(p, 1000); err == nil {
		t.Fatal("a non-root caller must be refused instead of getting the directory handed over")
	}
	if _, err := os.Stat(p.State); !os.IsNotExist(err) {
		t.Fatalf("state dir must not be created for a non-root caller, stat err = %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("secureTree called %v for a refused caller", *calls)
	}
}

func TestEnsureSystemStateSecuresTreeAndTightensCredentials(t *testing.T) {
	p := systemStatePaths(t)
	calls := stubSecureTree(t)
	// 旧版本留下的 0644 / 0755
	if err := os.MkdirAll(p.Subscriptions, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{p.ConfigFile, p.CustomizeFile, execx.LogPath(p.State), p.ActiveFile} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := ensureSystemState(p, 0); err != nil {
		t.Fatal(err)
	}

	if len(*calls) != 1 || (*calls)[0] != p.State {
		t.Fatalf("secureTree calls = %v, want exactly the state root", *calls)
	}
	want := map[string]os.FileMode{
		p.Subscriptions:        paths.SecretDirMode,
		p.ConfigFile:           paths.SecretFileMode,
		p.CustomizeFile:        paths.SecretFileMode,
		execx.LogPath(p.State): paths.SecretFileMode,
		p.ActiveFile:           0o644, // 只是订阅名，不含凭据
		p.State:                systemStateMode,
	}
	for path, mode := range want {
		st, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := st.Mode().Perm(); got != mode && !(path == p.State && got&0o005 == 0o005) {
			t.Errorf("%s mode = %v, want %v", filepath.Base(path), got, mode)
		}
	}
}

func TestSecureSystemStateIsNoOpWithoutRootOrState(t *testing.T) {
	calls := stubSecureTree(t)
	p := systemStatePaths(t)

	if err := SecureSystemState(p); err != nil { // 状态目录不存在，且测试不以 root 运行
		t.Fatal(err)
	}
	user := p
	user.UserMode = true
	if err := SecureSystemState(user); err != nil {
		t.Fatal(err)
	}

	if len(*calls) != 0 {
		t.Fatalf("secureTree called %v; only root may take over the system state", *calls)
	}
}

func TestPrivateGroupAcceptsOnlyTheUsersOwnPrimaryGroup(t *testing.T) {
	stubAccounts(t,
		map[string]user.User{
			"1000": {Uid: "1000", Gid: "1000", Username: "ares"},
			"1001": {Uid: "1001", Gid: "100", Username: "bob"},
		},
		map[string]string{"1000": "ares", "100": "users", "27": "sudo"})
	cases := []struct {
		name, uid, gid string
		want           int
	}{
		{"private primary group", "1000", "1000", 1000},
		{"shared primary group", "1001", "100", 0},
		{"supplementary group, not primary", "1000", "27", 0},
		{"root", "0", "0", 0},
		{"unknown user", "4242", "4242", 0},
		{"not via sudo", "", "", 0},
	}
	for _, c := range cases {
		if got := privateGroup(c.uid, c.gid); got != c.want {
			t.Errorf("%s: privateGroup(%q, %q) = %d, want %d", c.name, c.uid, c.gid, got, c.want)
		}
	}
}

func TestStateGroupPrefersLegacyOwnerThenSudoUser(t *testing.T) {
	uid, gid := strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid())
	stubAccounts(t,
		map[string]user.User{
			uid:    {Uid: uid, Gid: gid, Username: "admin"},
			"1500": {Uid: "1500", Gid: "1500", Username: "other"},
		},
		map[string]string{gid: "admin", "1500": "other"})
	dir := t.TempDir() // 测试以普通用户运行：目录归该用户，相当于旧版本交给调用用户的状态目录
	sudo := map[string]string{"SUDO_UID": "1500", "SUDO_GID": "1500"}

	got, err := stateGroup(dir, func(k string) string { return sudo[k] })

	if err != nil {
		t.Fatal(err)
	}
	if want := os.Getgid(); got != want {
		t.Fatalf("stateGroup = %d, want the legacy owner's private group %d, not the current sudo user's", got, want)
	}
}

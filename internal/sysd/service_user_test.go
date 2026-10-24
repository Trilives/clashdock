package sysd

import (
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/execx"
)

// recordRoot 替换 runRoot，记录命令而不执行。
func recordRoot(t *testing.T) *[][]string {
	t.Helper()
	var cmds [][]string
	orig := runRoot
	runRoot = func(cmd []string, _ string, _ *execx.Opt) (execx.Result, error) {
		cmds = append(cmds, cmd)
		return execx.Result{}, nil
	}
	t.Cleanup(func() { runRoot = orig })
	return &cmds
}

func stubServiceUser(t *testing.T, exists bool) {
	t.Helper()
	orig := lookupServiceUser
	lookupServiceUser = func(name string) (*user.User, error) {
		if exists {
			return &user.User{Username: name}, nil
		}
		return nil, user.UnknownUserError(name)
	}
	t.Cleanup(func() { lookupServiceUser = orig })
}

func TestEnsureServiceUserCreatesSystemAccountOnlyWhenMissing(t *testing.T) {
	cmds := recordRoot(t)
	stubServiceUser(t, true)
	if err := ensureServiceUser(); err != nil || len(*cmds) != 0 {
		t.Fatalf("existing account: err=%v cmds=%v, want no useradd", err, *cmds)
	}

	stubServiceUser(t, false)
	if err := ensureServiceUser(); err != nil {
		t.Fatal(err)
	}

	if len(*cmds) != 1 {
		t.Fatalf("cmds = %v, want one useradd", *cmds)
	}
	got := strings.Join((*cmds)[0], " ")
	for _, want := range []string{"useradd", "--system", "--user-group", "--no-create-home", "nologin", ServiceUser} {
		if !strings.Contains(got, want) {
			t.Errorf("useradd command %q lacks %q", got, want)
		}
	}
}

func TestRuntimeConfigIsReadableOnlyByServiceGroup(t *testing.T) {
	rt := rtPaths("mihomo")

	got := installConfigCmd("/tmp/staged.yaml", rt)

	want := []string{"install", "-m", "0640", "-o", "root", "-g", ServiceUser, "/tmp/staged.yaml", rt.Config}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("installConfigCmd = %v, want %v", got, want)
	}
}

func TestRuntimeLayoutKeepsDirRootOwnedAndHomeServiceOwned(t *testing.T) {
	rt := rtPaths("mihomo")

	steps := runtimeLayoutSteps(rt)

	want := [][]string{
		{"install", "-d", "-m", "0755", rt.Dir},
		{"install", "-d", "-o", ServiceUser, "-g", ServiceUser, "-m", "0700", rt.Home},
	}
	if !reflect.DeepEqual(steps[:2], want) {
		t.Fatalf("layout = %v, want %v", steps[:2], want)
	}
	if filepath.Dir(rt.Home) != rt.Dir || rt.Home == rt.Dir {
		t.Fatalf("home %s must be a subdirectory of the root-owned runtime %s", rt.Home, rt.Dir)
	}
}

// testRuntime 临时目录里的运行时布局，geo 列出要放进去的文件。
func testRuntime(t *testing.T, geo ...string) runtimePaths {
	t.Helper()
	d := t.TempDir()
	rt := runtimePaths{
		Dir: d, Home: filepath.Join(d, "home"), Bin: filepath.Join(d, "mihomo"),
		Config: filepath.Join(d, "mihomo.yaml"), UI: filepath.Join(d, "ui"),
		Geoip: filepath.Join(d, "geoip.metadb"), Geosite: filepath.Join(d, "geosite.dat"),
		Country: filepath.Join(d, "country.mmdb"), Unit: filepath.Join(d, "mihomo.service"),
	}
	for _, name := range geo {
		if err := os.WriteFile(filepath.Join(d, name), []byte("geo"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return rt
}

func TestRenderUnitRunsAsServiceUserWithWritableHome(t *testing.T) {
	rt := testRuntime(t, "country.mmdb", "geosite.dat")
	orig := lookPath
	lookPath = func(string) (string, error) { return "/usr/bin/ln", nil }
	t.Cleanup(func() { lookPath = orig })

	unit, err := renderUnit("mihomo", rt)

	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"User=" + ServiceUser + "\n",
		"Group=" + ServiceUser + "\n",
		"WorkingDirectory=" + rt.Home + "\n",
		"Environment=SAFE_PATHS=" + rt.Dir + "\n",
		"ExecStartPre=/usr/bin/ln -sf -t " + rt.Home + " " + rt.Country + " " + rt.Geosite + "\n",
		"ExecStart=" + rt.Bin + " -d " + rt.Home + " -f " + rt.Config + "\n",
		"AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW\n",
		"NoNewPrivileges=yes\n",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("unit lacks %q:\n%s", want, unit)
		}
	}
	for _, banned := range []string{"User=root", "ProtectSystem", "PrivateTmp", rt.Geoip} {
		if strings.Contains(unit, banned) {
			t.Errorf("unit must not contain %q (root identity / namespace option / absent geo file):\n%s", banned, unit)
		}
	}
}

func TestRenderUnitOmitsGeoLinksWithoutGeoFiles(t *testing.T) {
	unit, err := renderUnit("mihomo", testRuntime(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(unit, "ExecStartPre") {
		t.Fatalf("no geo files deployed, yet the unit links some:\n%s", unit)
	}
}

func TestDeployRuntimeStopsBeforeConfigWhenLayoutFails(t *testing.T) {
	var cmds [][]string
	orig := runRoot
	runRoot = func(cmd []string, _ string, _ *execx.Opt) (execx.Result, error) {
		cmds = append(cmds, cmd)
		if cmd[0] == "install" && cmd[1] == "-d" && strings.Contains(strings.Join(cmd, " "), "-o "+ServiceUser) {
			return execx.Result{}, errors.New("invalid user")
		}
		return execx.Result{}, nil
	}
	t.Cleanup(func() { runRoot = orig })
	p := fakeState(t)

	if err := deployRuntime(p, rtPaths("mihomo"), "/tmp/staged.yaml"); err == nil {
		t.Fatal("a failed home setup must abort the deployment")
	}

	for _, cmd := range cmds {
		if cmd[len(cmd)-1] == rtPaths("mihomo").Config {
			t.Fatalf("config installed after the layout failed: %v", cmds)
		}
	}
}

// runMigrateCache 以当前用户执行迁移脚本（chown 换成 true：测试不以 root 运行）。
func runMigrateCache(t *testing.T, oldPath, newPath string) {
	t.Helper()
	script := strings.Replace(migrateCacheScript, "chown -h", "true", 1)
	if out, err := exec.Command("sh", "-c", script, "sh", oldPath, newPath).CombinedOutput(); err != nil {
		t.Fatalf("migrate script: %v\n%s", err, out)
	}
}

func TestMigrateCacheMovesRootEraCacheOnce(t *testing.T) {
	rt := testRuntime(t)
	if err := os.Mkdir(rt.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	oldPath, newPath := filepath.Join(rt.Dir, "cache.db"), filepath.Join(rt.Home, "cache.db")
	if err := os.WriteFile(oldPath, []byte("selected"), 0o600); err != nil {
		t.Fatal(err)
	}

	runMigrateCache(t, oldPath, newPath)

	if got, _ := os.ReadFile(newPath); string(got) != "selected" {
		t.Fatalf("home cache = %q, want the root-era cache moved in", got)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old cache must be moved, stat err = %v", err)
	}
	runMigrateCache(t, oldPath, newPath) // 再次部署：旧文件已不在，什么都不做
}

func TestMigrateCacheKeepsExistingHomeCacheAndLinks(t *testing.T) {
	rt := testRuntime(t)
	if err := os.Mkdir(rt.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	oldPath, newPath := filepath.Join(rt.Dir, "cache.db"), filepath.Join(rt.Home, "cache.db")
	if err := os.WriteFile(oldPath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/shadow", newPath); err != nil {
		t.Fatal(err)
	}

	runMigrateCache(t, oldPath, newPath)

	if target, err := os.Readlink(newPath); err != nil || target != "/etc/shadow" {
		t.Fatalf("existing entry in home must be left alone: %q %v", target, err)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("old cache must stay when home already has one: %v", err)
	}
}

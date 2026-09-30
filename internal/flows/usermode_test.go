package flows

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/usermode"
)

// userModeHome 隔离 HOME / XDG / CLASHDOCK_HOME，返回用户模式路径。
func userModeHome(t *testing.T) paths.Paths {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("CLASHDOCK_HOME", usermode.StateDir())
	p := paths.Detect()
	p.UserMode = true
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}
	return p
}

// fakeUserService 不触碰真实 systemd 用户实例的 usermode.Service 替身。
type fakeUserService struct {
	backend usermode.Backend
	removed bool
}

func (f *fakeUserService) Backend() usermode.Backend { return f.backend }
func (f *fakeUserService) Installed() bool           { return !f.removed }
func (f *fakeUserService) Active() bool              { return false }
func (f *fakeUserService) Install() error            { return nil }
func (f *fakeUserService) Start() error              { return nil }
func (f *fakeUserService) Stop() error               { return nil }
func (f *fakeUserService) Restart() error            { return nil }
func (f *fakeUserService) Remove() error             { f.removed = true; return nil }
func (f *fakeUserService) Logs(int) (string, error)  { return "", nil }
func (f *fakeUserService) Describe() string          { return "/fake/unit.service" }

func TestUserHowToUseTextShowsProxyManagementAndLocations(t *testing.T) {
	i18n.SetLang(i18n.EN)
	p := userModeHome(t)
	text := userHowToUseText(p, &fakeUserService{backend: usermode.BackendSystemd})

	for _, want := range []string{
		`export http_proxy="http://127.0.0.1:7890"`,
		`export all_proxy="socks5://127.0.0.1:7890"`,
		`curl -x http://127.0.0.1:7890 https://www.google.com/generate_204`,
		"clashdock pause | resume",
		usermode.BinPath(),
		p.State,
		usermode.RuntimeDir(p),
		"/fake/unit.service",
		"systemctl --user status clashdock-mihomo",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("userHowToUseText() missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "./tool/update.sh") {
		t.Fatalf("updates now go through the user service menu, not package scripts:\n%s", text)
	}
}

func TestUserHowToUseTextFollowsCustomPort(t *testing.T) {
	p := userModeHome(t)
	cfg := config.Load(p)
	cfg["proxy_port"] = 7999
	if err := config.Save(p, cfg); err != nil {
		t.Fatal(err)
	}
	text := userHowToUseText(p, &fakeUserService{backend: usermode.BackendDetached})
	if !strings.Contains(text, "http://127.0.0.1:7999") {
		t.Fatalf("custom proxy port not reflected:\n%s", text)
	}
	if strings.Contains(text, "systemctl --user") {
		t.Fatalf("detached backend must not suggest systemctl:\n%s", text)
	}
}

func TestWithImpliedServiceRemoval(t *testing.T) {
	tests := []struct {
		in, want []int
	}{
		{[]int{userUninstallBinary}, []int{userUninstallBinary}},
		{[]int{userUninstallService, userUninstallData}, []int{userUninstallService, userUninstallData}},
		{[]int{userUninstallData}, []int{userUninstallService, userUninstallData}},
		{[]int{userUninstallBinary, userUninstallData}, []int{userUninstallService, userUninstallBinary, userUninstallData}},
	}
	for _, tt := range tests {
		if got := withImpliedServiceRemoval(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("withImpliedServiceRemoval(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestRunUserUninstallRemovesBinaryAndData(t *testing.T) {
	p := userModeHome(t)
	bin := usermode.BinPath()
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := &fakeUserService{backend: usermode.BackendSystemd}
	if err := runUserUninstall(p, svc, []int{userUninstallService, userUninstallBinary, userUninstallData}); err != nil {
		t.Fatalf("runUserUninstall: %v", err)
	}
	if !svc.removed {
		t.Error("service should be removed")
	}
	for _, path := range []string{bin, p.State} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", path)
		}
	}
}

func TestEnsurePureProxyForcesLocalOnlySettings(t *testing.T) {
	p := userModeHome(t)
	cfg := config.Load(p)
	cfg["enable_tun"] = true
	cfg["lan_proxy"] = true
	cfg["lan_panel"] = true
	if err := config.Save(p, cfg); err != nil {
		t.Fatal(err)
	}

	if err := ensurePureProxy(p); err != nil {
		t.Fatalf("ensurePureProxy: %v", err)
	}
	got := config.Load(p)
	for key := range pureProxyOverrides {
		if config.Bool(got, key) {
			t.Errorf("%s should be forced off in user mode", key)
		}
	}
}

func TestInstallUserBinarySkipsSameVersion(t *testing.T) {
	userModeHome(t)
	bin := usermode.BinPath()
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	installed := "#!/bin/sh\necho clashdock 1.2.3\n"
	if err := os.WriteFile(bin, []byte(installed), 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "clashdock")
	if err := os.WriteFile(src, []byte("package binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	handOff, err := installUserBinary(usermode.Info{ExecPath: src}, "1.2.3")
	if err != nil || !handOff {
		t.Fatalf("installUserBinary = %v, %v; same version should hand off without reinstalling", handOff, err)
	}
	if got, _ := os.ReadFile(bin); string(got) != installed {
		t.Fatal("same version must not replace the installed binary")
	}
}

func TestInstallUserBinaryFreshInstall(t *testing.T) {
	userModeHome(t)
	src := filepath.Join(t.TempDir(), "clashdock")
	if err := os.WriteFile(src, []byte("package binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	handOff, err := installUserBinary(usermode.Info{ExecPath: src}, "1.2.3")
	if err != nil || !handOff {
		t.Fatalf("installUserBinary = %v, %v; fresh install should hand off", handOff, err)
	}
	if got, _ := os.ReadFile(usermode.BinPath()); string(got) != "package binary" {
		t.Fatalf("fresh install should copy the package binary, got %q", got)
	}
	if !sameFile(usermode.BinPath(), usermode.BinPath()) {
		t.Fatal("sameFile must be reflexive")
	}
}

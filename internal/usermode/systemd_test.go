package usermode

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Trilives/clashdock/internal/paths"
)

type fakeRunner struct {
	calls   []string
	outputs map[string]string
	fail    map[string]bool
}

func (f *fakeRunner) run(name string, args ...string) (string, error) {
	call := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.calls = append(f.calls, call)
	for prefix, failed := range f.fail {
		if failed && strings.HasPrefix(call, prefix) {
			return f.outputs[prefix], errors.New("failed: " + call)
		}
	}
	for prefix, out := range f.outputs {
		if strings.HasPrefix(call, prefix) {
			return out, nil
		}
	}
	return "", nil
}

func userHome(t *testing.T) paths.Paths {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("CLASHDOCK_HOME", StateDir())
	return paths.Detect()
}

func TestRenderUserUnitQuotesAndEscapes(t *testing.T) {
	text, err := renderUserUnit("/home/a b/rt%1$x", "/home/a b/bin/mihomo", "/home/a b/rt%1$x/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"WorkingDirectory=/home/a b/rt%%1$x\n",
		`ExecStart="/home/a b/bin/mihomo" -d "/home/a b/rt%%1$$x" -f "/home/a b/rt%%1$$x/config.yaml"`,
		"Restart=on-failure",
		"WantedBy=default.target",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("unit missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "User=") || strings.Contains(text, "Capabilit") {
		t.Fatalf("user unit must not request root-only settings:\n%s", text)
	}
}

// newTestSystemd 构造不真正等待、is-active 按 active 回答的 systemd 后端。
func newTestSystemd(p paths.Paths, fr *fakeRunner, active bool) *systemdService {
	if fr.outputs == nil {
		fr.outputs = map[string]string{}
	}
	if active {
		fr.outputs["systemctl --user is-active"] = "active\n"
	} else {
		fr.outputs["systemctl --user is-active"] = "failed\n"
		fr.outputs["journalctl --user -u"] = "bind: address already in use\n"
	}
	svc := newSystemdService(p, fr.run)
	svc.sleep = func(time.Duration) {}
	return svc
}

func TestSystemdInstallWritesUnitAndEnablesService(t *testing.T) {
	p := userHome(t)
	fr := &fakeRunner{}
	svc := newTestSystemd(p, fr, true)

	if svc.Installed() {
		t.Fatal("fresh home must not have the unit installed")
	}
	if err := svc.Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !svc.Installed() {
		t.Fatal("unit file should exist after Install")
	}
	unit, _ := os.ReadFile(UnitPath())
	if !strings.Contains(string(unit), RuntimeDir(p)) || !strings.Contains(string(unit), p.MihomoBin) {
		t.Fatalf("unit must point at the user state runtime and kernel:\n%s", unit)
	}
	want := []string{
		"systemctl --user daemon-reload",
		"systemctl --user enable clashdock-mihomo.service",
		"systemctl --user restart clashdock-mihomo.service",
		"systemctl --user is-active clashdock-mihomo.service",
	}
	if strings.Join(fr.calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls = %q, want %q", fr.calls, want)
	}
}

func TestSystemdInstallStopsOnSystemctlFailure(t *testing.T) {
	p := userHome(t)
	fr := &fakeRunner{fail: map[string]bool{"systemctl --user enable": true}}
	if err := newTestSystemd(p, fr, true).Install(); err == nil {
		t.Fatal("expected enable failure to surface")
	}
	for _, c := range fr.calls {
		if strings.Contains(c, "restart") {
			t.Fatal("must not restart after enable failed")
		}
	}
}

func writeUnit(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(UnitPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(UnitPath(), []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSystemdRemoveDeletesUnitWhenNotLoaded(t *testing.T) {
	p := userHome(t)
	fr := &fakeRunner{
		fail:    map[string]bool{"systemctl --user stop": true},
		outputs: map[string]string{"systemctl --user stop": "Failed to stop clashdock-mihomo.service: Unit clashdock-mihomo.service not loaded.\n"},
	}
	svc := newSystemdService(p, fr.run)
	writeUnit(t)
	if err := svc.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if svc.Installed() {
		t.Fatal("unit file should be gone")
	}
}

func TestSystemdRemoveKeepsUnitWhenStopFails(t *testing.T) {
	p := userHome(t)
	fr := &fakeRunner{
		fail:    map[string]bool{"systemctl --user stop": true},
		outputs: map[string]string{"systemctl --user stop": "Failed to connect to bus: No medium found\n"},
	}
	svc := newSystemdService(p, fr.run)
	writeUnit(t)
	if err := svc.Remove(); err == nil {
		t.Fatal("expected an error when the service could not be stopped")
	}
	if !svc.Installed() {
		t.Fatal("unit must be kept so the still-running kernel stays manageable")
	}
}

func TestSystemdRestartReportsImmediateExit(t *testing.T) {
	p := userHome(t)
	svc := newTestSystemd(p, &fakeRunner{}, false)
	err := svc.Restart()
	if err == nil || !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("expected immediate-exit error with log tail, got %v", err)
	}
}

func TestSystemdActiveAndLogsFallback(t *testing.T) {
	p := userHome(t)
	fr := &fakeRunner{outputs: map[string]string{
		"systemctl --user is-active":        "active\n",
		"journalctl --user -u":              "-- No entries --\n",
		"journalctl --user-unit clashdock-": "line1\nline2\n",
	}}
	svc := newSystemdService(p, fr.run)
	if !svc.Active() {
		t.Fatal("expected active")
	}
	out, err := svc.Logs(5)
	if err != nil || out != "line1\nline2\n" {
		t.Fatalf("Logs = %q, %v; want fallback journal output", out, err)
	}
}

func TestSyncAndRestartSkipsWhenNotInstalled(t *testing.T) {
	p := userHome(t)
	fr := &fakeRunner{}
	if err := SyncAndRestart(p, newSystemdService(p, fr.run)); err != nil {
		t.Fatalf("SyncAndRestart: %v", err)
	}
	if len(fr.calls) != 0 {
		t.Fatalf("no service → no systemctl calls, got %q", fr.calls)
	}
}

func TestDeployValidatesBeforeInstalling(t *testing.T) {
	p := userHome(t)
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, p.MihomoBin, "#!/bin/sh\necho invalid config\nexit 1\n", 0o755)
	writeFile(t, p.ConfigFile, `{"mixed-port":7890}`, 0o644)
	writeFile(t, p.GeositeDat, "geosite", 0o644)
	fr := &fakeRunner{}
	svc := newTestSystemd(p, fr, true)

	if err := Deploy(p, svc); err == nil || !strings.Contains(err.Error(), "invalid config") {
		t.Fatalf("expected validation failure, got %v", err)
	}
	if svc.Installed() || len(fr.calls) != 0 {
		t.Fatalf("invalid config must not install the unit (calls %q)", fr.calls)
	}

	writeFile(t, p.MihomoBin, "#!/bin/sh\nexit 0\n", 0o755)
	if err := Deploy(p, svc); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !svc.Installed() {
		t.Fatal("valid config should install the unit")
	}
	if _, err := os.Stat(RuntimeConfig(p)); err != nil {
		t.Fatalf("runtime config should be staged: %v", err)
	}
}

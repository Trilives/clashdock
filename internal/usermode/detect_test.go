package usermode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassify(t *testing.T) {
	const (
		pkgKernel = "/home/u/clashdock_1.0_linux/deps/mihomo"
		userBin   = "/home/u/.local/bin/clashdock"
		userState = "/home/u/.local/share/clashdock"
	)
	base := launch{userBin: userBin, userState: userState}
	with := func(mut func(*launch)) launch {
		l := base
		mut(&l)
		return l
	}
	tests := []struct {
		name string
		in   launch
		want Mode
	}{
		{"env user forces user", with(func(l *launch) { l.exec = "/usr/bin/clashdock"; l.rootInstalled = true; l.envMode = "user" }), ModeUser},
		{"legacy env portable forces user", with(func(l *launch) { l.exec = "/usr/bin/clashdock"; l.envMode = "portable" }), ModeUser},
		{"env service forces service", with(func(l *launch) { l.exec = "/home/u/pkg/clashdock"; l.kernel = pkgKernel; l.envMode = "service" }), ModeService},
		{"package launch is user", with(func(l *launch) { l.exec = "/home/u/pkg/clashdock"; l.kernel = pkgKernel }), ModeUser},
		{"package launch wins over root service", with(func(l *launch) { l.exec = "/home/u/pkg/clashdock"; l.kernel = pkgKernel; l.rootInstalled = true }), ModeUser},
		{"root service wins over user service", with(func(l *launch) { l.exec = "/usr/bin/clashdock"; l.rootInstalled = true; l.userInstalled = true }), ModeService},
		{"user service installed is user", with(func(l *launch) { l.exec = "/usr/bin/clashdock"; l.userInstalled = true }), ModeUser},
		{"installed user binary is user", with(func(l *launch) { l.exec = userBin }), ModeUser},
		{"user binary wins over root service", with(func(l *launch) { l.exec = userBin; l.rootInstalled = true }), ModeUser},
		{"self-updated user binary is user", with(func(l *launch) { l.exec = userState + "/clashdock-versions/0.3.0/clashdock" }), ModeUser},
		{"system bin is service", with(func(l *launch) { l.exec = "/usr/bin/clashdock" }), ModeService},
		{"bare binary fallback is service", with(func(l *launch) { l.exec = "/tmp/clashdock" }), ModeService},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.in); got != tt.want {
				t.Fatalf("classify(%+v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSiblingDeps(t *testing.T) {
	dir := t.TempDir()
	exec := filepath.Join(dir, "clashdock")

	if depsDir, kernel := siblingDeps(exec); depsDir != "" || kernel != "" {
		t.Fatalf("expected no deps, got %q %q", depsDir, kernel)
	}

	depsDir := filepath.Join(dir, "deps")
	if err := os.MkdirAll(depsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	kernelPath := filepath.Join(depsDir, "mihomo")
	if err := os.WriteFile(kernelPath, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	gotDeps, gotKernel := siblingDeps(exec)
	if gotDeps != depsDir || gotKernel != kernelPath {
		t.Fatalf("siblingDeps = %q %q, want %q %q", gotDeps, gotKernel, depsDir, kernelPath)
	}
	if !(Info{Kernel: gotKernel}).FromPackage() {
		t.Fatal("Info with kernel should report FromPackage")
	}
}

func TestLayoutHonorsXDG(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	if got, want := StateDir(), filepath.Join(home, ".local/share/clashdock"); got != want {
		t.Fatalf("StateDir() = %q, want %q", got, want)
	}
	if got, want := UnitPath(), filepath.Join(home, ".config/systemd/user/clashdock-mihomo.service"); got != want {
		t.Fatalf("UnitPath() = %q, want %q", got, want)
	}
	if got, want := BinPath(), filepath.Join(home, ".local/bin/clashdock"); got != want {
		t.Fatalf("BinPath() = %q, want %q", got, want)
	}

	t.Setenv("XDG_DATA_HOME", "/data")
	t.Setenv("XDG_CONFIG_HOME", "relative/ignored")
	if got := StateDir(); got != "/data/clashdock" {
		t.Fatalf("StateDir() with XDG_DATA_HOME = %q", got)
	}
	if got, want := UnitDir(), filepath.Join(home, ".config/systemd/user"); got != want {
		t.Fatalf("relative XDG_CONFIG_HOME must be ignored: got %q want %q", got, want)
	}
}

func TestLegacyWorkdirNextToBinary(t *testing.T) {
	if got := LegacyWorkdir("/home/u/pkg/clashdock"); got != "/home/u/pkg/clashdock-data" {
		t.Fatalf("LegacyWorkdir = %q", got)
	}
}

package portable

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeKernel 写一个假的 mihomo：`-t` 直接退出 0（校验通过），否则长睡（模拟常驻）。
func fakeKernel(t *testing.T) (bin, runtimeDir string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "mihomo")
	script := "#!/bin/sh\nif [ \"$1\" = \"-t\" ]; then exit 0; fi\nexec sleep 300\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runtimeDir = filepath.Join(dir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "config.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return bin, runtimeDir
}

func TestSupervisorLifecycle(t *testing.T) {
	bin, rt := fakeKernel(t)
	sup := NewSupervisor(bin, rt)

	if err := sup.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := sup.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !sup.Running() {
		t.Fatal("expected running after Start")
	}
	// PID 文件已写。
	if _, err := os.Stat(filepath.Join(rt, "clashdock.pid")); err != nil {
		t.Errorf("expected pid file: %v", err)
	}

	if err := sup.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if sup.Running() {
		t.Fatal("expected stopped after Stop")
	}
	if _, err := os.Stat(filepath.Join(rt, "clashdock.pid")); !os.IsNotExist(err) {
		t.Errorf("expected pid file removed after Stop")
	}
}

func TestSupervisorRestart(t *testing.T) {
	bin, rt := fakeKernel(t)
	sup := NewSupervisor(bin, rt)
	if err := sup.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sup.Stop()
	if err := sup.Restart(); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if !sup.Running() {
		t.Fatal("expected running after Restart")
	}
}

func TestSupervisorValidateFails(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "mihomo")
	// `-t` 返回非 0 → 校验失败。
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	sup := NewSupervisor(bin, dir)
	if err := sup.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSupervisorDoneOnExit(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "mihomo")
	// 立即退出的内核：Done 应很快关闭，Running 变 false。
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	rt := filepath.Join(dir, "runtime")
	os.MkdirAll(rt, 0o755)
	sup := NewSupervisor(bin, rt)
	if err := sup.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	select {
	case <-sup.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("expected Done to close after kernel exits")
	}
	if sup.Running() {
		t.Fatal("expected not running after exit")
	}
}

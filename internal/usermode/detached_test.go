package usermode

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Trilives/clashdock/internal/paths"
)

// fakeKernelPaths 搭一个带假 mihomo 的 state：`-t` 直接退出 0（校验通过），否则长睡（模拟常驻）。
func fakeKernelPaths(t *testing.T, script string) paths.Paths {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CLASHDOCK_HOME", home)
	p := paths.Detect()
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.MihomoBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(RuntimeDir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RuntimeConfig(p), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const sleepyKernel = "#!/bin/sh\nif [ \"$1\" = \"-t\" ]; then exit 0; fi\necho started\nexec sleep 300\n"

func TestDetachedLifecycle(t *testing.T) {
	p := fakeKernelPaths(t, sleepyKernel)
	svc := newDetachedService(p)
	t.Cleanup(func() { svc.Stop() })

	if svc.Installed() || svc.Active() {
		t.Fatal("fresh state must be neither installed nor active")
	}
	if err := svc.Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !svc.Installed() || !svc.Active() {
		t.Fatal("expected installed and active after Install")
	}
	pid, _ := svc.livePID()

	if err := svc.Restart(); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	newPID, ok := svc.livePID()
	if !ok || newPID == pid {
		t.Fatalf("expected a new live pid after Restart, old=%d new=%d ok=%v", pid, newPID, ok)
	}

	waitFor(t, func() bool { out, _ := svc.Logs(10); return strings.Contains(out, "started") })

	if err := svc.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if svc.Installed() || svc.Active() {
		t.Fatal("expected neither installed nor active after Remove")
	}
	if _, err := os.Stat(svc.pidPath); !os.IsNotExist(err) {
		t.Fatal("pid file should be removed after Remove")
	}
}

func TestDetachedStartReportsImmediateExit(t *testing.T) {
	p := fakeKernelPaths(t, "#!/bin/sh\necho bind: address already in use\nexit 1\n")
	svc := newDetachedService(p)
	err := svc.Start()
	if err == nil || !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("expected start failure with log tail, got %v", err)
	}
	if svc.Active() {
		t.Fatal("exited kernel must not be reported active")
	}
	if _, err := os.Stat(svc.pidPath); !os.IsNotExist(err) {
		t.Fatal("pid file should be cleared after a failed start")
	}
}

func TestDetachedIgnoresForeignPID(t *testing.T) {
	p := fakeKernelPaths(t, sleepyKernel)
	svc := newDetachedService(p)
	// 当前测试进程的 PID 不带本服务运行时目录 → 不能被认作内核（防误杀）。
	if err := os.WriteFile(svc.pidPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if svc.Active() {
		t.Fatal("foreign pid must not be reported as active")
	}
	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestProcessZombieParsesState(t *testing.T) {
	if !processZombie("123 (my (odd) name) Z 1 2") {
		t.Fatal("expected zombie")
	}
	if processZombie("123 (mihomo) S 1 2") {
		t.Fatal("sleeping process is not a zombie")
	}
}

func TestTailLines(t *testing.T) {
	if got := tailLines("a\nb\nc\n", 2); got != "b\nc" {
		t.Fatalf("tailLines = %q", got)
	}
}

func TestValidateUsesRuntime(t *testing.T) {
	p := fakeKernelPaths(t, sleepyKernel)
	if err := Validate(p); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	bad := fakeKernelPaths(t, "#!/bin/sh\necho boom\nexit 1\n")
	err := Validate(bad)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected validation error carrying kernel output, got %v", err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

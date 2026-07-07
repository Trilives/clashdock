package portable

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassify(t *testing.T) {
	const pkgKernel = "/home/u/clashdock_1.0_linux/deps/mihomo" // 便携包旁的内核
	tests := []struct {
		name      string
		exec      string
		kernel    string // 旁边的 deps/mihomo，空=无
		installed bool
		env       string
		want      Mode
	}{
		// 环境变量显式覆盖优先级最高。
		{"env portable forces portable", "/usr/bin/clashdock", "", true, "portable", Portable},
		{"env service forces service", "/home/u/pkg/clashdock", pkgKernel, false, "service", Service},

		// 启动上下文决定：解压便携包（旁有 deps/mihomo）即轻量，哪怕已装服务。
		{"package launch is portable", "/home/u/pkg/clashdock", pkgKernel, false, "", Portable},
		{"package launch wins over installed service", "/home/u/pkg/clashdock", pkgKernel, true, "", Portable},

		// 无 deps 兄弟：已装服务或系统路径 → 完整模式。
		{"installed service is service", "/home/u/clashdock", "", true, "", Service},
		{"system bin is service", "/usr/bin/clashdock", "", false, "", Service},
		{"bare binary fallback is service", "/tmp/clashdock", "", false, "", Service},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.exec, tt.kernel, tt.installed, tt.env); got != tt.want {
				t.Fatalf("classify(%q, kernel=%q, installed=%v, env=%q) = %v, want %v",
					tt.exec, tt.kernel, tt.installed, tt.env, got, tt.want)
			}
		})
	}
}

func TestSiblingDeps(t *testing.T) {
	dir := t.TempDir()
	exec := filepath.Join(dir, "clashdock")

	// 无 deps/mihomo → 空。
	if depsDir, kernel := siblingDeps(exec); depsDir != "" || kernel != "" {
		t.Fatalf("expected no deps, got %q %q", depsDir, kernel)
	}

	// 造出 deps/mihomo。
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
}

func TestDirWritable(t *testing.T) {
	if !dirWritable(t.TempDir()) {
		t.Fatal("temp dir should be writable")
	}
	ro := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() != 0 && dirWritable(ro) {
		t.Fatal("0500 dir should not be writable for non-root")
	}
}

func TestDefaultWorkdirUnderExecDir(t *testing.T) {
	// 可执行文件所在目录可写时，工作目录应放在其旁（而非启动 cwd）。
	got := DefaultWorkdir()
	execDir := filepath.Dir(resolveExec())
	if want := filepath.Join(execDir, DefaultWorkdirName); got != want {
		t.Fatalf("DefaultWorkdir() = %q, want %q (next to binary)", got, want)
	}
}

func TestInSystemBinDir(t *testing.T) {
	tests := map[string]bool{
		"/usr/bin/clashdock":                       true,
		"/usr/local/bin/clashdock":                 true,
		"/home/user/clashdock_1.0_linux/clashdock": false,
		"/tmp/x/clashdock":                         false,
	}
	for path, want := range tests {
		if got := inSystemBinDir(path); got != want {
			t.Errorf("inSystemBinDir(%q) = %v, want %v", path, got, want)
		}
	}
}

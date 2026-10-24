package configfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeKernel 写一个假 mihomo：记录 -d 目录内容与配置，按 exitCode 退出。
func fakeKernel(t *testing.T, exitCode int) (bin, record string) {
	t.Helper()
	dir := t.TempDir()
	record = filepath.Join(dir, "record")
	bin = filepath.Join(dir, "mihomo")
	script := `#!/bin/sh
# 参数：-t -d <dir> -f <config>
ls "$3" > "` + record + `"
cat "$5" >> "` + record + `"
echo "boom: bad rule" >&2
exit ` + string(rune('0'+exitCode)) + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, record
}

func TestPrecheckRunsInPrivateDirWithAssetsAndRewrittenUI(t *testing.T) {
	bin, record := fakeKernel(t, 0)
	parent := t.TempDir()
	geosite := filepath.Join(t.TempDir(), "geosite.dat")
	if err := os.WriteFile(geosite, []byte("dat"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]any{"external-ui": "/var/lib/clashdock-runtime/ui", "mixed-port": 7890}

	err := Precheck(bin, parent, cfg, map[string]string{
		"geosite.dat":  geosite,
		"geoip.metadb": filepath.Join(parent, "missing"), // 不存在则跳过
	})

	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(record)
	for _, want := range []string{"geosite.dat", "config.yaml", "/.precheck-"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("check dir / config missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(string(got), "geoip.metadb") {
		t.Errorf("missing asset must be skipped:\n%s", got)
	}
	if strings.Contains(string(got), "clashdock-runtime") {
		t.Errorf("external-ui must point inside the check dir, not the live runtime:\n%s", got)
	}
	if cfg["external-ui"] != "/var/lib/clashdock-runtime/ui" {
		t.Errorf("caller's config mutated: %v", cfg["external-ui"])
	}
	if entries, _ := os.ReadDir(parent); len(entries) != 0 {
		t.Errorf("check dir left behind: %v", entries)
	}
}

func TestPrecheckLinksRelativeAssetsByAbsolutePath(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)
	if err := os.WriteFile("geosite.dat", []byte("GEOSITE-CONTENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	bin := filepath.Join(dir, "mihomo")
	script := "#!/bin/sh\ncat \"$3/geosite.dat\" > \"" + record + "\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Precheck(bin, t.TempDir(), map[string]any{}, map[string]string{"geosite.dat": "geosite.dat"}); err != nil {
		t.Fatal(err)
	}

	if got, _ := os.ReadFile(record); string(got) != "GEOSITE-CONTENT" {
		t.Fatalf("kernel read %q through the asset link, want the source content", got)
	}
}

func TestPrecheckFailureIsPrecheckErrorWithKernelOutput(t *testing.T) {
	bin, _ := fakeKernel(t, 1)

	err := Precheck(bin, t.TempDir(), map[string]any{}, nil)

	var pe *PrecheckError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, want *PrecheckError", err)
	}
	if !strings.Contains(pe.Output, "boom: bad rule") || !strings.Contains(err.Error(), "boom: bad rule") {
		t.Fatalf("kernel output not reported: %v", err)
	}
}

// TestPrecheckWithRealKernel 设置 MIHOMO_BIN 时用真实内核校验（CI 由测试工作流下载）。
func TestPrecheckWithRealKernel(t *testing.T) {
	bin := os.Getenv("MIHOMO_BIN")
	if bin == "" {
		t.Skip("MIHOMO_BIN not set")
	}
	good := map[string]any{"mixed-port": 7890, "mode": "rule", "rules": []any{"MATCH,DIRECT"}}
	if err := Precheck(bin, t.TempDir(), good, nil); err != nil {
		t.Fatalf("minimal valid config rejected: %v", err)
	}
	bad := map[string]any{"mixed-port": 7890, "rules": []any{"MATCH,NO-SUCH-GROUP"}}
	var pe *PrecheckError
	if err := Precheck(bin, t.TempDir(), bad, nil); !errors.As(err, &pe) {
		t.Fatalf("config with an unknown policy = %v, want *PrecheckError", err)
	}
}

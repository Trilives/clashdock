package portable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Trilives/clashdock/internal/paths"
)

// stateWith 在临时目录搭一个最小 state：config.yaml + geo 文件。
func stateWith(t *testing.T, files map[string]string) paths.Paths {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CLASHDOCK_HOME", home)
	p := paths.Detect()
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		full := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func TestStageRuntimeLayout(t *testing.T) {
	p := stateWith(t, map[string]string{
		"config.yaml":          "mixed-port: 7890\n",
		"ruleset/geosite.dat":  "geosite",
		"ruleset/country.mmdb": "country",
		"ruleset/geoip.metadb": "metadb",
	})

	if err := StageRuntime(p); err != nil {
		t.Fatalf("StageRuntime: %v", err)
	}

	rt := RuntimeDir(p)
	// geo 文件必须落在运行时根级（mihomo -d 在此查找），而非 ruleset/ 子目录。
	for _, name := range []string{"config.yaml", "geosite.dat", "country.mmdb", "geoip.metadb"} {
		if _, err := os.Stat(filepath.Join(rt, name)); err != nil {
			t.Errorf("expected %s at runtime root: %v", name, err)
		}
	}
}

func TestStageRuntimeMissingConfig(t *testing.T) {
	p := stateWith(t, map[string]string{
		"ruleset/geosite.dat": "geosite",
	})
	if err := StageRuntime(p); err == nil {
		t.Fatal("expected error when config.yaml is missing")
	}
}

func TestStageRuntimeMissingGeosite(t *testing.T) {
	p := stateWith(t, map[string]string{
		"config.yaml":          "mixed-port: 7890\n",
		"ruleset/country.mmdb": "country",
	})
	if err := StageRuntime(p); err == nil {
		t.Fatal("expected error when geosite.dat is missing")
	}
}

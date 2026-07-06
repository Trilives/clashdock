package sysd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Trilives/clashdock/internal/paths"
)

func freshnessTestPaths(t *testing.T) paths.Paths {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	ruleset := filepath.Join(root, "ruleset")
	p := paths.Paths{
		State:       root,
		Bin:         bin,
		MihomoBin:   filepath.Join(bin, "mihomo"),
		Ruleset:     ruleset,
		GeositeDat:  filepath.Join(ruleset, "geosite.dat"),
		GeoipMetadb: filepath.Join(ruleset, "geoip.metadb"),
		CountryMmdb: filepath.Join(ruleset, "country.mmdb"),
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ruleset, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.MihomoBin, []byte("bin-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.GeositeDat, []byte("geosite-v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAssetsStaleWithoutMarkerIsNotStale(t *testing.T) {
	p := freshnessTestPaths(t)
	if AssetsStale(p) {
		t.Fatal("no marker recorded yet (feature not used before) should not be reported as stale")
	}
}

func TestAssetsStaleDetectsDriftAfterRecord(t *testing.T) {
	p := freshnessTestPaths(t)
	if err := RecordDeployedAssets(p); err != nil {
		t.Fatal(err)
	}
	if AssetsStale(p) {
		t.Fatal("freshly recorded fingerprint should not be stale")
	}

	// mtime 精度在部分文件系统上可能只有秒级；确保新写入的时间戳能被区分出来。
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(p.MihomoBin, []byte("bin-v2-longer"), 0o755); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(p.MihomoBin, future, future); err != nil {
		t.Fatal(err)
	}

	if !AssetsStale(p) {
		t.Fatal("updated mihomo binary after last recorded deploy should be reported as stale")
	}
}

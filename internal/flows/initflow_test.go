package flows

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Trilives/clashdock/internal/paths"
)

func TestStartupResourcesReadyAcceptsBundledDebSeeds(t *testing.T) {
	p := testPaths(t)
	writeTestFile(t, p.MihomoBin)
	writeTestFile(t, p.GeositeDat)
	writeTestFile(t, p.CountryMmdb)

	if !startupResourcesReady(p) {
		t.Fatal("deb bundled mihomo + geosite + country.mmdb should be enough to start")
	}
}

func TestStartupResourcesReadyRequiresKernelAndBasicRules(t *testing.T) {
	p := testPaths(t)
	writeTestFile(t, p.MihomoBin)
	writeTestFile(t, p.GeositeDat)

	if startupResourcesReady(p) {
		t.Fatal("missing geoip.metadb/country.mmdb should require a download before start")
	}
}

func testPaths(t *testing.T) paths.Paths {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	ruleset := filepath.Join(root, "ruleset")
	return paths.Paths{
		State:       root,
		Bin:         bin,
		MihomoBin:   filepath.Join(bin, "mihomo"),
		Ruleset:     ruleset,
		GeositeDat:  filepath.Join(ruleset, "geosite.dat"),
		GeoipMetadb: filepath.Join(ruleset, "geoip.metadb"),
		CountryMmdb: filepath.Join(ruleset, "country.mmdb"),
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

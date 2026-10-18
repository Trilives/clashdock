package kernel

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSeedFromCopiesOnlyMissingFiles(t *testing.T) {
	p := tempPaths(t)
	seedDir := t.TempDir()
	binDir := filepath.Join(seedDir, "bin")
	rulesDir := filepath.Join(seedDir, "ruleset")
	writeFile(t, filepath.Join(binDir, "mihomo"), "seed-core")
	writeFile(t, filepath.Join(rulesDir, "geosite.dat"), "seed-geosite")
	writeFile(t, filepath.Join(rulesDir, "country.mmdb"), "seed-country")
	writeFile(t, filepath.Join(p.Ruleset, "country.mmdb"), "existing-country")

	seeded, err := SeedFrom(p, binDir, rulesDir)

	if err != nil {
		t.Fatalf("SeedFrom: %v", err)
	}
	want := []string{p.MihomoBin, filepath.Join(p.Ruleset, "geosite.dat")}
	if !reflect.DeepEqual(seeded, want) {
		t.Fatalf("seeded = %v, want %v", seeded, want)
	}
	if got := readFile(t, p.MihomoVersion); got != "bundled\n" {
		t.Fatalf("version = %q", got)
	}
	if got := readFile(t, filepath.Join(p.Ruleset, "country.mmdb")); got != "existing-country" {
		t.Fatalf("existing file overwritten: %q", got)
	}
	st, err := os.Stat(p.MihomoBin)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o755 {
		t.Fatalf("core mode = %o, want 755", st.Mode().Perm())
	}
}

func TestSeedFromKeepsExistingCore(t *testing.T) {
	p := tempPaths(t)
	writeFile(t, p.MihomoBin, "downloaded-core")
	binDir := t.TempDir()
	writeFile(t, filepath.Join(binDir, "mihomo"), "seed-core")

	seeded, err := SeedFrom(p, binDir, t.TempDir())

	if err != nil || len(seeded) != 0 {
		t.Fatalf("SeedFrom = %v, %v; want nothing seeded", seeded, err)
	}
	if got := readFile(t, p.MihomoBin); got != "downloaded-core" {
		t.Fatalf("core overwritten: %q", got)
	}
}

func TestCopyTreeDir(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "index.html"), "root")
	writeFile(t, filepath.Join(src, "assets", "app.js"), "js")
	dst := filepath.Join(t.TempDir(), "ui")

	if err := copyTreeDir(src, dst); err != nil {
		t.Fatalf("copyTreeDir: %v", err)
	}

	if readFile(t, filepath.Join(dst, "index.html")) != "root" || readFile(t, filepath.Join(dst, "assets", "app.js")) != "js" {
		t.Fatal("tree not copied")
	}
}

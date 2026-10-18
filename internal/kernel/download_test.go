package kernel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheValid(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, body []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	goodGz := filepath.Join(dir, "good.gz")
	writeGz(t, goodGz, []byte("binary"))
	goodTar := filepath.Join(dir, "good.tar.gz")
	writeTarGz(t, goodTar, []archiveEntry{{name: "a", body: "b"}})
	goodZip := filepath.Join(dir, "good.zip")
	writeZip(t, goodZip, []archiveEntry{{name: "a", body: "b"}})

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "missing", path: filepath.Join(dir, "missing.gz"), want: false},
		{name: "empty", path: write("empty.dat", nil), want: false},
		{name: "plain non-empty file", path: write("geosite.dat", []byte("x")), want: true},
		{name: "valid gz", path: goodGz, want: true},
		{name: "corrupt gz", path: write("bad.gz", []byte("not gzip")), want: false},
		{name: "valid tar.gz", path: goodTar, want: true},
		{name: "corrupt tgz", path: write("bad.tgz", []byte("not gzip")), want: false},
		{name: "valid zip", path: goodZip, want: true},
		{name: "corrupt zip", path: write("bad.zip", []byte("not zip")), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cacheValid(tt.path); got != tt.want {
				t.Fatalf("cacheValid(%s) = %v, want %v", filepath.Base(tt.path), got, tt.want)
			}
		})
	}
}

func TestDownloadToReusesValidCacheWithoutFetching(t *testing.T) {
	out := filepath.Join(t.TempDir(), "mihomo.gz")
	writeGz(t, out, []byte("binary"))

	// nil fetcher: any network attempt would panic, so success proves the cache was used.
	if err := downloadTo(nil, "https://example.invalid/mihomo.gz", out, false); err != nil {
		t.Fatalf("downloadTo: %v", err)
	}
}

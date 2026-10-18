package kernel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallGzipBinaryReplacesTargetAtomically(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "mihomo.gz")
	writeGz(t, archive, []byte("new-binary"))
	dst := filepath.Join(dir, "mihomo")
	if err := os.WriteFile(dst, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := installGzipBinary(archive, dst); err != nil {
		t.Fatalf("installGzipBinary: %v", err)
	}

	if got := readFile(t, dst); got != "new-binary" {
		t.Fatalf("content = %q", got)
	}
	st, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o100 == 0 {
		t.Fatalf("mode = %o, want executable", st.Mode().Perm())
	}
	if _, err := os.Stat(dst + ".new"); !os.IsNotExist(err) {
		t.Fatal("temporary .new file left behind")
	}
}

func TestInstallGzipBinaryRejectsCorruptArchive(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "mihomo.gz")
	if err := os.WriteFile(archive, []byte("not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "mihomo")

	if err := installGzipBinary(archive, dst); err == nil {
		t.Fatal("expected error for corrupt archive")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("target should not be created")
	}
}

func TestFindUIRoot(t *testing.T) {
	mk := func(t *testing.T, root string, files ...string) {
		t.Helper()
		for _, f := range files {
			p := filepath.Join(root, f)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Run("prefers index next to assets", func(t *testing.T) {
		root := t.TempDir()
		mk(t, root, "a/docs/index.html", "b/dist/index.html", "b/dist/assets/app.js")
		if got := findUIRoot(root); got != filepath.Join(root, "b", "dist") {
			t.Fatalf("findUIRoot = %q", got)
		}
	})
	t.Run("falls back to first index", func(t *testing.T) {
		root := t.TempDir()
		mk(t, root, "only/index.html")
		if got := findUIRoot(root); got != filepath.Join(root, "only") {
			t.Fatalf("findUIRoot = %q", got)
		}
	})
	t.Run("no index", func(t *testing.T) {
		if got := findUIRoot(t.TempDir()); got != "" {
			t.Fatalf("findUIRoot = %q, want empty", got)
		}
	})
}

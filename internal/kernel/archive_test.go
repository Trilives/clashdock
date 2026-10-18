package kernel

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

type archiveEntry struct {
	name  string
	body  string
	isDir bool
}

func writeGz(t *testing.T, path string, body []byte) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTarGz(t *testing.T, path string, entries []archiveEntry) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: 0o640, Size: int64(len(e.body)), Typeflag: tar.TypeReg}
		if e.isDir {
			hdr = &tar.Header{Name: e.name, Mode: 0o755, Typeflag: tar.TypeDir}
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if !e.isDir {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeZip(t *testing.T, path string, entries []archiveEntry) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if !e.isDir {
			if _, err := w.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestExtractTarGzWritesFilesWithMode(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "ui.tar.gz")
	writeTarGz(t, archive, []archiveEntry{
		{name: "./", isDir: true},
		{name: "dist/", isDir: true},
		{name: "dist/index.html", body: "<html>"},
	})
	out := filepath.Join(dir, "out")

	if err := extract(archive, out); err != nil {
		t.Fatalf("extract: %v", err)
	}

	target := filepath.Join(out, "dist", "index.html")
	if got := readFile(t, target); got != "<html>" {
		t.Fatalf("content = %q", got)
	}
	st, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", st.Mode().Perm())
	}
}

func TestExtractZipWritesFiles(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "ui.zip")
	writeZip(t, archive, []archiveEntry{
		{name: "dist/", isDir: true},
		{name: "dist/assets/app.js", body: "js"},
	})
	out := filepath.Join(dir, "out")

	if err := extract(archive, out); err != nil {
		t.Fatalf("extract: %v", err)
	}

	if got := readFile(t, filepath.Join(out, "dist", "assets", "app.js")); got != "js" {
		t.Fatalf("content = %q", got)
	}
}

func TestExtractRejectsEntriesOutsideOutDir(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]func(string){
		"evil.zip":    func(p string) { writeZip(t, p, []archiveEntry{{name: "../evil", body: "x"}}) },
		"evil.tar.gz": func(p string) { writeTarGz(t, p, []archiveEntry{{name: "../evil", body: "x"}}) },
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			archive := filepath.Join(dir, name)
			build(archive)

			err := extract(archive, filepath.Join(dir, "out-"+name))

			if err == nil {
				t.Fatal("expected zip-slip entry to be rejected")
			}
			if _, statErr := os.Stat(filepath.Join(dir, "evil")); !os.IsNotExist(statErr) {
				t.Fatal("entry was written outside outDir")
			}
		})
	}
}

func TestExtractRejectsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "ui.rar")
	if err := os.WriteFile(archive, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := extract(archive, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected unsupported format error")
	}
}

func TestSafeJoin(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")
	tests := []struct {
		name    string
		entry   string
		want    string
		wantErr bool
	}{
		{name: "root entry", entry: "./", want: out},
		{name: "nested file", entry: "a/b.txt", want: filepath.Join(out, "a", "b.txt")},
		{name: "parent escape", entry: "../x", wantErr: true},
		{name: "sibling prefix", entry: "../out-other/x", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safeJoin(out, tt.entry)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("safeJoin(%q) = %q, want error", tt.entry, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("safeJoin(%q) = %q, %v; want %q", tt.entry, got, err, tt.want)
			}
		})
	}
}

package usermode

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestInstallBinaryReplacesSymlinkWithoutTouchingTarget(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "pkg", "clashdock")
	writeFile(t, src, "new", 0o755)
	managed := filepath.Join(dir, "versions", "old", "clashdock")
	writeFile(t, managed, "old", 0o755)
	dst := filepath.Join(dir, "bin", "clashdock")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managed, dst); err != nil {
		t.Fatal(err)
	}

	if err := InstallBinary(src, dst); err != nil {
		t.Fatalf("InstallBinary: %v", err)
	}

	if got, _ := os.ReadFile(dst); string(got) != "new" {
		t.Fatalf("installed binary = %q, want new", got)
	}
	if got, _ := os.ReadFile(managed); string(got) != "old" {
		t.Fatalf("symlink target was overwritten: %q", got)
	}
	st, err := os.Lstat(dst)
	if err != nil || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm() != 0o755 {
		t.Fatalf("expected a regular 0755 file, got %v (%v)", st.Mode(), err)
	}
}

func TestRemoveStateRefusesDangerousPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	for _, dir := range []string{"/", home, filepath.Dir(home), "relative/dir"} {
		if err := RemoveState(dir); err == nil {
			t.Errorf("RemoveState(%q) should be refused", dir)
		}
	}
	stray := filepath.Join(home, "projects")
	writeFile(t, filepath.Join(stray, "main.go"), "package main", 0o644)
	if err := RemoveState(stray); err == nil {
		t.Fatal("directory without clashdock data must not be removed")
	}
	if _, err := os.Stat(filepath.Join(stray, "main.go")); err != nil {
		t.Fatal("refused removal must leave files intact")
	}
}

func TestRemoveStateRemovesClashdockData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	custom := filepath.Join(t.TempDir(), "data")
	writeFile(t, filepath.Join(custom, "customize.json"), "{}", 0o644)
	if err := RemoveState(custom); err != nil {
		t.Fatalf("RemoveState(custom data dir): %v", err)
	}
	def := StateDir()
	writeFile(t, filepath.Join(def, "bin", "mihomo"), "k", 0o755)
	if err := RemoveState(def); err != nil {
		t.Fatalf("RemoveState(default dir): %v", err)
	}
	for _, dir := range []string{custom, def} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", dir)
		}
	}
}

func TestRemoveBinaryMissingIsOK(t *testing.T) {
	if err := RemoveBinary(filepath.Join(t.TempDir(), "absent")); err != nil {
		t.Fatalf("RemoveBinary on missing file: %v", err)
	}
}

func TestBinaryVersion(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "clashdock")
	writeFile(t, bin, "#!/bin/sh\necho clashdock v9.9.9\n", 0o755)
	if got := BinaryVersion(bin); got != "v9.9.9" {
		t.Fatalf("BinaryVersion = %q", got)
	}
	if got := BinaryVersion(filepath.Join(t.TempDir(), "absent")); got != "" {
		t.Fatalf("missing binary should report empty version, got %q", got)
	}
}

func TestOnPath(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/home/u/.local/bin/")
	if !OnPath("/home/u/.local/bin") {
		t.Fatal("expected dir on PATH")
	}
	if OnPath("/opt/bin") {
		t.Fatal("unexpected dir on PATH")
	}
}

func TestMigrateLegacyCopiesDataAndSkipsRuntime(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "pkg", "clashdock-data")
	state := filepath.Join(root, "state")
	writeFile(t, filepath.Join(legacy, "subscriptions", "air", "meta.json"), "{}", 0o644)
	writeFile(t, filepath.Join(legacy, "customize.json"), `{"proxy_port":7891}`, 0o644)
	writeFile(t, filepath.Join(legacy, "bin", "mihomo"), "kernel", 0o755)
	writeFile(t, filepath.Join(legacy, "runtime", "config.yaml"), "stale", 0o644)

	if !NeedsMigration(legacy, state) {
		t.Fatal("expected migration to be needed")
	}
	if err := MigrateLegacy(legacy, state); err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}

	for _, rel := range []string{"subscriptions/air/meta.json", "customize.json", "bin/mihomo"} {
		if _, err := os.Stat(filepath.Join(state, rel)); err != nil {
			t.Errorf("expected %s migrated: %v", rel, err)
		}
	}
	if st, err := os.Stat(filepath.Join(state, "bin", "mihomo")); err != nil || st.Mode().Perm()&0o100 == 0 {
		t.Errorf("kernel must stay executable after migration: %v %v", st, err)
	}
	if _, err := os.Stat(filepath.Join(state, "runtime")); !os.IsNotExist(err) {
		t.Error("runtime dir must not be migrated")
	}
	if _, err := os.Stat(filepath.Join(legacy, "customize.json")); err != nil {
		t.Error("legacy data must be left in place")
	}
	if NeedsMigration(legacy, state) {
		t.Fatal("migration must not be offered again once the new state has subscriptions")
	}
}

func TestNeedsMigrationFalseWithoutLegacySubscriptions(t *testing.T) {
	root := t.TempDir()
	if NeedsMigration(filepath.Join(root, "none"), filepath.Join(root, "state")) {
		t.Fatal("no legacy data → no migration")
	}
	same := filepath.Join(root, "same")
	writeFile(t, filepath.Join(same, "subscriptions", "a", "meta.json"), "{}", 0o644)
	if NeedsMigration(same, same) {
		t.Fatal("same directory → no migration")
	}
}

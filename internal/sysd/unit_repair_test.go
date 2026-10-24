package sysd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairedUnitTextReplacesStaleVersionedEntry(t *testing.T) {
	versions := "/var/lib/clashdock/clashdock-versions"
	text := wdServiceText("mihomo", "mihomo", versions+"/0.3.0/clashdock")

	got, ok := repairedUnitText(text, "healthcheck", "/usr/bin/clashdock", func(path string) bool {
		return staleExecPath(path, versions)
	})

	if !ok {
		t.Fatal("an ExecStart inside the versions directory must be repaired")
	}
	if want := wdServiceText("mihomo", "mihomo", "/usr/bin/clashdock"); got != want {
		t.Fatalf("repaired unit =\n%s\nwant\n%s", got, want)
	}
}

func TestRepairedUnitTextLeavesHealthyOrForeignEntriesAlone(t *testing.T) {
	never := func(string) bool { return false }
	always := func(string) bool { return true }
	tests := []struct {
		name  string
		text  string
		stale func(string) bool
	}{
		{"healthy path", timerServiceText("/usr/local/bin/clashdock"), never},
		{"already stable", timerServiceText("/usr/bin/clashdock"), always},
		{"different subcommand", wdServiceText("mihomo", "mihomo", "/gone/clashdock"), always},
		{"no ExecStart", "[Unit]\nDescription=x\n", always},
	}
	for _, tt := range tests {
		if _, ok := repairedUnitText(tt.text, "update", "/usr/bin/clashdock", tt.stale); ok {
			t.Errorf("%s: unit must not be rewritten", tt.name)
		}
	}
}

func TestStaleExecPath(t *testing.T) {
	dir := t.TempDir()
	versions := filepath.Join(dir, "clashdock-versions")
	existing := filepath.Join(dir, "clashdock")
	if err := os.WriteFile(existing, nil, 0o755); err != nil {
		t.Fatal(err)
	}

	if staleExecPath(existing, versions) {
		t.Error("an existing binary outside the versions directory is not stale")
	}
	if !staleExecPath(filepath.Join(dir, "missing"), versions) {
		t.Error("a missing binary is stale")
	}
	if !staleExecPath(filepath.Join(versions, "0.3.1", "clashdock"), versions) {
		t.Error("a versioned binary is stale even while it still exists: pruning removes it later")
	}
}

func TestUnitTextsRecordGivenEntry(t *testing.T) {
	for _, text := range []string{wdServiceText("mihomo", "mihomo", "/usr/bin/clashdock"), timerServiceText("/usr/bin/clashdock")} {
		if !strings.Contains(text, "ExecStart=/usr/bin/clashdock ") {
			t.Errorf("unit does not use the given stable entry:\n%s", text)
		}
	}
}

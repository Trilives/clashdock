package sysd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/paths"
)

func TestStageRuntimeConfigAcceptsYAMLAndRewritesExternalUI(t *testing.T) {
	state := t.TempDir()
	p := paths.Paths{
		State:      state,
		ConfigFile: filepath.Join(state, "config.yaml"),
	}
	raw := "mixed-port: 7890\nexternal-ui: ui\nproxy-groups: []\n"
	if err := os.WriteFile(p.ConfigFile, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	staged, err := stageRuntimeConfig(p, runtimePaths{UI: "/etc/mihomo/ui"})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(staged)

	out, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"/etc/mihomo/ui"`) {
		t.Fatalf("external-ui was not rewritten in staged config:\n%s", out)
	}
}

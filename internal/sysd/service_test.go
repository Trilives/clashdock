package sysd

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/execx"
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

	staged, err := stageRuntimeConfig(p, runtimePaths{UI: "/var/lib/clashdock-runtime/ui"})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(staged)

	out, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"/var/lib/clashdock-runtime/ui"`) {
		t.Fatalf("external-ui was not rewritten in staged config:\n%s", out)
	}
}

func TestSyncStagedAndRestartInstallsLatestConfigBeforeValidateAndRestart(t *testing.T) {
	dir := t.TempDir()
	staged := filepath.Join(dir, "staged.yaml")
	rt := runtimePaths{
		Dir:    dir,
		Bin:    filepath.Join(dir, "mihomo"),
		Config: filepath.Join(dir, "mihomo.yaml"),
	}
	const (
		oldConfig = "mixed-port: 7890\n"
		newConfig = "mixed-port: 7891\n"
	)
	if err := os.WriteFile(staged, []byte(newConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rt.Config, []byte(oldConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls []string
	run := func(cmd []string, _ string, _ *execx.Opt) (execx.Result, error) {
		switch cmd[0] {
		case "install":
			calls = append(calls, "install")
			raw, err := os.ReadFile(cmd[3])
			if err != nil {
				return execx.Result{}, err
			}
			return execx.Result{}, os.WriteFile(cmd[4], raw, 0o644)
		case rt.Bin:
			calls = append(calls, "validate")
		case "systemctl":
			calls = append(calls, "restart")
		default:
			t.Fatalf("unexpected command: %v", cmd)
		}

		raw, err := os.ReadFile(rt.Config)
		if err != nil {
			return execx.Result{}, err
		}
		if got := string(raw); got != newConfig {
			t.Fatalf("%s observed stale config %q; want %q", calls[len(calls)-1], got, newConfig)
		}
		return execx.Result{}, nil
	}

	if err := syncStagedAndRestart(staged, rt, "mihomo", run); err != nil {
		t.Fatalf("syncStagedAndRestart() error = %v", err)
	}
	if want := []string{"install", "validate", "restart"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("command order = %v; want %v", calls, want)
	}
}

func TestSyncStagedAndRestartValidationErrorStopsRestart(t *testing.T) {
	dir := t.TempDir()
	staged := filepath.Join(dir, "staged.yaml")
	rt := runtimePaths{
		Dir:    dir,
		Bin:    filepath.Join(dir, "mihomo"),
		Config: filepath.Join(dir, "mihomo.yaml"),
	}
	if err := os.WriteFile(staged, []byte("invalid: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	validationErr := errors.New("invalid runtime config")
	var calls []string
	run := func(cmd []string, _ string, _ *execx.Opt) (execx.Result, error) {
		switch cmd[0] {
		case "install":
			calls = append(calls, "install")
			raw, err := os.ReadFile(cmd[3])
			if err != nil {
				return execx.Result{}, err
			}
			return execx.Result{}, os.WriteFile(cmd[4], raw, 0o644)
		case rt.Bin:
			calls = append(calls, "validate")
			return execx.Result{}, validationErr
		case "systemctl":
			calls = append(calls, "restart")
		}
		return execx.Result{}, nil
	}

	err := syncStagedAndRestart(staged, rt, "mihomo", run)
	if !errors.Is(err, validationErr) {
		t.Fatalf("syncStagedAndRestart() error = %v; want validation error", err)
	}
	if want := []string{"install", "validate"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("commands after validation failure = %v; want %v", calls, want)
	}
}

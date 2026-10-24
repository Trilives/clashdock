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

	data, err := runtimeConfigData(p, runtimePaths{UI: "/var/lib/clashdock-runtime/ui"})
	if err != nil {
		t.Fatal(err)
	}
	staged, err := stageRuntimeConfig(p, data)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(staged)
	if st, err := os.Stat(staged); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("staged config must be owner-only (node credentials), stat = %v, %v", st, err)
	}

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
			raw, err := os.ReadFile(cmd[len(cmd)-2])
			if err != nil {
				return execx.Result{}, err
			}
			return execx.Result{}, os.WriteFile(cmd[len(cmd)-1], raw, 0o644)
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
			raw, err := os.ReadFile(cmd[len(cmd)-2])
			if err != nil {
				return execx.Result{}, err
			}
			return execx.Result{}, os.WriteFile(cmd[len(cmd)-1], raw, 0o644)
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

// fakeState 一套能通过 preflight 的状态目录（内容是假的，不会被执行）。
func fakeState(t *testing.T) paths.Paths {
	t.Helper()
	t.Setenv("CLASHDOCK_HOME", t.TempDir())
	p := paths.Detect()
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{p.MihomoBin, p.GeositeDat, p.CountryMmdb} {
		if err := os.WriteFile(f, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(p.ConfigFile, []byte(`{"mixed-port":7890,"external-ui":"ui"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInstallPrecheckFailureTouchesNothing(t *testing.T) {
	if !execx.Have("systemctl") {
		t.Skip("preflight needs systemctl")
	}
	p := fakeState(t)
	checkErr := errors.New("kernel rejected config")
	var checkedBin string
	origCheck, origRun := precheckConfig, runRoot
	t.Cleanup(func() { precheckConfig, runRoot = origCheck, origRun })
	precheckConfig = func(bin, _ string, cfg map[string]any, assets map[string]string) error {
		checkedBin = bin
		if cfg["external-ui"] != rtPaths("mihomo").UI || assets["geosite.dat"] != p.GeositeDat {
			t.Errorf("precheck got cfg=%v assets=%v, want the config and geo files about to be deployed", cfg, assets)
		}
		return checkErr
	}
	runRoot = func(cmd []string, _ string, _ *execx.Opt) (execx.Result, error) {
		t.Fatalf("root command %v ran although the precheck failed", cmd)
		return execx.Result{}, nil
	}

	err := Install(p, "mihomo", true)

	if !errors.Is(err, checkErr) {
		t.Fatalf("Install() = %v, want the precheck error", err)
	}
	if checkedBin != p.MihomoBin {
		t.Fatalf("precheck used %q, want the state kernel about to be deployed", checkedBin)
	}
}

func TestSyncRuntimePrecheckFailureKeepsRuntimeConfig(t *testing.T) {
	p := fakeState(t)
	rt := runtimePaths{Dir: t.TempDir(), UI: "/runtime/ui"}
	rt.Bin, rt.Config, rt.Geosite = filepath.Join(rt.Dir, "mihomo"), filepath.Join(rt.Dir, "mihomo.yaml"), filepath.Join(rt.Dir, "geosite.dat")
	checkErr := errors.New("kernel rejected config")
	check := func(bin, _ string, _ map[string]any, assets map[string]string) error {
		if bin != rt.Bin || assets["geosite.dat"] != rt.Geosite {
			t.Errorf("precheck got bin=%q assets=%v, want the running kernel and its geo files", bin, assets)
		}
		return checkErr
	}
	run := func(cmd []string, _ string, _ *execx.Opt) (execx.Result, error) {
		t.Fatalf("root command %v ran although the precheck failed", cmd)
		return execx.Result{}, nil
	}

	if err := syncRuntime(p, rt, "mihomo", check, run); !errors.Is(err, checkErr) {
		t.Fatalf("syncRuntime() = %v, want the precheck error", err)
	}
	if entries, _ := os.ReadDir(p.State); len(entries) == 0 {
		t.Fatal("state dir unexpectedly empty")
	}
	for _, e := range mustReadDir(t, p.State) {
		if strings.HasPrefix(e, ".runtime-config.") {
			t.Errorf("staged config %s left behind", e)
		}
	}
}

func mustReadDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

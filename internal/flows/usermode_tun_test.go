package flows

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/errs"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/usermode"
)

// tunRecorder 记录替身被调用的情况。
type tunRecorder struct {
	confirmAnswer bool
	sudoErr       error
	rootErr       map[string]error // 以命令首个参数之后的第一个参数（能力串或 -r）为键
	hasCaps       bool
	applyErr      error
	tunUp         bool

	sudoCalls int
	rootCmds  [][]string
	applied   []bool
}

func (r *tunRecorder) deps() userTunDeps {
	return userTunDeps{
		confirm: func(string, bool) (bool, error) { return r.confirmAnswer, nil },
		ensureSudo: func(string) error {
			r.sudoCalls++
			return r.sudoErr
		},
		runRoot: func(cmd []string, _ string, _ *execx.Opt) (execx.Result, error) {
			r.rootCmds = append(r.rootCmds, cmd)
			return execx.Result{}, r.rootErr[cmd[1]]
		},
		hasCaps: func(string) bool { return r.hasCaps },
		apply: func(_ paths.Paths, enable bool) error {
			r.applied = append(r.applied, enable)
			if enable {
				return r.applyErr
			}
			return nil
		},
		tunUp: func() bool { return r.tunUp },
	}
}

// tunUserHome 用户模式路径 + 已部署的用户单元（toggle 要求服务已部署）。
func tunUserHome(t *testing.T, tunOn bool) paths.Paths {
	t.Helper()
	p := userModeHome(t)
	if err := os.MkdirAll(filepath.Dir(usermode.UnitPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(usermode.UnitPath(), []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Load(p)
	cfg["enable_tun"] = tunOn
	if err := config.Save(p, cfg); err != nil {
		t.Fatal(err)
	}
	return p
}

func grantCmd(p paths.Paths) []string {
	return []string{"setcap", usermode.TunCapsSpec, p.MihomoBin}
}

func revokeCmd(p paths.Paths) []string {
	return []string{"setcap", "-r", p.MihomoBin}
}

func TestEnableUserTunGrantsCapsAndApplies(t *testing.T) {
	p := tunUserHome(t, false)
	r := &tunRecorder{confirmAnswer: true, tunUp: true}

	if err := toggleUserTun(p, r.deps()); err != nil {
		t.Fatalf("toggleUserTun: %v", err)
	}

	if r.sudoCalls != 1 {
		t.Fatalf("sudo calls = %d, want 1", r.sudoCalls)
	}
	if !reflect.DeepEqual(r.rootCmds, [][]string{grantCmd(p)}) {
		t.Fatalf("root commands = %v", r.rootCmds)
	}
	if !reflect.DeepEqual(r.applied, []bool{true}) {
		t.Fatalf("applied = %v, want [true]", r.applied)
	}
}

func TestEnableUserTunSkipsSudoWhenCapsPresent(t *testing.T) {
	p := tunUserHome(t, false)
	r := &tunRecorder{confirmAnswer: true, hasCaps: true, tunUp: true}

	if err := toggleUserTun(p, r.deps()); err != nil {
		t.Fatalf("toggleUserTun: %v", err)
	}

	if r.sudoCalls != 0 || len(r.rootCmds) != 0 {
		t.Fatalf("caps already present must not prompt sudo: sudo=%d root=%v", r.sudoCalls, r.rootCmds)
	}
	if !reflect.DeepEqual(r.applied, []bool{true}) {
		t.Fatalf("applied = %v, want [true]", r.applied)
	}
}

func TestEnableUserTunConfirmCancelChangesNothing(t *testing.T) {
	p := tunUserHome(t, false)
	r := &tunRecorder{confirmAnswer: false}

	err := toggleUserTun(p, r.deps())

	if !errors.Is(err, errs.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
	if r.sudoCalls != 0 || len(r.rootCmds) != 0 || len(r.applied) != 0 {
		t.Fatalf("cancel must not touch anything: sudo=%d root=%v applied=%v", r.sudoCalls, r.rootCmds, r.applied)
	}
}

func TestEnableUserTunSudoCancelChangesNothing(t *testing.T) {
	p := tunUserHome(t, false)
	r := &tunRecorder{confirmAnswer: true, sudoErr: errs.ErrCancelled}

	err := toggleUserTun(p, r.deps())

	if !errors.Is(err, errs.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
	if len(r.rootCmds) != 0 || len(r.applied) != 0 {
		t.Fatalf("failed authorization must not run root commands or apply: root=%v applied=%v", r.rootCmds, r.applied)
	}
	if config.Bool(config.Load(p), "enable_tun") {
		t.Fatal("enable_tun must stay off")
	}
}

func TestEnableUserTunSetcapFailureChangesNothing(t *testing.T) {
	p := tunUserHome(t, false)
	r := &tunRecorder{confirmAnswer: true, rootErr: map[string]error{usermode.TunCapsSpec: errors.New("setcap: not found")}}

	err := toggleUserTun(p, r.deps())

	if err == nil || !strings.Contains(err.Error(), "setcap") {
		t.Fatalf("err = %v, want setcap failure", err)
	}
	if len(r.applied) != 0 {
		t.Fatalf("setcap failure must not apply: %v", r.applied)
	}
}

func TestEnableUserTunRollsBackWhenApplyFails(t *testing.T) {
	p := tunUserHome(t, false)
	r := &tunRecorder{confirmAnswer: true, applyErr: errors.New("restart failed")}

	err := toggleUserTun(p, r.deps())

	if err == nil {
		t.Fatal("apply failure must be reported")
	}
	if !reflect.DeepEqual(r.applied, []bool{true, false}) {
		t.Fatalf("applied = %v, want [true false] (rollback)", r.applied)
	}
	if !reflect.DeepEqual(r.rootCmds, [][]string{grantCmd(p), revokeCmd(p)}) {
		t.Fatalf("root commands = %v, want grant then revoke", r.rootCmds)
	}
}

func TestEnableUserTunRollsBackWhenDeviceMissing(t *testing.T) {
	p := tunUserHome(t, false)
	r := &tunRecorder{confirmAnswer: true, tunUp: false}

	err := toggleUserTun(p, r.deps())

	if err == nil {
		t.Fatal("missing TUN device must be reported")
	}
	if !reflect.DeepEqual(r.applied, []bool{true, false}) {
		t.Fatalf("applied = %v, want [true false] (rollback)", r.applied)
	}
	if !reflect.DeepEqual(r.rootCmds, [][]string{grantCmd(p), revokeCmd(p)}) {
		t.Fatalf("root commands = %v, want grant then revoke", r.rootCmds)
	}
}

func TestEnableUserTunRollbackKeepsPreexistingCaps(t *testing.T) {
	p := tunUserHome(t, false)
	r := &tunRecorder{confirmAnswer: true, hasCaps: true, tunUp: false}

	if err := toggleUserTun(p, r.deps()); err == nil {
		t.Fatal("missing TUN device must be reported")
	}
	if len(r.rootCmds) != 0 {
		t.Fatalf("caps granted earlier must not be revoked by this rollback: %v", r.rootCmds)
	}
}

func TestDisableUserTunNeedsNoSudo(t *testing.T) {
	p := tunUserHome(t, true)
	r := &tunRecorder{}

	if err := toggleUserTun(p, r.deps()); err != nil {
		t.Fatalf("toggleUserTun: %v", err)
	}

	if r.sudoCalls != 0 || len(r.rootCmds) != 0 {
		t.Fatalf("disabling TUN must not use sudo: sudo=%d root=%v", r.sudoCalls, r.rootCmds)
	}
	if !reflect.DeepEqual(r.applied, []bool{false}) {
		t.Fatalf("applied = %v, want [false]", r.applied)
	}
}

func TestReconcileUserTunRegrantsAfterCoreUpdate(t *testing.T) {
	p := tunUserHome(t, true)
	r := &tunRecorder{confirmAnswer: true}

	reconcileUserTun(p, r.deps())

	if !reflect.DeepEqual(r.rootCmds, [][]string{grantCmd(p)}) {
		t.Fatalf("root commands = %v, want re-grant", r.rootCmds)
	}
	if !config.Bool(config.Load(p), "enable_tun") {
		t.Fatal("re-granted TUN must stay enabled")
	}
}

func TestReconcileUserTunDisablesWhenDeclined(t *testing.T) {
	p := tunUserHome(t, true)
	r := &tunRecorder{confirmAnswer: false}

	reconcileUserTun(p, r.deps())

	if r.sudoCalls != 0 || len(r.rootCmds) != 0 {
		t.Fatalf("declined re-grant must not use sudo: sudo=%d root=%v", r.sudoCalls, r.rootCmds)
	}
	if config.Bool(config.Load(p), "enable_tun") {
		t.Fatal("declined re-grant must turn TUN off")
	}
}

func TestReconcileUserTunNoopWhenCapsPresentOrTunOff(t *testing.T) {
	for _, c := range []struct {
		name    string
		tunOn   bool
		hasCaps bool
	}{{"caps present", true, true}, {"tun off", false, false}} {
		p := tunUserHome(t, c.tunOn)
		r := &tunRecorder{hasCaps: c.hasCaps}
		reconcileUserTun(p, r.deps())
		if r.sudoCalls != 0 || len(r.rootCmds) != 0 || config.Bool(config.Load(p), "enable_tun") != c.tunOn {
			t.Fatalf("%s: reconcile must be a no-op", c.name)
		}
	}
}

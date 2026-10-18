package flows

import (
	"errors"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/errs"
	"github.com/Trilives/clashdock/internal/paths"
)

type singBoxRecorder struct {
	installed, active bool
	answer            bool
	confirmErr        error
	busy              []int

	asked, stopped bool
}

func (r *singBoxRecorder) deps() singBoxDeps {
	return singBoxDeps{
		state: func() (bool, bool) { return r.installed, r.active },
		confirm: func(string, bool) (bool, error) {
			r.asked = true
			return r.answer, r.confirmErr
		},
		stop:      func() error { r.stopped = true; return nil },
		busyPorts: func(...int) []int { return r.busy },
	}
}

func singBoxPaths(t *testing.T) paths.Paths {
	t.Helper()
	t.Setenv("CLASHDOCK_HOME", t.TempDir())
	return paths.Detect()
}

func TestSingBoxAbsentNoPrompt(t *testing.T) {
	r := &singBoxRecorder{}
	if err := resolveSingBoxConflict(singBoxPaths(t), r.deps()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.asked || r.stopped {
		t.Fatal("no sing-box unit: must neither ask nor stop anything")
	}
}

func TestSingBoxStoppedOnlyWithConsent(t *testing.T) {
	r := &singBoxRecorder{installed: true, active: true, answer: true}
	if err := resolveSingBoxConflict(singBoxPaths(t), r.deps()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !r.asked || !r.stopped {
		t.Fatalf("asked=%v stopped=%v, want both after consent", r.asked, r.stopped)
	}
}

func TestSingBoxDeclinedWithBusyPortsFails(t *testing.T) {
	r := &singBoxRecorder{installed: true, active: true, busy: []int{7890, 9090}}
	err := resolveSingBoxConflict(singBoxPaths(t), r.deps())
	if err == nil || !strings.Contains(err.Error(), "7890") {
		t.Fatalf("err = %v, want explicit port conflict", err)
	}
	if r.stopped {
		t.Fatal("declined: sing-box must not be stopped")
	}
}

func TestSingBoxDeclinedWithFreePortsContinues(t *testing.T) {
	r := &singBoxRecorder{installed: true, active: true}
	if err := resolveSingBoxConflict(singBoxPaths(t), r.deps()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.stopped {
		t.Fatal("declined: sing-box must not be stopped")
	}
}

func TestSingBoxInstalledButStoppedDeclinedContinues(t *testing.T) {
	r := &singBoxRecorder{installed: true, busy: []int{7890}}
	if err := resolveSingBoxConflict(singBoxPaths(t), r.deps()); err != nil {
		t.Fatalf("stopped sing-box holds no ports; err = %v", err)
	}
	if r.stopped {
		t.Fatal("declined: sing-box must not be touched")
	}
}

func TestSingBoxPromptCancelPropagates(t *testing.T) {
	r := &singBoxRecorder{installed: true, active: true, confirmErr: errs.ErrCancelled}
	if err := resolveSingBoxConflict(singBoxPaths(t), r.deps()); !errors.Is(err, errs.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
	if r.stopped {
		t.Fatal("cancelled prompt must not stop sing-box")
	}
}

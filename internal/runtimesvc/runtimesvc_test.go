package runtimesvc

import (
	"testing"

	"github.com/Trilives/clashdock/internal/paths"
)

func TestForSelectsTargetByMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, ok := For(paths.Paths{}).(systemRuntime); !ok {
		t.Fatal("default mode must target the systemd system service")
	}
	p := paths.Paths{State: t.TempDir(), UserMode: true}
	got, ok := For(p).(userRuntime)
	if !ok {
		t.Fatal("user mode must target the user service")
	}
	if got.Installed() {
		t.Fatal("fresh user home must not report an installed user service")
	}
	if err := got.SyncAndRestart(); err != nil {
		t.Fatalf("SyncAndRestart without a deployed user service should be a no-op: %v", err)
	}
}

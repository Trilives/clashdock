package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Trilives/clashdock/internal/flows"
	"github.com/Trilives/clashdock/internal/paths"
)

func TestNeedsElevationOnlyForStateCommandsAsNonRoot(t *testing.T) {
	tests := []struct {
		cmd              string
		isRoot, readable bool
		want             bool
	}{
		{"", false, true, false}, // 交互式主菜单按需提权，见 menuNeedsRootAtStart
		{"init", false, true, true},
		{"modify", false, true, true},
		{"update", false, true, true},
		{"uninstall", false, true, true},
		{"pause", false, true, true},
		{"resume", false, true, true},
		{"nettest", false, true, false}, // 管理员能读设置时不必输入密码
		{"nettest", false, false, true},
		{"version", false, false, false},
		{"help", false, false, false},
		{"healthcheck", false, false, false}, // 只由 root 的 watchdog 单元运行，从不自行提权
		{"update", true, true, false},
	}
	for _, tt := range tests {
		if got := needsElevation(tt.cmd, tt.isRoot, tt.readable); got != tt.want {
			t.Errorf("needsElevation(%q, root=%v, readable=%v) = %v, want %v",
				tt.cmd, tt.isRoot, tt.readable, got, tt.want)
		}
	}
}

func TestMenuNeedsRootAtStartOnlyOnFirstRunOrUnreadableSettings(t *testing.T) {
	tests := []struct {
		name string
		st   startState
		want bool
	}{
		{"deployed, settings readable", startState{true, true, true}, false},
		{"no state yet: init follows", startState{false, true, false}, true},
		{"settings unreadable: group is not the caller's", startState{true, false, false}, true},
		{"language never chosen: first run writes it", startState{true, true, false}, true},
	}
	for _, tt := range tests {
		if got := menuNeedsRootAtStart(tt.st); got != tt.want {
			t.Errorf("%s: menuNeedsRootAtStart = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestProbeStartStateReadsLanguageOnlyWhenReadable(t *testing.T) {
	t.Setenv("CLASHDOCK_HOME", filepath.Join(t.TempDir(), "state"))
	t.Setenv("CLASHDOCK_LANG", "")
	p := paths.Detect()
	if got := probeStartState(p); got != (startState{false, true, false}) {
		t.Fatalf("missing state = %+v", got)
	}
	if err := os.MkdirAll(p.State, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.CustomizeFile, []byte(`{"language":"zh"}`), 0o640); err != nil {
		t.Fatal(err)
	}

	if got := probeStartState(p); got != (startState{true, true, true}) {
		t.Fatalf("deployed state = %+v, want everything ready", got)
	}
}

func TestSplitResumeStripsOnlyTheLeadingFlag(t *testing.T) {
	args, resume := splitResume([]string{flows.ResumeFlag + "config"})
	if len(args) != 0 || resume != "config" {
		t.Fatalf("splitResume = (%v, %q), want ([], config)", args, resume)
	}
	args, resume = splitResume([]string{"nettest", flows.ResumeFlag + "config"})
	if !reflect.DeepEqual(args, []string{"nettest", flows.ResumeFlag + "config"}) || resume != "" {
		t.Fatalf("splitResume = (%v, %q), a later flag is not a resume request", args, resume)
	}
}

func TestEveryStateCommandIsDispatched(t *testing.T) {
	// stateCommands 与 main 的分发表须一致：漏掉的入口会以普通用户身份写 root 独占的目录。
	dispatched := map[string]bool{"init": true, "modify": true, "uninstall": true,
		"update": true, "pause": true, "resume": true}
	for cmd := range stateCommands {
		if !dispatched[cmd] {
			t.Errorf("stateCommands has %q, which main does not dispatch", cmd)
		}
	}
	for cmd := range dispatched {
		if !stateCommands[cmd] {
			t.Errorf("main dispatches %q, which must prepare a root-owned state directory", cmd)
		}
	}
}

func TestEveryRootMenuEntryCanBeResumed(t *testing.T) {
	// RequireRoot 以菜单项 id 作恢复目标：id 重复或为空时，提权后会进错菜单或回不到原处。
	seen := map[string]bool{}
	for _, e := range mainMenu() {
		if e.id == "" || seen[e.id] {
			t.Errorf("menu entry id %q is empty or duplicated", e.id)
		}
		seen[e.id] = true
	}
	for _, extra := range []string{resumeInit, resumeRedeploy, flows.ResumeUpdate} {
		if seen[extra] {
			t.Errorf("resume target %q collides with a main menu entry", extra)
		}
	}
}

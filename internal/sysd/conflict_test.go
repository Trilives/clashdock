package sysd

import (
	"strings"
	"testing"
)

// fakeUnits 按 systemctl 参数返回预设输出。
func fakeUnits(loadState, activeState string) unitQuery {
	return func(args ...string) string {
		switch args[0] {
		case "show":
			return loadState
		case "is-active":
			return activeState
		}
		return ""
	}
}

func TestConflictState(t *testing.T) {
	cases := []struct {
		name, load, active string
		installed, running bool
	}{
		{"not installed", "not-found", "inactive", false, false},
		{"installed and stopped", "loaded", "inactive", true, false},
		{"installed and running", "loaded", "active", true, true},
	}
	for _, c := range cases {
		installed, running := conflictState(fakeUnits(c.load, c.active))
		if installed != c.installed || running != c.running {
			t.Errorf("%s: got (%v, %v), want (%v, %v)", c.name, installed, running, c.installed, c.running)
		}
	}
}

func TestConflictStateQueriesSingBoxUnit(t *testing.T) {
	var seen []string
	conflictState(func(args ...string) string {
		seen = append(seen, strings.Join(args, " "))
		return "loaded"
	})
	for _, q := range seen {
		if !strings.HasSuffix(q, ConflictingName+".service") {
			t.Fatalf("query %q must target %s.service", q, ConflictingName)
		}
	}
}

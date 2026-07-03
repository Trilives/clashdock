package paths

import (
	"strings"
	"testing"
)

func TestClashdockHomeWins(t *testing.T) {
	t.Setenv("CLASHDOCK_HOME", "/tmp/cdhome")
	if got := Detect().State; got != "/tmp/cdhome" {
		t.Fatalf("State = %s, CLASHDOCK_HOME 应最优先", got)
	}
}

func TestFixedDefault(t *testing.T) {
	t.Setenv("CLASHDOCK_HOME", "")
	if got := Detect().State; got != DefaultStateRoot {
		t.Fatalf("State = %s, 默认应为固定目录 %s", got, DefaultStateRoot)
	}
}

func TestDerivedPaths(t *testing.T) {
	t.Setenv("CLASHDOCK_HOME", "/tmp/cd")
	p := Detect()
	checks := map[string]string{
		p.ConfigFile:    "/tmp/cd/config.yaml",
		p.CustomizeFile: "/tmp/cd/customize.json",
		p.ActiveFile:    "/tmp/cd/active",
		p.MihomoBin:     "/tmp/cd/bin/mihomo",
		p.GeositeDat:    "/tmp/cd/ruleset/geosite.dat",
	}
	for got, want := range checks {
		if got != want {
			t.Errorf("路径 = %s, 期望 %s", got, want)
		}
	}
	if !strings.HasPrefix(p.SubscriptionDir("Hua"), "/tmp/cd/subscriptions") {
		t.Error("SubscriptionDir 应位于 subscriptions/ 下")
	}
}

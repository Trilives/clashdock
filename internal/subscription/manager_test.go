package subscription

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/paths"
)

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"  Hua  ": "Hua",
		"a/b\\c":  "a-b-c",
		"x .. y":  "x---y",
		"":        "sub",
		". ":      "sub",
		"多 词  订阅": "多-词-订阅",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestMetaRoundtripPythonCompatible(t *testing.T) {
	t.Setenv("CLASHDOCK_HOME", t.TempDir())
	p := paths.Detect()
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}
	// Python 版写出的 meta.json（字段名快照）
	pyMeta := `{
  "name": "Hua",
  "url": "https://example.com/sub",
  "source_type": "clash",
  "apply_overlay": false,
  "created_at": "2026-07-01T10:00:00+00:00",
  "updated_at": "2026-07-02T10:00:00+00:00",
  "last_node_count": 42
}`
	dir := p.SubscriptionDir("Hua")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "meta.json"), []byte(pyMeta), 0o644)

	sub := Get(p, "Hua")
	if sub == nil {
		t.Fatal("应能直读 Python 版 meta.json")
	}
	if sub.Name != "Hua" || sub.SourceType != "clash" || sub.LastNodeCount != 42 {
		t.Errorf("meta 字段解析不符: %+v", sub)
	}

	os.WriteFile(p.ActiveFile, []byte("Hua\n"), 0o644)
	if active := GetActive(p); active == nil || active.Name != "Hua" {
		t.Error("GetActive 应解析 active 指针")
	}

	subs := ListAll(p)
	if len(subs) != 1 || subs[0].Name != "Hua" {
		t.Errorf("ListAll = %+v", subs)
	}
}

func TestFetchProxyCandidatesDefaultsToDirect(t *testing.T) {
	cfg := map[string]any{"download_proxy": "http://192.168.1.10:7890"}

	if got := fetchProxyCandidates(cfg, false); got != nil {
		t.Fatalf("default no-proxy choice should fetch directly, got %v", got)
	}
	got := fetchProxyCandidates(cfg, true)
	want := []string{"http://127.0.0.1:7890", "http://192.168.1.10:7890"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("opt-in proxy choice should try local mixed-port then download_proxy, got %v", got)
	}
}

func TestPrimaryProxy(t *testing.T) {
	if got := primaryProxy(nil); got != "" {
		t.Fatalf("primaryProxy(nil) = %q, want empty", got)
	}
	if got := primaryProxy([]string{"", "http://x:1"}); got != "http://x:1" {
		t.Fatalf("primaryProxy should skip empty candidates, got %q", got)
	}
}

func TestApplyActiveWithSyncPublishesLatestConfigBeforeRestart(t *testing.T) {
	t.Setenv("CLASHDOCK_HOME", t.TempDir())
	p := paths.Detect()
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.SubscriptionDir("Hua"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("mixed-port: 7891\nrules:\n  - MATCH,DIRECT\n")
	if err := os.WriteFile(configFile(p, "Hua"), want, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ConfigFile, []byte("mixed-port: 7890\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	called := false
	err := applyActiveWithSync(p, "Hua", func(got paths.Paths) error {
		called = true
		data, err := os.ReadFile(got.ConfigFile)
		if err != nil {
			return err
		}
		if string(data) != string(want) {
			t.Fatalf("sync saw stale config:\n%s", data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("runtime sync was not called")
	}
	active, err := os.ReadFile(p.ActiveFile)
	if err != nil || string(active) != "Hua\n" {
		t.Fatalf("active pointer = %q, err=%v", active, err)
	}
}

func TestApplyActiveWithSyncReturnsRuntimeSyncFailure(t *testing.T) {
	t.Setenv("CLASHDOCK_HOME", t.TempDir())
	p := paths.Detect()
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.SubscriptionDir("Hua"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configFile(p, "Hua"), []byte("mixed-port: 7891\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("restart failed")
	err := applyActiveWithSync(p, "Hua", func(paths.Paths) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("applyActiveWithSync error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "配置已保存") || !strings.Contains(err.Error(), "同步到运行时失败") {
		t.Fatalf("error should explain saved/runtime split, got %q", err)
	}
}

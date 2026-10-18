package flows

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/configfile"
	"github.com/Trilives/clashdock/internal/paths"
)

func prepareMainGroupCustomize(t *testing.T, keywords []string) paths.Paths {
	t.Helper()
	t.Setenv("CLASHDOCK_HOME", t.TempDir())
	p := paths.Detect()
	customize := config.Defaults()
	customize["main_group_keywords"] = append([]string(nil), keywords...)
	if err := config.Save(p, customize); err != nil {
		t.Fatal(err)
	}
	return p
}

func nodeSelectConfig(groups ...map[string]any) map[string]any {
	items := make([]any, len(groups))
	for i, group := range groups {
		items[i] = group
	}
	return map[string]any{"proxy-groups": items}
}

func nodeSelectGroup(name string, proxies ...string) map[string]any {
	members := make([]any, len(proxies))
	for i, proxy := range proxies {
		members[i] = proxy
	}
	return map[string]any{"name": name, "type": "select", "proxies": members}
}

func TestPersistPinnedSelectionWithSyncPublishesLatestConfigBeforeRestart(t *testing.T) {
	t.Setenv("CLASHDOCK_HOME", t.TempDir())
	p := paths.Detect()
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.SubscriptionDir("airport"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.SubscriptionDir("airport"), "meta.json"), []byte(`{"name":"airport"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ActiveFile, []byte("airport\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := []byte(`{"proxy-groups":[{"name":"手动选择","type":"select","proxies":["香港 01","日本 01"]}]}`)
	subCfg := filepath.Join(p.SubscriptionDir("airport"), "config.yaml")
	if err := os.WriteFile(p.ConfigFile, old, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(subCfg, old, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := map[string]any{
		"proxy-groups": []any{
			map[string]any{
				"name":    "手动选择",
				"type":    "select",
				"proxies": []any{"香港 01", "日本 01"},
			},
		},
	}

	called := false
	err := persistPinnedSelectionWithSync(p, cfg, "手动选择", "日本 01", p.ConfigFile, func(got paths.Paths) error {
		called = true
		assertPinnedNode(t, got.ConfigFile, "手动选择", "日本 01")
		assertPinnedNode(t, subCfg, "手动选择", "日本 01")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("runtime sync was not called")
	}
}

func TestPersistPinnedSelectionWithSyncReturnsRuntimeSyncFailure(t *testing.T) {
	t.Setenv("CLASHDOCK_HOME", t.TempDir())
	p := paths.Detect()
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}

	cfg := map[string]any{
		"proxy-groups": []any{
			map[string]any{
				"name":    "手动选择",
				"type":    "select",
				"proxies": []any{"香港 01", "日本 01"},
			},
		},
	}
	wantErr := errors.New("restart failed")
	err := persistPinnedSelectionWithSync(p, cfg, "手动选择", "日本 01", p.ConfigFile, func(paths.Paths) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("persistPinnedSelectionWithSync error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "节点首选已保存") || !strings.Contains(err.Error(), "同步到运行时失败") {
		t.Fatalf("error should explain saved/runtime split, got %q", err)
	}
	assertPinnedNode(t, p.ConfigFile, "手动选择", "日本 01")
}

func assertPinnedNode(t *testing.T, configPath, groupName, want string) {
	t.Helper()
	cfg, err := configfile.Read(configPath)
	if err != nil {
		t.Fatal(err)
	}
	group, err := pickGroup(cfg, groupName, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := preferredNode(group); got != want {
		t.Fatalf("preferredNode(%s) = %q, want %q", groupName, got, want)
	}
}

func fakeClashAPI(t *testing.T, proxiesStatus int, proxiesBody string) map[string]any {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version":
			w.Write([]byte(`{"version":"test"}`))
		case "/proxies":
			w.WriteHeader(proxiesStatus)
			w.Write([]byte(proxiesBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return map[string]any{"external-controller": strings.TrimPrefix(srv.URL, "http://")}
}

func TestConnectRuntimeWithoutControllerSkipsAPI(t *testing.T) {
	rt := connectRuntime(map[string]any{})

	if rt.apiOK || rt.selectionsOK || rt.selections == nil || len(rt.selections) != 0 {
		t.Fatalf("connectRuntime = %+v, want unreachable with empty selections", rt)
	}
}

func TestConnectRuntimeReadsCurrentSelections(t *testing.T) {
	cfg := fakeClashAPI(t, http.StatusOK, `{"proxies":{"Proxy":{"now":"HK 01"},"HK 01":{}}}`)

	rt := connectRuntime(cfg)

	if !rt.apiOK || !rt.selectionsOK {
		t.Fatalf("connectRuntime = %+v, want API and selections available", rt)
	}
	if !reflect.DeepEqual(rt.selections, map[string]string{"Proxy": "HK 01"}) {
		t.Fatalf("selections = %v", rt.selections)
	}
}

func TestConnectRuntimeKeepsAPIWhenSelectionsFail(t *testing.T) {
	cfg := fakeClashAPI(t, http.StatusInternalServerError, ``)

	rt := connectRuntime(cfg)

	if !rt.apiOK || rt.selectionsOK || len(rt.selections) != 0 {
		t.Fatalf("connectRuntime = %+v, want API reachable but selections unavailable", rt)
	}
}

func TestBuildRegionMenuOrdersRegionsOtherThenSubgroups(t *testing.T) {
	buckets := map[string][]string{
		"sg":     {"SG 01"},
		"hk":     {"HK 01", "HK 02"},
		otherKey: {"DE 01"},
	}

	menu := buildRegionMenu(buckets, []string{"Auto"})

	var got [][]string
	for _, e := range menu {
		got = append(got, e.items)
	}
	want := [][]string{{"HK 01", "HK 02"}, {"SG 01"}, {"DE 01"}, {"Auto"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("menu items = %v, want %v", got, want)
	}
}

func TestBuildRegionMenuEmpty(t *testing.T) {
	if menu := buildRegionMenu(map[string][]string{}, nil); len(menu) != 0 {
		t.Fatalf("menu = %v, want empty", menu)
	}
}

func TestPreferredIndex(t *testing.T) {
	items := []string{"A", "B", "C"}
	tests := []struct {
		name, runtime, pinned string
		want                  int
	}{
		{name: "runtime current wins", runtime: "C", pinned: "B", want: 2},
		{name: "falls back to pinned", pinned: "B", want: 1},
		{name: "runtime not listed", runtime: "Z", pinned: "B", want: 0},
		{name: "nothing known", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := preferredIndex(items, tt.runtime, tt.pinned); got != tt.want {
				t.Fatalf("preferredIndex = %d, want %d", got, tt.want)
			}
		})
	}
}

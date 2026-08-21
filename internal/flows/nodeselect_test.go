package flows

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/configfile"
	"github.com/Trilives/clashdock/internal/paths"
)

func TestPickGroupDoesNotGuessLargestSelectWhenMainGroupIsUnrecognized(t *testing.T) {
	cfg := nodeSelectConfig(
		nodeSelectGroup("入口", "香港 01"),
		nodeSelectGroup("机场线路", "日本 01", "新加坡 01", "美国 01"),
	)

	group, err := pickGroup(cfg, "", []string{"主选择", "proxy"})
	if err == nil {
		t.Fatalf("pickGroup() = %v, nil；未命中主选择组时不应猜测成员最多的 select 组", group)
	}
	if group != nil {
		t.Fatalf("pickGroup() group = %v, want nil", group)
	}
	if !strings.Contains(err.Error(), "未识别到主选择组") {
		t.Fatalf("pickGroup() error = %q，应可判定为未识别主选择组", err)
	}
}

func TestResolveMainGroupPrependsEnteredKeywordPersistsAndRecognizesImmediately(t *testing.T) {
	p := prepareMainGroupCustomize(t, []string{"旧关键词"})
	cfg := nodeSelectConfig(
		nodeSelectGroup("Alpha Control", "香港 01", "日本 01"),
		nodeSelectGroup("Other Pool", "新加坡 01"),
	)

	inputCalls := 0
	group, err := resolveMainGroup(p, cfg, "", func(prompt string) (string, error) {
		inputCalls++
		if !strings.Contains(prompt, "主选择组") {
			t.Fatalf("输入提示 = %q，应说明主选择组", prompt)
		}
		return "Alpha", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if inputCalls != 1 {
		t.Fatalf("input 调用次数 = %d, want 1", inputCalls)
	}
	if got := group["name"]; got != "Alpha Control" {
		t.Fatalf("resolveMainGroup() name = %v, want Alpha Control", got)
	}
	if got := config.StrList(config.Load(p), "main_group_keywords"); len(got) < 2 || got[0] != "Alpha" || got[1] != "旧关键词" {
		t.Fatalf("保存后的 main_group_keywords = %v, want [Alpha 旧关键词 ...]", got)
	}
}

func TestResolveMainGroupEmptyInputDoesNotWriteCustomize(t *testing.T) {
	p := prepareMainGroupCustomize(t, []string{"旧关键词"})
	before, err := os.ReadFile(p.CustomizeFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg := nodeSelectConfig(nodeSelectGroup("Unknown Pool", "香港 01"))

	group, err := resolveMainGroup(p, cfg, "", func(string) (string, error) {
		return "  \t ", nil
	})
	if err == nil || !strings.Contains(err.Error(), "未识别到主选择组，无法切换") {
		t.Fatalf("resolveMainGroup() = %v, %v；空输入应返回无法切换提示", group, err)
	}
	if group != nil {
		t.Fatalf("resolveMainGroup() group = %v, want nil", group)
	}
	after, readErr := os.ReadFile(p.CustomizeFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("空输入后 customize.json 被改写\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestResolveMainGroupPropagatesInputErrorWithoutWritingCustomize(t *testing.T) {
	p := prepareMainGroupCustomize(t, []string{"旧关键词"})
	before, err := os.ReadFile(p.CustomizeFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg := nodeSelectConfig(nodeSelectGroup("Unknown Pool", "香港 01"))
	wantErr := errors.New("input cancelled")

	group, err := resolveMainGroup(p, cfg, "", func(string) (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("resolveMainGroup() error = %v, want wrapped %v", err, wantErr)
	}
	if group != nil {
		t.Fatalf("resolveMainGroup() group = %v, want nil", group)
	}
	after, readErr := os.ReadFile(p.CustomizeFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("输入失败后 customize.json 被改写\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestResolveMainGroupRejectsUnmatchedKeywordWithoutWritingCustomize(t *testing.T) {
	p := prepareMainGroupCustomize(t, []string{"旧关键词"})
	before, err := os.ReadFile(p.CustomizeFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg := nodeSelectConfig(nodeSelectGroup("Unknown Pool", "香港 01"))

	group, err := resolveMainGroup(p, cfg, "", func(string) (string, error) {
		return "不存在的关键词", nil
	})
	if err == nil || !strings.Contains(err.Error(), "未匹配") {
		t.Fatalf("resolveMainGroup() = %v, %v；无效关键词应返回未匹配提示", group, err)
	}
	if group != nil {
		t.Fatalf("resolveMainGroup() group = %v, want nil", group)
	}
	after, readErr := os.ReadFile(p.CustomizeFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("无效关键词输入后 customize.json 被改写\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestCurrentNodeSummaryOnlyIncludesRecognizedMainGroup(t *testing.T) {
	cfg := nodeSelectConfig(
		nodeSelectGroup("Main Select", "香港 01", "日本 01"),
		nodeSelectGroup("Fallback Select", "新加坡 01", "美国 01"),
	)
	selections := map[string]string{
		"Main Select":     "日本 01",
		"Fallback Select": "美国 01",
	}

	lines := currentNodeSummary(cfg, "Main Select", selections, true)
	if len(lines) != 1 {
		t.Fatalf("currentNodeSummary() = %v, want exactly one main-group line", lines)
	}
	if !strings.Contains(lines[0], "Main Select") || !strings.Contains(lines[0], "日本 01") || !strings.Contains(lines[0], "当前运行") {
		t.Fatalf("主选择组当前状态 = %q，缺少组名、当前节点或运行态", lines[0])
	}
	if strings.Contains(lines[0], "Fallback Select") || strings.Contains(lines[0], "美国 01") {
		t.Fatalf("主选择组摘要不应包含其它组状态：%q", lines[0])
	}
}

func TestCurrentNodeSummaryFallsBackToMainGroupConfiguredPreference(t *testing.T) {
	cfg := nodeSelectConfig(
		nodeSelectGroup("Main Select", "香港 01", "日本 01"),
		nodeSelectGroup("Fallback Select", "美国 01"),
	)

	lines := currentNodeSummary(cfg, "Main Select", nil, false)
	if len(lines) != 1 {
		t.Fatalf("currentNodeSummary() = %v, want exactly one main-group line", lines)
	}
	if !strings.Contains(lines[0], "Main Select") || !strings.Contains(lines[0], "香港 01") ||
		!strings.Contains(lines[0], "配置首选") || !strings.Contains(lines[0], "非运行时状态") {
		t.Fatalf("主选择组配置状态 = %q，缺少组名、配置首选或非运行时标记", lines[0])
	}
	if strings.Contains(lines[0], "Fallback Select") || strings.Contains(lines[0], "美国 01") {
		t.Fatalf("主选择组摘要不应包含其它组状态：%q", lines[0])
	}
}

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

func TestCurrentNodeLabelsDistinguishRuntimeAndConfiguredPreferred(t *testing.T) {
	names := []string{"香港 01", "日本 01", "新加坡 01"}
	labels := currentNodeLabels(
		names,
		map[string]int{"香港 01": 38, "日本 01": 61, "新加坡 01": 75},
		"香港 01",
		"日本 01",
		true,
	)

	runtimeLabel := nodeLabel(t, names, labels, "香港 01")
	if !strings.Contains(runtimeLabel, "当前运行") {
		t.Fatalf("运行时当前节点标签 = %q，应包含“当前运行”", runtimeLabel)
	}
	if strings.Contains(runtimeLabel, "配置首选") {
		t.Fatalf("仅为运行时当前节点的标签 = %q，不应标成配置首选", runtimeLabel)
	}

	configuredLabel := nodeLabel(t, names, labels, "日本 01")
	if !strings.Contains(configuredLabel, "配置首选") {
		t.Fatalf("配置固定首选节点标签 = %q，应包含“配置首选”", configuredLabel)
	}
	if strings.Contains(configuredLabel, "当前运行") {
		t.Fatalf("仅为配置首选节点的标签 = %q，不应标成当前运行", configuredLabel)
	}
}

func TestCurrentNodeLabelsCanShowRuntimeAndConfiguredOnSameNode(t *testing.T) {
	names := []string{"香港 01"}
	labels := currentNodeLabels(names, map[string]int{"香港 01": 38}, "香港 01", "香港 01", true)
	label := nodeLabel(t, names, labels, "香港 01")

	if !strings.Contains(label, "当前运行") || !strings.Contains(label, "配置首选") {
		t.Fatalf("同为运行时当前与配置首选的节点标签 = %q，应同时显示两种状态", label)
	}
}

func TestCurrentNodeLabelsMarksConfiguredStateAsNonRuntimeWhenAPIUnavailable(t *testing.T) {
	names := []string{"香港 01", "日本 01"}
	labels := currentNodeLabels(names, nil, "", "日本 01", false)
	configuredLabel := nodeLabel(t, names, labels, "日本 01")

	if !strings.Contains(configuredLabel, "配置首选") || !strings.Contains(configuredLabel, "非运行时状态") {
		t.Fatalf("API 不可用时的配置首选标签 = %q，应明确标注配置态且非运行时", configuredLabel)
	}
	if strings.Contains(configuredLabel, "当前运行") {
		t.Fatalf("API 不可用时的配置首选标签 = %q，不应冒充当前运行状态", configuredLabel)
	}
}

func nodeLabel(t *testing.T, names, labels []string, node string) string {
	t.Helper()
	if len(labels) != len(names) {
		t.Fatalf("标签数量 = %d，节点数量 = %d", len(labels), len(names))
	}
	for i, name := range names {
		if name == node {
			return labels[i]
		}
	}
	t.Fatalf("测试节点 %q 不在节点列表中", node)
	return ""
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

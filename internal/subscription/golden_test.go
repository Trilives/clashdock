package subscription

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// golden.json 是重写前实现冻结下来的兼容性基线。
// 本测试保证 Go 重写与旧输出语义等价（JSON 归一化后 DeepEqual）。

type goldenPair struct {
	Name      string         `json:"name"`
	Input     map[string]any `json:"input"`
	Customize map[string]any `json:"customize"`
	Output    map[string]any `json:"output"`
	Info      map[string]any `json:"info"`
	Error     string         `json:"error"`
}

type goldenFile struct {
	Patch        []goldenPair `json:"patch"`
	Overlay      []goldenPair `json:"overlay"`
	RegionGroups []goldenPair `json:"regiongroups"`
}

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	raw, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal("读取 golden.json:", err)
	}
	var g goldenFile
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal("解析 golden.json:", err)
	}
	return g
}

// normalize 经 JSON 往返，将 int/float、[]string/[]any 等表示差异归一。
func normalize(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal("归一化序列化:", err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal("归一化反序列化:", err)
	}
	return out
}

func assertSemanticEqual(t *testing.T, name, kind string, got, want any) {
	t.Helper()
	g, w := normalize(t, got), normalize(t, want)
	if !reflect.DeepEqual(g, w) {
		gj, _ := json.MarshalIndent(g, "", "  ")
		wj, _ := json.MarshalIndent(w, "", "  ")
		t.Errorf("[%s] %s 与兼容性 golden 不一致\nGo:\n%s\nGolden:\n%s", name, kind, gj, wj)
	}
}

// deepCopy golden 输入在多个断言间复用，避免被 Apply 系列原地修改。
func deepCopy(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(m)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal("深拷贝失败:", err)
	}
	return out
}

func TestGoldenPatch(t *testing.T) {
	for _, c := range loadGolden(t).Patch {
		t.Run(c.Name, func(t *testing.T) {
			got, err := Apply(deepCopy(t, c.Input), c.Customize, "/golden-root/state/ui")
			if c.Error != "" {
				if err == nil {
					t.Fatalf("[%s] 期望 PatchError(%s), 实际成功", c.Name, c.Error)
				}
				return
			}
			if err != nil {
				t.Fatalf("[%s] 意外失败: %v", c.Name, err)
			}
			assertSemanticEqual(t, c.Name, "output", got, c.Output)
		})
	}
}

func TestGoldenOverlay(t *testing.T) {
	for _, c := range loadGolden(t).Overlay {
		t.Run(c.Name, func(t *testing.T) {
			got, info := ApplyOverlay(deepCopy(t, c.Input), c.Customize)
			assertSemanticEqual(t, c.Name, "output", got, c.Output)
			assertSemanticEqual(t, c.Name, "info", info, c.Info)
		})
	}
}

func TestGoldenRegionGroups(t *testing.T) {
	for _, c := range loadGolden(t).RegionGroups {
		t.Run(c.Name, func(t *testing.T) {
			got, info := ApplyRegionGroups(deepCopy(t, c.Input), c.Customize)
			assertSemanticEqual(t, c.Name, "output", got, c.Output)
			assertSemanticEqual(t, c.Name, "info", info, c.Info)
		})
	}
}

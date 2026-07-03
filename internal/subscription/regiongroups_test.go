package subscription

import (
	"encoding/json"
	"testing"
)

// 与 Python 版 tests/test_regiongroups.py 对应的行为测试。
const regionBaseJSON = `{
  "proxies": [
    {"name": "🇭🇰 HK-01", "type": "ss"},
    {"name": "🇭🇰 HK-02", "type": "ss"},
    {"name": "🇸🇬 SG-01", "type": "ss"},
    {"name": "🇺🇸 US-01", "type": "ss"}
  ],
  "proxy-groups": [
    {"name": "Proxies", "type": "select",
     "proxies": ["🇭🇰 HK-01", "🇭🇰 HK-02", "🇸🇬 SG-01", "🇺🇸 US-01", "DIRECT"]}
  ],
  "rules": ["GEOIP,CN,DIRECT", "MATCH,Proxies"]
}`

var regionCustomize = map[string]any{
	"generate_sg_groups": true,
	"prefer_keywords":    []any{"SG", "新加坡"},
	"generate_hk_groups": true,
	"hk_prefer_keywords": []any{"HK", "香港"},
}

func regionBase(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(regionBaseJSON), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func groupByName(t *testing.T, config map[string]any, name string) map[string]any {
	t.Helper()
	for _, g := range groupsOf(config) {
		if anyToStr(g["name"]) == name {
			return g
		}
	}
	return nil
}

func strSlice(v any) []string { return strListOf(v) }

func eqStrs(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestApplyInsertsIntoMain(t *testing.T) {
	config, info := ApplyRegionGroups(regionBase(t), regionCustomize)

	names := strSlice(info["region_groups"])
	if !eqStrs(names, "SG-Auto", "HK-Auto") {
		t.Fatalf("region_groups = %v, 期望 [SG-Auto HK-Auto]", names)
	}
	hk := groupByName(t, config, "HK-Auto")
	if hk == nil {
		t.Fatal("HK-Auto 应加入 proxy-groups")
	}
	if hk["type"] != "url-test" {
		t.Error("HK-Auto 应为 url-test（自动测速）")
	}
	if !eqStrs(strSlice(hk["proxies"]), "🇭🇰 HK-01", "🇭🇰 HK-02") {
		t.Errorf("HK-Auto 成员 = %v, 应按关键词聚合两个香港节点", hk["proxies"])
	}
	main := groupByName(t, config, "Proxies")
	members := strSlice(main["proxies"])
	if len(members) < 2 || members[0] != "SG-Auto" || members[1] != "HK-Auto" {
		t.Errorf("聚合组应插到主选择组最前: %v", members)
	}
	found := false
	for _, m := range members {
		if m == "🇺🇸 US-01" {
			found = true
		}
	}
	if !found {
		t.Error("主选择组原有节点应保留")
	}
}

func TestDisabledRegionNoop(t *testing.T) {
	config, info := ApplyRegionGroups(regionBase(t), map[string]any{})
	if len(strSlice(info["region_groups"])) != 0 {
		t.Error("无地区开启时不应建组")
	}
	gs := groupsOf(config)
	if len(gs) != 1 || anyToStr(gs[0]["name"]) != "Proxies" {
		t.Error("proxy-groups 应保持不变")
	}
}

func TestNoMatchSkipped(t *testing.T) {
	cz := map[string]any{"generate_sg_groups": true, "prefer_keywords": []any{"不存在的地区"}}
	_, info := ApplyRegionGroups(regionBase(t), cz)
	if len(strSlice(info["region_groups"])) != 0 {
		t.Error("无命中节点不应建组")
	}
}

func TestIdempotentReuse(t *testing.T) {
	base := regionBase(t)
	gs := base["proxy-groups"].([]any)
	base["proxy-groups"] = append(gs, map[string]any{
		"name": "HK-Auto", "type": "url-test", "proxies": []any{"🇭🇰 HK-01"},
	})

	config, info := ApplyRegionGroups(base, map[string]any{
		"generate_hk_groups": true, "hk_prefer_keywords": []any{"HK"},
	})
	count := 0
	for _, g := range groupsOf(config) {
		if anyToStr(g["name"]) == "HK-Auto" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("HK-Auto 不应重复创建, 实际 %d 个", count)
	}
	names := strSlice(info["region_groups"])
	if !eqStrs(names, "HK-Auto") {
		t.Errorf("复用的组仍应计入: %v", names)
	}
}

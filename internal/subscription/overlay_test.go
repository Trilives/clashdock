package subscription

import (
	"encoding/json"
	"strings"
	"testing"
)

// 与 Python 版 tests/test_overlay.py 对应的行为测试。
const overlayBaseJSON = `{
  "proxies": [
    {"name": "HK-01", "type": "ss"},
    {"name": "SG-01", "type": "ss"},
    {"name": "US-01", "type": "ss"}
  ],
  "proxy-groups": [
    {"name": "Proxies", "type": "select", "proxies": ["HK-01", "SG-01", "US-01", "DIRECT"]}
  ],
  "rules": ["GEOIP,CN,DIRECT", "MATCH,Proxies"]
}`

var overlayCustomize = map[string]any{
	"ai_domain_suffixes":        []any{"openai.com", "claude.ai"},
	"streaming_domain_suffixes": []any{"netflix.com"},
	"direct_domain_suffixes":    []any{"example.cn"},
	"generate_sg_groups":        true,
	"prefer_keywords":           []any{"SG", "新加坡"},
	"generate_hk_groups":        true,
	"hk_prefer_keywords":        []any{"HK", "香港"},
}

func overlayBase(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(overlayBaseJSON), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestOverlayApply(t *testing.T) {
	config, info := ApplyOverlay(overlayBase(t), overlayCustomize)

	if info["overlay"] != true {
		t.Fatal("overlay 应生效")
	}
	if info["overlay_main"] != "Proxies" {
		t.Error("应定位主选择组 Proxies")
	}
	names := map[string]bool{}
	for _, g := range groupsOf(config) {
		names[anyToStr(g["name"])] = true
	}
	for _, want := range []string{"AI", "Streaming", "SG-Auto", "HK-Auto"} {
		if !names[want] {
			t.Errorf("应新增 %s 组", want)
		}
	}

	rules := strSlice(config["rules"])
	head := rules
	if len(head) > 6 {
		head = head[:6]
	}
	hasAI, hasStreaming, hasDirect := false, false, false
	for _, r := range head {
		if strings.Contains(r, "openai.com") && strings.HasSuffix(r, ",AI") {
			hasAI = true
		}
		if strings.Contains(r, "netflix.com") && strings.HasSuffix(r, ",Streaming") {
			hasStreaming = true
		}
		if strings.Contains(r, "example.cn") && strings.HasSuffix(r, ",DIRECT") {
			hasDirect = true
		}
	}
	if !hasAI || !hasStreaming || !hasDirect {
		t.Errorf("AI/流媒体/直连规则应插在头部: %v", head)
	}
	if rules[len(rules)-1] != "MATCH,Proxies" {
		t.Error("订阅原规则应保留在后")
	}

	sg := groupByName(t, config, "SG-Auto")
	if !eqStrs(strSlice(sg["proxies"]), "SG-01") {
		t.Errorf("SG 组应按关键词筛节点: %v", sg["proxies"])
	}
}

func TestNoMainGroup(t *testing.T) {
	_, info := ApplyOverlay(map[string]any{
		"proxies": []any{}, "proxy-groups": []any{}, "rules": []any{},
	}, overlayCustomize)
	if info["overlay"] != false {
		t.Error("无 select 组时不应叠加")
	}
}

func TestNameDedup(t *testing.T) {
	base := overlayBase(t)
	gs := base["proxy-groups"].([]any)
	base["proxy-groups"] = append(gs, map[string]any{
		"name": "AI", "type": "select", "proxies": []any{"HK-01"},
	})
	_, info := ApplyOverlay(base, map[string]any{"ai_domain_suffixes": []any{"openai.com"}})
	found := false
	for _, n := range strSlice(info["overlay_groups"]) {
		if n == "AI-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("AI 重名应生成 AI-1: %v", info["overlay_groups"])
	}
}

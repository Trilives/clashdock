// 地区自动测速聚合组（可选增强，独立于 overlay）。
//
// 机场订阅通常已自带按地区分的 select 组（HK/SG/JP…，但需手动逐个挑节点）。本文件在其上
// 额外生成 url-test 聚合组（如 SG-Auto / HK-Auto）：按节点名关键词聚合该地区节点、自动
// 选最低延迟，并把聚合组插入主选择组前部，使其可直接作为出口选用。
//
// 与 overlay（AI / 流媒体自定义分流）相互独立：由 customize.generate_sg_groups /
// generate_hk_groups 各地区独立开关。overlay 也复用本文件的基础函数构造同样的地区组，
// 故两者共存时不会重复建组（同名 url-test 组会被复用）。
package subscription

import (
	"fmt"
	"strings"
)

// 主选择组定位关键词（与 node_select 一致）。
var mainGroupKeywords = []string{
	"proxy", "节点选择", "节点", "选择", "select", "🚀", "手动",
	"代理", "代理选择", "手动选择", "选择节点",
}

var builtinNames = map[string]bool{
	"DIRECT": true, "REJECT": true, "REJECT-DROP": true,
	"PASS": true, "COMPATIBLE": true, "GLOBAL": true,
}

// RegionSpec (启用字段, 关键词字段, 聚合组名)：开启对应字段才生成该地区聚合组。
type RegionSpec struct {
	Flag         string
	KeywordField string
	Tag          string
}

var RegionSpecs = []RegionSpec{
	{"generate_sg_groups", "prefer_keywords", "SG-Auto"},
	{"generate_hk_groups", "hk_prefer_keywords", "HK-Auto"},
}

func groupsOf(config map[string]any) []map[string]any {
	gs, ok := config["proxy-groups"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(gs))
	for _, g := range gs {
		if m, ok := g.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// mainGroupName 定位主选择组：先按内置 + customize.main_group_keywords 追加的
// 关键词匹配，未命中则退化为成员数最多的 select 组。
func mainGroupName(config, customize map[string]any) string {
	var selects []map[string]any
	for _, g := range groupsOf(config) {
		if s, _ := g["type"].(string); s == "select" {
			selects = append(selects, g)
		}
	}
	if len(selects) == 0 {
		return ""
	}
	keywords := append([]string(nil), mainGroupKeywords...)
	for _, kw := range strListOf(customize["main_group_keywords"]) {
		if kw != "" {
			keywords = append(keywords, strings.ToLower(kw))
		}
	}
	for _, g := range selects {
		lower := strings.ToLower(anyToStr(g["name"]))
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return anyToStr(g["name"])
			}
		}
	}
	best := selects[0]
	for _, g := range selects[1:] {
		if lenList(g["proxies"]) > lenList(best["proxies"]) {
			best = g
		}
	}
	return anyToStr(best["name"])
}

func uniqName(base string, taken map[string]bool) string {
	name := base
	for i := 1; taken[name]; i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	taken[name] = true
	return name
}

// regionGroup 按关键词从 proxies 筛节点，构造 url-test 地区组；无命中返回 nil。
func regionGroup(config map[string]any, keywords []string, tag string, taken map[string]bool) (string, map[string]any) {
	proxies, _ := config["proxies"].([]any)
	names := make([]any, 0)
	for _, p := range proxies {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		name := anyToStr(m["name"])
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		for _, k := range keywords {
			if strings.Contains(lower, strings.ToLower(k)) {
				names = append(names, name)
				break
			}
		}
	}
	if len(names) == 0 {
		return "", nil
	}
	name := uniqName(tag, taken)
	return name, map[string]any{
		"name": name, "type": "url-test", "proxies": names,
		"url": "https://www.gstatic.com/generate_204", "interval": 300, "tolerance": 50,
	}
}

// BuiltGroup Build 的产物：Group 为 nil 表示「同名 url-test 组已存在，复用即可」。
type BuiltGroup struct {
	Name  string
	Group map[string]any
}

// BuildRegionGroups 按 customize 中开启的地区构造 url-test 聚合组
// （避免与 overlay 等其它路径重复建组）。
func BuildRegionGroups(config, customize map[string]any, taken map[string]bool) []BuiltGroup {
	existing := map[string]map[string]any{}
	for _, g := range groupsOf(config) {
		existing[anyToStr(g["name"])] = g
	}
	var out []BuiltGroup
	for _, spec := range RegionSpecs {
		if !truthy(customize, spec.Flag, false) {
			continue
		}
		if prior, ok := existing[spec.Tag]; ok {
			if s, _ := prior["type"].(string); s == "url-test" {
				out = append(out, BuiltGroup{Name: spec.Tag}) // 复用已有同名聚合组
				continue
			}
		}
		if name, group := regionGroup(config, strListOf(customize[spec.KeywordField]), spec.Tag, taken); group != nil {
			out = append(out, BuiltGroup{Name: name, Group: group})
		}
	}
	return out
}

// ApplyRegionGroups 独立应用：生成地区聚合组并插入主选择组前部。返回 (config, info)。
func ApplyRegionGroups(config, customize map[string]any) (map[string]any, map[string]any) {
	groups := groupsOf(config)
	taken := make(map[string]bool, len(groups)+len(builtinNames))
	for _, g := range groups {
		if n := anyToStr(g["name"]); n != "" {
			taken[n] = true
		}
	}
	for b := range builtinNames {
		taken[b] = true
	}

	built := BuildRegionGroups(config, customize, taken)
	names := make([]any, 0, len(built))
	for _, b := range built {
		names = append(names, b.Name)
	}
	if len(names) == 0 {
		return config, map[string]any{"region_groups": []any{}}
	}

	newGroups := make([]map[string]any, 0, len(built))
	for _, b := range built {
		if b.Group != nil {
			newGroups = append(newGroups, b.Group)
		}
	}
	main := mainGroupName(config, customize)
	if main != "" {
		for _, g := range groups {
			if anyToStr(g["name"]) == main {
				if members, ok := g["proxies"].([]any); ok {
					have := make(map[string]bool, len(members))
					for _, m := range members {
						have[anyToStr(m)] = true
					}
					add := make([]any, 0, len(names))
					for _, n := range names {
						if !have[anyToStr(n)] {
							add = append(add, n)
						}
					}
					g["proxies"] = append(add, members...)
					break
				}
			}
		}
	}

	rebuilt := make([]any, 0, len(newGroups)+len(groups))
	for _, g := range newGroups {
		rebuilt = append(rebuilt, g)
	}
	for _, g := range groups {
		rebuilt = append(rebuilt, g)
	}
	config["proxy-groups"] = rebuilt

	info := map[string]any{"region_groups": names}
	if main != "" {
		info["region_main"] = main
	} else {
		info["region_main"] = nil
	}
	return config, info
}

// 主选择组识别与组成员分类（地区分桶 / 子组），供节点切换与固定节点共用。
package flows

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

// mihomo 策略组类型（可作为「子组」展示）
var groupTypes = map[string]bool{
	"select": true, "url-test": true, "fallback": true, "load-balance": true, "relay": true,
}

var builtinNodes = map[string]bool{
	"DIRECT": true, "REJECT": true, "REJECT-DROP": true,
	"PASS": true, "COMPATIBLE": true, "GLOBAL": true,
}

var infoKeywords = []string{"Traffic:", "Expire:", "剩余流量", "过期时间", "剩余", "套餐", "官网", "订阅", "重置"}

var errMainGroupUnrecognized = errors.New("main selector group unrecognized")

type region struct {
	key   string
	label string
	kws   []string
}

var regions = []region{
	{"hk", "🇭🇰 香港", []string{"香港", "hong kong", "hongkong"}},
	{"tw", "🇹🇼 台湾", []string{"台湾", "臺灣", "taiwan"}},
	{"jp", "🇯🇵 日本", []string{"日本", "japan", "东京", "大阪"}},
	{"kr", "🇰🇷 韩国", []string{"韩国", "韓國", "korea", "首尔"}},
	{"sg", "🇸🇬 新加坡", []string{"新加坡", "singapore", "狮城", "獅城"}},
	{"us", "🇺🇸 美国", []string{"美国", "united states", "america", "硅谷", "洛杉矶", "圣何塞"}},
}

const otherKey, otherLabel = "other", "🌐 其他地区"

func groupsOf(cfg map[string]any) []map[string]any {
	gs, ok := cfg["proxy-groups"].([]any)
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

// pickGroup 定位目标 select 分组：forced 指定时精确匹配；否则按 keywords 顺序
// 逐个尝试，第一个命中分组名的关键词即采用该分组（先到先得，顺序即优先级）。
// 关键词全都不命中时不猜测，避免把节点切换错误地施加到其它 select 组。
func pickGroup(cfg map[string]any, forced string, keywords []string) (map[string]any, error) {
	var selects []map[string]any
	for _, g := range groupsOf(cfg) {
		if t, _ := g["type"].(string); t == "select" {
			selects = append(selects, g)
		}
	}
	if len(selects) == 0 {
		return nil, fmt.Errorf("%s", i18n.T("配置里没有 select 策略组，无法切换节点"))
	}
	if forced != "" {
		for _, g := range selects {
			if g["name"] == forced {
				return g, nil
			}
		}
		return nil, fmt.Errorf(i18n.T("指定分组 '%s' 不存在"), forced)
	}
	// 逐关键词扫描（而非逐分组扫描）：关键词列表顺序即优先级，用户新增的关键词
	// 插在最前，能在内置关键词之前抢先命中目标分组。
	for _, kw := range keywords {
		kw = strings.ToLower(kw)
		if kw == "" {
			continue
		}
		for _, g := range selects {
			if strings.Contains(strings.ToLower(fmt.Sprint(g["name"])), kw) {
				return g, nil
			}
		}
	}
	return nil, fmt.Errorf("%s: %w", i18n.T("未识别到主选择组，无法切换"), errMainGroupUnrecognized)
}

// resolveMainGroup 在自动识别失败时允许补充一个关键词。新关键词只有在本次配置中
// 确实命中 select 组后才写盘，并插到列表首位，供当前与后续切换立即复用。
func resolveMainGroup(
	p paths.Paths,
	cfg map[string]any,
	forced string,
	input func(string) (string, error),
) (map[string]any, error) {
	customize := config.Load(p)
	keywords := config.StrList(customize, "main_group_keywords")
	target, err := pickGroup(cfg, forced, keywords)
	if err == nil || forced != "" || !errors.Is(err, errMainGroupUnrecognized) {
		return target, err
	}

	entered, inputErr := input(i18n.T("未识别到主选择组，请输入组名或识别关键词（直接回车取消）"))
	if inputErr != nil {
		return nil, inputErr
	}
	entered = strings.TrimSpace(entered)
	if entered == "" {
		return nil, fmt.Errorf("%s", i18n.T("未识别到主选择组，无法切换"))
	}

	updatedKeywords := make([]string, 0, len(keywords)+1)
	updatedKeywords = append(updatedKeywords, entered)
	for _, keyword := range keywords {
		if keyword != entered {
			updatedKeywords = append(updatedKeywords, keyword)
		}
	}
	target, err = pickGroup(cfg, "", updatedKeywords)
	if err != nil {
		if errors.Is(err, errMainGroupUnrecognized) {
			return nil, fmt.Errorf(i18n.T("输入的主选择组识别关键词 %q 未匹配任何 select 分组"), entered)
		}
		return nil, err
	}

	updatedCustomize := make(map[string]any, len(customize))
	for key, value := range customize {
		updatedCustomize[key] = value
	}
	updatedCustomize["main_group_keywords"] = updatedKeywords
	if err := config.Save(p, updatedCustomize); err != nil {
		return nil, fmt.Errorf(i18n.T("保存主选择组识别关键词失败: %w"), err)
	}
	return target, nil
}

func classify(name string) string {
	low := strings.ToLower(name)
	for _, r := range regions {
		for _, kw := range r.kws {
			if strings.Contains(name, kw) || strings.Contains(low, kw) {
				return r.key
			}
		}
	}
	return otherKey
}

func isInfo(name string) bool {
	for _, kw := range infoKeywords {
		if strings.Contains(name, kw) {
			return true
		}
	}
	return false
}

// collectMembers 把组成员分为「按地区分桶的真实节点」与「子组」。
func collectMembers(cfg, group map[string]any) (map[string][]string, []string) {
	typeByName := map[string]string{}
	for _, g := range groupsOf(cfg) {
		name, _ := g["name"].(string)
		t, _ := g["type"].(string)
		typeByName[name] = t
	}
	buckets := map[string][]string{}
	var subgroups []string
	members, _ := group["proxies"].([]any)
	for _, m := range members {
		name := fmt.Sprint(m)
		switch {
		case groupTypes[typeByName[name]]:
			subgroups = append(subgroups, name)
		case builtinNodes[name] || isInfo(name):
		default:
			buckets[classify(name)] = append(buckets[classify(name)], name)
		}
	}
	return buckets, subgroups
}

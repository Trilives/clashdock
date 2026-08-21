// 交互式切换 / 固定首选节点（对应 node_select.py）。
//
// 把选中项设为目标 select 组（默认主选择组）的第一个成员，使重启后稳定停在该节点；
// 服务在跑时还经 Clash API 实时切换，并并发实测延迟。选组持久化由
// profile.store-selected + cache.db 负责；改写成员顺序作为跨重启兜底。
package flows

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/term"

	"github.com/Trilives/clashdock/internal/clashapi"
	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/configfile"
	"github.com/Trilives/clashdock/internal/errs"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/jsonx"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/subscription"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/tui"
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

// measure 并发实测延迟，带 TTY 进度。
func measure(api *clashapi.Client, names []string) map[string]int {
	if len(names) == 0 {
		return nil
	}
	tty := term.IsTerminal(int(os.Stdout.Fd()))
	if !tty {
		execx.Info(fmt.Sprintf(i18n.T("测速中（%d 个节点）…"), len(names)))
	}
	results := make(map[string]int, len(names))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, min(16, len(names)))
	done := 0
	for _, name := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ms, ok := api.Delay(n)
			mu.Lock()
			if ok {
				results[n] = ms
			}
			done++
			if tty {
				fmt.Printf(i18n.T("\r\033[K  测速中… %d/%d"), done, len(names))
			}
			mu.Unlock()
		}(name)
	}
	wg.Wait()
	if tty {
		fmt.Print("\r\033[K")
	}
	execx.Ok(fmt.Sprintf(i18n.T("测速完成：%d/%d 可用"), len(results), len(names)))
	return results
}

func fmtDelay(results map[string]int, name string) string {
	if ms, ok := results[name]; ok {
		return fmt.Sprintf("%dms", ms)
	}
	return i18n.T("超时")
}

func preferredNode(group map[string]any) string {
	members, _ := group["proxies"].([]any)
	if len(members) == 0 {
		return ""
	}
	return fmt.Sprint(members[0])
}

// currentNodeLabels 同时展示运行时当前节点与配置中的固定首选。runtimeOK=false
// 表示无法读取 API 状态，此时配置首选必须明确标成非运行时状态。
func currentNodeLabels(names []string, delays map[string]int, runtimeCurrent, pinned string, runtimeOK bool) []string {
	labels := make([]string, len(names))
	for i, name := range names {
		parts := []string{name}
		if delays != nil {
			parts = append(parts, fmtDelay(delays, name))
		}
		var states []string
		if runtimeOK && name == runtimeCurrent {
			states = append(states, i18n.T("当前运行"))
		}
		if name == pinned {
			states = append(states, i18n.T("配置首选"))
			if !runtimeOK {
				states = append(states, i18n.T("非运行时状态"))
			}
		}
		label := strings.Join(parts, "   ")
		if len(states) > 0 {
			label += "（" + strings.Join(states, "，") + "）"
		}
		labels[i] = label
	}
	return labels
}

func currentNodeSummary(cfg map[string]any, targetName string, selections map[string]string, runtimeOK bool) []string {
	for _, group := range groupsOf(cfg) {
		name := fmt.Sprint(group["name"])
		if name != targetName {
			continue
		}
		if runtimeOK {
			if current := selections[name]; current != "" {
				return []string{fmt.Sprintf("  • %s → %s（%s）", name, current, i18n.T("当前运行"))}
			}
			return nil
		}
		if group["type"] == "select" {
			if pinned := preferredNode(group); pinned != "" {
				return []string{fmt.Sprintf("  • %s → %s（%s，%s）", name, pinned,
					i18n.T("配置首选"), i18n.T("非运行时状态"))}
			}
		}
		return nil
	}
	return nil
}

func printCurrentNodeSummary(cfg map[string]any, targetName string, selections map[string]string, runtimeOK bool) {
	lines := currentNodeSummary(cfg, targetName, selections, runtimeOK)
	if len(lines) == 0 {
		return
	}
	execx.Info(i18n.T("主选择组当前节点："))
	for _, line := range lines {
		fmt.Println(line)
	}
}

// persistFirst 把选中节点提为目标组首成员，双写生效配置与订阅配置（跨重启兜底）。
func persistFirst(cfg map[string]any, groupName, node string, files []string) error {
	for _, g := range groupsOf(cfg) {
		if t, _ := g["type"].(string); t == "select" && g["name"] == groupName {
			members, _ := g["proxies"].([]any)
			out := make([]any, 0, len(members)+1)
			out = append(out, node)
			for _, m := range members {
				if fmt.Sprint(m) != node {
					out = append(out, m)
				}
			}
			g["proxies"] = out
			break
		}
	}
	payload, err := jsonx.MarshalPretty(cfg)
	if err != nil {
		return err
	}
	for _, f := range files {
		tmp := f + ".tmp"
		if err := os.WriteFile(tmp, payload, 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, f); err != nil {
			return err
		}
	}
	return nil
}

// pickResult 两级菜单选完节点后的结果，供临时切换 / 固定切换两个流程各自处理。
type pickResult struct {
	cfg       map[string]any
	groupName string
	node      string
	api       *clashapi.Client
	apiOK     bool
}

// pickNode 两级菜单（地区/分组 → 节点）交互选择，不做任何写盘/切换——
// 是「节点切换」与「固定节点」两个流程共用的选择器。
func pickNode(p paths.Paths, configPath, group string) (*pickResult, error) {
	cfg, err := configfile.Read(configPath)
	if err != nil {
		return nil, err
	}
	target, err := resolveMainGroup(p, cfg, group, func(prompt string) (string, error) {
		return tui.Ask(prompt, tui.AskOpts{AllowEmpty: true})
	})
	if err != nil {
		return nil, err
	}
	groupName := fmt.Sprint(target["name"])
	buckets, subgroups := collectMembers(cfg, target)
	if len(buckets) == 0 && len(subgroups) == 0 {
		return nil, fmt.Errorf(i18n.T("分组 '%s' 下没有可选项"), groupName)
	}

	// 节点切换走 Clash API 热切换，直接连 API 实时测速/切换
	api := clashapi.FromConfig(cfg)
	apiOK := api != nil && api.Reachable()
	runtimeSelections := map[string]string{}
	runtimeOK := false
	if apiOK {
		execx.Info(i18n.T("已连上 Clash API，列表将实时测速。"))
		if selections, serr := api.CurrentSelections(); serr != nil {
			execx.Warn(fmt.Sprintf(i18n.T("读取 Clash API 当前节点失败：%v"), serr))
		} else {
			runtimeSelections = selections
			runtimeOK = true
		}
	} else {
		execx.Info(i18n.T("Clash API 不可达，跳过测速。"))
	}
	printCurrentNodeSummary(cfg, groupName, runtimeSelections, runtimeOK)
	runtimeCurrent := runtimeSelections[groupName]
	pinned := preferredNode(target)

	type menuEntry struct {
		label string
		items []string
	}
	var firstMenu []menuEntry
	for _, r := range regions {
		if len(buckets[r.key]) > 0 {
			firstMenu = append(firstMenu, menuEntry{i18n.T(r.label), buckets[r.key]})
		}
	}
	if len(buckets[otherKey]) > 0 {
		firstMenu = append(firstMenu, menuEntry{i18n.T(otherLabel), buckets[otherKey]})
	}
	if len(subgroups) > 0 {
		firstMenu = append(firstMenu, menuEntry{i18n.T("🧭 子组（自动测速 / 故障转移）"), subgroups})
	}

	// esc 在第二步只退回第一步；^R 才穿透放弃本次切换
	var selected string
	idx := 0
	for {
		labels := make([]string, len(firstMenu))
		for i, e := range firstMenu {
			labels[i] = fmt.Sprintf(i18n.T("%s（%d）"), e.label, len(e.items))
		}
		i, err := tui.Select(i18n.T("选择地区 / 分组"), labels, tui.SelectOpts{BackLabel: i18n.T("退出切换节点"), Initial: idx})
		if err != nil {
			return nil, err
		}
		idx = i
		entry := firstMenu[i]

		var delays map[string]int
		if apiOK {
			delays = measure(api, entry.items)
		}
		nodeLabels := currentNodeLabels(entry.items, delays, runtimeCurrent, pinned, runtimeOK)
		initial := 0
		preferred := runtimeCurrent
		if preferred == "" {
			preferred = pinned
		}
		for j, name := range entry.items {
			if name == preferred {
				initial = j
				break
			}
		}
		nidx, err := tui.Select(entry.label, nodeLabels, tui.SelectOpts{SaveLabel: i18n.T("返回地区/分组"), BackLabel: i18n.T("放弃并退出"), Initial: initial})
		if err != nil {
			if errors.Is(err, errs.ErrSaveExit) {
				continue // 返回地区/分组选择，重新选
			}
			return nil, err
		}
		selected = entry.items[nidx]
		break
	}
	return &pickResult{cfg: cfg, groupName: groupName, node: selected, api: api, apiOK: apiOK}, nil
}

// NodeSwitchLive 临时切换节点：仅经 Clash API 热切换，不写盘、不重启——
// 服务重启或切换/刷新订阅后失效，适合"先试试看"的场景。需要服务正在运行。
func NodeSwitchLive(p paths.Paths, configPath, group string) error {
	if configPath == "" {
		configPath = p.ConfigFile
	}
	r, err := pickNode(p, configPath, group)
	if err != nil {
		return err
	}
	if !r.apiOK {
		return fmt.Errorf("%s", i18n.T("Clash API 不可达，临时切换需要服务正在运行（如需跨重启保留，请改用「固定节点」）"))
	}
	if err := r.api.Switch(r.groupName, r.node); err != nil {
		return err
	}
	execx.Ok(fmt.Sprintf(i18n.T("已临时切换 %s → %s（不写盘，重启/切换订阅后失效）"), r.groupName, r.node))
	return nil
}

// NodeSelect 两级菜单（地区/分组 → 节点）切换节点；是否固定为首选（写盘，
// 跨重启/服务重建后仍保留）由用户显式确认，固定后同步配置并重启已安装的服务。
func NodeSelect(p paths.Paths, configPath, group string) error {
	if configPath == "" {
		configPath = p.ConfigFile
	}
	r, err := pickNode(p, configPath, group)
	if err != nil {
		return err
	}

	if r.apiOK {
		if err := r.api.Switch(r.groupName, r.node); err != nil {
			execx.Warn(fmt.Sprintf(i18n.T("Clash API 实时切换失败：%v"), err))
		} else {
			execx.Ok(fmt.Sprintf(i18n.T("已通过 Clash API 实时切换 %s → %s"), r.groupName, r.node))
		}
	}

	pin, err := tui.Confirm(i18n.T("固定为该分组首选节点？（写入配置，跨重启/切换订阅后仍保留；否则仅本次生效）"), true)
	if err != nil || !pin {
		return err
	}

	var syncRuntime func(paths.Paths) error
	if sysd.IsInstalled(sysd.DefaultName) {
		syncRuntime = func(got paths.Paths) error {
			return sysd.SyncAndRestart(got, sysd.DefaultName)
		}
	}
	if err := persistPinnedSelectionWithSync(p, r.cfg, r.groupName, r.node, configPath, syncRuntime); err != nil {
		return err
	}
	execx.Ok(fmt.Sprintf(i18n.T("已固定 %s 首选 = %s"), r.groupName, r.node))
	return nil
}

func persistPinnedSelectionWithSync(
	p paths.Paths,
	cfg map[string]any,
	groupName, node, configPath string,
	syncRuntime func(paths.Paths) error,
) error {
	// 写生效配置 + 当前 active 订阅的 config.yaml（双写以跨重启持久）
	targets := []string{configPath}
	if active := subscription.GetActive(p); active != nil {
		subCfg := filepath.Join(p.SubscriptionDir(active.Name), "config.yaml")
		if _, err := os.Stat(subCfg); err == nil && subCfg != configPath {
			targets = append(targets, subCfg)
		}
	}
	if err := persistFirst(cfg, groupName, node, targets); err != nil {
		return err
	}
	if syncRuntime != nil {
		if err := syncRuntime(p); err != nil {
			return fmt.Errorf(i18n.T("节点首选已保存，但同步到运行时失败: %w"), err)
		}
	}
	return nil
}

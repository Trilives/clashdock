// 自定义分流叠加层（可选，默认关闭）。
//
// 在机场订阅自带的 proxy-groups / rules 之上叠加项目自定义分流，而非替换：
//   - 新增 AI / Streaming 选择组（引用订阅主选择组 + 现有地区子组 + DIRECT）；
//   - 可选按关键词从订阅节点筛出 SG / HK 地区 url-test 组（复用 regiongroups）；
//   - 在 rules 头部插入 AI / 流媒体 / 直连域名规则（优先于订阅原规则命中）。
//
// 仅在 customize.enable_overlay=true 时由 manager 调用。引用的组名因机场而异，故主选择组
// 靠启发式定位，新组名遇冲突自动加后缀。地区聚合组构造委托 regiongroups，两者共存不重复建组。
package subscription

// ApplyOverlay 在已 patch 的运行时配置上叠加自定义分流。返回 (config, info)。
func ApplyOverlay(config, customize map[string]any) (map[string]any, map[string]any) {
	main := mainGroupName(config)
	if main == "" {
		// 没有可引用的主选择组，放弃叠加（保持订阅原状）
		return config, map[string]any{"overlay": false, "overlay_reason": "未找到主选择组"}
	}

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

	newGroups := make([]map[string]any, 0, 4)

	// 可选地区组（委托 regiongroups 构造，保持与独立功能一致）
	regionNames := make([]string, 0, len(RegionSpecs))
	for _, b := range BuildRegionGroups(config, customize, taken) {
		regionNames = append(regionNames, b.Name)
		if b.Group != nil {
			newGroups = append(newGroups, b.Group)
		}
	}

	members := make([]any, 0, len(regionNames)+2)
	members = append(members, main)
	for _, n := range regionNames {
		members = append(members, n)
	}
	members = append(members, "DIRECT")
	aiName := uniqName("AI", taken)
	streamingName := uniqName("Streaming", taken)
	newGroups = append(newGroups,
		map[string]any{"name": aiName, "type": "select", "proxies": append([]any(nil), members...)},
		map[string]any{"name": streamingName, "type": "select", "proxies": append([]any(nil), members...)})

	// 叠加规则（插到 rules 头部，优先命中）
	newRules := make([]any, 0)
	for _, suf := range strListOf(customize["direct_domain_suffixes"]) {
		newRules = append(newRules, "DOMAIN-SUFFIX,"+suf+",DIRECT")
	}
	for _, suf := range strListOf(customize["ai_domain_suffixes"]) {
		newRules = append(newRules, "DOMAIN-SUFFIX,"+suf+","+aiName)
	}
	for _, suf := range strListOf(customize["streaming_domain_suffixes"]) {
		newRules = append(newRules, "DOMAIN-SUFFIX,"+suf+","+streamingName)
	}

	rebuilt := make([]any, 0, len(newGroups)+len(groups))
	for _, g := range newGroups {
		rebuilt = append(rebuilt, g)
	}
	for _, g := range groups {
		rebuilt = append(rebuilt, g)
	}
	config["proxy-groups"] = rebuilt

	oldRules, _ := config["rules"].([]any)
	rules := make([]any, 0, len(newRules)+len(oldRules))
	rules = append(rules, newRules...)
	rules = append(rules, oldRules...)
	config["rules"] = rules

	overlayGroups := make([]any, 0, len(newGroups))
	for _, g := range newGroups {
		overlayGroups = append(overlayGroups, g["name"])
	}
	info := map[string]any{
		"overlay":        true,
		"overlay_main":   main,
		"overlay_groups": overlayGroups,
		"overlay_rules":  len(newRules),
	}
	return config, info
}

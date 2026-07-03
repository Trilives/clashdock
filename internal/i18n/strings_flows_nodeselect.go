package i18n

func init() {
	register(map[string]string{
		"配置里没有 select 策略组，无法切换节点": "No select proxy group in the config; cannot switch node",
		"指定分组 '%s' 不存在":           "Specified group '%s' does not exist",
		"分组 '%s' 下没有可选项":          "No selectable items under group '%s'",

		"已连上 Clash API，列表将实时测速。": "Connected to the Clash API; the list will show live latency.",
		"Clash API 不可达，跳过测速。":    "Clash API unreachable, skipping latency test.",

		"测速中（%d 个节点）…":         "Testing latency (%d nodes)…",
		"\r\033[K  测速中… %d/%d": "\r\033[K  Testing latency… %d/%d",
		"测速完成：%d/%d 可用":        "Latency test done: %d/%d available",
		"超时":                   "Timeout",

		// 地区菜单标签（对应 regions 数据里的 label 字段；kws 匹配关键词保持中文原样，不翻译）
		"🇭🇰 香港":  "🇭🇰 Hong Kong",
		"🇹🇼 台湾":  "🇹🇼 Taiwan",
		"🇯🇵 日本":  "🇯🇵 Japan",
		"🇰🇷 韩国":  "🇰🇷 Korea",
		"🇸🇬 新加坡": "🇸🇬 Singapore",
		"🇺🇸 美国":  "🇺🇸 United States",
		"🌐 其他地区": "🌐 Other regions",

		"🧭 子组（自动测速 / 故障转移）": "🧭 Subgroups (auto url-test / failover)",
		"%s（%d）":    "%s (%d)",
		"选择地区 / 分组": "Select region / group",
		"退出切换节点":    "Exit node switching",
		"返回地区/分组":   "Back to region/group",
		"放弃并退出":     "Discard and exit",

		"已固定 %s 首选 = %s":             "Pinned %s preferred = %s",
		"Clash API 实时切换失败：%v":        "Clash API live switch failed: %v",
		"已通过 Clash API 实时切换 %s → %s": "Live-switched via Clash API %s → %s",
		"重启服务以确保生效？":                 "Restart the service to ensure it takes effect?",
	})
}

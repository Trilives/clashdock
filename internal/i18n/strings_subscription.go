package i18n

func init() {
	register(map[string]string{
		"订阅根必须是映射（YAML mapping）。":                        "The subscription root must be a mapping (YAML mapping).",
		"订阅缺少 proxies 列表或为空，无法作为 mihomo 配置。":             "The subscription is missing a proxies list or it's empty; it can't be used as a mihomo config.",
		"已开启 LAN 面板（lan_panel）但未设置 secret，拒绝在无密钥下放开控制器。": "LAN panel (lan_panel) is enabled but no secret is set; refusing to expose the controller without one.",
		"解析订阅 YAML: %w":       "Failed to parse subscription YAML: %w",
		"订阅 YAML 根必须是映射。":     "The subscription YAML root must be a mapping.",
		"订阅 YAML 解析失败或根不是映射。": "Failed to parse subscription YAML, or its root is not a mapping.",

		"内容看起来更像「%s」而非你选择的「%s」。": "The content looks more like \"%s\" than the \"%s\" you selected.",

		"拉取订阅失败: %w":            "Failed to fetch subscription: %w",
		"拉取失败（%v），第 %d/%d 次重试…": "Fetch failed (%v), retrying (%d/%d)…",

		"未找到主选择组": "Main selector group not found",

		"subconverter 返回内容无法解析为 Clash":                               "subconverter's response could not be parsed as Clash config",
		"subconverter 解析失败：%w。可更换后端，或开启应急本地解析 base64_local_fallback": "subconverter parsing failed: %w. Try a different backend, or enable the local fallback parser (base64_local_fallback)",
		"subconverter 失败，改用应急本地解析：%v":                                "subconverter failed, falling back to local parsing: %v",
		"未配置 subconverter 后端，且未开启应急本地解析（base64_local_fallback）":      "No subconverter backend configured, and the local fallback parser (base64_local_fallback) is not enabled",
		"本地解析未得到任何节点":                                                "Local parsing yielded no nodes",
		"subconverter 返回内容不含 proxies，可能后端不可用或订阅无效":                   "subconverter's response has no proxies; the backend may be unavailable or the subscription invalid",

		"订阅「%s」已存在，请改名或先删除":                 "Subscription \"%s\" already exists; rename or delete it first",
		"订阅不存在: %s":                         "Subscription does not exist: %s",
		"本地缺少订阅原文，改为联网刷新。":                  "Local subscription source is missing, refreshing from the network instead.",
		"用本地原文重新生成「%s」（不重新拉取）…":             "Regenerating \"%s\" from the local source (not re-fetching)…",
		"拉取订阅「%s」…":                         "Fetching subscription \"%s\"…",
		"经 subconverter 将 base64 转为 Clash…": "Converting base64 to Clash via subconverter…",
		"生成 mihomo 配置（直用订阅 + 最小改写）…":        "Generating mihomo config (using the subscription directly, minimal rewrite)…",
		"叠加自定义分流（overlay）…":                 "Applying custom routing overlay…",
		"已生成地区自动测速聚合组：":                     "Generated region auto url-test group(s): ",
		"启用了地区聚合组，但未匹配到对应地区节点（检查关键词与开关）。":   "Region groups are enabled, but no matching regional nodes were found (check the keywords and toggles).",
		"订阅「%s」就绪：%v 节点 / %v 策略组 / %v 规则":   "Subscription \"%s\" ready: %v nodes / %v proxy groups / %v rules",

		"已切换生效订阅: ":                "Active subscription switched to: ",
		"配置已切换，但同步到服务失败：%v":        "Config switched, but syncing to the service failed: %v",
		"已删除当前生效订阅；请切换到其它订阅或重新添加。": "The active subscription was deleted; please switch to another subscription or add a new one.",
		"已删除订阅: ":                  "Subscription deleted: ",
		"目标名已存在: %s":               "Target name already exists: %s",
		"已改名: %s → %s":             "Renamed: %s → %s",
	})
}

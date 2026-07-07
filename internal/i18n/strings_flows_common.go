package i18n

func init() {
	register(map[string]string{
		"Clash / mihomo 订阅（★推荐：直用机场配置，凭证不外泄）":      "Clash / mihomo subscription (★recommended: use provider config directly, credentials stay local)",
		"通用 base64 订阅（经 subconverter 云端解析为 Clash）": "Generic base64 subscription (parsed to Clash via cloud subconverter)",

		"本地 YAML 文件（直接导入为订阅，不联网拉取）": "Local YAML file (import directly as a subscription, no network fetch)",
		"订阅名称，留空=时间戳":               "Subscription name, empty = timestamp",
		"选择订阅来源类型":                  "Select subscription source type",
		"订阅链接，留空=暂不配置":              "Subscription URL, empty = skip for now",
		"本地 YAML 文件路径，留空=暂不配置":      "Local YAML file path, empty = skip for now",
		"使用代理拉取此订阅？默认否＝直连（优先本机代理端口，失败再回退下载代理）":                 "Use a proxy to fetch this subscription? Default no = direct (prefers the local proxy port, then falls back to the download proxy)",
		"是否叠加自定义分流（AI / 流媒体 / 地区组）？默认否＝直接沿用机场订阅自带的策略组与规则（推荐）。": "Overlay custom routing (AI / streaming / region groups)? Default no = keep the subscription's own proxy groups and rules as-is (recommended).",

		"创建数据目录 ": "Create data directory ",

		"未配置 GitHub Token，匿名 API 限额较低（60 次/小时），高频操作易被限流。": "No GitHub token configured; the anonymous API rate limit is low (60 requests/hour) and frequent operations may get throttled.",
		"现在添加 GitHub Token？":                "Add a GitHub token now?",
		"Token 保存失败：":                       "Failed to save token: ",
		"GitHub Token 已保存到 customize.json。": "GitHub token saved to customize.json.",

		"YAML 配置文件路径":              "Path to the YAML config file",
		"已导入 YAML 配置文件，并设为当前生效配置。": "YAML config file imported and set as the active config.",
		"服务已安装，立即同步并重启以使用该配置？":     "The service is installed; sync and restart now to use this config?",
		"本地文件路径不能为空":               "Local file path must not be empty",
		"读取本地文件: %w":               "Failed to read the local file: %w",
		"请输入文件路径，而不是目录: %s":        "Please provide a file path, not a directory: %s",
		"解析 YAML 配置文件: %w":         "Failed to parse the YAML config file: %w",
	})
}

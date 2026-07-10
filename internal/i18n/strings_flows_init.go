package i18n

func init() {
	register(map[string]string{
		"初始化":       "Initialize",
		"初始化（首次部署）": "Initialize (first-time deployment)",
		"种子接管失败（不影响后续下载）：": "Seed takeover failed (does not affect later downloads): ",

		"订阅/资源下载代理 IP:端口（出海慢时走它，如 192.168.1.10:7890），留空=保留当前/无则直连": "Subscription/resource download proxy IP:port (used when overseas access is slow, e.g. 192.168.1.10:7890); empty = keep current, or direct if none",
		"启用 TUN 模式？（整机流量自动走代理；否=纯代理，需各 App 手动设代理）":                 "Enable TUN mode? (all system traffic auto-routes through the proxy; no = pure proxy mode, each app must set its own proxy)",
		"开启局域网代理？（让局域网其他主机可用本机作为代理，监听 0.0.0.0:7890）":               "Enable LAN proxy? (lets other hosts on the LAN use this machine as a proxy, listening on 0.0.0.0:7890)",
		"把代理环境变量写入 ~/.bashrc？（新开终端自动走 127.0.0.1:7890）":             "Write proxy environment variables to ~/.bashrc? (new terminals will automatically use 127.0.0.1:7890)",
		"更新防火墙放行 7890 端口？":      "Update firewall to allow port 7890?",
		"撤销防火墙放行 7890":          "Revoke firewall rule for port 7890",
		"撤销防火墙放行代理端口":           "Revoke firewall rule for the proxy port",
		"本地代理端口（默认 7890，被占用可改）": "Local proxy port (default 7890, change if occupied)",

		// 初始化单屏表单（initform.go）
		"ClashDock 初始化": "ClashDock Initialization",
		"开始初始化":         "Start Initialization",
		"基础设置":          "Basic Settings",
		"订阅设置":          "Subscription Settings",
		"下载代理（IP:端口，留空=直连）":       "Download proxy (IP:port, empty = direct)",
		"启用 TUN 模式（整机流量自动走代理）":    "Enable TUN mode (all traffic auto-routes through the proxy)",
		"开启局域网代理（监听 0.0.0.0:%d）":  "Enable LAN proxy (listen on 0.0.0.0:%d)",
		"把代理变量写入 ~/.bashrc":       "Write proxy variables to ~/.bashrc",
		"放行防火墙 %d 端口":             "Allow firewall port %d",
		"直连 UID（逗号分隔，防 SSH 断连）":   "Direct-connect UIDs (comma-separated, avoids SSH drop)",
		"Fake-IP 过滤规则（逗号分隔）":      "Fake-IP filter rules (comma-separated)",
		"提示：更多细项可在启动后于「配置变更」中设置。": "Tip: configure more details later under Configuration Changes after startup.",
		"订阅名称":                    "Subscription name",
		"订阅类型":                    "Subscription type",
		"本地 YAML":                 "Local YAML",
		"文件路径（留空=暂不配置）":           "File path (empty = configure later)",
		"订阅链接（留空=暂不配置）":           "Subscription URL (empty = configure later)",
		"使用代理拉取订阅（默认否=直连）":        "Fetch subscription via proxy (default no = direct)",
		"叠加自定义分流（AI / 流媒体 / 地区组）": "Overlay custom routing (AI / streaming / region groups)",

		"已跳过订阅与服务注册，结束初始化。设置已保存，":    "Skipped subscription and service registration; initialization ended. Settings have been saved; ",
		"稍后可在主菜单「订阅 → 添加订阅」补配并启动服务。": "you can later finish setup via the main menu 'Subscriptions → Add subscription' and start the service.",

		"卸载服务 mihomo": "Uninstall mihomo service",

		"配置已就绪，是否现在切换 / 固定节点？": "Config is ready. Switch / pin a node now?",
		"初始化完成。": "Initialization complete.",

		"删除订阅 ": "Delete subscription ",

		"检测到本地已有 %d 个订阅记录，是否直接使用现有订阅？": "Detected %d existing subscription(s) locally. Use the existing subscription directly?",
		"已使用现有订阅：%s": "Using existing subscription: %s",

		"使用本地内核与基础规则启动服务（系统包种子或既有资源）。":                                               "Starting the service with the local core and basic rules (system package seed or existing resources).",
		"未找到本地内核或基础规则：请通过安装包（deb 已内置，便携包运行 install.sh）安装内核，或在「运行时管理 → 更新内核」手动下载后重试。": "Local core or basic rules not found: install the core via the package (bundled in the .deb, or run install.sh from the portable archive), or download it manually via 'Runtime management → Update core', then retry.",
		"缺少 mihomo 内核或基础规则，无法注册并启动服务":                                                "Missing mihomo core or basic rules; cannot register and start the service",

		"本地内核缺失，本次暂不注册/启动服务：": "Local core missing; skipping service registration/startup this time: ",
		"已保留本次配置与订阅；补齐内核（重装安装包或「运行时管理 → 更新内核」）后重新执行初始化即可完成服务注册。": "Your configuration and subscriptions are kept; install the core (reinstall the package, or 'Runtime management → Update core'), then re-run initialization to finish service registration.",

		"服务已启动，自动下载/更新 geo 数据 / Web UI…": "Service started; automatically downloading/updating geo data / Web UI…",
		"已更新资源，重新部署运行时并重启服务…":            "Resources updated; redeploying the runtime and restarting the service…",

		"安装网络切换自愈？":    "Install network self-healing?",
		"卸载网络自愈":       "Uninstall network self-healing",
		"安装每周自动更新定时器？": "Install weekly auto-update timer?",
		"卸载每周更新":       "Uninstall weekly update",
	})
}

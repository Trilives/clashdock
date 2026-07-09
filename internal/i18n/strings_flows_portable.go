package i18n

func init() {
	register(map[string]string{
		"便携模式（轻量，未安装系统服务）":                      "Portable mode (lightweight, no system service installed)",
		"当前为轻量模式：不注册系统服务、不修改系统路径、不需 root。":      "Lightweight mode: no system service registered, no system paths modified, no root required.",
		"需要完整服务（开机自启 / TUN / 局域网代理）请运行安装脚本：%s。": "For full services (auto-start / TUN / LAN proxy), run the install script: %s.",
		"工作目录：%s": "Working directory: %s",

		"从便携包接管内核/规则失败：": "Failed to seed core/rules from the portable archive: ",
		"便携包内未找到 mihomo 内核（deps/mihomo）；请使用完整便携包，或在完整服务模式下下载内核。": "mihomo core (deps/mihomo) not found in the portable archive; use the full portable archive, or download the core in full-service mode.",

		"写入定制层失败（继续）：":  "Failed to write customization layer (continuing): ",
		"未提供订阅，无法启动内核。": "No subscription provided; cannot start the core.",

		"选择订阅":    "Select subscription",
		"（当前）":    " (current)",
		"＋ 添加新订阅": "+ Add a new subscription",

		"配置校验失败，未启动内核。":                              "Config validation failed; core not started.",
		"内核已启动（本机代理 127.0.0.1:7890）。":                "Core started (local proxy 127.0.0.1:7890).",
		"Web 面板（若便携包含 UI）：http://127.0.0.1:9090/ui/": "Web panel (if the portable archive bundles a UI): http://127.0.0.1:9090/ui/",

		"▶ 运行中":       "▶ Running",
		"■ 已停止（异常退出）": "■ Stopped (exited unexpectedly)",
		"切换节点":        "Switch node",
		"最新日志":        "Latest Log",
		"重启内核":        "Restart core",
		"停止并退出":       "Stop and exit",
		"按回车返回菜单… ":   "Press Enter to return to the menu… ",
		"How to Use":  "How to Use",
		"轻量模式只提供本机代理；保持 clashdock 窗口运行，其他程序按需配置代理环境变量。": "Lightweight mode only provides a local proxy; keep the clashdock window running and configure proxy environment variables for other programs as needed.",
		"当前终端临时生效：": "Temporary setup for the current terminal:",
		"测试当前代理：":   "Test the current proxy:",
		"启动方式：":     "Start commands:",
		"离线维护脚本（在解压目录内，无需 root）：":                                "Offline maintenance scripts (in the extracted directory, no root needed):",
		"  ./tool/update.sh   更新 clashdock 本体 / mihomo 内核 / 规则集": "  ./tool/update.sh   Update the clashdock binary / mihomo core / rule set",
		"  ./tool/nettest.sh  测试直连与本机代理的连通性和出口 IP":               "  ./tool/nettest.sh  Test direct vs. local-proxy connectivity and egress IP",
		"图形面板：便携包不含 Web UI，需要请用完整版或在线面板。":                        "Web panel: the portable archive bundles no Web UI; use the full version or an online panel if you need one.",
		"退出 clashdock 后，轻量模式内核会同步停止。":                            "After clashdock exits, the lightweight-mode core stops with it.",

		"调整本地代理端口 / 下载代理？（默认端口 7890，通常无需修改）": "Adjust the local proxy port / download proxy? (default port 7890; usually no change needed)",
		"便携设置": "Portable Settings",
		"保存":   "Save",
		"使用默认": "Use defaults",

		"暂无日志：":          "No log yet: ",
		"内核已重启。":         "Core restarted.",
		"收到退出信号，正在停止内核…": "Received exit signal; stopping the core…",
	})
}

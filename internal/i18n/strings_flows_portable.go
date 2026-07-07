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

		"配置校验失败，未启动内核。":                              "Config validation failed; core not started.",
		"内核已启动（本机代理 127.0.0.1:7890）。":                "Core started (local proxy 127.0.0.1:7890).",
		"Web 面板（若便携包含 UI）：http://127.0.0.1:9090/ui/": "Web panel (if the portable archive bundles a UI): http://127.0.0.1:9090/ui/",

		"▶ 运行中":       "▶ Running",
		"■ 已停止（异常退出）": "■ Stopped (exited unexpectedly)",
		"切换节点":        "Switch node",
		"查看内核日志":      "View core log",
		"重启内核":        "Restart core",
		"停止并退出":       "Stop and exit",

		"暂无日志：":          "No log yet: ",
		"内核已重启。":         "Core restarted.",
		"收到退出信号，正在停止内核…": "Received exit signal; stopping the core…",
	})
}

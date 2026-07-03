package i18n

func init() {
	register(map[string]string{
		"mihomo 部署系统": "mihomo deployment system",
		"退出":          "Exit",
		"再见。":         "Bye.",

		"初始化（首次部署）":     "Initialize (first-time deployment)",
		"配置变更（需重启生效）":   "Config Changes (restart to apply)",
		"运行时管理（无需重启）":   "Runtime Management (no restart)",
		"工具":            "Tools",
		"卸载所有服务":        "Uninstall All Services",
		"语言 / Language": "Language / 语言",

		"暂停 / 启动服务": "Pause / Start Service",
		"暂停服务 ⏸":    "Pause Service ⏸",
		"启动服务 ▶":    "Start Service ▶",

		"未检测到已注册的服务，是否现在进行初始化？": "No registered service detected. Run initialization now?",

		"用法: clashdock [init|modify|nettest|uninstall|update|pause|resume|version]\n不带参数则进入交互式主菜单。": "Usage: clashdock [init|modify|nettest|uninstall|update|pause|resume|version]\nRun without arguments to enter the interactive main menu.",
		"未知子命令: %s\n%s\n": "Unknown subcommand: %s\n%s\n",

		"独立面板刷新失败：": "Failed to refresh the standalone panel: ",

		"监听端口":                    "listen port",
		"绑定地址":                    "bind address",
		"静态文件目录":                  "static file directory",
		"webui-serve 需要 --dir":    "webui-serve requires --dir",
		"静态面板服务: http://%s/ ← %s": "Static panel service: http://%s/ <- %s",
	})
}

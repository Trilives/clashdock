package i18n

func init() {
	register(map[string]string{
		"mihomo 部署系统": "mihomo deployment system",
		"退出":          "Exit",
		"再见。":         "Bye.",

		"初始化（首次部署）":     "Initialize (first-time deployment)",
		"配置变更":          "Config Changes",
		"运行时管理":         "Runtime Management",
		"工具":            "Tools",
		"卸载":            "Uninstall",
		"语言 / Language": "Language / 语言",

		"暂停 / 启动服务": "Pause / Start Service",
		"暂停服务 ⏸":    "Pause Service ⏸",
		"启动服务 ▶":    "Start Service ▶",

		"未检测到已注册的服务，是否现在进行初始化？": "No registered service detected. Run initialization now?",

		"用法: clashdock [user|init|modify|nettest|uninstall|update|pause|resume|version]\n不带参数则进入交互式主菜单（从解压的便携包目录启动或已部署用户服务时进入用户模式）。": "Usage: clashdock [user|init|modify|nettest|uninstall|update|pause|resume|version]\nRun without arguments to enter the interactive main menu (user mode is entered when launched from an extracted portable archive or when the user service is deployed).",
		"未知子命令: %s\n%s\n": "Unknown subcommand: %s\n%s\n",

		"可执行文件已被旧版自更新改成符号链接，apt 升级会与之冲突；可在「工具 → 更新 → clashdock 自身」恢复为 dpkg 管理。": "The executable was turned into a symlink by an older self-update and will conflict with apt upgrades; restore dpkg management under \"Tools → Update → clashdock itself\".",
		"清理旧版自更新目录失败：":                            "Failed to clean up the old self-update directory: ",
		"修复伴生单元入口失败：":                             "Failed to repair the companion unit entry point: ",
		"系统模式由 root 管理，正在通过 sudo 重新启动 clashdock…": "System mode is managed by root; restarting clashdock through sudo…",
	})
}

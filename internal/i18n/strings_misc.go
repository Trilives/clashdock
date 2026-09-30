package i18n

func init() {
	register(map[string]string{
		// internal/firewall
		"未探测到防火墙工具，请自行确认放行 %d/tcp,udp（或本机无防火墙）。": "No firewall tool detected; please manually confirm port %d/tcp,udp is open (or this machine has no firewall).",
		"经 %s 放行 %d/tcp,udp …": "Allowing %d/tcp,udp via %s…",
		"已放行 %d 端口（%s）。":       "Port %d allowed (%s).",
		"经 %s 撤销放行 %d …":       "Revoking allow-rule for %d via %s…",
		"更新防火墙":                "Update firewall",
		"nftables 规则请手动移除：nft -a list chain inet filter input 查看句柄后 delete。": "For nftables, please remove the rule manually: run `nft -a list chain inet filter input` to find the handle, then `delete` it.",

		// internal/proxyenv
		"已写入代理环境变量到 %s（新开终端生效；当前终端可 `source %s`）。": "Proxy environment variables written to %s (effective in new terminals; run `source %s` in the current one).",
		"已从 %s 移除代理环境变量。":                          "Proxy environment variables removed from %s.",

		// internal/fetchx
		"直连可达，跳过代理。":           "Direct connection reachable, skipping proxy.",
		"  %s 通道失败（%v），改直连重试…": "  %s channel failed (%v), retrying direct…",

		// internal/configfile
		"配置根节点不是映射":   "The config root node is not a mapping",
		"解析配置 %s: %w": "Failed to parse config %s: %w",

		// internal/txn
		"已取消「%s」。":        "\"%s\" cancelled.",
		"「%s」出错：%v":       "\"%s\" failed: %v",
		"还原文件 ":           "Restore file ",
		"删除新建文件 ":         "Delete newly created file ",
		"还原 ":             "Restore ",
		"删除新建路径 ":         "Delete newly created path ",
		"正在回退「%s」已应用的改动…": "Rolling back changes applied by \"%s\"…",
		"  回退失败: %s (%v)": "  Rollback failed: %s (%v)",
		"  已回退: ":         "  Rolled back: ",
		"回退完成，但有 %d 项失败，请手动检查。": "Rollback finished, but %d item(s) failed; please check manually.",
		"已回退到操作前状态。":            "Rolled back to the state before the operation.",

		// 此前遗漏的界面文案（由 coverage_test 发现）
		"检测到本地内核/geo 数据已更新，但运行中的服务尚未使用最新版本，是否现在重启应用？": "Local core/geo data has been updated but the running service is not using it yet. Restart now to apply it?",
		"已应用最新内核/geo 数据并重启服务。":                        "Applied the latest core/geo data and restarted the service.",
		"日志启用失败：": "Failed to enable logging: ",
		"直连":      "direct",
		"  %s 失败（%v），改下一通道重试…": "  %s failed (%v); retrying via the next channel…",
		"TUN 模式下直连可能仍被路由劫持；是否临时暂停服务以确保本次直连成功？（拉取完成后自动恢复）": "In TUN mode a direct fetch may still be hijacked by routing; temporarily pause the service to ensure this fetch is really direct? (resumes automatically afterwards)",
		"回车返回主菜单… ":                    "Press Enter to return to the main menu… ",
		"资源更新失败：":                      "Resource update failed: ",
		"可稍后在「工具 → 更新」重试。":             "You can retry later via 'Tools → Update'.",
		"重新部署运行时失败：":                   "Failed to redeploy the runtime: ",
		"  代理候选失败（%v），改下一候选重试…":        "  Proxy candidate failed (%v); trying the next candidate…",
		"临时暂停服务以确保本次直连不被 TUN 路由劫持…":    "Temporarily pausing the service so this direct fetch is not hijacked by TUN routing…",
		"暂停服务失败，继续直连拉取：":               "Failed to pause the service; continuing the direct fetch: ",
		"恢复服务失败，请手动启动：":                "Failed to resume the service; please start it manually: ",
		"未找到 Web UI，面板将不可用；可稍后执行更新补齐。": "Web UI not found; the panel will be unavailable. You can fetch it later via Update.",
		"记录资源部署指纹失败（不影响服务本身）：":         "Failed to record the deployed-asset fingerprint (the service itself is unaffected): ",
	})
}

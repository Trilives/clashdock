package i18n

func init() {
	register(map[string]string{
		"使用缓存: ":          "Using cache: ",
		"丢弃无效缓存: ":        "Discarding invalid cache: ",
		"下载: ":            "Downloading: ",
		"下载文件完整性校验失败: %s": "Downloaded file integrity check failed: %s",
		"下载文件为空: %s":      "Downloaded file is empty: %s",

		"查找最新 mihomo 版本…":            "Looking up the latest mihomo version…",
		"未找到架构 %s 的 Linux mihomo 资源": "No Linux mihomo asset found for architecture %s",
		"内核已部署: ":                    "Core deployed: ",

		"geo 数据已更新": "geo data updated",

		"查找最新 Web UI 版本…":         "Looking up the latest Web UI version…",
		"未从 %s releases 找到 UI 资源": "No UI asset found in %s releases",
		"未能定位 UI 根目录: %s":         "Could not locate the UI root directory: %s",
		"Web UI 已部署":              "Web UI deployed",
		"Web UI 更新失败（不影响内核与 geo 数据，可稍后重试）：%v": "Web UI update failed (core and geo data are unaffected; you can retry later): %v",

		"不支持的压缩格式: %s": "Unsupported archive format: %s",
		"非法压缩条目路径: %s": "Illegal archive entry path: %s",

		"下载代理（直连不可用时回退）: ":                "Download proxy (fallback when direct connection is unavailable): ",
		"已从系统包接管 %d 个种子文件（离线可用；后续可在线更新）。": "Took over %d seed file(s) from the system package (usable offline; can be updated online later).",
	})
}

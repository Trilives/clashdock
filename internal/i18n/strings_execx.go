package i18n

func init() {
	register(map[string]string{
		"[信息] ": "[info] ",
		"[完成] ": "[ok] ",
		"[注意] ": "[warn] ",
		"[错误] ": "[error] ",

		"命令失败(%d): %s": "command failed(%d): %s",
		"启动命令 %s: %w":  "failed to start command %s: %w",

		"需要管理员权限，但未找到 sudo，请改用 root 运行": "administrator privileges required, but sudo was not found; please run as root instead",
		"需要管理员权限。": "requires administrator privileges.",
		"该操作":      "this operation",

		"经 sudo 重新执行后仍不是 root，请检查 sudoers 配置或直接以 root 运行": "still not root after re-running through sudo; check your sudoers configuration or run as root directly",

		"拒绝把 %s 写进以 root 运行的单元（%v）；请从 root 属主的位置运行，如 /usr/bin/clashdock": "refusing to record %s in a unit that runs as root (%v); run clashdock from a root-owned location such as /usr/bin/clashdock",
		"符号链接层数过多":         "too many levels of symbolic links",
		"%s 不归 root 所有":    "%s is not owned by root",
		"%s 可被非 root 用户写入": "%s is writable by non-root users",
	})
}

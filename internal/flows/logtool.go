// 「最新日志」工具（完整模式）：查看 mihomo 服务（内核）运行日志与 clashdock 应用日志。
// 便携模式的「最新日志」直接 tail 内核 stdout 文件（见 portable.go）；完整模式下内核
// 由 systemd 托管，运行日志走 journald，故这里改用 journalctl 读取。
package flows

import (
	"fmt"
	"os"
	"strings"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/tui"
)

// LatestLogTool 完整模式「最新日志」（「工具」菜单的一项）：先展示 mihomo 服务近
// 若干行运行日志（journald），再展示 clashdock 应用日志文件（enable_log 时才有）。
// 查看后按回车返回，避免日志一闪而过。
func LatestLogTool(p paths.Paths) error {
	execx.Header(i18n.T("最新日志"))

	// 1. mihomo 内核运行日志：完整模式由 systemd 托管，走 journald。
	fmt.Println(i18n.T("【mihomo 服务日志】"))
	printServiceJournal(sysd.DefaultName)

	// 2. clashdock 应用日志文件（定制层 enable_log 打开时记录到此文件）。
	logPath := execx.LogPath(p.State)
	fmt.Println()
	fmt.Printf(i18n.T("【clashdock 日志】%s\n"), logPath)
	if _, err := os.Stat(logPath); err == nil {
		printLogTail(logPath)
	} else {
		execx.Info(i18n.T("应用日志未启用或暂无内容（可在「配置变更 → 部署设置」开启日志）。"))
	}

	fmt.Println()
	tui.Pause(i18n.T("回车返回主菜单… "))
	return nil
}

// printServiceJournal 打印指定服务近 logTailLines 行 journald 日志（尽力而为：
// 无 journalctl 或无读取权限时只警告，不报错）。
func printServiceJournal(name string) {
	if !execx.Have("journalctl") {
		execx.Warn(i18n.T("未找到 journalctl，无法读取服务日志。"))
		return
	}
	r, err := execx.Run([]string{
		"journalctl", "-u", name + ".service",
		"-n", fmt.Sprint(logTailLines), "--no-pager",
	}, &execx.Opt{Capture: true})
	out := strings.TrimSpace(r.Stdout)
	if err != nil || out == "" {
		execx.Warn(i18n.T("暂无服务日志（服务未运行或无读取权限，可尝试 sudo journalctl -u mihomo）。"))
		return
	}
	fmt.Println(out)
}

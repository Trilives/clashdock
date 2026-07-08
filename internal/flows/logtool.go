// 「最新日志」工具（完整模式）：先让用户选择查看 mihomo 服务（内核）运行日志还是
// clashdock 应用日志，再单独一屏展示所选那一份。便携模式的「最新日志」直接 tail 内核
// stdout 文件（见 portable.go）；完整模式下内核由 systemd 托管，运行日志走 journald，
// 故这里改用 journalctl 读取。
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

// LatestLogTool 完整模式「最新日志」（「工具」菜单的一项）：先让用户选择要看哪一份
// 日志——mihomo 内核运行日志（journald）或 clashdock 应用日志（enable_log 时才有）——
// 再在单独一屏展示所选日志，看完按回车回到选择菜单。避免两份日志堆在一屏、互相淹没。
func LatestLogTool(p paths.Paths) error {
	idx := 0
	for {
		options := []string{i18n.T("mihomo 服务日志"), i18n.T("clashdock 日志")}
		i, err := tui.Select(i18n.T("最新日志"), options, tui.SelectOpts{BackLabel: i18n.T("返回上层"), Initial: idx})
		if err != nil {
			return nil
		}
		idx = i
		execx.Header(options[i])
		if i == 0 {
			showMihomoLog()
		} else {
			showClashdockLog(p)
		}
		fmt.Println()
		tui.Pause(i18n.T("回车返回… "))
	}
}

// showMihomoLog 展示 mihomo 内核运行日志：完整模式由 systemd 托管，走 journald。
func showMihomoLog() {
	printServiceJournal(sysd.DefaultName)
}

// showClashdockLog 展示 clashdock 应用日志文件（定制层 enable_log 打开时记录到此文件）。
func showClashdockLog(p paths.Paths) {
	logPath := execx.LogPath(p.State)
	fmt.Printf(i18n.T("日志文件：%s\n"), logPath)
	if _, err := os.Stat(logPath); err == nil {
		printLogTail(logPath)
	} else {
		execx.Info(i18n.T("应用日志未启用或暂无内容（可在「配置变更 → 部署设置」开启日志）。"))
	}
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

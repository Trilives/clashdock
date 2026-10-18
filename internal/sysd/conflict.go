package sysd

import (
	"strings"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
)

// ConflictingName 与 mihomo 争用代理端口 / TUN 的已知系统服务。它可能是用户自行部署的
// （如 sboxkit），clashdock 不会自动停用它：只在交互式初始化中询问，经同意后才停用。
const ConflictingName = "sing-box"

// unitQuery 执行 systemctl 并返回去空白的标准输出（测试替换为替身）。
type unitQuery func(args ...string) string

func systemctlValue(args ...string) string {
	r, _ := execx.Run(append([]string{"systemctl"}, args...), &execx.Opt{Capture: true})
	return strings.TrimSpace(r.Stdout)
}

// ConflictingServiceState 冲突服务是否已安装（单元可加载）、是否正在运行。
func ConflictingServiceState() (installed, active bool) {
	return conflictState(systemctlValue)
}

func conflictState(q unitQuery) (installed, active bool) {
	unit := ConflictingName + ".service"
	if q("show", "-p", "LoadState", "--value", unit) != "loaded" {
		return false, false
	}
	return true, q("is-active", unit) == "active"
}

// StopConflictingService 停止并禁用冲突服务（经用户确认后调用）。不删除单元文件，
// 之后可用 systemctl enable --now 恢复。
func StopConflictingService() error {
	unit := ConflictingName + ".service"
	for _, args := range [][]string{{"systemctl", "stop", unit}, {"systemctl", "disable", unit}} {
		if _, err := execx.RunRoot(args, i18n.T("停用 sing-box 服务"), nil); err != nil {
			return err
		}
	}
	execx.Ok(i18n.T("已停止并禁用 sing-box 服务（单元文件保留，可用 sudo systemctl enable --now sing-box 恢复）。"))
	return nil
}

// warnIfConflictRunning 冲突服务仍在运行时提示（Install 不再自动停用它）。
func warnIfConflictRunning(q unitQuery) {
	if _, active := conflictState(q); active {
		execx.Warn(i18n.T("sing-box 服务仍在运行，可能与 mihomo 争用端口或 TUN 导致其启动失败；如需停用：sudo systemctl disable --now sing-box"))
	}
}

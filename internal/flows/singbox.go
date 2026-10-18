// 初始化注册系统服务前处理已安装的 sing-box 服务：它可能是用户自行部署的，先询问再停用；
// 用户保留它且端口已被占用时明确报错，不静默接管。
package flows

import (
	"fmt"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/tui"
	"github.com/Trilives/clashdock/internal/usermode"
)

// singBoxDeps 外部依赖（测试替换为替身）。
type singBoxDeps struct {
	state     func() (installed, active bool)
	confirm   func(string, bool) (bool, error)
	stop      func() error
	busyPorts func(...int) []int
}

func defaultSingBoxDeps() singBoxDeps {
	return singBoxDeps{
		state:     sysd.ConflictingServiceState,
		confirm:   tui.Confirm,
		stop:      sysd.StopConflictingService,
		busyPorts: usermode.BusyPorts,
	}
}

// resolveSingBoxConflict 已安装 sing-box 服务时询问是否停用（默认否）。同意 → 停止并禁用
// （不删除单元文件）；拒绝且它正在运行 → 检查 mihomo 要用的端口，被占用则返回错误。
func resolveSingBoxConflict(p paths.Paths, d singBoxDeps) error {
	installed, active := d.state()
	if !installed {
		return nil
	}
	status := i18n.T("已停止")
	if active {
		status = i18n.T("正在运行")
	}
	ok, err := d.confirm(fmt.Sprintf(i18n.T("检测到已安装的 sing-box 服务（%s），它会与 mihomo 争用代理端口 / TUN。停止并禁用它？（不删除其文件，之后可用 sudo systemctl enable --now sing-box 恢复）"), status), false)
	if err != nil {
		return err
	}
	if ok {
		return d.stop()
	}
	if !active {
		return nil
	}
	cfg := config.Load(p)
	if busy := d.busyPorts(config.ProxyPort(cfg), config.ControllerPort(cfg)); len(busy) > 0 {
		return fmt.Errorf(i18n.T("已保留 sing-box 服务，但 mihomo 需要的端口 %v 已被占用；请停用 sing-box，或改用其它端口后重新初始化"), busy)
	}
	execx.Warn(i18n.T("已保留 sing-box 服务；两者同时开启 TUN 会争用路由，建议只在其中一个开启 TUN。"))
	return nil
}

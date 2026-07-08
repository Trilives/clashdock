// 各系统组件的小型交互菜单（对应 resilience/timer/service 各自的 menu_flow / toggle_flow）。
package flows

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/subscription"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/tui"
)

func resilienceMenuFlow() error {
	installed := sysd.ResilienceInstalled()
	status := i18n.T("未安装")
	opts := []string{i18n.T("安装网络自愈")}
	if installed {
		status = i18n.T("已安装")
		opts = []string{i18n.T("调整探测间隔"), i18n.T("卸载网络自愈")}
	}
	idx, err := tui.Select(fmt.Sprintf(i18n.T("网络自愈设置（当前：%s）"), status), opts, tui.SelectOpts{})
	if err != nil {
		return nil
	}
	switch {
	case !installed:
		return sysd.InstallResilience(sysd.ResilienceOptions{})
	case idx == 0:
		interval, err := tui.Ask(i18n.T("探测间隔（如 2min / 90s）"), tui.AskOpts{Default: "2min"})
		if err != nil {
			return nil
		}
		return sysd.InstallResilience(sysd.ResilienceOptions{Interval: interval})
	default:
		return sysd.RemoveResilience(sysd.DefaultName)
	}
}

func timerMenuFlow() error {
	installed := sysd.TimerInstalled()
	status := i18n.T("未安装")
	opts := []string{i18n.T("安装每周更新定时器")}
	if installed {
		status = i18n.T("已安装")
		opts = []string{i18n.T("改时间表"), i18n.T("卸载定时器")}
	}
	idx, err := tui.Select(fmt.Sprintf(i18n.T("每周更新定时器（当前：%s）"), status), opts, tui.SelectOpts{})
	if err != nil {
		return nil
	}
	switch {
	case !installed:
		return sysd.InstallTimer("")
	case idx == 0:
		cal, err := tui.Ask(i18n.T("OnCalendar 表达式"), tui.AskOpts{Default: sysd.DefaultOnCalendar})
		if err != nil {
			return nil
		}
		return sysd.InstallTimer(cal)
	default:
		return sysd.RemoveTimer()
	}
}

// ServiceToggleFlow 主菜单『暂停 / 启动服务』统一入口。
func ServiceToggleFlow(p paths.Paths) error {
	if !sysd.IsInstalled(sysd.DefaultName) {
		execx.Warn(i18n.T("服务尚未安装，请先执行『初始化（首次部署）』。"))
		return nil
	}
	active := sysd.IsActive(sysd.DefaultName)
	execx.Header(i18n.T("暂停 / 启动服务"))
	state := i18n.T("已停止")
	if active {
		state = i18n.T("运行中")
	}
	fmt.Printf(i18n.T("  主服务 %s.service：%s\n"), sysd.DefaultName, state)
	for _, unit := range sysd.CompanionUnits() {
		fmt.Printf(i18n.T("  伴生单元 %s：状态见 systemctl\n"), unit)
	}
	action := i18n.T("启动")
	if active {
		action = i18n.T("暂停")
	}
	ok, err := tui.Confirm(fmt.Sprintf(i18n.T("确认%s全部服务？"), action), true)
	if err != nil || !ok {
		return nil
	}
	if active {
		return sysd.Pause(sysd.DefaultName)
	}
	return sysd.Resume(sysd.DefaultName)
}

func serviceSettings(p paths.Paths) error {
	act, err := tui.Select(i18n.T("服务设置"), []string{i18n.T("查看状态"), i18n.T("重启服务"), i18n.T("重载服务（删除并重建）")}, tui.SelectOpts{})
	if err != nil {
		return nil
	}
	switch act {
	case 0:
		sysd.Status(sysd.DefaultName)
	case 1:
		execx.RunRoot([]string{"systemctl", "restart", "mihomo.service"}, i18n.T("重启服务"), nil)
	default:
		return reloadService(p)
	}
	return nil
}

// reloadService 「重载服务」：仅重启服务仍异常时（如内核 TUN 与其它 overlay 网络
// 抢路由后状态错乱），彻底删除服务再重建——先按本地原文重新生成生效订阅的
// config.yaml（重建订阅文件），删除 systemd 单元与运行时，再完整 Install 重新
// 部署内核 / geo / 配置并启动。等价于「同步重启」做不到的一次干净重来。
func reloadService(p paths.Paths) error {
	if !sysd.IsInstalled(sysd.DefaultName) {
		execx.Warn(i18n.T("服务尚未安装，请先执行『初始化（首次部署）』。"))
		return nil
	}
	ok, err := tui.Confirm(i18n.T("将删除并重建服务（重新生成订阅配置、重装运行时并重启），确认继续？"), true)
	if err != nil || !ok {
		return err
	}
	// 1. 重新生成生效订阅的 config.yaml（本地原文重建，不重新拉取链接）。
	if active := subscription.GetActive(p); active != nil {
		if _, rerr := subscription.Rebuild(p, active.Name); rerr != nil {
			execx.Warn(i18n.T("重建订阅配置失败（继续用现有 config.yaml）：") + rerr.Error())
		} else {
			execx.Ok(fmt.Sprintf(i18n.T("已重建生效订阅配置：%s"), active.Name))
		}
	}
	// 2. 彻底删除服务与运行时。
	if err := sysd.Remove(p, sysd.DefaultName, true); err != nil {
		return err
	}
	// 3. 完整重建服务（重新部署内核 / geo / UI / 配置并启动）。
	if err := sysd.Install(p, sysd.DefaultName, true); err != nil {
		return err
	}
	execx.Ok(i18n.T("服务已重载（已删除并重建）。"))
	return nil
}

// printAccessHint 初始化完成后的访问方式提示。
func printAccessHint(p paths.Paths) {
	cfg := config.Load(p)
	lanPanel := config.Bool(cfg, "lan_panel")
	host := "127.0.0.1"
	if lanPanel {
		host = "0.0.0.0"
	}
	if _, err := os.Stat(filepath.Join(p.UI, "index.html")); err == nil {
		execx.Info(fmt.Sprintf(i18n.T("Web UI（mihomo 内置路径）: http://%s:9090/ui/"), host))
	}
	if host == "127.0.0.1" {
		execx.Info(i18n.T("远程查看建议用 SSH 端口转发： ssh -N -L 9090:127.0.0.1:9090 user@server"))
	}
	if config.Bool(cfg, "lan_proxy") {
		execx.Info(i18n.T("局域网代理已开启：其他主机可设置 http/socks 代理为 本机IP:7890"))
	}
}

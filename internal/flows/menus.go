// 各系统组件的小型交互菜单（对应 webui/resilience/timer/service 各自的 menu_flow / toggle_flow）。
package flows

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/firewall"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/tui"
)

// webuiSetupInteractive 交互式安装/重配独立面板。返回最终端口（取消 / 无效为 0）。
func webuiSetupInteractive(p paths.Paths, defaultPort int, lan bool) (int, error) {
	cfg := config.Load(p)
	port := defaultPort
	if port == 0 {
		port = config.Int(cfg, "webui_port")
	}
	if port == 0 {
		port = sysd.DefaultWebUIPort
	}
	raw, err := tui.Ask(i18n.T("独立面板端口"), tui.AskOpts{Default: strconv.Itoa(port)})
	if err != nil {
		return 0, err
	}
	port, perr := strconv.Atoi(raw)
	if perr != nil {
		execx.Warn(i18n.T("端口需为整数，已取消。"))
		return 0, nil
	}
	if err := sysd.InstallWebUI(p, sysd.WebUIOptions{Port: port, Lan: lan}); err != nil {
		return 0, err
	}
	cfg["webui_port"] = port
	if err := config.Save(p, cfg); err != nil {
		return port, err
	}
	if lan {
		ok, err := tui.Confirm(fmt.Sprintf(i18n.T("更新防火墙放行 %d 端口？"), port), true)
		if err == nil && ok {
			firewall.Allow(port)
		}
	}
	return port, nil
}

func webuiMenuFlow(p paths.Paths) error {
	installed := sysd.WebUIInstalled()
	status := i18n.T("未安装")
	opts := []string{i18n.T("安装独立面板（根路径直接打开）")}
	if installed {
		status = i18n.T("已安装")
		opts = []string{i18n.T("重新配置 / 换端口"), i18n.T("卸载独立面板")}
	}
	idx, err := tui.Select(fmt.Sprintf(i18n.T("独立 Web 面板（当前：%s）"), status), opts, tui.SelectOpts{})
	if err != nil {
		return nil // 取消返回上层
	}
	if installed && idx == 1 {
		return sysd.RemoveWebUI()
	}
	lan := config.Bool(config.Load(p), "lan_panel")
	_, err = webuiSetupInteractive(p, 0, lan)
	return err
}

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
	act, err := tui.Select(i18n.T("服务设置"), []string{i18n.T("查看状态"), i18n.T("重启服务"), i18n.T("同步当前配置并重启")}, tui.SelectOpts{})
	if err != nil {
		return nil
	}
	switch act {
	case 0:
		sysd.Status(sysd.DefaultName)
	case 1:
		execx.RunRoot([]string{"systemctl", "restart", "mihomo.service"}, i18n.T("重启服务"), nil)
	default:
		return sysd.SyncAndRestart(p, sysd.DefaultName)
	}
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
	if sysd.WebUIInstalled() {
		port := config.Int(cfg, "webui_port")
		if port == 0 {
			port = sysd.DefaultWebUIPort
		}
		disp := "127.0.0.1"
		if lanPanel {
			disp = host
		}
		execx.Info(fmt.Sprintf(i18n.T("Web 面板（根路径直开）: http://%s:%d/"), disp, port))
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

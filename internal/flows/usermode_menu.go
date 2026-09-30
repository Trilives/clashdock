// 用户模式管理菜单：节点 / 配置 / 服务开关 / 工具 / 卸载，全部围绕用户服务开展。
package flows

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/errs"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/proxyenv"
	"github.com/Trilives/clashdock/internal/runtimesvc"
	"github.com/Trilives/clashdock/internal/tui"
	"github.com/Trilives/clashdock/internal/usermode"
)

// logTailLines 「最新日志」显示的行数。
const logTailLines = 40

var userModifyConfigOptions = []string{
	"订阅管理（增 / 删 / 改名 / 切换 / 刷新）",
	"本地设置（端口 / 下载 / DNS）",
	"自定义分流叠加（AI / 流媒体 / 地区组）",
}

// userModeMenu 主菜单循环；卸载后退出。
func userModeMenu(p paths.Paths, version string) error {
	idx := 0
	for {
		svc := usermode.NewService(p)
		options := []string{
			i18n.T("节点切换"),
			i18n.T("固定节点"),
			i18n.T("配置变更"),
			userToggleLabel(svc),
			i18n.T("重启服务"),
			i18n.T("工具"),
			i18n.T("语言 / Language"),
			i18n.T("卸载"),
		}
		title := fmt.Sprintf(i18n.T("clashdock 用户模式 · %s"), userStatusLabel(svc))
		i, err := tui.Select(title, options, tui.SelectOpts{BackLabel: i18n.T("退出"), Initial: idx})
		if err != nil {
			fmt.Println(i18n.T("再见。"))
			return nil
		}
		idx = i
		var aerr error
		switch i {
		case 0:
			aerr = NodeSwitchLive(p, p.ConfigFile, "")
		case 1:
			aerr = NodeSelect(p, p.ConfigFile, "")
		case 2:
			aerr = UserModifyConfig(p)
		case 3:
			aerr = UserServiceToggle(p)
		case 4:
			aerr = restartUserService(p)
		case 5:
			aerr = userToolsMenu(p, version)
		case 6:
			aerr = PickLanguage(p)
		case 7:
			var removed bool
			if removed, aerr = UserUninstall(p); removed && aerr == nil {
				return nil
			}
		}
		if aerr != nil && !errors.Is(aerr, errs.ErrCancelled) {
			execx.Error(aerr.Error())
		}
	}
}

func userStatusLabel(svc usermode.Service) string {
	switch {
	case !svc.Installed():
		return i18n.T("未部署")
	case svc.Active():
		return i18n.T("▶ 运行中")
	}
	return i18n.T("■ 已停止")
}

func userToggleLabel(svc usermode.Service) string {
	if svc.Active() {
		return i18n.T("停止服务 ⏸")
	}
	return i18n.T("启动服务 ▶")
}

// UserModifyConfig 用户模式的配置变更会话：与完整模式共用事务骨架（esc 保存 / ^R 回退），
// 字段只开放用户模式可用的本地设置（无 TUN / 局域网 / 防火墙）。
func UserModifyConfig(p paths.Paths) error {
	return modifySession(p, "配置变更", userModifyConfigOptions, []func() error{
		func() error { return subscriptionsMenu(p) },
		func() error { return editFieldGroupFlow(p, userModifyConfigOptions[1], config.UserModeFields) },
		func() error { return editFieldGroupFlow(p, userModifyConfigOptions[2], config.OverlayFields) },
	})
}

// UserServiceToggle 启动 / 停止用户服务（服务保持部署与自启，只改变当前运行状态）。
func UserServiceToggle(p paths.Paths) error {
	svc := runtimesvc.For(p)
	if !svc.Installed() {
		execx.Warn(i18n.T("用户服务尚未部署，请运行 clashdock user 完成部署。"))
		return nil
	}
	if svc.Active() {
		if err := svc.Pause(); err != nil {
			return err
		}
		execx.Ok(i18n.T("用户服务已停止。"))
		return nil
	}
	if err := svc.Resume(); err != nil {
		return err
	}
	execx.Ok(i18n.T("用户服务已启动。"))
	return nil
}

// restartUserService 重新铺运行时（配置 / geo / UI）并重启，也用于从异常退出恢复。
func restartUserService(p paths.Paths) error {
	svc := runtimesvc.For(p)
	if !svc.Installed() {
		execx.Warn(i18n.T("用户服务尚未部署，请运行 clashdock user 完成部署。"))
		return nil
	}
	if err := svc.SyncAndRestart(); err != nil {
		return err
	}
	execx.Ok(i18n.T("用户服务已重启。"))
	return nil
}

// userToolsMenu 工具：网络测试 / 最新日志 / 更新（内核 / Web UI / geo / clashdock 自身）/ How to Use。
func userToolsMenu(p paths.Paths, version string) error {
	options := []string{i18n.T("网络测试"), i18n.T("最新日志"), i18n.T("更新"), i18n.T("How to Use（用法与文件位置）")}
	idx := 0
	for {
		i, err := tui.Select(i18n.T("工具"), options, tui.SelectOpts{BackLabel: i18n.T("返回上层"), Initial: idx})
		if err != nil {
			return nil
		}
		idx = i
		var aerr error
		switch i {
		case 0:
			aerr = Nettest(config.ProxyPort(config.Load(p)))
		case 1:
			printUserLogs(usermode.NewService(p))
			tui.Pause(i18n.T("按回车返回菜单… "))
		case 2:
			aerr = updateMenuFlow(p, version)
		case 3:
			fmt.Println(userHowToUseText(p, usermode.NewService(p)))
			tui.Pause(i18n.T("按回车返回菜单… "))
		}
		if aerr != nil && !errors.Is(aerr, errs.ErrCancelled) {
			execx.Error(aerr.Error())
		}
	}
}

func printUserLogs(svc usermode.Service) {
	out, err := svc.Logs(logTailLines)
	if err != nil || strings.TrimSpace(out) == "" {
		msg := i18n.T("暂无日志")
		if err != nil {
			msg += "：" + err.Error()
		}
		execx.Warn(msg)
		return
	}
	fmt.Println(strings.TrimRight(out, "\n"))
}

// userHowToUseText 用法说明：代理环境变量、测试命令、管理命令与文件位置。
func userHowToUseText(p paths.Paths, svc usermode.Service) string {
	port := config.ProxyPort(config.Load(p))
	httpURL := fmt.Sprintf("http://%s:%d", proxyenv.ProxyHost, port)
	socksURL := fmt.Sprintf("socks5://%s:%d", proxyenv.ProxyHost, port)
	lines := []string{
		"",
		i18n.T("How to Use"),
		fmt.Sprintf(i18n.T("用户模式在后台以用户服务运行 mihomo，本机代理 %s；关闭 clashdock 不影响代理。"), httpURL),
		"",
		i18n.T("当前终端临时生效："),
		fmt.Sprintf(`export http_proxy="%s"`, httpURL),
		`export https_proxy="$http_proxy"`,
		fmt.Sprintf(`export all_proxy="%s"`, socksURL),
		`export HTTP_PROXY="$http_proxy"`,
		`export HTTPS_PROXY="$https_proxy"`,
		`export ALL_PROXY="$all_proxy"`,
		`export no_proxy="localhost,127.0.0.1,::1"`,
		`export NO_PROXY="$no_proxy"`,
		"",
		i18n.T("测试当前代理："),
		fmt.Sprintf("curl -x %s https://www.google.com/generate_204", httpURL),
		"",
		i18n.T("管理命令："),
		i18n.T("  clashdock                 打开管理菜单"),
		i18n.T("  clashdock pause | resume  停止 / 启动用户服务"),
		i18n.T("  clashdock uninstall       卸载用户模式"),
	}
	if svc.Backend() == usermode.BackendSystemd {
		lines = append(lines, "  systemctl --user status "+usermode.UnitName)
	}
	lines = append(lines,
		"",
		i18n.T("文件位置："),
		fmt.Sprintf(i18n.T("  本体：%s"), usermode.BinPath()),
		fmt.Sprintf(i18n.T("  数据目录：%s"), p.State),
		fmt.Sprintf(i18n.T("  运行时：%s"), usermode.RuntimeDir(p)),
		fmt.Sprintf(i18n.T("  服务：%s"), svc.Describe()),
		"",
		fmt.Sprintf(i18n.T("图形面板：在「工具 → 更新 → Web UI」下载后可访问 http://127.0.0.1:%d/ui/"), config.ControllerPort(config.Load(p))),
	)
	return strings.Join(lines, "\n")
}

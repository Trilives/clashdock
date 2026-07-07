// 便携/轻量模式主流程：从解压后的便携包直接前台跑内核，不安装 systemd 服务。
// clashdock 停在前台充当监护进程，mihomo 作为子进程随其存活，退出即停。
package flows

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/portable"
	"github.com/Trilives/clashdock/internal/proxyenv"
	"github.com/Trilives/clashdock/internal/subscription"
	"github.com/Trilives/clashdock/internal/tui"
)

// logTailLines 「查看日志」显示的行数。
const logTailLines = 40

// PortableRun 便携模式入口：提示轻量模式 → 准备本地工作目录与内核种子 → 添加/复用
// 订阅（走既有 Add Subscription 路径，强制纯代理）→ 铺运行时 → 前台监护内核。
func PortableRun(p paths.Paths, info portable.Info) error {
	if err := p.EnsureStateDirs(); err != nil {
		return fmt.Errorf("prepare workdir: %w", err)
	}
	// 启动第一步（与完整模式一致）：配置文件里没设置过语言才弹语言选择。
	if err := EnsureLanguage(p); err != nil {
		return err
	}

	execx.Header(i18n.T("便携模式（轻量，未安装系统服务）"))
	execx.Info(i18n.T("当前为轻量模式：不注册系统服务、不修改系统路径、不需 root。"))
	execx.Info(fmt.Sprintf(i18n.T("需要完整服务（开机自启 / TUN / 局域网代理）请运行安装脚本：%s。"), "./install.sh"))
	execx.Info(fmt.Sprintf(i18n.T("工作目录：%s"), p.State))

	// 内核与基础规则：优先从便携包 deps/ 目录种子接管；已在 state 里则跳过。
	if info.DepsDir != "" {
		if _, err := kernel.SeedFrom(p, info.DepsDir, filepath.Join(info.DepsDir, "rules")); err != nil {
			execx.Warn(i18n.T("从便携包接管内核/规则失败：") + err.Error())
		}
	}
	if _, err := os.Stat(p.MihomoBin); err != nil {
		return fmt.Errorf("%s", i18n.T("便携包内未找到 mihomo 内核（deps/mihomo）；请使用完整便携包，或在完整服务模式下下载内核。"))
	}

	// 便携模式只做纯代理（无 root 无法开 TUN / 改防火墙）：强制关掉 TUN 与局域网代理，
	// 使订阅改写生成的 config.yaml 监听本机 mixed-port。
	forcePureProxy(p)

	if err := ensurePortableSubscription(p); err != nil {
		return err
	}

	if err := portable.StageRuntime(p); err != nil {
		return fmt.Errorf("stage runtime: %w", err)
	}

	sup := portable.NewSupervisor(p.MihomoBin, portable.RuntimeDir(p))
	if err := sup.Validate(); err != nil {
		return fmt.Errorf("%s\n%w", i18n.T("配置校验失败，未启动内核。"), err)
	}
	if err := sup.Start(); err != nil {
		return err
	}
	// 前台所有权：无论正常退出还是收到信号，都确保子进程随之终止（不留后台孤儿）。
	defer sup.Stop()
	stopOnSignal(sup)

	execx.Ok(i18n.T("内核已启动（本机代理 127.0.0.1:7890）。"))
	execx.Info(i18n.T("Web 面板（若便携包含 UI）：http://127.0.0.1:9090/ui/"))
	return portableSupervisorLoop(p, sup)
}

// forcePureProxy 便携模式落盘纯代理定制层：关闭 TUN / 局域网代理。
func forcePureProxy(p paths.Paths) {
	cfg := config.Load(p)
	cfg["enable_tun"] = false
	cfg["lan_proxy"] = false
	if err := config.Save(p, cfg); err != nil {
		execx.Warn(i18n.T("写入定制层失败（继续）：") + err.Error())
	}
}

// ensurePortableSubscription 已有订阅时直接进入「选择订阅」界面（列出现有订阅，末项
// 是「添加新订阅」）；没有任何订阅则直接引导添加首个订阅（走既有 Add 路径）。
func ensurePortableSubscription(p paths.Paths) error {
	existing := subscription.ListAll(p)
	if len(existing) == 0 {
		return addPortableSubscription(p)
	}

	activeName := ""
	if active := subscription.GetActive(p); active != nil {
		activeName = active.Name
	}
	options := make([]string, 0, len(existing)+1)
	initial := 0
	for idx, s := range existing {
		label := s.Name
		if s.Name == activeName {
			label += i18n.T("（当前）")
			initial = idx
		}
		options = append(options, label)
	}
	addIdx := len(options)
	options = append(options, i18n.T("＋ 添加新订阅"))

	sel, err := tui.Select(i18n.T("选择订阅"), options, tui.SelectOpts{Initial: initial})
	if err != nil {
		return err
	}
	if sel == addIdx {
		return addPortableSubscription(p)
	}
	target := existing[sel].Name
	if err := subscription.Switch(p, target); err != nil {
		return err
	}
	execx.Ok(fmt.Sprintf(i18n.T("已使用现有订阅：%s"), target))
	return nil
}

// addPortableSubscription 引导添加一个新订阅并设为 active（走既有 Add 路径）。
func addPortableSubscription(p paths.Paths) error {
	info, err := askNewSubscription(p)
	if err != nil {
		return err
	}
	if info == nil {
		return fmt.Errorf("%s", i18n.T("未提供订阅，无法启动内核。"))
	}
	if _, err := subscription.Add(p, info.Name, info.URL, info.SourceType,
		info.ApplyOverlay, true, info.FetchViaProxy, info.PauseForDirect); err != nil {
		return err
	}
	return nil
}

// portableSupervisorLoop 监护循环：内核在跑，菜单可切节点 / 看日志 / 重启 / 停止退出。
func portableSupervisorLoop(p paths.Paths, sup *portable.Supervisor) error {
	idx := 0
	for {
		status := i18n.T("▶ 运行中")
		if !sup.Running() {
			status = i18n.T("■ 已停止（异常退出）")
		}
		options := []string{
			i18n.T("切换节点"),
			i18n.T("查看内核日志"),
			i18n.T("重启内核"),
			i18n.T("How to Use"),
			i18n.T("停止并退出"),
		}
		i, err := tui.Select(status, options, tui.SelectOpts{BackLabel: i18n.T("停止并退出"), Initial: idx})
		if err != nil {
			// esc/退出 → 停止内核并结束（defer sup.Stop 兜底）。
			return nil
		}
		idx = i
		switch i {
		case 0:
			if serr := NodeSelect(p, "", ""); serr != nil {
				execx.Warn(serr.Error())
			}
		case 1:
			printLogTail(sup.LogPath())
		case 2:
			if serr := restartPortableKernel(p, sup); serr != nil {
				execx.Warn(serr.Error())
			}
		case 3:
			printPortableHowToUse(p)
		case 4:
			return nil
		}
	}
}

func printPortableHowToUse(p paths.Paths) {
	fmt.Println(portableHowToUseText(p))
}

func portableHowToUseText(p paths.Paths) string {
	httpURL := fmt.Sprintf("http://%s:%d", proxyenv.ProxyHost, proxyenv.ProxyPort)
	socksURL := fmt.Sprintf("socks5://%s:%d", proxyenv.ProxyHost, proxyenv.ProxyPort)
	return strings.Join([]string{
		"",
		i18n.T("How to Use"),
		i18n.T("轻量模式只提供本机代理；保持 clashdock 窗口运行，其他程序按需配置代理环境变量。"),
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
		i18n.T("启动方式："),
		"./clashdock",
		"./clashdock run",
		"",
		fmt.Sprintf(i18n.T("工作目录：%s"), p.State),
		i18n.T("退出 clashdock 后，轻量模式内核会同步停止。"),
	}, "\n")
}

// restartPortableKernel 重新铺运行时配置并重启内核（套用固定节点等改动）。
func restartPortableKernel(p paths.Paths, sup *portable.Supervisor) error {
	if err := portable.StageRuntime(p); err != nil {
		return err
	}
	if err := sup.Restart(); err != nil {
		return err
	}
	execx.Ok(i18n.T("内核已重启。"))
	return nil
}

// printLogTail 打印内核日志末尾若干行。
func printLogTail(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		execx.Warn(i18n.T("暂无日志：") + err.Error())
		return
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > logTailLines {
		lines = lines[len(lines)-logTailLines:]
	}
	fmt.Println(strings.Join(lines, "\n"))
}

// stopOnSignal 收到 SIGINT/SIGTERM 时终止内核并退出进程（前台监护语义）。
func stopOnSignal(sup *portable.Supervisor) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		execx.Info(i18n.T("收到退出信号，正在停止内核…"))
		sup.Stop()
		os.Exit(0)
	}()
}

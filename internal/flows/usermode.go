// 用户模式主流程：免 root，以用户级服务（systemctl --user，无 systemd 用户实例时为
// 后台进程）运行 mihomo。便携包目录里的 ./clashdock 负责把本体、内核与规则装进用户
// 主目录并交接给已安装的本体；之后 clashdock 只做管理，关闭它不影响代理。
package flows

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/subscription"
	"github.com/Trilives/clashdock/internal/tui"
	"github.com/Trilives/clashdock/internal/usermode"
)

// UserModeRun 用户模式入口：从便携包启动时先安装 / 交接；用户服务未部署则引导部署；
// 最后进入管理菜单。
func UserModeRun(p paths.Paths, info usermode.Info, version string) error {
	if err := p.EnsureStateDirs(); err != nil {
		return fmt.Errorf("prepare workdir: %w", err)
	}
	// 启动第一步（与完整模式一致）：配置文件里没设置过语言才弹语言选择。
	if err := EnsureLanguage(p); err != nil {
		return err
	}
	if info.FromPackage() {
		handOff, err := prepareFromPackage(p, info, version)
		if err != nil {
			return err
		}
		if handOff {
			handOffToInstalled(info)
		}
	}
	if err := ensurePureProxy(p); err != nil {
		execx.Warn(i18n.T("按纯代理重建订阅配置失败（继续）：") + err.Error())
	}
	svc := usermode.NewService(p)
	if !svc.Installed() {
		if err := userSetup(p, svc); err != nil {
			return err
		}
	}
	return userModeMenu(p, version)
}

// prepareFromPackage 便携包启动：迁移旧便携数据 → 接管内核 / 规则 → 安装本体。
// 返回是否应交接给已安装本体（用户拒绝替换旧版本时继续用当前程序）。
func prepareFromPackage(p paths.Paths, info usermode.Info, version string) (bool, error) {
	execx.Header(i18n.T("用户模式（免 root，用户级服务）"))
	execx.Info(i18n.T("用户模式不需要 root：mihomo 以当前用户身份在后台运行，关闭 clashdock 不影响代理。"))
	execx.Info(fmt.Sprintf(i18n.T("需要 TUN / 局域网代理等完整能力请改用系统安装：%s。"), "sudo ./install.sh"))
	if err := maybeMigrateLegacy(p, info); err != nil {
		execx.Warn(i18n.T("迁移旧便携数据失败（继续）：") + err.Error())
	}
	if _, err := kernel.SeedFrom(p, info.DepsDir, filepath.Join(info.DepsDir, "rules")); err != nil {
		execx.Warn(i18n.T("从便携包接管内核/规则失败：") + err.Error())
	}
	return installUserBinary(info, version)
}

// maybeMigrateLegacy 旧版便携模式把数据放在包目录旁的 clashdock-data；新位置还没有
// 订阅时询问是否复制过来（旧目录原样保留）。
func maybeMigrateLegacy(p paths.Paths, info usermode.Info) error {
	legacy := usermode.LegacyWorkdir(info.ExecPath)
	if !usermode.NeedsMigration(legacy, p.State) {
		return nil
	}
	ok, err := tui.Confirm(fmt.Sprintf(i18n.T("发现旧便携模式数据 %s，复制到 %s 继续使用？"), legacy, p.State), true)
	if err != nil || !ok {
		return err
	}
	if err := usermode.MigrateLegacy(legacy, p.State); err != nil {
		return err
	}
	execx.Ok(fmt.Sprintf(i18n.T("已迁移旧数据；确认无误后可删除 %s。"), legacy))
	return nil
}

// installUserBinary 把便携包里的 clashdock 装到 ~/.local/bin；已装同版本则跳过，
// 版本不同时询问是否替换。返回已安装本体是否与当前程序同版本（可安全交接）。
func installUserBinary(info usermode.Info, version string) (bool, error) {
	bin := usermode.BinPath()
	if sameFile(info.ExecPath, bin) {
		return false, nil
	}
	if installed := usermode.BinaryVersion(bin); installed != "" {
		if installed == version {
			return true, nil
		}
		ok, err := tui.Confirm(fmt.Sprintf(i18n.T("已安装 clashdock %s，替换为此便携包的 %s？"), installed, version), true)
		if err != nil || !ok {
			return false, err
		}
	}
	if err := usermode.InstallBinary(info.ExecPath, bin); err != nil {
		return false, fmt.Errorf(i18n.T("安装 clashdock 本体失败：%w"), err)
	}
	execx.Ok(fmt.Sprintf(i18n.T("已安装 clashdock 到 %s"), bin))
	if dir := filepath.Dir(bin); !usermode.OnPath(dir) {
		execx.Warn(fmt.Sprintf(i18n.T("%s 不在 PATH 中：可把 export PATH=\"$HOME/.local/bin:$PATH\" 加入 ~/.bashrc，或直接运行 %s。"), dir, bin))
	}
	return true, nil
}

// handOffToInstalled 用已安装的本体替换当前进程继续（之后自更新、卸载都作用于它，
// 便携包目录可随意删除）。成功时不返回；失败则提示并继续用当前程序。
func handOffToInstalled(info usermode.Info) {
	bin := usermode.BinPath()
	if sameFile(info.ExecPath, bin) || !fileExists(bin) {
		return
	}
	execx.Info(fmt.Sprintf(i18n.T("切换到已安装的 %s 继续…"), bin))
	err := syscall.Exec(bin, []string{bin, "user"}, os.Environ())
	execx.Warn(i18n.T("切换失败，继续使用当前程序：") + err.Error())
}

func sameFile(a, b string) bool {
	if resolved, err := filepath.EvalSymlinks(b); err == nil {
		b = resolved
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// userSetup 首次部署用户服务：内核就位 → 可选端口 / 下载代理 → 订阅 → 部署并启动。
func userSetup(p paths.Paths, svc usermode.Service) error {
	execx.Header(i18n.T("部署用户服务"))
	execx.Info(fmt.Sprintf(i18n.T("数据目录：%s"), p.State))
	if _, err := os.Stat(p.MihomoBin); err != nil {
		// .deb / install.sh 装过系统种子时也可直接接管（只读复制，无需 root）。
		kernel.SeedFromSystem(p)
	}
	if _, err := os.Stat(p.MihomoBin); err != nil {
		return fmt.Errorf("%s", i18n.T("未找到 mihomo 内核：请在解压的便携包目录里运行 ./clashdock 完成安装。"))
	}
	if err := maybeUserProxySettings(p); err != nil {
		return err
	}
	if err := ensureUserSubscription(p); err != nil {
		return err
	}
	if err := usermode.Deploy(p, svc); err != nil {
		return fmt.Errorf(i18n.T("部署用户服务失败：%w"), err)
	}
	execx.Ok(fmt.Sprintf(i18n.T("用户服务已启动（本机代理 127.0.0.1:%d）。"), config.ProxyPort(config.Load(p))))
	afterDeployHints(svc)
	fmt.Println(userHowToUseText(p, svc))
	tui.Pause(i18n.T("按回车返回菜单… "))
	return nil
}

// afterDeployHints systemd 后端询问开启 linger（登出后继续运行、开机自启）；后台
// 进程后端说明其局限。
func afterDeployHints(svc usermode.Service) {
	if svc.Backend() == usermode.BackendDetached {
		execx.Warn(i18n.T("未检测到 systemd 用户实例，内核以后台进程运行：不会开机自启，重启系统后请重新运行 clashdock 启动。"))
		return
	}
	if usermode.LingerEnabled() {
		return
	}
	ok, err := tui.Confirm(i18n.T("开启 linger？登出后用户服务继续运行，并在开机时自动启动。"), true)
	if err != nil || !ok {
		execx.Info(i18n.T("未开启 linger：用户服务随登录会话启动，全部会话登出后停止。"))
		return
	}
	if err := usermode.EnableLinger(); err != nil {
		execx.Warn(fmt.Sprintf(i18n.T("开启 linger 失败：%v；可请管理员执行 sudo loginctl enable-linger %s。"), err, currentUser()))
		return
	}
	execx.Ok(i18n.T("已开启 linger。"))
}

func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return strconv.Itoa(os.Getuid())
}

// maybeUserProxySettings 可选调整：本地代理端口 / 控制器端口（默认 7890 / 9090）与下载代理。
// 端口未被占用时默认不打扰；多人共用机器上默认端口常被别人的代理占用，此时提醒并默认进入
// 修改，改完仍冲突会再次询问（用户可选择不改、照常继续）。
func maybeUserProxySettings(p paths.Paths) error {
	for {
		cfg := config.Load(p)
		busy := usermode.BusyPorts(config.ProxyPort(cfg), config.ControllerPort(cfg))
		if len(busy) > 0 {
			execx.Warn(fmt.Sprintf(i18n.T("端口 %v 已被占用（可能是其他用户或程序的代理），请改用其他端口。"), busy))
		}
		adjust, err := tui.Confirm(i18n.T("调整本地代理端口 / 控制器端口 / 下载代理？（默认 7890 / 9090，通常无需修改）"), len(busy) > 0)
		if err != nil || !adjust {
			return err
		}
		submitted, err := userSettingsForm(p, cfg)
		if err != nil || !submitted {
			return err
		}
	}
}

// userSettingsForm 单屏表单收集端口与下载代理并落盘；两个端口相同时保留原端口。
func userSettingsForm(p paths.Paths, cfg map[string]any) (bool, error) {
	res, err := tui.Form(i18n.T("用户模式设置"), []tui.Field{
		{Key: "proxy_port", Kind: tui.FieldText,
			Label: i18n.T("本地代理端口（默认 7890，被占用可改）"), Text: strconv.Itoa(config.ProxyPort(cfg))},
		{Key: "controller_port", Kind: tui.FieldText,
			Label: i18n.T("控制器端口（默认 9090，被占用可改）"), Text: strconv.Itoa(config.ControllerPort(cfg))},
		{Key: "download_proxy", Kind: tui.FieldText, AllowEmpty: true,
			Label: i18n.T("下载代理（IP:端口，留空=直连）"), Text: stripScheme(config.Str(cfg, "download_proxy")),
			Placeholder: "192.168.1.10:7890"},
	}, tui.FormOpts{SubmitLabel: i18n.T("保存"), CancelLabel: i18n.T("使用默认")})
	if err != nil || !res.Submitted {
		return false, err
	}
	proxyPort := portFromText(res.Text("proxy_port"), config.ProxyPort(cfg))
	controllerPort := portFromText(res.Text("controller_port"), config.ControllerPort(cfg))
	if proxyPort == controllerPort {
		execx.Warn(i18n.T("本地代理端口与控制器端口不能相同，端口保持不变。"))
	} else {
		cfg["proxy_port"], cfg["controller_port"] = proxyPort, controllerPort
	}
	cfg["download_proxy"] = normalizeProxy(res.Text("download_proxy"))
	if err := config.Save(p, cfg); err != nil {
		return true, fmt.Errorf(i18n.T("写入定制层失败：%w"), err)
	}
	return true, nil
}

// pureProxyOverrides 用户模式没有 root，无法开 TUN / 改防火墙，面板也只对本机开放。
var pureProxyOverrides = map[string]bool{"enable_tun": false, "lan_proxy": false, "lan_panel": false}

// ensurePureProxy 把定制层收敛为纯本机代理；确有改动且存在生效订阅时，用本地原文
// 重建订阅配置（避免沿用按 TUN / 局域网生成的旧配置）。
func ensurePureProxy(p paths.Paths) error {
	cfg := config.Load(p)
	changed := false
	for key, want := range pureProxyOverrides {
		if config.Bool(cfg, key) != want {
			cfg[key] = want
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := config.Save(p, cfg); err != nil {
		return err
	}
	if active := subscription.GetActive(p); active != nil {
		_, err := subscription.Rebuild(p, active.Name)
		return err
	}
	return nil
}

// ensureUserSubscription 已有订阅时进入「选择订阅」（末项「添加新订阅」）；没有任何
// 订阅则直接引导添加首个订阅（走既有 Add 路径）。
func ensureUserSubscription(p paths.Paths) error {
	existing := subscription.ListAll(p)
	if len(existing) == 0 {
		return addUserSubscription(p)
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
		return addUserSubscription(p)
	}
	target := existing[sel].Name
	if err := subscription.Switch(p, target); err != nil {
		return err
	}
	execx.Ok(fmt.Sprintf(i18n.T("已使用现有订阅：%s"), target))
	return nil
}

// addUserSubscription 引导添加一个新订阅并设为 active（走既有 Add 路径）。
func addUserSubscription(p paths.Paths) error {
	info, err := askNewSubscription(p)
	if err != nil {
		return err
	}
	if info == nil {
		return fmt.Errorf("%s", i18n.T("未提供订阅，无法启动内核。"))
	}
	_, err = subscription.Add(p, info.Name, info.URL, info.SourceType,
		info.ApplyOverlay, true, info.FetchViaProxy, info.PauseForDirect)
	return err
}

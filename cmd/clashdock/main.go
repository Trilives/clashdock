// clashdock：在 Linux 上交互式部署 / 管理 mihomo（Clash.Meta）。
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/errs"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/flows"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/portable"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/tui"
)

// version 由构建注入：-ldflags "-X main.version=..."。
var version = "dev"

func usageText() string {
	return i18n.T("用法: clashdock [run|init|modify|nettest|uninstall|update|pause|resume|version]\n不带参数则进入交互式主菜单（从解压的便携包目录启动时自动进入便携模式）。")
}

// portableRequested 用户显式请求便携模式：`clashdock run` 或 `clashdock --portable`。
func portableRequested(args []string) bool {
	return len(args) > 0 && (args[0] == "run" || args[0] == "--portable")
}

func main() {
	args := os.Args[1:]

	// 便携/轻量模式判定：显式 `run`/`--portable`，或无参数时从解压便携包目录启动
	// （旁有 deps/mihomo、未安装系统服务）。命中则把工作目录指向 ./clashdock-data
	// 并走前台监护流程，不注册服务、不改系统路径。见 internal/portable。
	pInfo := portable.Detect(sysd.IsInstalled(sysd.DefaultName))
	if runPortable := portableRequested(args) || (len(args) == 0 && pInfo.Mode == portable.Portable); runPortable {
		if os.Getenv("CLASHDOCK_HOME") == "" {
			os.Setenv("CLASHDOCK_HOME", portable.DefaultWorkdir())
		}
		p := paths.Detect()
		setupLanguage(p)
		setupLogging(p)
		exitFlow(flows.PortableRun(p, pInfo))
	}

	p := paths.Detect()
	setupLanguage(p)
	setupLogging(p)
	if len(args) == 0 {
		exitIfErr(flows.EnsureStateRoot(p))
		os.Exit(interactive(p))
	}
	switch args[0] {
	case "init", "modify", "uninstall", "update", "pause", "resume":
		exitIfErr(flows.EnsureStateRoot(p))
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println("clashdock " + version)
	case "-h", "--help", "help":
		fmt.Println(usageText())
	case "init":
		exitFlow(flows.Init(p))
	case "modify":
		exitFlow(flows.ModifyConfig(p))
	case "nettest":
		exitFlow(flows.Nettest())
	case "uninstall":
		exitFlow(flows.Uninstall(p))
	case "update":
		os.Exit(runUpdate(p))
	case "pause":
		exitFlow(sysd.Pause(sysd.DefaultName))
	case "resume":
		exitFlow(sysd.Resume(sysd.DefaultName))
	case "healthcheck":
		os.Exit(sysd.RunHealthcheck(args[1:]))
	default:
		fmt.Fprintf(os.Stderr, i18n.T("未知子命令: %s\n%s\n"), args[0], usageText())
		os.Exit(2)
	}
}

// setupLanguage 确定界面语言：CLASHDOCK_LANG 环境变量 > customize.json 里的
// language 字段 > 默认英文。config.Load 在状态目录不存在时安全返回默认值。
func setupLanguage(p paths.Paths) {
	if v := os.Getenv("CLASHDOCK_LANG"); v == "zh" {
		i18n.SetLang(i18n.ZH)
		return
	} else if v == "en" {
		i18n.SetLang(i18n.EN)
		return
	}
	if config.Str(config.Load(p), "language") == "zh" {
		i18n.SetLang(i18n.ZH)
	}
}

// setupLogging customize.json 的 enable_log=true 时，把 execx 的输出额外写入
// 日志文件（超限自动裁剪旧内容，见 internal/execx/log.go）。
func setupLogging(p paths.Paths) {
	if !config.Bool(config.Load(p), "enable_log") {
		return
	}
	if err := execx.EnableLog(execx.LogPath(p.State), 0); err != nil {
		execx.Warn(i18n.T("日志启用失败：") + err.Error())
	}
}

func exitIfErr(err error) {
	if err != nil {
		execx.Error(err.Error())
		os.Exit(1)
	}
}

func exitFlow(err error) {
	if err != nil && !errors.Is(err, errs.ErrCancelled) {
		execx.Error(err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

// switchLabel 主菜单服务开关项标签：随主服务当前状态变化。
func switchLabel() string {
	if !sysd.IsInstalled(sysd.DefaultName) {
		return i18n.T("暂停 / 启动服务")
	}
	if sysd.IsActive(sysd.DefaultName) {
		return i18n.T("暂停服务 ⏸")
	}
	return i18n.T("启动服务 ▶")
}

type firstRunDeps struct {
	isInstalled func(string) bool
	confirm     func(string, bool) (bool, error)
	initFlow    func(paths.Paths) error
	reportError func(string)
}

func defaultFirstRunDeps() firstRunDeps {
	return firstRunDeps{
		isInstalled: sysd.IsInstalled,
		confirm:     tui.Confirm,
		initFlow:    flows.Init,
		reportError: execx.Error,
	}
}

// maybeOfferAssetRedeploy 每次进入交互式主菜单都检查一次：内核/geo 数据是否
// 在 state/ 侧被下载更新过，但运行中的服务还没有跟着重新部署（sysd.AssetsStale）。
// 是则询问是否现在重启应用；否则不打扰。
func maybeOfferAssetRedeploy(p paths.Paths) {
	if !sysd.IsInstalled(sysd.DefaultName) || !sysd.AssetsStale(p) {
		return
	}
	ok, err := tui.Confirm(
		i18n.T("检测到本地内核/geo 数据已更新，但运行中的服务尚未使用最新版本，是否现在重启应用？"), true)
	if err != nil || !ok {
		return
	}
	if err := sysd.Install(p, sysd.DefaultName, true); err != nil {
		execx.Error(err.Error())
		return
	}
	execx.Ok(i18n.T("已应用最新内核/geo 数据并重启服务。"))
}

func maybeOfferFirstRunInit(p paths.Paths, deps firstRunDeps) {
	if deps.isInstalled(sysd.DefaultName) {
		return
	}
	ok, err := deps.confirm(i18n.T("未检测到已注册的服务，是否现在进行初始化？"), true)
	if err == nil && ok {
		if ierr := deps.initFlow(p); ierr != nil && !errors.Is(ierr, errs.ErrCancelled) {
			deps.reportError(ierr.Error())
		}
	}
}

func interactive(p paths.Paths) int {
	// 启动第一步：配置文件里没设置过语言（且未用 CLASHDOCK_LANG 指定）才弹语言选择。
	if err := flows.EnsureLanguage(p); err != nil {
		execx.Error(err.Error())
	}
	// 服务单元不存在时询问是否现在进入初始化。已停止但仍注册的服务不算首次运行；
	// 此处只看单元文件是否存在。
	maybeOfferFirstRunInit(p, defaultFirstRunDeps())
	maybeOfferAssetRedeploy(p)
	idx := 0
	for {
		// 顺序按常用程度排列：运行时管理/配置变更/网络测试等日常操作在前，
		// 暂停/启动服务其次，卸载这类低频/破坏性操作放最后。
		options := []string{
			i18n.T("运行时管理"),
			i18n.T("配置变更"),
			i18n.T("工具"),
			switchLabel(),
			i18n.T("语言 / Language"),
			i18n.T("卸载"),
		}
		i, err := tui.Select(i18n.T("mihomo 部署系统"), options, tui.SelectOpts{BackLabel: i18n.T("退出"), Initial: idx})
		if err != nil {
			fmt.Println(i18n.T("再见。"))
			return 0
		}
		idx = i
		var aerr error
		switch i {
		case 0:
			aerr = flows.ModifyRuntime(p, version)
		case 1:
			aerr = flows.ModifyConfig(p)
		case 2:
			aerr = flows.ToolsMenu(p)
		case 3:
			aerr = flows.ServiceToggleFlow(p)
		case 4:
			aerr = flows.PickLanguage(p)
		case 5:
			aerr = flows.Uninstall(p)
		}
		if aerr != nil && !errors.Is(aerr, errs.ErrCancelled) {
			execx.Error(aerr.Error())
		}
	}
}

// runUpdate 非交互全量更新（每周定时器的执行目标）：内核+geo+UI 强制更新 →
// 完整重新部署运行时并重启（SyncAndRestart 只同步 config.yaml，不会重新拷贝
// 新下载的二进制/geo 文件，这里必须用完整 Install 才能让更新真正生效）。
func runUpdate(p paths.Paths) int {
	if _, err := kernel.DownloadAll(p, kernel.Options{Force: true, WithUI: true}); err != nil {
		execx.Error(err.Error())
		return 1
	}
	if sysd.IsInstalled(sysd.DefaultName) {
		if err := sysd.Install(p, sysd.DefaultName, true); err != nil {
			execx.Error(err.Error())
			return 1
		}
	}
	return 0
}

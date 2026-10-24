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
	"github.com/Trilives/clashdock/internal/selfupdate"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/tui"
	"github.com/Trilives/clashdock/internal/usermode"
)

// version 由构建注入：-ldflags "-X main.version=..."。
var version = "dev"

func usageText() string {
	return i18n.T("用法: clashdock [user|init|modify|nettest|uninstall|update|pause|resume|version]\n不带参数则进入交互式主菜单（从解压的便携包目录启动或已部署用户服务时进入用户模式）。")
}

func main() {
	args := os.Args[1:]

	// 用户模式判定：显式 `user`（旧名 `run` / `--portable`），或从解压便携包目录启动 /
	// 已部署用户服务。命中则数据目录指向 ~/.local/share/clashdock，服务走用户级单元，
	// 不提权、不写系统路径。见 internal/usermode 与 cmd/clashdock/usermode.go。
	uInfo := usermode.Detect(sysd.IsInstalled(sysd.DefaultName))
	if userModeApplies(args, uInfo) {
		os.Exit(runUserMode(args, uInfo))
	}

	p := paths.Detect()
	rest, resume := splitResume(args)
	cmd := ""
	if len(rest) > 0 {
		cmd = rest[0]
	}
	setupLanguage(p) // 先于提权：「经 sudo 重新启动」的提示也按设置的语言显示
	isRoot := execx.IsRoot()
	if needsElevation(cmd, isRoot, settingsReadable(p)) ||
		(cmd == "" && !isRoot && menuNeedsRootAtStart(probeStartState(p))) {
		exitIfErr(elevate(args))
	}
	trusted := prepareState(p, cmd)
	setupLogging(p, trusted)
	args = rest
	if len(args) == 0 {
		os.Exit(interactive(p, resume))
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
		exitFlow(flows.Nettest(config.ProxyPort(config.Load(p))))
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
// language 字段 > 默认英文。config.Load 在状态目录不存在时安全返回默认值；
// 读不到 customize.json（系统模式下属组不是本人）时不去尝试，免得打印读取失败。
func setupLanguage(p paths.Paths) {
	if v := os.Getenv("CLASHDOCK_LANG"); v == "zh" {
		i18n.SetLang(i18n.ZH)
		return
	} else if v == "en" {
		i18n.SetLang(i18n.EN)
		return
	}
	if !settingsReadable(p) {
		return
	}
	if config.Str(config.Load(p), "language") == "zh" {
		i18n.SetLang(i18n.ZH)
	}
}

// setupLogging customize.json 的 enable_log=true 时，把 execx 的输出额外写入
// 日志文件（超限自动裁剪旧内容，见 internal/execx/log.go）。trusted=false 表示
// 状态目录未确认可安全写入（见 prepareState），不写日志。
func setupLogging(p paths.Paths, trusted bool) {
	if !trusted || !config.Bool(config.Load(p), "enable_log") {
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
	os.Exit(flowExitCode(err))
}

// flowExitCode 流程错误 → 退出码：取消不算失败，其余错误打印并返回 1。
func flowExitCode(err error) int {
	if err != nil && !errors.Is(err, errs.ErrCancelled) {
		execx.Error(err.Error())
		return 1
	}
	return 0
}

// reportFlowError 交互菜单里的流程错误：打印后回到菜单，取消不算错误。
func reportFlowError(err error) {
	flowExitCode(err)
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
		initFlow: func(p paths.Paths) error {
			if err := flows.RequireRoot(p, resumeInit); err != nil {
				return err
			}
			return flows.Init(p)
		},
		reportError: execx.Error,
	}
}

// maybeOfferRedeploy 每次进入交互式主菜单都检查一次：服务单元来自旧版本（如仍以 root
// 运行 mihomo，sysd.UnitOutdated），或内核/geo 数据在 state/ 侧被下载更新过、运行中的
// 服务还没有跟着重新部署（sysd.AssetsStale）。是则询问是否现在重新部署；否则不打扰。
func maybeOfferRedeploy(p paths.Paths) {
	if !sysd.IsInstalled(sysd.DefaultName) {
		return
	}
	var prompt string
	switch {
	case sysd.UnitOutdated(sysd.DefaultName):
		prompt = i18n.T("服务单元来自旧版本（例如 mihomo 仍以 root 运行），是否现在重新部署并重启？")
	case sysd.AssetsStale(p):
		prompt = i18n.T("检测到本地内核/geo 数据已更新，但运行中的服务尚未使用最新版本，是否现在重启应用？")
	default:
		return
	}
	ok, err := tui.Confirm(prompt, true)
	if err != nil || !ok {
		return
	}
	if err := flows.RequireRoot(p, resumeRedeploy); err != nil {
		reportFlowError(err)
		return
	}
	redeployAssets(p)
}

// redeployAssets 按当前版本重新部署运行时与服务单元（含 state/ 侧最新的内核/geo 数据）并重启。
func redeployAssets(p paths.Paths) {
	if err := sysd.Install(p, sysd.DefaultName, true); err != nil {
		execx.Error(err.Error())
		return
	}
	execx.Ok(i18n.T("已重新部署并重启服务。"))
}

// warnIfDebNeedsRepair dpkg 管理的安装：旧版自更新把可执行文件改成了符号链接时提示
// 恢复（需要联网下载 .deb）。已由 dpkg 重新接管时的版本目录清理见 prepareState。
func warnIfDebNeedsRepair(p paths.Paths) {
	if selfupdate.NeedsDebRepair(p) {
		execx.Warn(i18n.T("可执行文件已被旧版自更新改成符号链接，apt 升级会与之冲突；可在「工具 → 更新 → clashdock 自身」恢复为 dpkg 管理。"))
	}
}

// repairCompanionUnits 旧版本可能把 watchdog / 周更单元的入口写成自更新版本目录里的
// 文件（随旧版本清理失效），启动时顺带改回稳定路径；失败只提示，不影响后续操作。
func repairCompanionUnits(p paths.Paths) {
	if err := sysd.RepairCompanionUnits(p); err != nil {
		execx.Warn(i18n.T("修复伴生单元入口失败：") + err.Error())
	}
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

// 提权重启后恢复的非菜单操作（菜单项以 menuEntry.id 恢复）。
const (
	resumeInit     = "init"
	resumeRedeploy = "redeploy"
)

// menuEntry 主菜单项。needsRoot：系统模式下会写状态目录，普通用户进入前经
// flows.RequireRoot 提权，root 进程随后以 id 直接回到这一项。
type menuEntry struct {
	id        string
	label     func() string
	needsRoot bool
	run       func(paths.Paths) error
}

// mainMenu 顺序按常用程度排列：运行时管理/配置变更/网络测试等日常操作在前，
// 暂停/启动服务其次，卸载这类低频/破坏性操作放最后。工具只读（其中的更新自行提权），
// 服务开关只在 systemctl 时按需 sudo，都不必为进入菜单而提权。
func mainMenu() []menuEntry {
	return []menuEntry{
		{"runtime", func() string { return i18n.T("运行时管理") }, true, flows.ModifyRuntime},
		{"config", func() string { return i18n.T("配置变更") }, true, flows.ModifyConfig},
		{"tools", func() string { return i18n.T("工具") }, false,
			func(p paths.Paths) error { return flows.ToolsMenu(p, version) }},
		{"service", switchLabel, false, flows.ServiceToggleFlow},
		{"language", func() string { return i18n.T("语言 / Language") }, true, flows.PickLanguage},
		{"uninstall", func() string { return i18n.T("卸载") }, true, flows.Uninstall},
	}
}

func runMenuEntry(p paths.Paths, e menuEntry) error {
	if e.needsRoot {
		if err := flows.RequireRoot(p, e.id); err != nil {
			return err
		}
	}
	return e.run(p)
}

// startupChecks 打开主菜单前的一次性检查（提权重启后不再重复）。
func startupChecks(p paths.Paths) {
	// 配置文件里没设置过语言（且未用 CLASHDOCK_LANG 指定）才弹语言选择。
	if err := flows.EnsureLanguage(p); err != nil {
		execx.Error(err.Error())
	}
	// 服务单元不存在时询问是否现在进入初始化。已停止但仍注册的服务不算首次运行；
	// 此处只看单元文件是否存在。
	maybeOfferFirstRunInit(p, defaultFirstRunDeps())
	warnIfDebNeedsRepair(p)
	repairCompanionUnits(p)
	maybeOfferRedeploy(p)
}

// resumeAfterElevation 提权重启后直接执行 resume 指向的操作，返回主菜单初始光标。
func resumeAfterElevation(p paths.Paths, menu []menuEntry, resume string) int {
	for i, e := range menu {
		if e.id == resume {
			reportFlowError(runMenuEntry(p, e))
			return i
		}
	}
	if err := flows.RequireRoot(p, resume); err != nil {
		reportFlowError(err)
		return 0
	}
	switch resume {
	case resumeInit:
		reportFlowError(flows.Init(p))
	case resumeRedeploy:
		redeployAssets(p)
	case flows.ResumeUpdate:
		reportFlowError(flows.UpdateMenu(p, version))
		return menuIndex(menu, "tools")
	}
	return 0
}

func menuIndex(menu []menuEntry, id string) int {
	for i, e := range menu {
		if e.id == id {
			return i
		}
	}
	return 0
}

// interactive 交互式主菜单。resume 非空表示刚经 flows.RequireRoot 提权重启。
func interactive(p paths.Paths, resume string) int {
	menu := mainMenu()
	idx := 0
	if resume == "" {
		startupChecks(p)
	} else {
		idx = resumeAfterElevation(p, menu, resume)
	}
	for {
		options := make([]string, len(menu))
		for i, e := range menu {
			options[i] = e.label()
		}
		i, err := tui.Select(i18n.T("mihomo 部署系统"), options, tui.SelectOpts{BackLabel: i18n.T("退出"), Initial: idx})
		if err != nil {
			fmt.Println(i18n.T("再见。"))
			return 0
		}
		idx = i
		reportFlowError(runMenuEntry(p, menu[i]))
	}
}

// runUpdate 非交互全量更新（每周定时器的执行目标）：内核+geo+UI 强制更新 →
// 完整重新部署运行时并重启（SyncAndRestart 只同步 config.yaml，不会重新拷贝
// 新下载的二进制/geo 文件，这里必须用完整 Install 才能让更新真正生效）。
func runUpdate(p paths.Paths) int {
	repairCompanionUnits(p)
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

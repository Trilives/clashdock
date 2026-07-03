// clashdock：在 Linux 上交互式部署 / 管理 mihomo（Clash.Meta）。
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/errs"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/flows"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/tui"
)

// version 由构建注入：-ldflags "-X main.version=..."。
var version = "dev"

func usageText() string {
	return i18n.T("用法: clashdock [init|modify|nettest|uninstall|update|pause|resume|version]\n不带参数则进入交互式主菜单。")
}

func main() {
	p := paths.Detect()
	setupLanguage(p)
	args := os.Args[1:]
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
	case "webui-serve":
		os.Exit(serveUI(args[1:]))
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

func interactive(p paths.Paths) int {
	// 服务不存在（含已停止但仍注册的情况，IsInstalled 只看单元文件是否存在）时
	// 自动进入初始化；主菜单不再需要单独的「初始化」入口。
	if !sysd.IsInstalled(sysd.DefaultName) {
		if err := flows.Init(p); err != nil && !errors.Is(err, errs.ErrCancelled) {
			execx.Error(err.Error())
		}
	}
	idx := 0
	for {
		// 顺序按常用程度排列：暂停/启动与切节点等日常操作在前，卸载这类
		// 低频/破坏性操作放最后。
		options := []string{
			switchLabel(),
			i18n.T("运行时管理（无需重启）"),
			i18n.T("配置变更（需重启生效）"),
			i18n.T("网络测试"),
			i18n.T("语言 / Language"),
			i18n.T("卸载所有服务"),
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
			aerr = flows.ServiceToggleFlow(p)
		case 1:
			aerr = flows.ModifyRuntime(p)
		case 2:
			aerr = flows.ModifyConfig(p)
		case 3:
			aerr = flows.Nettest()
		case 4:
			aerr = languageMenu(p)
		case 5:
			aerr = flows.Uninstall(p)
		}
		if aerr != nil && !errors.Is(aerr, errs.ErrCancelled) {
			execx.Error(aerr.Error())
		}
	}
}

// languageMenu 语言 / Language：标题与选项本身直接写死双语字面量（不经过
// i18n.T），因为这是语言选择器自身——用户在任何当前语言状态下都要能看懂两个
// 选项各自对应哪种语言。
func languageMenu(p paths.Paths) error {
	current := 0
	if i18n.Current() == i18n.ZH {
		current = 1
	}
	i, err := tui.Select("Language / 语言", []string{"English", "中文"}, tui.SelectOpts{Initial: current})
	if err != nil {
		return nil // esc/^R 取消，语言不变
	}
	lang := i18n.EN
	if i == 1 {
		lang = i18n.ZH
	}
	cfg := config.Load(p)
	cfg["language"] = string(lang)
	if err := config.Save(p, cfg); err != nil {
		return err
	}
	i18n.SetLang(lang)
	return nil
}

// runUpdate 非交互全量更新（每周定时器的执行目标）：
// 内核+geo+UI 强制更新 → 独立面板重新暂存 → 服务同步重启。
func runUpdate(p paths.Paths) int {
	if _, err := kernel.DownloadAll(p, kernel.Options{Force: true, WithUI: true}); err != nil {
		execx.Error(err.Error())
		return 1
	}
	cfg := config.Load(p)
	if err := sysd.RefreshWebUI(p, config.Int(cfg, "webui_port"), config.Bool(cfg, "lan_panel")); err != nil {
		execx.Warn(i18n.T("独立面板刷新失败：") + err.Error())
	}
	if sysd.IsInstalled(sysd.DefaultName) {
		if err := sysd.SyncAndRestart(p, sysd.DefaultName); err != nil {
			execx.Error(err.Error())
			return 1
		}
	}
	return 0
}

// serveUI 极简静态文件服务（mihomo-webui.service 的执行目标，
// 取代 Python 版的 python3 -m http.server）。
func serveUI(args []string) int {
	fs := flag.NewFlagSet("webui-serve", flag.ContinueOnError)
	port := fs.Int("port", sysd.DefaultWebUIPort, i18n.T("监听端口"))
	bind := fs.String("bind", "127.0.0.1", i18n.T("绑定地址"))
	dir := fs.String("dir", "", i18n.T("静态文件目录"))
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dir == "" {
		execx.Error(i18n.T("webui-serve 需要 --dir"))
		return 2
	}
	addr := fmt.Sprintf("%s:%d", *bind, *port)
	execx.Info(fmt.Sprintf(i18n.T("静态面板服务: http://%s/ ← %s"), addr, *dir))
	if err := http.ListenAndServe(addr, http.FileServer(http.Dir(*dir))); err != nil {
		execx.Error(err.Error())
		return 1
	}
	return 0
}

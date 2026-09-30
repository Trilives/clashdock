package main

import (
	"fmt"
	"os"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/flows"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/runtimesvc"
	"github.com/Trilives/clashdock/internal/usermode"
)

// userEntryArgs 显式进入用户模式的参数（run / --portable 为旧便携模式的入口名，保留兼容）。
var userEntryArgs = map[string]bool{"user": true, "run": true, "--portable": true}

// userModeCommands 在用户模式下改为作用于用户服务的子命令。init / healthcheck 只属于
// 完整模式（系统服务），即使检测到用户模式也照常走完整模式。
var userModeCommands = map[string]bool{
	"modify": true, "nettest": true, "pause": true, "resume": true, "update": true, "uninstall": true,
	"version": true, "--version": true, "-v": true, "-h": true, "--help": true, "help": true,
}

// userModeApplies 是否进入用户模式分发。
func userModeApplies(args []string, info usermode.Info) bool {
	if len(args) > 0 && userEntryArgs[args[0]] {
		return true
	}
	if info.Mode != usermode.ModeUser {
		return false
	}
	return len(args) == 0 || userModeCommands[args[0]]
}

// userPaths 用户模式的路径：数据目录默认 ~/.local/share/clashdock（CLASHDOCK_HOME 仍可覆盖，
// 并随环境传给交接后的已安装本体）。
func userPaths() paths.Paths {
	if os.Getenv("CLASHDOCK_HOME") == "" {
		os.Setenv("CLASHDOCK_HOME", usermode.StateDir())
	}
	p := paths.Detect()
	p.UserMode = true
	return p
}

// runUserMode 用户模式子命令分发，返回进程退出码。
func runUserMode(args []string, info usermode.Info) int {
	p := userPaths()
	setupLanguage(p)
	setupLogging(p)
	if len(args) == 0 || userEntryArgs[args[0]] {
		return flowExitCode(flows.UserModeRun(p, info, version))
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println("clashdock " + version)
	case "-h", "--help", "help":
		fmt.Println(usageText())
	case "modify":
		return flowExitCode(flows.UserModifyConfig(p))
	case "nettest":
		return flowExitCode(flows.Nettest(config.ProxyPort(config.Load(p))))
	case "pause":
		return flowExitCode(userPause(p))
	case "resume":
		return flowExitCode(userResume(p))
	case "update":
		return runUserUpdate(p)
	case "uninstall":
		_, err := flows.UserUninstall(p)
		return flowExitCode(err)
	}
	return 0
}

func userPause(p paths.Paths) error {
	if err := runtimesvc.For(p).Pause(); err != nil {
		return err
	}
	execx.Ok(i18n.T("用户服务已停止。"))
	return nil
}

func userResume(p paths.Paths) error {
	svc := runtimesvc.For(p)
	if !svc.Installed() {
		return fmt.Errorf("%s", i18n.T("用户服务尚未部署，请运行 clashdock user 完成部署。"))
	}
	if err := svc.Resume(); err != nil {
		return err
	}
	execx.Ok(i18n.T("用户服务已启动。"))
	return nil
}

// runUserUpdate 非交互全量更新（内核 + geo + Web UI）后重新部署用户服务运行时并重启。
// 服务已在运行时优先走本机代理下载。
func runUserUpdate(p paths.Paths) int {
	svc := runtimesvc.For(p)
	opts := kernel.Options{Force: true, WithUI: true, LocalProxyFirst: svc.Active()}
	if _, err := kernel.DownloadAll(p, opts); err != nil {
		execx.Error(err.Error())
		return 1
	}
	if svc.Installed() {
		if err := svc.Redeploy(); err != nil {
			execx.Error(err.Error())
			return 1
		}
		execx.Ok(i18n.T("已应用最新内核/geo 数据并重启服务。"))
	}
	return 0
}

package main

import (
	"errors"
	"os"
	"strings"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/flows"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/selfupdate"
)

// 系统模式的权限模型：状态目录只有 root 可写，root 运行的服务与定时器因此只读写 root
// 属主的文件；属组是管理员的私有组，管理员不经 sudo 也能读取设置、配置与日志。
// 只读操作不提权；写状态的子命令在启动时经 sudo 提权一次，交互式主菜单则在进入写状态
// 的菜单项时才提权（flows.RequireRoot），不会每次打开都要求输入密码。

// stateCommands 必须以 root 运行的子命令：写状态目录或操作系统服务。
var stateCommands = map[string]bool{
	"init": true, "modify": true, "uninstall": true, "update": true,
	"pause": true, "resume": true,
}

// stateReaders 读状态的入口：以 root 运行时（watchdog 的 healthcheck、sudo nettest）
// 先确认目录树已收归 root。
var stateReaders = map[string]bool{"nettest": true, "healthcheck": true}

// needsElevation 子命令启动前是否要经 sudo 提权（主菜单见 menuNeedsRootAtStart）。
// healthcheck 由 root 的 watchdog 单元运行，从不自行提权；nettest 只在读不到设置时提权。
func needsElevation(cmd string, isRoot, settingsReadable bool) bool {
	switch {
	case isRoot:
		return false
	case stateCommands[cmd]:
		return true
	case cmd == "nettest":
		return !settingsReadable
	}
	return false
}

// startState 普通用户打开主菜单时的状态目录情况。
type startState struct {
	stateExists      bool
	settingsReadable bool
	languageSet      bool
}

func probeStartState(p paths.Paths) startState {
	_, err := os.Stat(p.State)
	st := startState{stateExists: err == nil, settingsReadable: settingsReadable(p)}
	st.languageSet = strings.TrimSpace(os.Getenv("CLASHDOCK_LANG")) != "" ||
		(st.settingsReadable && config.LanguageConfigured(p))
	return st
}

// menuNeedsRootAtStart 主菜单只在首次运行（数据目录或界面语言尚未就绪，接下来必然
// 写入）或读不到设置（状态目录属组不是本人，见 flows.EnsureStateRoot）时启动即提权。
func menuNeedsRootAtStart(st startState) bool {
	return !st.stateExists || !st.settingsReadable || !st.languageSet
}

// settingsReadable 当前身份能否读取 customize.json；文件不存在也算可读（取默认值）。
func settingsReadable(p paths.Paths) bool {
	f, err := os.Open(p.CustomizeFile)
	if err == nil {
		f.Close()
		return true
	}
	return errors.Is(err, os.ErrNotExist)
}

// splitResume 去掉 flows.RequireRoot 附带的 --resume=<菜单项>，返回其余参数与菜单项。
func splitResume(args []string) ([]string, string) {
	if len(args) > 0 && strings.HasPrefix(args[0], flows.ResumeFlag) {
		return args[1:], strings.TrimPrefix(args[0], flows.ResumeFlag)
	}
	return args, ""
}

// elevate 经 sudo 以 root 重新执行当前命令，成功时不返回。
func elevate(args []string) error {
	exe, err := execx.StableExecutablePath()
	if err != nil {
		return err
	}
	execx.Info(i18n.T("系统模式由 root 管理，正在通过 sudo 重新启动 clashdock…"))
	return execx.ReexecAsRoot(exe, args)
}

// prepareState 按入口准备状态目录，返回 root 能否放心写入其中（决定是否写日志）。
// root 运行的入口先把整棵树收归 root：旧版本把目录交给了调用用户，升级后首个
// root 入口（含每 2 分钟一次的 watchdog）即完成迁移。普通用户只读，不做任何准备。
func prepareState(p paths.Paths, cmd string) bool {
	if !execx.IsRoot() {
		return false
	}
	if cmd == "" || stateCommands[cmd] {
		exitIfErr(flows.EnsureStateRoot(p))
		// dpkg 已重新接管可执行文件时清理旧版自更新的版本目录（需 root）。
		if err := selfupdate.RetireLegacyVersions(p); err != nil {
			execx.Warn(i18n.T("清理旧版自更新目录失败：") + err.Error())
		}
		return true
	}
	if !stateReaders[cmd] {
		return false
	}
	if err := flows.SecureSystemState(p); err != nil {
		execx.Warn(err.Error())
		return false
	}
	return true
}

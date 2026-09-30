// Package usermode 用户模式（免 root）：以当前用户身份部署 mihomo，并维护一个
// 用户级服务（systemctl --user；没有 systemd 用户实例时退化为脱离终端的后台进程）。
// 不写系统路径、不提权；clashdock 只做管理，不需要前台驻留。
//
// 布局（均在用户主目录下，见 layout.go）：
//
//	~/.local/bin/clashdock                        本体（从便携包安装，可自更新）
//	~/.local/share/clashdock                       数据（订阅 / 定制层 / 内核 / geo）
//	~/.local/share/clashdock/runtime               mihomo 工作目录（-d）
//	~/.config/systemd/user/clashdock-mihomo.service 用户级服务单元
//
// 依赖方向：usermode 属领域包，可被 flows / runtimesvc 调用；本包不反向依赖
// flows / sysd，「是否已安装系统服务」由调用方（cmd/clashdock）查好后以布尔传入 Detect。
package usermode

import (
	"os"
	"path/filepath"
	"strings"
)

// Mode 运行模式。
type Mode int

const (
	// ModeService 完整服务模式：注册 systemd 系统单元、写系统路径（默认）。
	ModeService Mode = iota
	// ModeUser 用户模式：免 root，维护用户级服务。
	ModeUser
)

// Info 模式判定结果。
type Info struct {
	Mode Mode
	// ExecPath 当前可执行文件的解析路径。
	ExecPath string
	// DepsDir 解压目录里的 deps/ 兄弟目录（含 mihomo 与 rules/），无则为空。
	DepsDir string
	// Kernel deps/mihomo 的路径，无则为空。
	Kernel string
}

// FromPackage 当前二进制是否从解压的便携包目录启动（旁有 deps/mihomo）。
func (i Info) FromPackage() bool { return i.Kernel != "" }

// launch 判定所需的启动上下文（纯数据，便于测试）。
type launch struct {
	exec          string
	kernel        string
	rootInstalled bool
	userInstalled bool
	envMode       string
	userBin       string
	userState     string
}

// Detect 判定运行模式。rootInstalled 为调用方查得的「系统主服务是否已注册」。
func Detect(rootInstalled bool) Info {
	exec := resolveExec()
	depsDir, kernel := siblingDeps(exec)
	state := StateDir()
	return Info{
		Mode: classify(launch{
			exec:          exec,
			kernel:        kernel,
			rootInstalled: rootInstalled,
			userInstalled: fileExists(UnitPath()) || fileExists(detachedMarker(state)),
			envMode:       os.Getenv("CLASHDOCK_MODE"),
			userBin:       BinPath(),
			userState:     state,
		}),
		ExecPath: exec,
		DepsDir:  depsDir,
		Kernel:   kernel,
	}
}

// classify 纯判定逻辑。核心原则：由**启动上下文**决定模式。优先级（先命中先返回）：
//  1. 环境变量 CLASHDOCK_MODE=user|portable|service 显式覆盖（portable 为旧名）；
//  2. 可执行文件旁存在 deps/mihomo → User（解压的便携包，用于安装/管理用户模式）；
//  3. 可执行文件就是用户模式安装的本体（~/.local/bin/clashdock 或其自更新托管版本）→ User
//     ——即使管理员另装了系统服务，用户运行自己的本体也应管理自己的用户服务；
//  4. 已注册系统服务 → Service（去管理现有完整安装）；
//  5. 已部署用户服务 → User；
//  6. 兜底 → Service。
func classify(l launch) Mode {
	switch strings.ToLower(strings.TrimSpace(l.envMode)) {
	case "user", "portable":
		return ModeUser
	case "service":
		return ModeService
	}
	switch {
	case l.kernel != "":
		return ModeUser
	case l.exec != "" && (l.exec == l.userBin || isUnder(l.exec, l.userState)):
		return ModeUser
	case l.rootInstalled:
		return ModeService
	case l.userInstalled:
		return ModeUser
	}
	return ModeService
}

func isUnder(path, dir string) bool {
	return dir != "" && strings.HasPrefix(path, strings.TrimRight(dir, "/")+"/")
}

// resolveExec 解析当前可执行文件的真实路径（跟随符号链接），失败回退 os.Args[0]。
func resolveExec() string {
	exe, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

// siblingDeps 探测可执行文件同目录下的 deps/ 兄弟目录（便携包解压布局）。
// 返回 (depsDir, kernelPath)；不存在时返回空串。
func siblingDeps(exec string) (string, string) {
	depsDir := filepath.Join(filepath.Dir(exec), "deps")
	kernel := filepath.Join(depsDir, "mihomo")
	if st, err := os.Stat(kernel); err == nil && !st.IsDir() {
		return depsDir, kernel
	}
	return "", ""
}

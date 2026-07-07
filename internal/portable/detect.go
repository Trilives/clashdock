// Package portable 便携/轻量模式：从解压后的便携包目录直接运行 mihomo，不安装
// systemd 服务、不写系统路径、不提权。内核与基础规则取自解压目录里的 deps/ 兄弟
// 目录，工作数据落在当前目录下的 ./clashdock-data。
//
// 与「完整服务模式」的区别在于：这里 clashdock 停在前台充当监护进程，mihomo 作为
// 子进程随其生命周期存活（见 supervise.go），退出即停；节点切换等管理经 Clash API
// 完成，不依赖 systemd。
//
// 依赖方向：portable 属领域包，可被 flows 调用；本包不反向依赖 flows / sysd，
// 「是否已安装系统服务」由调用方（cmd/clashdock）查好后以布尔传入 Detect。
package portable

import (
	"os"
	"path/filepath"
	"strings"
)

// Mode 运行模式。
type Mode int

const (
	// Service 完整服务模式：注册 systemd 单元、写系统路径（默认）。
	Service Mode = iota
	// Portable 便携/轻量模式：前台直接跑内核，不装服务。
	Portable
)

// DefaultWorkdirName 便携模式在当前目录下创建的工作目录名（稳定命名，便于复用
// 上次的订阅，而非每次随机临时目录）。
const DefaultWorkdirName = "clashdock-data"

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

// systemBinDirs 被视为「已安装到系统」的可执行文件目录前缀。
var systemBinDirs = []string{"/usr/bin/", "/usr/local/bin/", "/bin/", "/sbin/", "/usr/sbin/"}

// Detect 判定运行模式。installed 为调用方查得的「主服务是否已注册」。
func Detect(installed bool) Info {
	exec := resolveExec()
	depsDir, kernel := siblingDeps(exec)
	return Info{
		Mode:     classify(exec, kernel, installed, os.Getenv("CLASHDOCK_MODE")),
		ExecPath: exec,
		DepsDir:  depsDir,
		Kernel:   kernel,
	}
}

// classify 纯判定逻辑（便于测试）。核心原则：由**启动上下文**决定模式——从解压便携
// 包目录启动（旁有 deps/mihomo）即轻量模式，无论系统是否已装过服务。优先级（先命中
// 先返回）：
//  1. 环境变量 CLASHDOCK_MODE=portable|service 显式覆盖；
//  2. 可执行文件旁存在 deps/mihomo → Portable（解压的便携包二进制，唯一具备此特征，
//     即便机器上另有已安装服务，也按「用户此刻运行的是便携包」处理）；
//  3. 已注册系统服务 → Service（去管理现有安装）；
//  4. 可执行文件位于系统路径（/usr/bin 等）→ Service；
//  5. 兜底 → Service。
func classify(exec, kernel string, installed bool, envMode string) Mode {
	switch strings.ToLower(strings.TrimSpace(envMode)) {
	case "portable":
		return Portable
	case "service":
		return Service
	}
	if kernel != "" {
		return Portable
	}
	if installed {
		return Service
	}
	if inSystemBinDir(exec) {
		return Service
	}
	return Service
}

// DefaultWorkdir 便携模式默认工作目录：当前工作目录下的 ./clashdock-data。
func DefaultWorkdir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return DefaultWorkdirName
	}
	return filepath.Join(cwd, DefaultWorkdirName)
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
	dir := filepath.Dir(exec)
	depsDir := filepath.Join(dir, "deps")
	kernel := filepath.Join(depsDir, "mihomo")
	if st, err := os.Stat(kernel); err == nil && !st.IsDir() {
		return depsDir, kernel
	}
	return "", ""
}

// inSystemBinDir 可执行文件是否位于系统 bin 目录。
func inSystemBinDir(exec string) bool {
	for _, d := range systemBinDirs {
		if strings.HasPrefix(exec, d) {
			return true
		}
	}
	return false
}

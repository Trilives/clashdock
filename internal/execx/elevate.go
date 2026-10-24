package execx

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Trilives/clashdock/internal/i18n"
)

// elevationEnv 提权重执行时保留的环境变量（见 docs/COMMANDS.md「环境变量」）。sudo 默认
// 清空环境；经 --preserve-env 只传变量名，值不出现在命令行里（GITHUB_TOKEN 等不会被
// 其他用户从 ps 看到），且仅在已设置时请求，未设置时不需要 sudoers 允许保留环境。
//
// 不含 CLASHDOCK_HOME：它决定 root 收归（SecureTree）与执行内核的目录，若随提权传入，
// 能改管理员 shell 配置的进程就能让 root 接管任意目录、执行其中的文件。root 只用固定
// 状态目录，除非 root 自己设置该变量。
var elevationEnv = []string{
	"CLASHDOCK_MODE", "CLASHDOCK_LANG", "DOWNLOAD_PROXY",
	"GITHUB_TOKEN", "GH_TOKEN", "MIHOMO_NO_PROXY", "NO_COLOR",
}

// ReexecAsRoot 经 sudo 以 root 重新执行 exe args…，成功时不返回。exe 应是用户调用的
// 路径（StableExecutablePath，如 /usr/bin/clashdock），单元里记录的入口才保持稳定。
func ReexecAsRoot(exe string, args []string) error {
	if os.Getenv("SUDO_UID") != "" {
		// 已经过一次 sudo 仍不是 root（如 sudoers 的 runas_default 指向他人）：不再循环提权。
		return fmt.Errorf("%s", i18n.T("经 sudo 重新执行后仍不是 root，请检查 sudoers 配置或直接以 root 运行"))
	}
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("%s", i18n.T("需要管理员权限，但未找到 sudo，请改用 root 运行"))
	}
	return syscall.Exec(sudo, sudoArgv(sudo, exe, args, os.Getenv), os.Environ())
}

// sudoArgv 组装 `sudo [--preserve-env=A,B] -- exe args…`。
func sudoArgv(sudo, exe string, args []string, getenv func(string) string) []string {
	argv := []string{sudo}
	var keep []string
	for _, key := range elevationEnv {
		if getenv(key) != "" {
			keep = append(keep, key)
		}
	}
	if len(keep) > 0 {
		argv = append(argv, "--preserve-env="+strings.Join(keep, ","))
	}
	argv = append(argv, "--", exe)
	return append(argv, args...)
}

// StableExecutablePath 用户调用的可执行文件路径（保留 /usr/bin/clashdock 这类稳定
// 符号链接）。os.Executable 在 Linux 上读 /proc/self/exe，返回解析到底的版本目录文件，
// 写进 systemd 单元后会随自更新清理旧版本而失效。
func StableExecutablePath() (string, error) {
	return stableExecutablePath(os.Args[0], exec.LookPath)
}

func stableExecutablePath(arg0 string, lookPath func(string) (string, error)) (string, error) {
	path := arg0
	if !strings.ContainsRune(path, os.PathSeparator) {
		found, err := lookPath(path)
		if err != nil {
			return "", err
		}
		path = found
	}
	return filepath.Abs(path)
}

// InvokingUserHome 发起操作的真实用户的家目录：经 sudo 运行时是 SUDO_USER 的家目录
// （sudo 可能把 HOME 换成 /root），否则是当前用户的。
func InvokingUserHome() string {
	if u := InvokingSudoUser(); u != nil {
		return u.HomeDir
	}
	home, _ := os.UserHomeDir()
	return home
}

// InvokingSudoUser root 代真实用户（SUDO_USER）操作时返回该用户，否则 nil。
func InvokingSudoUser() *user.User {
	name := os.Getenv("SUDO_USER")
	if name == "" || name == "root" || !IsRoot() {
		return nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return nil
	}
	return u
}

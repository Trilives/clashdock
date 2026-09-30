package usermode

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

// Backend 用户服务的后端类型。
type Backend string

const (
	// BackendSystemd systemctl --user 管理的用户级单元（开机/登录自启、异常自动重启）。
	BackendSystemd Backend = "systemd"
	// BackendDetached 没有 systemd 用户实例时的退化方案：脱离终端的后台进程
	// （不随 clashdock 退出，但不会开机自启，也不会异常自动重启）。
	BackendDetached Backend = "detached"
)

// Service 用户服务的统一操作面。所有方法都不需要 root。
type Service interface {
	Backend() Backend
	// Installed 服务是否已部署（单元文件 / 后台进程标记存在）。
	Installed() bool
	// Active 内核是否正在运行。
	Active() bool
	// Install 部署服务（写单元 / 标记，设为自启）并（重）启动；运行时须已铺好。
	Install() error
	Start() error
	Stop() error
	Restart() error
	// Remove 停止并删除服务（不动数据目录）。
	Remove() error
	// Logs 最近 lines 行内核日志。
	Logs(lines int) (string, error)
	// Describe 服务位置的简短说明（单元文件或 PID/日志文件），供界面展示。
	Describe() string
}

// runner 执行外部命令并返回合并输出（便于测试注入）。
type runner func(name string, args ...string) (string, error)

func execRunner(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// NewService 选择用户服务后端：已部署的后端优先（保持一致）；都没部署时，有可用的
// systemd 用户实例就用它，否则退化为后台进程。
func NewService(p paths.Paths) Service {
	sd := newSystemdService(p, execRunner)
	if sd.Installed() {
		return sd
	}
	dt := newDetachedService(p)
	if dt.Installed() {
		return dt
	}
	if SystemdUserAvailable() {
		return sd
	}
	return dt
}

// SystemdUserAvailable 当前会话能否连上 systemd 用户实例（容器 / 无 logind 的 SSH
// 会话等场景下连不上）。
func SystemdUserAvailable() bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	return exec.Command("systemctl", "--user", "show-environment").Run() == nil
}

// Validate 用 `mihomo -t` 预校验运行时配置与 geo 数据，避免起一个必然崩溃的内核。
func Validate(p paths.Paths) error {
	out, err := exec.Command(p.MihomoBin, "-t", "-d", RuntimeDir(p), "-f", RuntimeConfig(p)).CombinedOutput()
	if err != nil {
		return fmt.Errorf(i18n.T("配置校验失败：%s"), strings.TrimSpace(string(out)))
	}
	return nil
}

// preflight 部署前检查：内核与生效配置必须就位（geo 由 StageRuntime 检查）。
func preflight(p paths.Paths) error {
	if _, err := os.Stat(p.MihomoBin); err != nil {
		return fmt.Errorf("%s", i18n.T("未找到 mihomo 内核；请从便携包目录运行 ./clashdock 接管内核，或在「工具 → 更新 → 内核」下载。"))
	}
	if _, err := os.Stat(p.ConfigFile); err != nil {
		return fmt.Errorf("%s", i18n.T("未找到生效配置 config.yaml，请先添加订阅"))
	}
	return nil
}

// prepare 铺运行时并校验：重启 / 部署前的公共步骤，保证服务只会用通过校验的最新配置启动。
func prepare(p paths.Paths) error {
	if err := preflight(p); err != nil {
		return err
	}
	if err := StageRuntime(p); err != nil {
		return err
	}
	return Validate(p)
}

// Deploy 铺运行时 → 校验 → 部署并启动服务（首次部署或完整重建用）。
func Deploy(p paths.Paths, svc Service) error {
	if err := prepare(p); err != nil {
		return err
	}
	return svc.Install()
}

// SyncAndRestart 重新铺运行时（配置 + geo + UI）→ 校验 → 重启服务。用户模式的
// 运行时只是普通文件复制，开销很小，因此配置同步与资源重新部署共用这一条路径。
// 服务未部署时静默跳过（只落盘，不启动）。
func SyncAndRestart(p paths.Paths, svc Service) error {
	if !svc.Installed() {
		return nil
	}
	if err := prepare(p); err != nil {
		return err
	}
	return svc.Restart()
}

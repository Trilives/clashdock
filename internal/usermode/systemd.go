package usermode

import (
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

//go:embed assets/user.service.tmpl
var userUnitTmpl string

// systemdService systemctl --user 管理的用户级单元。内核直接运行 state/bin/mihomo
// （内核更新走临时文件 + 改名，不会与运行中的进程冲突），工作目录为 state/runtime。
type systemdService struct {
	unitPath   string
	unit       string
	bin        string
	runtimeDir string
	config     string
	run        runner
	sleep      func(time.Duration)
}

func newSystemdService(p paths.Paths, run runner) *systemdService {
	return &systemdService{
		unitPath:   UnitPath(),
		unit:       UnitName + ".service",
		bin:        p.MihomoBin,
		runtimeDir: RuntimeDir(p),
		config:     RuntimeConfig(p),
		run:        run,
		sleep:      time.Sleep,
	}
}

func (s *systemdService) Backend() Backend { return BackendSystemd }
func (s *systemdService) Describe() string { return s.unitPath }
func (s *systemdService) Installed() bool  { return fileExists(s.unitPath) }

func (s *systemdService) systemctl(args ...string) (string, error) {
	return s.run("systemctl", append([]string{"--user"}, args...)...)
}

func (s *systemdService) Active() bool {
	out, _ := s.systemctl("is-active", s.unit)
	return strings.TrimSpace(out) == "active"
}

// Install 写单元 → daemon-reload → enable → restart（已在跑则套用新单元与配置）。
func (s *systemdService) Install() error {
	text, err := renderUserUnit(s.runtimeDir, s.bin, s.config)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(UnitDir(), 0o755); err != nil {
		return fmt.Errorf("create unit dir: %w", err)
	}
	if err := writeFileAtomic(s.unitPath, []byte(text), 0o644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", s.unit}} {
		if _, err := s.systemctl(args...); err != nil {
			return err
		}
	}
	return s.Restart()
}

func (s *systemdService) Stop() error { _, err := s.systemctl("stop", s.unit); return err }

func (s *systemdService) Start() error {
	if _, err := s.systemctl("start", s.unit); err != nil {
		return err
	}
	return s.confirmStarted()
}

func (s *systemdService) Restart() error {
	if _, err := s.systemctl("restart", s.unit); err != nil {
		return err
	}
	return s.confirmStarted()
}

// confirmStarted Type=simple 的 start/restart 在进程拉起后就返回；稍等再确认仍在运行，
// 内核立即退出（端口冲突等）时带上日志末尾报错，而不是报告「已启动」。
func (s *systemdService) confirmStarted() error {
	s.sleep(startSettle)
	if s.Active() {
		return nil
	}
	logs, _ := s.Logs(failureLogLines)
	return fmt.Errorf("%s\n%s", i18n.T("内核启动后立即退出，最近日志："), logs)
}

// Remove 停止 / 禁用 / 删除单元。连不上用户实例等导致 stop 失败时不删单元（否则内核
// 还在跑、单元却没了，clashdock 再也管不到它）；单元本就未加载则视为已停止。
func (s *systemdService) Remove() error {
	if out, err := s.systemctl("stop", s.unit); err != nil && !strings.Contains(out, "not loaded") {
		return fmt.Errorf(i18n.T("停止用户服务失败，未删除单元：%w"), err)
	}
	s.systemctl("disable", s.unit)
	if err := os.Remove(s.unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit: %w", err)
	}
	s.systemctl("daemon-reload")
	s.systemctl("reset-failed", s.unit)
	return nil
}

// Logs 内核日志在用户 journal 里；部分系统（易失性 journal）用户 journal 不单独
// 存储，退而按 _SYSTEMD_USER_UNIT 在可读的 journal 里查。
func (s *systemdService) Logs(lines int) (string, error) {
	n := strconv.Itoa(lines)
	out, err := s.run("journalctl", "--user", "-u", s.unit, "-n", n, "--no-pager", "-o", "cat")
	if err == nil && strings.TrimSpace(out) != "" && !strings.HasPrefix(strings.TrimSpace(out), "-- No entries") {
		return out, nil
	}
	return s.run("journalctl", "--user-unit", s.unit, "-n", n, "--no-pager", "-o", "cat")
}

// renderUserUnit 渲染用户单元。ExecStart 各参数加引号（路径可能含空格），% 转义为
// %%（systemd 说明符）；WorkingDirectory 取整行值，只需转义 %。
func renderUserUnit(runtimeDir, bin, config string) (string, error) {
	t, err := template.New("unit").Parse(userUnitTmpl)
	if err != nil {
		return "", err
	}
	execStart := strings.Join([]string{quoteArg(bin), "-d", quoteArg(runtimeDir), "-f", quoteArg(config)}, " ")
	var sb strings.Builder
	err = t.Execute(&sb, struct{ RuntimeDir, ExecStart string }{escapeSpecifiers(runtimeDir), execStart})
	return sb.String(), err
}

// escapeSpecifiers 转义 systemd 说明符（%），WorkingDirectory 等路径类设置只做这一项。
func escapeSpecifiers(s string) string { return strings.ReplaceAll(s, "%", "%%") }

// quoteArg ExecStart 参数：反斜杠 / 引号转义后加引号，另外转义说明符与 $ 变量展开。
func quoteArg(s string) string {
	s = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", "$$").Replace(s)
	return `"` + escapeSpecifiers(s) + `"`
}

// LingerEnabled 当前用户是否开启了 lingering（登出后用户服务继续运行、开机即启动）。
func LingerEnabled() bool {
	out, err := execRunner("loginctl", "show-user", strconv.Itoa(os.Getuid()), "-p", "Linger", "--value")
	return err == nil && strings.TrimSpace(out) == "yes"
}

// EnableLinger 为当前用户开启 lingering。多数发行版的 polkit 策略允许用户为自己开启；
// 不允许时返回错误，由调用方提示改用 `sudo loginctl enable-linger <user>`。
func EnableLinger() error {
	_, err := execRunner("loginctl", "enable-linger")
	return err
}

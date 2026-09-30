package usermode

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

const (
	// stopGrace 发出 SIGTERM 后等待内核优雅退出的时间，超时再 SIGKILL。
	stopGrace = 5 * time.Second
	// stopPoll 等待进程退出时的轮询间隔。
	stopPoll = 100 * time.Millisecond
	// startSettle 启动后等待多久再确认内核仍存活（配置错误 / 端口冲突通常立即退出）。
	startSettle = 500 * time.Millisecond
	// failureLogLines 启动失败时附带的日志行数。
	failureLogLines = 10
)

// detachedService 没有 systemd 用户实例时的后端：mihomo 以新会话（setsid）脱离
// 终端在后台运行，clashdock 退出后继续存活；stdout/stderr 写 runtime/mihomo.log，
// PID 记在 runtime/mihomo.pid。不提供开机自启与异常自动重启。
type detachedService struct {
	bin        string
	runtimeDir string
	config     string
	logPath    string
	pidPath    string
	marker     string
}

func newDetachedService(p paths.Paths) *detachedService {
	rt := RuntimeDir(p)
	return &detachedService{
		bin:        p.MihomoBin,
		runtimeDir: rt,
		config:     RuntimeConfig(p),
		logPath:    filepath.Join(rt, "mihomo.log"),
		pidPath:    filepath.Join(rt, "mihomo.pid"),
		marker:     detachedMarker(p.State),
	}
}

func (s *detachedService) Backend() Backend { return BackendDetached }
func (s *detachedService) Describe() string { return s.pidPath }
func (s *detachedService) Installed() bool  { return fileExists(s.marker) }

// Install 写部署标记并（重）启动内核。
func (s *detachedService) Install() error {
	if err := os.WriteFile(s.marker, []byte(string(BackendDetached)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write marker: %w", err)
	}
	return s.Restart()
}

// Start 启动后台内核；已在运行则直接返回。每次启动截断旧日志，避免无限增长。
func (s *detachedService) Start() error {
	if s.Active() {
		return nil
	}
	logf, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logf.Close()
	cmd := exec.Command(s.bin, "-d", s.runtimeDir, "-f", s.config)
	cmd.Dir = s.runtimeDir
	cmd.Stdout = logf
	cmd.Stderr = logf
	// 新会话：脱离控制终端，不随 clashdock 退出或终端挂断而收到 SIGHUP。
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start mihomo: %w", err)
	}
	pid := cmd.Process.Pid
	// clashdock 存活期间回收子进程，避免内核提前退出时留下僵尸让 Active 误判。
	go cmd.Wait()
	if err := os.WriteFile(s.pidPath, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		syscall.Kill(-pid, syscall.SIGKILL)
		return fmt.Errorf("write pid file: %w", err)
	}
	return s.confirmStarted()
}

// confirmStarted 启动后稍等再确认内核仍在运行；立即退出时带上日志末尾报错，
// 而不是报告「已启动」。
func (s *detachedService) confirmStarted() error {
	time.Sleep(startSettle)
	if s.Active() {
		return nil
	}
	os.Remove(s.pidPath)
	logs, _ := s.Logs(failureLogLines)
	return fmt.Errorf("%s\n%s", i18n.T("内核启动后立即退出，最近日志："), logs)
}

// Stop 终止内核进程组：先 SIGTERM，超时后 SIGKILL，并清理 PID 文件。
func (s *detachedService) Stop() error {
	pid, ok := s.livePID()
	if !ok {
		os.Remove(s.pidPath)
		return nil
	}
	// 负号 = 向整个进程组发信号（setsid 后 pgid == 内核 pid）。
	syscall.Kill(-pid, syscall.SIGTERM)
	if s.waitExit(pid, stopGrace) {
		os.Remove(s.pidPath)
		return nil
	}
	syscall.Kill(-pid, syscall.SIGKILL)
	if !s.waitExit(pid, stopGrace) {
		return fmt.Errorf(i18n.T("无法停止内核进程 %d"), pid)
	}
	os.Remove(s.pidPath)
	return nil
}

// waitExit 在 timeout 内轮询等待进程退出，返回是否已退出。
func (s *detachedService) waitExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !s.matches(pid) {
			return true
		}
		time.Sleep(stopPoll)
	}
	return !s.matches(pid)
}

func (s *detachedService) Restart() error {
	if err := s.Stop(); err != nil {
		return err
	}
	return s.Start()
}

func (s *detachedService) Active() bool {
	_, ok := s.livePID()
	return ok
}

// Remove 停止内核并删除部署标记（不动数据目录）。
func (s *detachedService) Remove() error {
	if err := s.Stop(); err != nil {
		return err
	}
	if err := os.Remove(s.marker); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove marker: %w", err)
	}
	return nil
}

// Logs 日志文件末尾 lines 行。
func (s *detachedService) Logs(lines int) (string, error) {
	b, err := os.ReadFile(s.logPath)
	if err != nil {
		return "", err
	}
	return tailLines(string(b), lines), nil
}

// livePID 读 PID 文件并确认该进程仍是本服务的内核（防 PID 复用误杀）。
func (s *detachedService) livePID() (int, bool) {
	raw, err := os.ReadFile(s.pidPath)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, s.matches(pid)
}

// matches 进程存活（非僵尸）且工作目录就是本服务的运行时目录。内核以运行时目录为
// cwd 启动，exec 包装脚本也会继承它；无关进程复用该 PID 时 cwd 不会恰好相同。
func (s *detachedService) matches(pid int) bool {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil || processZombie(string(stat)) {
		return false
	}
	cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return false
	}
	want := s.runtimeDir
	if resolved, err := filepath.EvalSymlinks(want); err == nil {
		want = resolved
	}
	return cwd == want
}

// processZombie 解析 /proc/<pid>/stat 的状态字段（命令名可能含空格/括号，取最后一个 ')' 之后）。
func processZombie(stat string) bool {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 || i+2 >= len(stat) {
		return false
	}
	return stat[i+2] == 'Z'
}

// tailLines 取文本末尾 n 行。
func tailLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

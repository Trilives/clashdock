package portable

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// stopGrace 发出 SIGTERM 后等待内核优雅退出的时间，超时再 SIGKILL。
const stopGrace = 5 * time.Second

// Supervisor 便携模式下监护 mihomo 子进程：前台所有权，随 clashdock 生命周期存活，
// 退出即停（进程组一并杀掉，不留后台孤儿）。非并发安全字段由内部锁保护。
type Supervisor struct {
	kernelBin  string
	runtimeDir string
	configPath string
	logPath    string
	pidPath    string

	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{} // 每次 Start 新建；子进程退出后关闭
	exit error         // 子进程退出错误（Wait 的返回）
}

// NewSupervisor 构造监护器。kernelBin 为要运行的 mihomo 二进制，runtimeDir 为
// 工作目录（`-d`），其中应已由 StageRuntime 铺好 config.yaml 与 geo 文件。
func NewSupervisor(kernelBin, runtimeDir string) *Supervisor {
	return &Supervisor{
		kernelBin:  kernelBin,
		runtimeDir: runtimeDir,
		configPath: filepath.Join(runtimeDir, "config.yaml"),
		logPath:    filepath.Join(runtimeDir, "mihomo.log"),
		pidPath:    filepath.Join(runtimeDir, "clashdock.pid"),
	}
}

// LogPath 内核日志文件路径（stdout/stderr 重定向目标）。
func (s *Supervisor) LogPath() string { return s.logPath }

// Validate 用 `mihomo -t` 预校验配置与 geo 数据，避免起一个必然崩溃的子进程。
func (s *Supervisor) Validate() error {
	cmd := exec.Command(s.kernelBin, "-t", "-d", s.runtimeDir, "-f", s.configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("config validation failed: %s", string(out))
	}
	return nil
}

// Start 启动内核子进程。已在运行则直接返回。stdout/stderr 追加写入日志文件；
// 设置独立进程组以便退出时整组终止。
func (s *Supervisor) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil && s.running() {
		return nil
	}
	logf, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	cmd := exec.Command(s.kernelBin, "-d", s.runtimeDir, "-f", s.configPath)
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		logf.Close()
		return fmt.Errorf("start mihomo: %w", err)
	}
	s.cmd = cmd
	done := make(chan struct{})
	s.done = done
	os.WriteFile(s.pidPath, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644)
	go func() {
		err := cmd.Wait()
		logf.Close()
		s.mu.Lock()
		s.exit = err
		s.mu.Unlock()
		close(done)
	}()
	return nil
}

// Stop 终止内核子进程组：先 SIGTERM，超时后 SIGKILL，并清理 PID 文件。
func (s *Supervisor) Stop() error {
	s.mu.Lock()
	cmd := s.cmd
	done := s.done
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pgid := cmd.Process.Pid
	// 负号 = 向整个进程组发信号（Setpgid 后 pgid == 子进程 pid）。
	syscall.Kill(-pgid, syscall.SIGTERM)
	select {
	case <-done:
	case <-time.After(stopGrace):
		syscall.Kill(-pgid, syscall.SIGKILL)
		if done != nil {
			<-done
		}
	}
	os.Remove(s.pidPath)
	s.mu.Lock()
	s.cmd = nil
	s.mu.Unlock()
	return nil
}

// Restart 停止后重新启动内核（用于套用新配置 / 从异常退出恢复）。
func (s *Supervisor) Restart() error {
	if err := s.Stop(); err != nil {
		return err
	}
	return s.Start()
}

// Running 内核子进程是否在运行。
func (s *Supervisor) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running()
}

// Done 返回一个在子进程退出后关闭的通道；未启动时返回 nil。
func (s *Supervisor) Done() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done
}

// running 需在持锁状态调用：探测子进程是否仍存活。
func (s *Supervisor) running() bool {
	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

// Package runtimesvc 服务目标选择：订阅切换 / 节点固定 / 资源更新等改动落盘后，
// 需要把配置同步到「正在运行 mihomo 的那个服务」。完整模式是 systemd 系统单元
// （internal/sysd，经 sudo），用户模式是用户级服务（internal/usermode，免 root）。
// 调用方只依赖这里的 Runtime 接口，按 paths.Paths.UserMode 取对应实现。
package runtimesvc

import (
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/usermode"
)

// Runtime 运行 mihomo 的服务目标。
type Runtime interface {
	// Installed 服务是否已部署。
	Installed() bool
	// Active 服务是否在运行。
	Active() bool
	// SyncAndRestart 把 state 的生效配置同步到运行时，校验通过后重启。
	SyncAndRestart() error
	// Redeploy 内核 / geo / UI 更新后完整重新部署运行时并重启。
	Redeploy() error
	// Pause 停止服务（保持已部署 / 自启）。
	Pause() error
	// Resume 启动服务。
	Resume() error
}

// For 按运行模式返回服务目标。
func For(p paths.Paths) Runtime {
	if p.UserMode {
		return userRuntime{p: p, svc: usermode.NewService(p)}
	}
	return systemRuntime{p: p}
}

type systemRuntime struct{ p paths.Paths }

func (s systemRuntime) Installed() bool       { return sysd.IsInstalled(sysd.DefaultName) }
func (s systemRuntime) Active() bool          { return sysd.IsActive(sysd.DefaultName) }
func (s systemRuntime) SyncAndRestart() error { return sysd.SyncAndRestart(s.p, sysd.DefaultName) }
func (s systemRuntime) Redeploy() error       { return sysd.Install(s.p, sysd.DefaultName, true) }
func (s systemRuntime) Pause() error          { return sysd.Pause(sysd.DefaultName) }
func (s systemRuntime) Resume() error         { return sysd.Resume(sysd.DefaultName) }

type userRuntime struct {
	p   paths.Paths
	svc usermode.Service
}

func (u userRuntime) Installed() bool       { return u.svc.Installed() }
func (u userRuntime) Active() bool          { return u.svc.Active() }
func (u userRuntime) SyncAndRestart() error { return usermode.SyncAndRestart(u.p, u.svc) }

// Redeploy 用户模式的运行时是普通文件复制，完整重新部署与同步重启是同一条路径。
func (u userRuntime) Redeploy() error { return usermode.SyncAndRestart(u.p, u.svc) }
func (u userRuntime) Pause() error    { return u.svc.Stop() }
func (u userRuntime) Resume() error   { return u.svc.Start() }

// 用户模式按需开启 TUN：不做完整安装，开启时才经 sudo 给用户模式内核文件授予网络
// 能力（setcap），取消或失败不留下提权改动；关闭 TUN 不需要 sudo。
package flows

import (
	"fmt"
	"time"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/errs"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/runtimesvc"
	"github.com/Trilives/clashdock/internal/subscription"
	"github.com/Trilives/clashdock/internal/tui"
	"github.com/Trilives/clashdock/internal/usermode"
)

// tunDeviceTimeout 开启后等待 TUN 设备出现的时长。
const tunDeviceTimeout = 10 * time.Second

// userTunDeps 外部依赖（测试替换为替身）：确认提示、sudo 预热与执行、能力检测、
// 应用 enable_tun 并重启服务、TUN 设备检测。
type userTunDeps struct {
	confirm    func(string, bool) (bool, error)
	ensureSudo func(string) error
	runRoot    func([]string, string, *execx.Opt) (execx.Result, error)
	hasCaps    func(string) bool
	apply      func(paths.Paths, bool) error
	tunUp      func() bool
}

func defaultUserTunDeps() userTunDeps {
	return userTunDeps{
		confirm:    tui.Confirm,
		ensureSudo: execx.EnsureSudoNoHint,
		runRoot:    execx.RunRoot,
		hasCaps:    usermode.HasTunCaps,
		apply:      applyUserTun,
		tunUp:      func() bool { return usermode.WaitTunDevice(subscription.TunDevice, tunDeviceTimeout) },
	}
}

// userTunLabel 菜单项标签：随当前 enable_tun 变化。
func userTunLabel(p paths.Paths) string {
	if config.Bool(config.Load(p), "enable_tun") {
		return i18n.T("关闭 TUN 模式")
	}
	return i18n.T("开启 TUN 模式（需 sudo 授权）")
}

// UserTunToggle 开启 / 关闭用户模式 TUN。
func UserTunToggle(p paths.Paths) error {
	return toggleUserTun(p, defaultUserTunDeps())
}

func toggleUserTun(p paths.Paths, d userTunDeps) error {
	if !runtimesvc.For(p).Installed() {
		execx.Warn(i18n.T("用户服务尚未部署，请运行 clashdock user 完成部署。"))
		return nil
	}
	if config.Bool(config.Load(p), "enable_tun") {
		if err := d.apply(p, false); err != nil {
			return err
		}
		execx.Ok(i18n.T("已关闭 TUN，恢复为本机纯代理。"))
		return nil
	}
	return enableUserTun(p, d)
}

// enableUserTun 确认 → 内核缺能力时经 sudo setcap 授予 → 写配置并重启 → 确认 TUN 设备
// 已出现。任一步失败都回退：配置改回关闭，本次授予的能力经 setcap -r 撤销。
func enableUserTun(p paths.Paths, d userTunDeps) error {
	ok, err := d.confirm(fmt.Sprintf(i18n.T("开启 TUN 需要管理员授权：将经 sudo 为用户模式内核 %s 授予网络能力（不保存密码，可随时关闭）。继续？"), p.MihomoBin), false)
	if err != nil {
		return err
	}
	if !ok {
		return errs.ErrCancelled
	}
	granted := false
	if !d.hasCaps(p.MihomoBin) {
		if err := grantTunCaps(p, d); err != nil {
			return err
		}
		granted = true
	}
	if err := d.apply(p, true); err != nil {
		rollbackUserTun(p, d, granted)
		return fmt.Errorf(i18n.T("开启 TUN 失败，已回退：%w"), err)
	}
	if !d.tunUp() {
		rollbackUserTun(p, d, granted)
		return fmt.Errorf("%s", i18n.T("TUN 设备未能创建（家目录可能挂载为 nosuid 或不支持文件能力），已回退为纯代理；详情见「工具 → 最新日志」。"))
	}
	execx.Ok(i18n.T("TUN 已开启：整机流量经 mihomo 代理。"))
	return nil
}

// grantTunCaps 经 sudo 给内核文件授予 TUN 能力；授权被取消或失败时不做任何改动。
func grantTunCaps(p paths.Paths, d userTunDeps) error {
	if err := d.ensureSudo(i18n.T("开启 TUN")); err != nil {
		execx.Warn(i18n.T("未取得管理员授权，TUN 保持关闭。"))
		return err
	}
	if _, err := d.runRoot([]string{"setcap", usermode.TunCapsSpec, p.MihomoBin}, i18n.T("授予 TUN 能力"), nil); err != nil {
		return fmt.Errorf(i18n.T("授予 TUN 能力失败（需要 setcap，Debian/Ubuntu 在 libcap2-bin 包中）：%w"), err)
	}
	return nil
}

// rollbackUserTun 开启失败后的回退：配置改回关闭并重启；撤销本次授予的能力。
func rollbackUserTun(p paths.Paths, d userTunDeps, granted bool) {
	if err := d.apply(p, false); err != nil {
		execx.Warn(i18n.T("回退 TUN 配置失败：") + err.Error())
	}
	if !granted {
		return
	}
	if _, err := d.runRoot([]string{"setcap", "-r", p.MihomoBin}, i18n.T("撤销 TUN 能力"), nil); err != nil {
		execx.Warn(i18n.T("撤销 TUN 能力失败：") + err.Error())
	}
}

// applyUserTun 写入 enable_tun、用本地原文重建生效订阅并重启用户服务。
func applyUserTun(p paths.Paths, enable bool) error {
	if err := saveEnableTun(p, enable); err != nil {
		return err
	}
	return runtimesvc.For(p).SyncAndRestart()
}

// saveEnableTun 只改 enable_tun 并重建生效订阅（TUN 段与直连进程规则随之生成），不重启。
func saveEnableTun(p paths.Paths, enable bool) error {
	cfg := config.Load(p)
	updated := make(map[string]any, len(cfg))
	for k, v := range cfg {
		updated[k] = v
	}
	updated["enable_tun"] = enable
	if err := config.Save(p, updated); err != nil {
		return fmt.Errorf(i18n.T("写入定制层失败：%w"), err)
	}
	if active := subscription.GetActive(p); active != nil {
		if _, err := subscription.Rebuild(p, active.Name); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileUserTun 内核更新后、重新部署前调用：TUN 开着但新内核文件不带能力时询问是否
// 重新授权；拒绝或失败则把 TUN 改为关闭（不重启，由调用方的重新部署生效）。
func ReconcileUserTun(p paths.Paths) {
	reconcileUserTun(p, defaultUserTunDeps())
}

func reconcileUserTun(p paths.Paths, d userTunDeps) {
	if !p.UserMode || !config.Bool(config.Load(p), "enable_tun") || d.hasCaps(p.MihomoBin) {
		return
	}
	ok, err := d.confirm(i18n.T("内核文件已更换，原 TUN 授权随之失效。现在经 sudo 重新授权？（选否将关闭 TUN）"), true)
	if err == nil && ok {
		if err = grantTunCaps(p, d); err == nil {
			return
		}
		execx.Warn(err.Error())
	}
	if err := saveEnableTun(p, false); err != nil {
		execx.Warn(i18n.T("关闭 TUN 失败：") + err.Error())
		return
	}
	execx.Warn(i18n.T("已关闭 TUN，恢复为本机纯代理；可在主菜单重新开启。"))
}

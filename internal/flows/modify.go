// 更改配置全流程（对应 flows/modify.py）。
//
// 整个会话包在一个事务里：配置类改动（订阅增删改 / 切换 / 刷新 / 定制层）都是
// 临时的——esc「保存并退出」才提交；主菜单 ^R 回退本次会话的全部配置改动。
// 系统类操作（更新内核 / 节点实时切换 / 服务重启 / 自愈 / 定时器）为即时生效，
// 不在会话回退范围内（菜单项以「※即时」标注）。
package flows

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/errs"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/subscription"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/tui"
	"github.com/Trilives/clashdock/internal/txn"
)

var modifyOptions = []string{
	"订阅管理（增 / 删 / 改名 / 切换 / 刷新）",
	"编辑定制层（TUN / 局域网 / 面板 / 自定义分流 …）",
	"切换 / 固定节点 ※即时",
	"更新 内核 / UI / geo 数据 ※即时",
	"服务设置（重启 / 状态）※即时",
	"独立 Web 面板（根路径直开）※即时",
	"网络自愈设置 ※即时",
	"每周更新定时器 ※即时",
}

// Modify 更改配置会话。
func Modify(p paths.Paths) error {
	return txn.Run("更改配置", func(session *txn.Transaction) error {
		// 会话开始即快照配置类路径，使任意改动都能被 ^R 统一回退
		for _, path := range []string{p.ConfigFile, p.ActiveFile, p.CustomizeFile, p.Subscriptions} {
			if err := session.Snapshot(path); err != nil {
				return err
			}
		}
		// 回退发生在文件还原之后（LIFO，最先登记 → 最后执行）：把运行中的服务对齐回退后的配置
		session.AddUndo("同步服务到回退后的配置", func() error { resyncService(p); return nil })

		handlers := []func() error{
			func() error { return subscriptionsMenu(p) },
			func() error { return editCustomizeFlow(p) },
			func() error { return NodeSelect(p, p.ConfigFile, "") },
			func() error { return updateCoreFlow(p) },
			func() error { return serviceSettings(p) },
			func() error { return webuiMenuFlow(p) },
			func() error { return resilienceMenuFlow() },
			func() error { return timerMenuFlow() },
		}
		idx := 0
		for {
			i, err := tui.Select("更改配置", modifyOptions,
				tui.SelectOpts{BackLabel: "回退并退出", SaveLabel: "保存并退出", Initial: idx})
			if err != nil {
				if errors.Is(err, errs.ErrSaveExit) {
					return nil // esc = 保存并退出 → 事务提交
				}
				return err // 主菜单 ^R → 回退整个会话
			}
			idx = i
			if err := handlers[i](); err != nil {
				if errors.Is(err, errs.ErrSaveExit) {
					return nil // 子菜单选了「保存并退出」→ 提交整个会话
				}
				if errors.Is(err, errs.ErrCancelled) {
					continue // 单个操作中途取消 → 回主菜单，会话改动仍在缓冲中
				}
				execx.Error(err.Error()) // 单个操作失败不终结会话
			}
		}
	})
}

func resyncService(p paths.Paths) {
	if sysd.IsInstalled(sysd.DefaultName) && fileExists(p.ConfigFile) {
		if err := sysd.SyncAndRestart(p, sysd.DefaultName); err != nil {
			execx.Warn(fmt.Sprintf("服务同步失败：%v", err))
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// --------------------------------------------------------------------------
// 订阅管理
// --------------------------------------------------------------------------

func subscriptionsMenu(p paths.Paths) error {
	act := 0
	for {
		subs := subscription.ListAll(p)
		activeName := ""
		if active := subscription.GetActive(p); active != nil {
			activeName = active.Name
		}
		execx.Header("订阅管理")
		if len(subs) == 0 {
			fmt.Println("  • （暂无订阅）")
		}
		for _, s := range subs {
			line := fmt.Sprintf("%s  [%s, %d 节点]", s.Name, s.SourceType, s.LastNodeCount)
			if s.Name == activeName {
				line += "  ← 生效"
			}
			fmt.Println("  • " + line)
		}
		a, err := tui.Select("订阅操作",
			[]string{"添加订阅", "导入本地 config.yaml", "切换生效订阅", "刷新订阅", "重命名", "删除订阅"},
			tui.SelectOpts{BackLabel: "返回上层", Initial: act})
		if err != nil {
			return nil // 返回上层菜单（改动仍在会话缓冲中）
		}
		act = a
		ops := []func() error{
			func() error { return subAdd(p) },
			func() error { return importConfigFlow(p) },
			func() error { return subSwitch(p) },
			func() error { return subRefresh(p) },
			func() error { return subRename(p) },
			func() error { return subRemove(p) },
		}
		if err := ops[a](); err != nil {
			if errors.Is(err, errs.ErrCancelled) {
				continue
			}
			execx.Error(err.Error())
		}
	}
}

// maybeNodeSelect 订阅链接变化后，提示是否进入「切换 / 固定节点」。
func maybeNodeSelect(p paths.Paths) error {
	ok, err := tui.Confirm("订阅已更新，是否现在切换 / 固定节点？", false)
	if err != nil || !ok {
		return err
	}
	return NodeSelect(p, p.ConfigFile, "")
}

func subAdd(p paths.Paths) error {
	info, err := askNewSubscription()
	if err != nil {
		return err
	}
	if info == nil {
		execx.Warn("订阅链接留空，已取消添加。")
		return nil
	}
	setActive := subscription.GetActive(p) == nil
	if !setActive {
		setActive, err = tui.Confirm("设为生效订阅？", true)
		if err != nil {
			return err
		}
	}
	if _, err := subscription.Add(p, info.Name, info.URL, info.SourceType, info.ApplyOverlay, setActive); err != nil {
		return err
	}
	if setActive {
		return maybeNodeSelect(p)
	}
	return nil
}

func pickSub(p paths.Paths, prompt string) (string, error) {
	subs := subscription.ListAll(p)
	if len(subs) == 0 {
		execx.Warn("暂无订阅。")
		return "", nil
	}
	names := make([]string, len(subs))
	for i, s := range subs {
		names[i] = s.Name
	}
	idx, err := tui.Select(prompt, names, tui.SelectOpts{})
	if err != nil {
		return "", err
	}
	return names[idx], nil
}

func subSwitch(p paths.Paths) error {
	name, err := pickSub(p, "切换到哪个订阅")
	if err != nil || name == "" {
		return err
	}
	if err := subscription.Switch(p, name); err != nil {
		return err
	}
	return maybeNodeSelect(p)
}

func subRefresh(p paths.Paths) error {
	name, err := pickSub(p, "刷新哪个订阅")
	if err != nil || name == "" {
		return err
	}
	active := subscription.GetActive(p)
	if _, err := subscription.Refresh(p, name); err != nil {
		return err
	}
	if active != nil && active.Name == name {
		return maybeNodeSelect(p)
	}
	return nil
}

func subRename(p paths.Paths) error {
	name, err := pickSub(p, "重命名哪个订阅")
	if err != nil || name == "" {
		return err
	}
	newName, err := tui.Ask("新名称", tui.AskOpts{AllowEmpty: false})
	if err != nil {
		return err
	}
	return subscription.Rename(p, name, newName)
}

func subRemove(p paths.Paths) error {
	name, err := pickSub(p, "删除哪个订阅")
	if err != nil || name == "" {
		return err
	}
	ok, err := tui.Confirm(fmt.Sprintf("确认删除订阅「%s」？", name), false)
	if err != nil || !ok {
		return err
	}
	return subscription.RemoveSub(p, name)
}

// --------------------------------------------------------------------------
// 其它
// --------------------------------------------------------------------------

func editCustomizeFlow(p paths.Paths) error {
	changed, err := EditCustomize(p)
	if err != nil {
		return err
	}
	active := subscription.GetActive(p)
	if changed && active != nil {
		ok, err := tui.Confirm("立即用本地原文重新生成生效订阅并重启？（不重新拉取链接）", true)
		if err != nil {
			return err
		}
		if ok {
			_, err = subscription.Rebuild(p, active.Name)
			return err
		}
	}
	return nil
}

func updateCoreFlow(p paths.Paths) error {
	ok, err := tui.Confirm("更新 内核 / UI / geo 数据？", true)
	if err != nil || !ok {
		return err
	}
	_, hasUIErr := os.Stat(filepath.Join(p.UI, "index.html"))
	hasUI := hasUIErr == nil
	ensureGithubToken(p)
	if _, err := kernel.DownloadAll(p, kernel.Options{Force: true, WithUI: hasUI}); err != nil {
		return err
	}
	if hasUI {
		cfg := config.Load(p)
		if err := sysd.RefreshWebUI(p, config.Int(cfg, "webui_port"), config.Bool(cfg, "lan_panel")); err != nil {
			execx.Warn("独立面板刷新失败：" + err.Error())
		}
	}
	if fileExists(p.ConfigFile) && sysd.IsInstalled(sysd.DefaultName) {
		return sysd.SyncAndRestart(p, sysd.DefaultName)
	}
	return nil
}

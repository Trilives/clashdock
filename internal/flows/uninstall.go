// 卸载全流程（对应 flows/uninstall.py）：勾选式移除服务 / 自愈 / 定时器 / 产物 / 订阅。
package flows

import (
	"fmt"
	"os"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/proxyenv"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/tui"
)

// Uninstall 卸载流程。
func Uninstall(p paths.Paths) error {
	items := []string{
		i18n.T("systemd 服务"),
		i18n.T("网络自愈（NM 钩子 + watchdog）"),
		i18n.T("每周更新定时器"),
		i18n.T("独立 Web 面板服务"),
		i18n.T("清理产物（内核 / UI / 下载缓存 / geo 数据）"),
		i18n.T("清理所有订阅与配置（含整个状态目录）"),
	}
	chosen, err := tui.MultiSelect(i18n.T("卸载（勾选要移除的项）"), items, []int{0, 1, 2, 3})
	if err != nil {
		return nil // 取消
	}
	if len(chosen) == 0 {
		execx.Info(i18n.T("未选择任何项，已取消。"))
		return nil
	}
	execx.Header(i18n.T("即将卸载"))
	for _, i := range chosen {
		fmt.Println("  - " + items[i])
	}
	ok, err := tui.Confirm(i18n.T("确认执行？"), false)
	if err != nil || !ok {
		execx.Info(i18n.T("已取消。"))
		return nil
	}

	actions := []func() error{
		func() error { return sysd.Remove(p, sysd.DefaultName, true) },
		func() error { return sysd.RemoveResilience(sysd.DefaultName) },
		sysd.RemoveTimer,
		func() error {
			if sysd.WebUIInstalled() {
				return sysd.RemoveWebUI()
			}
			execx.Info(i18n.T("未安装独立 Web 面板服务，跳过。"))
			return nil
		},
		func() error {
			for _, d := range []string{p.Bin, p.UI, p.Downloads, p.Ruleset} {
				os.RemoveAll(d)
			}
			execx.Ok(i18n.T("已清理本地产物（内核 / UI / 缓存 / geo 数据）。"))
			return nil
		},
		func() error {
			proxyenv.Remove() // 清掉 bashrc 代理变量，避免残留指向失效代理
			os.RemoveAll(p.State)
			execx.Ok(i18n.T("已清理状态目录（所有订阅与配置）。"))
			return nil
		},
	}
	for _, i := range chosen {
		if err := actions[i](); err != nil {
			execx.Error(fmt.Sprintf(i18n.T("移除「%s」失败：%v"), items[i], err))
		}
	}
	execx.Ok(i18n.T("卸载流程结束。"))
	return nil
}

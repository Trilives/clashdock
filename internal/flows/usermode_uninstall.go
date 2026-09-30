// 用户模式卸载：勾选移除用户服务 / 已安装本体 / 全部数据，均无需 root。
package flows

import (
	"fmt"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/tui"
	"github.com/Trilives/clashdock/internal/usermode"
)

const (
	userUninstallService = iota
	userUninstallBinary
	userUninstallData
)

// UserUninstall 卸载用户模式；返回是否实际移除了内容（调用方据此退出菜单）。
func UserUninstall(p paths.Paths) (bool, error) {
	items := []string{
		i18n.T("用户服务（停止并删除）"),
		fmt.Sprintf(i18n.T("clashdock 本体（%s）"), usermode.BinPath()),
		fmt.Sprintf(i18n.T("全部数据：订阅 / 配置 / 内核（%s）"), p.State),
	}
	chosen, err := tui.MultiSelect(i18n.T("卸载用户模式（勾选要移除的项）"), items,
		[]int{userUninstallService, userUninstallBinary})
	if err != nil || len(chosen) == 0 {
		execx.Info(i18n.T("已取消。"))
		return false, nil
	}
	chosen = withImpliedServiceRemoval(chosen)
	execx.Header(i18n.T("即将卸载"))
	for _, i := range chosen {
		fmt.Println("  - " + items[i])
	}
	ok, err := tui.Confirm(i18n.T("确认执行？"), false)
	if err != nil || !ok {
		execx.Info(i18n.T("已取消。"))
		return false, nil
	}
	return true, runUserUninstall(p, usermode.NewService(p), chosen)
}

// withImpliedServiceRemoval 删除数据目录会让服务指向不存在的内核与运行时，因此勾选
// 「全部数据」时一并移除服务。
func withImpliedServiceRemoval(chosen []int) []int {
	hasData, hasService := false, false
	for _, i := range chosen {
		hasData = hasData || i == userUninstallData
		hasService = hasService || i == userUninstallService
	}
	if !hasData || hasService {
		return chosen
	}
	return append([]int{userUninstallService}, chosen...)
}

func runUserUninstall(p paths.Paths, svc usermode.Service, chosen []int) error {
	var failed error
	for _, i := range chosen {
		var err error
		switch i {
		case userUninstallService:
			if err = svc.Remove(); err == nil {
				execx.Ok(i18n.T("已删除用户服务。"))
			}
		case userUninstallBinary:
			if err = usermode.RemoveBinary(usermode.BinPath()); err == nil {
				execx.Ok(i18n.T("已删除 clashdock 本体。"))
			}
		case userUninstallData:
			if err = usermode.RemoveState(p.State); err == nil {
				execx.Ok(i18n.T("已清理数据目录（所有订阅与配置）。"))
			}
		}
		if err != nil {
			execx.Error(fmt.Sprintf(i18n.T("卸载步骤失败：%v"), err))
			failed = err
		}
	}
	if failed != nil {
		return fmt.Errorf(i18n.T("部分卸载步骤失败：%w"), failed)
	}
	execx.Ok(i18n.T("用户模式卸载完成。"))
	return nil
}

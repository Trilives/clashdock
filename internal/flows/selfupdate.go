// clashdock 自更新交互流程（对应 internal/selfupdate 的下载 / 校验 / 切版本）。
package flows

import (
	"fmt"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/selfupdate"
	"github.com/Trilives/clashdock/internal/tui"
)

// SelfUpdateFlow 更新 clashdock 自身：查询最新版本 → 确认 → 下载/校验/切换。
func SelfUpdateFlow(p paths.Paths, currentVersion string) error {
	execx.Info(fmt.Sprintf(i18n.T("当前版本：%s，正在查询最新版本…"), currentVersion))
	latest, err := selfupdate.LatestVersion(p)
	if err != nil {
		return err
	}
	if latest == currentVersion {
		execx.Ok(fmt.Sprintf(i18n.T("已是最新版本（%s）。"), currentVersion))
		return nil
	}
	ok, err := tui.Confirm(fmt.Sprintf(i18n.T("发现新版本 %s（当前 %s），现在更新？"), latest, currentVersion), true)
	if err != nil || !ok {
		return err
	}
	version, alreadyLatest, err := selfupdate.Update(p, currentVersion)
	if err != nil {
		return err
	}
	if alreadyLatest {
		execx.Ok(fmt.Sprintf(i18n.T("已是最新版本（%s）。"), version))
		return nil
	}
	execx.Ok(fmt.Sprintf(i18n.T("clashdock 已更新到 %s，下次运行即生效。"), version))
	return nil
}

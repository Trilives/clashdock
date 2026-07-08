// 初始化（首次部署）全流程（对应 flows/init.py）。
// 流程按模块提交：服务启动前的核心步骤可整体回退；服务启动后，外部资源更新、
// 伴生单元等后续模块只回退各自尚未提交的改动，不反向卸载已启动的主服务。
package flows

import (
	"fmt"
	"os"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/errs"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/firewall"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/proxyenv"
	"github.com/Trilives/clashdock/internal/subscription"
	"github.com/Trilives/clashdock/internal/sysd"
	"github.com/Trilives/clashdock/internal/tui"
	"github.com/Trilives/clashdock/internal/txn"
)

// Init 初始化流程；ErrCancelled 由 txn.Run 回退并吞掉。
func Init(p paths.Paths) error {
	return txn.Run(i18n.T("初始化"), func(t *txn.Transaction) error {
		execx.Header(i18n.T("初始化（首次部署）"))

		// 0. deb 种子接管（若从系统包安装，离线即可获得内核与基础规则）
		if _, err := kernel.SeedFromSystem(p); err != nil {
			execx.Warn(i18n.T("种子接管失败（不影响后续下载）：") + err.Error())
		}

		// 1. 若本地已有订阅记录（如 migrate 后重新初始化），先问是否直接复用；
		// 复用时表单只收集基础设置，不再重复填订阅。
		existing := subscription.ListAll(p)
		reuse := false
		if len(existing) > 0 {
			useLocal, err := tui.Confirm(
				fmt.Sprintf(i18n.T("检测到本地已有 %d 个订阅记录，是否直接使用现有订阅？"), len(existing)), true)
			if err != nil {
				return err
			}
			reuse = useLocal
		}

		// 2. 单屏表单一次性收集基础设置（+ 首个订阅）。取消 → 回退整个事务。
		s, ok, err := runInitForm(p, !reuse)
		if err != nil {
			return err
		}
		if !ok {
			return errs.ErrCancelled
		}

		// 3. 落盘基础设置。
		cfg := config.Load(p)
		cfg["download_proxy"] = s.downloadProxy
		cfg["proxy_port"] = s.proxyPort
		cfg["enable_tun"] = s.enableTun
		cfg["lan_proxy"] = s.lanProxy
		if err := t.BackupFile(p.CustomizeFile); err != nil {
			return err
		}
		if err := config.Save(p, cfg); err != nil {
			return err
		}

		// TUN 关闭=纯代理：按表单选择把代理变量写入 bashrc。
		if !s.enableTun && s.writeBashrc {
			if err := t.BackupFile(proxyenv.TargetBashrc()); err != nil {
				return err
			}
			if _, err := proxyenv.Write(config.ProxyPort(cfg)); err != nil {
				return err
			}
		}

		// 局域网代理按表单选择放行防火墙端口（端口取 proxy_port，默认 7890）。
		if s.lanProxy && s.allowPort {
			port := config.ProxyPort(cfg)
			t.AddUndo(i18n.T("撤销防火墙放行代理端口"), func() error { firewall.Revoke(port); return nil })
			firewall.Allow(port)
		}

		// 4. 添加首个订阅（或复用现有）。
		ready, err := applyInitSubscription(p, t, s, reuse, existing)
		if err != nil {
			return err
		}
		if !ready {
			execx.Info(i18n.T("已跳过订阅与服务注册，结束初始化。设置已保存，") +
				i18n.T("稍后可在主菜单「订阅 → 添加订阅」补配并启动服务。"))
			return nil // 正常返回 → 事务提交，保留步骤 1-2 成果
		}

		// 3. 注册并启动 systemd 服务。内核与基础规则由安装包提供（deb 种子接管，或
		// 便携包 install.sh 装入系统路径），初始化不再下载内核。资源缺失时不回滚本次
		// 会话已完成的设置与订阅——保留它们，提示先补齐内核后重新执行初始化即可（会
		// 检测到已有订阅并跳过重新添加，直接重试服务注册）。
		if err := ensureStartupResources(p); err != nil {
			execx.Warn(i18n.T("本地内核缺失，本次暂不注册/启动服务：") + err.Error())
			execx.Info(i18n.T("已保留本次配置与订阅；补齐内核（重装安装包或「运行时管理 → 更新内核」）后重新执行初始化即可完成服务注册。"))
			t.Commit()
			return nil
		}
		t.AddUndo(i18n.T("卸载服务 mihomo"), func() error { return sysd.Remove(p, sysd.DefaultName, true) })
		if err := sysd.Install(p, sysd.DefaultName, true); err != nil {
			return err
		}
		// 主服务已运行就是持久边界；后续可选模块失败时不应反向卸载它。
		t.Commit()

		// 4. 服务先跑起来；再自动下载/更新 geo 与 Web UI（内核不下载，随包捆绑；
		// 失败不影响已启动的服务）。
		optionalPostStartUpdate(p)
		t.Commit()

		// 5. 可选增强：网络自愈 / 每周更新
		if err := optionalExtras(t); err != nil {
			return err
		}
		t.Commit()

		// 6. 提示切换 / 固定节点
		ok, err = tui.Confirm(i18n.T("配置已就绪，是否现在切换 / 固定节点？"), false)
		if err != nil {
			return err
		}
		if ok {
			if err := NodeSelect(p, p.ConfigFile, ""); err != nil {
				return err
			}
		}

		execx.Ok(i18n.T("初始化完成。"))
		printAccessHint(p)
		return nil
	})
}

// applyInitSubscription 应用初始化表单收集到的订阅设置：reuse=true 时直接切到现有
// 订阅；否则用表单填的信息新建首个订阅（链接留空则跳过，返回 ready=false 表示不注册
// 服务）。本地 YAML 也是作为一个真正的订阅条目创建，而不是走单独的「本地文件覆盖」。
func applyInitSubscription(p paths.Paths, t *txn.Transaction, s *initSettings, reuse bool, existing []subscription.Subscription) (bool, error) {
	if err := t.BackupFile(p.ConfigFile); err != nil {
		return false, err
	}
	if err := t.BackupFile(p.ActiveFile); err != nil {
		return false, err
	}

	if reuse {
		target := existing[0].Name
		if active := subscription.GetActive(p); active != nil {
			target = active.Name
		}
		if err := subscription.Switch(p, target); err != nil {
			return false, err
		}
		execx.Ok(fmt.Sprintf(i18n.T("已使用现有订阅：%s"), target))
		return true, nil
	}

	if !s.hasSub {
		return false, nil
	}
	sub, err := subscription.Add(p, s.sub.Name, s.sub.URL, s.sub.SourceType, s.sub.ApplyOverlay, true, s.sub.FetchViaProxy, s.sub.PauseForDirect)
	if err != nil {
		return false, err
	}
	t.AddUndo(i18n.T("删除订阅 ")+sub.Name, func() error { return subscription.RemoveSub(p, sub.Name) })
	return true, nil
}

func startupResourcesReady(p paths.Paths) bool {
	if _, err := os.Stat(p.MihomoBin); err != nil {
		return false
	}
	if _, err := os.Stat(p.GeositeDat); err != nil {
		return false
	}
	if _, err := os.Stat(p.GeoipMetadb); err == nil {
		return true
	}
	if _, err := os.Stat(p.CountryMmdb); err == nil {
		return true
	}
	return false
}

// ensureStartupResources 服务启动前检查本地内核与基础规则是否就绪。内核与基础规则
// 现由安装包提供——deb 种子接管，或便携包 install.sh 装入系统路径——初始化不再
// 下载内核。缺失即返回错误，由调用方（Init）软失败处理：不回滚已完成的设置与订阅，
// 提示用户先补齐内核（重装安装包或「运行时管理 → 更新内核」）后重试。
func ensureStartupResources(p paths.Paths) error {
	if startupResourcesReady(p) {
		execx.Info(i18n.T("使用本地内核与基础规则启动服务（系统包种子或既有资源）。"))
		return nil
	}
	return fmt.Errorf("%s", i18n.T("未找到本地内核或基础规则：请通过安装包（deb 已内置，便携包运行 install.sh）安装内核，或在「运行时管理 → 更新内核」手动下载后重试。"))
}

// optionalPostStartUpdate 服务已启动后自动下载/更新 geo 数据与 Web UI（不再下载
// 内核——内核随安装包捆绑，更新由用户在「运行时管理 → 更新内核」显式触发）；服务
// 已运行，优先走本机 mixed-port 下载（出海更稳），失败回退 download_proxy、最后
// 直连。失败只警告，不影响已启动的服务。
func optionalPostStartUpdate(p paths.Paths) {
	execx.Info(i18n.T("服务已启动，自动下载/更新 geo 数据 / Web UI…"))
	ensureGithubToken(p)
	if _, err := kernel.DownloadAll(p, kernel.Options{Force: true, WithUI: true, LocalProxyFirst: true, SkipCore: true}); err != nil {
		execx.Warn(i18n.T("资源更新失败：") + err.Error())
		execx.Info(i18n.T("可稍后在「运行时管理 → 更新」重试。"))
		return
	}
	execx.Info(i18n.T("已更新资源，重新部署运行时并重启服务…"))
	if err := sysd.Install(p, sysd.DefaultName, true); err != nil {
		execx.Warn(i18n.T("重新部署运行时失败：") + err.Error())
		execx.Info(i18n.T("可稍后在「运行时管理 → 更新」重试。"))
	}
}

func optionalExtras(t *txn.Transaction) error {
	ok, err := tui.Confirm(i18n.T("安装网络切换自愈？"), true)
	if err != nil {
		return err
	}
	if ok {
		t.AddUndo(i18n.T("卸载网络自愈"), func() error { return sysd.RemoveResilience(sysd.DefaultName) })
		if err := sysd.InstallResilience(sysd.ResilienceOptions{}); err != nil {
			return err
		}
	}
	ok, err = tui.Confirm(i18n.T("安装每周自动更新定时器？"), false)
	if err != nil {
		return err
	}
	if ok {
		t.AddUndo(i18n.T("卸载每周更新"), sysd.RemoveTimer)
		if err := sysd.InstallTimer(""); err != nil {
			return err
		}
	}
	return nil
}

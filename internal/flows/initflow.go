// 初始化（首次部署）全流程（对应 flows/init.py）。
// 整个流程包在事务内：任意步骤 ESC / 出错都会回退已应用的改动。
package flows

import (
	"fmt"
	"os"

	"github.com/Trilives/clashdock/internal/config"
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
// 第一步先选语言（在事务外执行，不随后续步骤的取消/回退而撤销），
// 确保后续所有提示都以用户选定的语言展示。
func Init(p paths.Paths) error {
	if err := PickLanguage(p); err != nil {
		return err
	}
	return txn.Run(i18n.T("初始化"), func(t *txn.Transaction) error {
		execx.Header(i18n.T("初始化（首次部署）"))

		// 0. deb 种子接管（若从系统包安装，离线即可获得内核与基础规则）
		if _, err := kernel.SeedFromSystem(p); err != nil {
			execx.Warn(i18n.T("种子接管失败（不影响后续下载）：") + err.Error())
		}

		// 1. 局域网下载代理 / TUN / 局域网代理
		cfg := config.Load(p)
		proxy, err := tui.Ask(
			i18n.T("订阅/资源下载代理 IP:端口（出海慢时走它，如 192.168.1.10:7890），留空=保留当前/无则直连"),
			tui.AskOpts{Default: stripScheme(config.Str(cfg, "download_proxy")), AllowEmpty: true})
		if err != nil {
			return err
		}
		cfg["download_proxy"] = normalizeProxy(proxy)
		// TUN 模式：全局透明代理；关则纯代理，需各 App 自设代理
		enableTun, err := tui.Confirm(i18n.T("启用 TUN 模式？（整机流量自动走代理；否=纯代理，需各 App 手动设代理）"),
			config.Bool(cfg, "enable_tun"))
		if err != nil {
			return err
		}
		cfg["enable_tun"] = enableTun
		lanProxy, err := tui.Confirm(i18n.T("开启局域网代理？（让局域网其他主机可用本机作为代理，监听 0.0.0.0:7890）"),
			config.Bool(cfg, "lan_proxy"))
		if err != nil {
			return err
		}
		cfg["lan_proxy"] = lanProxy
		if err := t.BackupFile(p.CustomizeFile); err != nil {
			return err
		}
		if err := config.Save(p, cfg); err != nil {
			return err
		}

		// TUN 关闭=纯代理：可选把代理变量写入 bashrc
		if !enableTun {
			ok, err := tui.Confirm(i18n.T("把代理环境变量写入 ~/.bashrc？（新开终端自动走 127.0.0.1:7890）"), true)
			if err != nil {
				return err
			}
			if ok {
				if err := t.BackupFile(proxyenv.TargetBashrc()); err != nil {
					return err
				}
				if _, err := proxyenv.Write(); err != nil {
					return err
				}
			}
		}

		// 局域网代理需放行防火墙端口
		if lanProxy {
			ok, err := tui.Confirm(i18n.T("更新防火墙放行 7890 端口？"), true)
			if err != nil {
				return err
			}
			if ok {
				t.AddUndo(i18n.T("撤销防火墙放行 7890"), func() error { firewall.Revoke(firewall.ProxyPort); return nil })
				firewall.Allow(firewall.ProxyPort)
			}
		}

		// 2. 添加首个订阅，或直接导入本地 config.yaml。
		ready, err := initialConfigSource(p, t)
		if err != nil {
			return err
		}
		if !ready {
			execx.Info(i18n.T("已跳过订阅与服务注册，结束初始化。设置已保存，") +
				i18n.T("稍后可在主菜单「订阅 → 添加订阅 / 导入 config.yaml」补配并启动服务。"))
			return nil // 正常返回 → 事务提交，保留步骤 1-2 成果
		}

		// 3. 注册并启动 systemd 服务。deb 安装已内置内核与基础规则，优先直接使用；
		// 非 deb / 资源缺失场景才在启动前下载兜底。
		if err := ensureStartupResources(p); err != nil {
			return err
		}
		t.AddUndo(i18n.T("卸载服务 mihomo"), func() error { return sysd.Remove(p, sysd.DefaultName, true) })
		if err := sysd.Install(p, sysd.DefaultName, true); err != nil {
			return err
		}

		// 4. 服务先跑起来；再询问是否在线下载/更新内核、geo 与可选 Web UI。
		if err := optionalPostStartUpdate(p, t); err != nil {
			return err
		}

		// 5. 可选增强：网络自愈 / 每周更新
		if err := optionalExtras(t); err != nil {
			return err
		}

		// 6. 提示切换 / 固定节点
		ok, err := tui.Confirm(i18n.T("配置已就绪，是否现在切换 / 固定节点？"), false)
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

func initialConfigSource(p paths.Paths, t *txn.Transaction) (bool, error) {
	source, err := tui.Select(i18n.T("配置来源"), []string{i18n.T("添加订阅链接"), i18n.T("导入本地 config.yaml")}, tui.SelectOpts{BackLabel: i18n.T("暂不配置")})
	if err != nil {
		return false, nil
	}
	if err := t.BackupFile(p.ConfigFile); err != nil {
		return false, err
	}
	if err := t.BackupFile(p.ActiveFile); err != nil {
		return false, err
	}
	if source == 1 {
		path, err := tui.Ask(i18n.T("config.yaml 文件路径"), tui.AskOpts{AllowEmpty: false})
		if err != nil {
			return false, err
		}
		if err := importConfigFromFile(p, path); err != nil {
			return false, err
		}
		execx.Ok(i18n.T("已导入 config.yaml。"))
		return true, nil
	}

	info, err := askNewSubscription()
	if err != nil {
		return false, err
	}
	if info == nil {
		return false, nil
	}
	sub, err := subscription.Add(p, info.Name, info.URL, info.SourceType, info.ApplyOverlay, true)
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

func ensureStartupResources(p paths.Paths) error {
	if startupResourcesReady(p) {
		execx.Info(i18n.T("使用本地内核与基础规则启动服务（系统包种子或既有资源）。"))
		return nil
	}
	execx.Warn(i18n.T("未找到本地内核或基础规则；非 .deb 安装/种子缺失时需要先下载才能启动服务。"))
	ok, err := tui.Confirm(i18n.T("现在下载内核和基础规则以便启动服务？"), true)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s", i18n.T("缺少 mihomo 内核或基础规则，无法注册并启动服务"))
	}
	ensureGithubToken(p)
	if _, err := kernel.DownloadAll(p, kernel.Options{WithUI: false}); err != nil {
		return err
	}
	return nil
}

func optionalPostStartUpdate(p paths.Paths, t *txn.Transaction) error {
	ok, err := tui.Confirm(i18n.T("服务已启动。现在下载/更新内核、geo 数据和可选 Web 管理面板？"), false)
	if err != nil || !ok {
		return err
	}
	wantUI, err := tui.Confirm(i18n.T("同时下载 Web 管理面板（浏览器查看 / 切换节点）？"), true)
	if err != nil {
		return err
	}
	suffix := ""
	if wantUI {
		suffix = " / Web UI"
	}
	execx.Info(i18n.T("下载/更新 内核 / geo 数据") + suffix + i18n.T("（出海慢时会用上面的代理）…"))
	ensureGithubToken(p)
	if _, err := kernel.DownloadAll(p, kernel.Options{Force: true, WithUI: wantUI}); err != nil {
		return err
	}
	execx.Info(i18n.T("已更新资源，重新部署运行时并重启服务…"))
	if err := sysd.Install(p, sysd.DefaultName, true); err != nil {
		return err
	}
	if wantUI {
		return maybeSetupWebui(p, t)
	}
	return nil
}

// maybeSetupWebui 可选：为面板启用『根路径直接打开』（独立静态服务）。
func maybeSetupWebui(p paths.Paths, t *txn.Transaction) error {
	ok, err := tui.Confirm(i18n.T("为面板启用『根路径直接打开』？（独立端口，浏览器开根地址即用，免去 /ui 后缀）"), true)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	lan := config.Bool(config.Load(p), "lan_panel")
	t.AddUndo(i18n.T("卸载独立 Web 面板"), sysd.RemoveWebUI)
	port, err := webuiSetupInteractive(p, 0, lan) // 内部已写回 customize.webui_port
	if err != nil {
		return err
	}
	if lan && port != 0 {
		fp := port
		t.AddUndo(i18n.T("撤销防火墙放行面板端口"), func() error { firewall.Revoke(fp); return nil })
	}
	return nil
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

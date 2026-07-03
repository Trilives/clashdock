// 初始化（首次部署）全流程（对应 flows/init.py）。
// 整个流程包在事务内：任意步骤 ESC / 出错都会回退已应用的改动。
package flows

import (
	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/firewall"
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
	return txn.Run("初始化", func(t *txn.Transaction) error {
		execx.Header("初始化（首次部署）")

		// 0. deb 种子接管（若从系统包安装，离线即可获得内核与基础规则）
		if _, err := kernel.SeedFromSystem(p); err != nil {
			execx.Warn("种子接管失败（不影响后续下载）：" + err.Error())
		}

		// 1. 局域网下载代理 / TUN / 局域网代理
		cfg := config.Load(p)
		proxy, err := tui.Ask(
			"下载代理 IP:端口（出海慢时走它，如 192.168.1.10:7890），留空=保留当前/无则直连",
			tui.AskOpts{Default: stripScheme(config.Str(cfg, "download_proxy")), AllowEmpty: true})
		if err != nil {
			return err
		}
		cfg["download_proxy"] = normalizeProxy(proxy)
		// TUN 模式：全局透明代理；关则纯代理，需各 App 自设代理
		enableTun, err := tui.Confirm("启用 TUN 模式？（整机流量自动走代理；否=纯代理，需各 App 手动设代理）",
			config.Bool(cfg, "enable_tun"))
		if err != nil {
			return err
		}
		cfg["enable_tun"] = enableTun
		lanProxy, err := tui.Confirm("开启局域网代理？（让局域网其他主机可用本机作为代理，监听 0.0.0.0:7890）",
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
			ok, err := tui.Confirm("把代理环境变量写入 ~/.bashrc？（新开终端自动走 127.0.0.1:7890）", true)
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
			ok, err := tui.Confirm("更新防火墙放行 7890 端口？", true)
			if err != nil {
				return err
			}
			if ok {
				t.AddUndo("撤销防火墙放行 7890", func() error { firewall.Revoke(firewall.ProxyPort); return nil })
				firewall.Allow(firewall.ProxyPort)
			}
		}

		// 2. 下载内核 + geo 数据（Web UI 可选）
		wantUI, err := tui.Confirm("下载 Web 管理面板（浏览器查看 / 切换节点）？", true)
		if err != nil {
			return err
		}
		suffix := ""
		if wantUI {
			suffix = "/ Web UI"
		}
		execx.Info("下载 内核 / geo 数据" + suffix + "（出海慢时会用上面的代理）…")
		ensureGithubToken(p)
		if _, err := kernel.DownloadAll(p, kernel.Options{WithUI: wantUI}); err != nil {
			return err
		}

		// 3. 添加首个订阅（链接留空=暂不配置，直接结束初始化）
		info, err := askNewSubscription()
		if err != nil {
			return err
		}
		if info == nil {
			execx.Info("已跳过订阅与服务注册，结束初始化。内核/规则已下载、设置已保存，" +
				"稍后可在主菜单「订阅 → 添加订阅」补配并启动服务。")
			return nil // 正常返回 → 事务提交，保留步骤 1-3 成果
		}
		if err := t.BackupFile(p.ConfigFile); err != nil {
			return err
		}
		if err := t.BackupFile(p.ActiveFile); err != nil {
			return err
		}
		sub, err := subscription.Add(p, info.Name, info.URL, info.SourceType, info.ApplyOverlay, true)
		if err != nil {
			return err
		}
		t.AddUndo("删除订阅 "+sub.Name, func() error { return subscription.RemoveSub(p, sub.Name) })

		// 4. 注册 systemd 服务
		t.AddUndo("卸载服务 mihomo", func() error { return sysd.Remove(p, sysd.DefaultName, true) })
		start, err := tui.Confirm("现在就启动服务？（否=仅设开机自启）", true)
		if err != nil {
			return err
		}
		if err := sysd.Install(p, sysd.DefaultName, start); err != nil {
			return err
		}

		// 5. 可选：独立 Web 面板（根路径直接打开）
		if wantUI {
			if err := maybeSetupWebui(p, t); err != nil {
				return err
			}
		}

		// 6. 可选增强：网络自愈 / 每周更新
		if err := optionalExtras(t); err != nil {
			return err
		}

		// 7. 提示切换 / 固定节点
		ok, err := tui.Confirm("订阅已配置，是否现在切换 / 固定节点？", false)
		if err != nil {
			return err
		}
		if ok {
			if err := NodeSelect(p, p.ConfigFile, ""); err != nil {
				return err
			}
		}

		execx.Ok("初始化完成。")
		printAccessHint(p)
		return nil
	})
}

// maybeSetupWebui 可选：为面板启用『根路径直接打开』（独立静态服务）。
func maybeSetupWebui(p paths.Paths, t *txn.Transaction) error {
	ok, err := tui.Confirm("为面板启用『根路径直接打开』？（独立端口，浏览器开根地址即用，免去 /ui 后缀）", true)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	lan := config.Bool(config.Load(p), "lan_panel")
	t.AddUndo("卸载独立 Web 面板", sysd.RemoveWebUI)
	port, err := webuiSetupInteractive(p, 0, lan) // 内部已写回 customize.webui_port
	if err != nil {
		return err
	}
	if lan && port != 0 {
		fp := port
		t.AddUndo("撤销防火墙放行面板端口", func() error { firewall.Revoke(fp); return nil })
	}
	return nil
}

func optionalExtras(t *txn.Transaction) error {
	ok, err := tui.Confirm("安装网络切换自愈？", true)
	if err != nil {
		return err
	}
	if ok {
		t.AddUndo("卸载网络自愈", func() error { return sysd.RemoveResilience(sysd.DefaultName) })
		if err := sysd.InstallResilience(sysd.ResilienceOptions{}); err != nil {
			return err
		}
	}
	ok, err = tui.Confirm("安装每周自动更新定时器？", false)
	if err != nil {
		return err
	}
	if ok {
		t.AddUndo("卸载每周更新", sysd.RemoveTimer)
		if err := sysd.InstallTimer(""); err != nil {
			return err
		}
	}
	return nil
}

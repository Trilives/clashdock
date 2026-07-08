// 初始化单屏表单：把「基础设置」（下载代理 / TUN / 局域网 / bashrc / 防火墙端口）与
// 「订阅设置」（名称 / 类型 / 链接或路径 / 拉取代理 / 自定义分流）汇总到同一张表单里
// 一次性填写，最后统一提交。字段随选择动态显隐（TUN 开 → 隐藏 bashrc；未开局域网 →
// 隐藏放行端口；本地 YAML → 链接改文件路径、隐藏拉取代理），终端过矮时自动滚动/分节。
// 见 internal/tui/form.go。
package flows

import (
	"strconv"
	"strings"
	"time"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/tui"
)

// initSettings 初始化表单的收集结果。
type initSettings struct {
	downloadProxy string
	proxyPort     int
	enableTun     bool
	lanProxy      bool
	writeBashrc   bool
	allowPort     bool

	hasSub bool
	sub    newSub
}

// portFromText 把端口文本解析为 1-65535 的端口号，非法/留空回退 def。
func portFromText(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n >= 1 && n <= 65535 {
		return n
	}
	return def
}

// runInitForm 展示初始化单屏表单并收集设置。includeSub=false 时只收集基础设置
// （复用现有订阅的重新初始化场景）。第二返回值为 false 表示用户取消。
func runInitForm(p paths.Paths, includeSub bool) (*initSettings, bool, error) {
	cfg := config.Load(p)
	tunOn := func(s *tui.FormState) bool { return s.Bool("enable_tun") }
	lanOn := func(s *tui.FormState) bool { return s.Bool("lan_proxy") }
	basic := i18n.T("基础设置")

	fields := []tui.Field{
		{Key: "download_proxy", Section: basic, Kind: tui.FieldText,
			Label: i18n.T("下载代理（IP:端口，留空=直连）"), AllowEmpty: true,
			Text: stripScheme(config.Str(cfg, "download_proxy")), Placeholder: "192.168.1.10:7890"},
		{Key: "proxy_port", Section: basic, Kind: tui.FieldText,
			Label: i18n.T("本地代理端口（默认 7890，被占用可改）"), Text: strconv.Itoa(config.ProxyPort(cfg))},
		{Key: "enable_tun", Section: basic, Kind: tui.FieldToggle,
			Label: i18n.T("启用 TUN 模式（整机流量自动走代理）"), Bool: config.Bool(cfg, "enable_tun")},
		{Key: "lan_proxy", Section: basic, Kind: tui.FieldToggle,
			Label: i18n.T("开启局域网代理（监听 0.0.0.0:7890）"), Bool: config.Bool(cfg, "lan_proxy")},
		// TUN 开启时整机流量已走代理，无需 bashrc 变量 → 隐藏。
		{Key: "write_bashrc", Section: basic, Kind: tui.FieldToggle,
			Label: i18n.T("把代理变量写入 ~/.bashrc"), Bool: true,
			Visible: func(s *tui.FormState) bool { return !tunOn(s) }},
		// 仅开启局域网代理时才需要放行防火墙端口 → 未开则隐藏。
		{Key: "allow_port", Section: basic, Kind: tui.FieldToggle,
			Label: i18n.T("放行防火墙 7890 端口"), Bool: true, Visible: lanOn},
	}
	if includeSub {
		sub := i18n.T("订阅设置")
		fields = append(fields,
			tui.Field{Key: "sub_name", Section: sub, Kind: tui.FieldText,
				Label: i18n.T("订阅名称"), Text: time.Now().Format("sub-20060102-150405")},
			tui.Field{Key: "sub_type", Section: sub, Kind: tui.FieldChoice,
				Label: i18n.T("订阅类型"), Choices: []string{"Clash / mihomo", "base64", i18n.T("本地 YAML")}, ChoiceIdx: 0},
			// 本地 YAML 类型时把「订阅链接」标签换成「文件路径」。
			tui.Field{Key: "sub_url", Section: sub, Kind: tui.FieldText, AllowEmpty: true,
				LabelFn: func(s *tui.FormState) string {
					if s.ChoiceIndex("sub_type") == 2 {
						return i18n.T("文件路径（留空=暂不配置）")
					}
					return i18n.T("订阅链接（留空=暂不配置）")
				}},
			// 本地 YAML 不联网拉取 → 隐藏「使用代理拉取」。
			tui.Field{Key: "fetch_proxy", Section: sub, Kind: tui.FieldToggle,
				Label: i18n.T("使用代理拉取订阅（默认否=直连）"), Bool: false,
				Visible: func(s *tui.FormState) bool { return s.ChoiceIndex("sub_type") != 2 }},
			tui.Field{Key: "overlay", Section: sub, Kind: tui.FieldToggle,
				Label: i18n.T("叠加自定义分流（AI / 流媒体 / 地区组）"), Bool: false},
		)
	}

	res, err := tui.Form(i18n.T("ClashDock 初始化"), fields,
		tui.FormOpts{SubmitLabel: i18n.T("开始初始化"), CancelLabel: i18n.T("取消")})
	if err != nil {
		return nil, false, err
	}
	if !res.Submitted {
		return nil, false, nil
	}

	s := &initSettings{
		downloadProxy: normalizeProxy(res.Text("download_proxy")),
		proxyPort:     portFromText(res.Text("proxy_port"), config.ProxyPort(cfg)),
		enableTun:     res.Bool("enable_tun"),
		lanProxy:      res.Bool("lan_proxy"),
		writeBashrc:   res.Bool("write_bashrc"),
		allowPort:     res.Bool("allow_port"),
	}
	if includeSub {
		if err := fillSubSetting(s, res); err != nil {
			return nil, false, err
		}
	}
	return s, true, nil
}

// fillSubSetting 把订阅相关表单字段转成 newSub（链接留空=暂不配置，hasSub=false）。
func fillSubSetting(s *initSettings, res *tui.FormResult) error {
	url := strings.TrimSpace(res.Text("sub_url"))
	if url == "" {
		return nil
	}
	sourceType := sourceTypes[res.ChoiceIndex("sub_type")]
	if sourceType == "local" {
		resolved, err := resolveLocalPath(url)
		if err != nil {
			return err
		}
		url = resolved
	}
	s.hasSub = true
	s.sub = newSub{
		Name:          res.Text("sub_name"),
		URL:           url,
		SourceType:    sourceType,
		ApplyOverlay:  res.Bool("overlay"),
		FetchViaProxy: sourceType != "local" && res.Bool("fetch_proxy"),
		// 初始化阶段主服务尚未运行，直连不会被自身 TUN 劫持，无需临时暂停。
		PauseForDirect: false,
	}
	return nil
}

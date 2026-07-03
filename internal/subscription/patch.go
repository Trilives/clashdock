// 直用订阅 + 最小改写：把机场原生 Clash/mihomo 订阅改写为可部署的运行时配置。
//
// 不做协议转换、不重建分流。订阅自带的 proxies / proxy-groups / rules /
// rule-providers / proxy-providers / dns 全部原样保留，只覆写「部署/运行时」必需字段：
//
//   - 本地代理端口（统一 mixed-port=7890，删除冲突的 port/socks-port/redir-port）
//   - 局域网开关（allow-lan / bind-address，由 lan_proxy 决定）
//   - 外部控制器与面板（external-controller / external-ui / secret，由 lan_panel 决定）
//   - TUN（按 enable_tun 整段覆写，由本部署层统一控制）
//   - 选组持久化（profile.store-selected）
//   - DNS（订阅自带则保留；缺失时注入可用的最小默认，TUN 模式需要）
//
// 输出为普通 map，由调用方以 JSON 写成 config.yaml（JSON 是合法 YAML，
// mihomo 直接解析，省掉 YAML dumper）。
package subscription

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/Trilives/clashdock/internal/i18n"
)

const (
	MixedPort           = 7890
	ControllerPort      = 9090
	TunDevice           = "mihomo"
	DefaultBootstrapDNS = "223.5.5.5"
)

// 默认 TUN 排除网段：本地 / 私网 / 常见 overlay（Tailscale 等），避免被 TUN 劫持。
// 须与 internal/config 的 DefaultTunRouteExclude 保持一致（与 Python 版同样刻意不互相引用）。
var defaultTunRouteExclude = []string{
	"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"169.254.0.0/16", "100.64.0.0/10", "::1/128", "fc00::/7", "fe80::/10",
}

// PatchError 订阅无法改写为运行时配置。
type PatchError struct{ Msg string }

func (e *PatchError) Error() string { return e.Msg }

// buildTun 按 enable_tun 构造 tun 段。关闭时仅 enable:false（纯代理模式）。
func buildTun(customize map[string]any) map[string]any {
	if !truthy(customize, "enable_tun", true) {
		return map[string]any{"enable": false}
	}
	exclude := strListOf(customize["tun_route_exclude_cidrs"])
	if len(exclude) == 0 {
		exclude = defaultTunRouteExclude
	}
	tun := map[string]any{
		"enable":                true,
		"stack":                 strOr(customize["tun_stack"], "gvisor"),
		"device":                TunDevice,
		"auto-route":            true,
		"auto-detect-interface": true,
		"dns-hijack":            []any{"any:53"},
		"route-exclude-address": toAnyList(exclude),
	}
	if uids := intListOf(customize["tun_exclude_uids"]); len(uids) > 0 {
		// mihomo 用 include/exclude 包级路由；UID 排除经 so-mark/规则，留作占位字段
		list := make([]any, 0, len(uids))
		for _, u := range uids {
			list = append(list, u)
		}
		tun["exclude-uid"] = list
	}
	return tun
}

// defaultDNS 订阅无 dns 段时注入的最小可用默认（fake-ip，配合 TUN）。
func defaultDNS(customize map[string]any) map[string]any {
	bootstrap := strOr(customize["bootstrap_dns_server"], DefaultBootstrapDNS)
	return map[string]any{
		"enable":             true,
		"listen":             "0.0.0.0:1053",
		"ipv6":               false,
		"enhanced-mode":      "fake-ip",
		"fake-ip-range":      "198.18.0.1/16",
		"fake-ip-filter":     []any{"*.lan", "*.local", "localhost.ptlogin2.qq.com"},
		"default-nameserver": []any{bootstrap},
		"nameserver":         []any{bootstrap, "https://doh.pub/dns-query"},
		"fallback":           []any{"https://1.1.1.1/dns-query", "https://dns.google/dns-query"},
	}
}

func toAnyList(ss []string) []any {
	out := make([]any, 0, len(ss))
	for _, s := range ss {
		out = append(out, s)
	}
	return out
}

// Apply 对机场 Clash 订阅 map 做最小改写，返回运行时 mihomo 配置 map。
// uiDir 为面板静态资源目录（external-ui 指向它）。业务字段引用原样保留。
func Apply(clash map[string]any, customize map[string]any, uiDir string) (map[string]any, error) {
	if clash == nil {
		return nil, &PatchError{i18n.T("订阅根必须是映射（YAML mapping）。")}
	}
	if proxies, ok := clash["proxies"].([]any); !ok || len(proxies) == 0 {
		return nil, &PatchError{i18n.T("订阅缺少 proxies 列表或为空，无法作为 mihomo 配置。")}
	}

	cfg := make(map[string]any, len(clash)+8) // 浅拷贝，业务字段引用保留
	for k, v := range clash {
		cfg[k] = v
	}

	// 1. 本地代理端口：统一 mixed-port，删除会冲突的其它入站端口
	delete(cfg, "port")
	delete(cfg, "socks-port")
	delete(cfg, "redir-port")
	delete(cfg, "tproxy-port")
	cfg["mixed-port"] = MixedPort

	// 2. 局域网代理
	lanProxy := truthy(customize, "lan_proxy", false)
	cfg["allow-lan"] = lanProxy
	if lanProxy {
		cfg["bind-address"] = "*"
	} else {
		delete(cfg, "bind-address")
	}

	// 3. 外部控制器 + 面板（默认仅本机；lan_panel 才放开）
	lanPanel := truthy(customize, "lan_panel", false)
	host := "127.0.0.1"
	if lanPanel {
		host = "0.0.0.0"
	}
	cfg["external-controller"] = fmt.Sprintf("%s:%d", host, ControllerPort)
	cfg["external-ui"] = uiDir
	secret := strOr(customize["secret"], "")
	switch {
	case secret != "":
		cfg["secret"] = secret
	case lanPanel:
		return nil, &PatchError{i18n.T("已开启 LAN 面板（lan_panel）但未设置 secret，拒绝在无密钥下放开控制器。")}
	default:
		delete(cfg, "secret")
	}

	// 4. mode / log-level 缺省兜底（订阅有则保留）
	if _, ok := cfg["mode"]; !ok {
		cfg["mode"] = "rule"
	}
	if _, ok := cfg["log-level"]; !ok {
		cfg["log-level"] = "warning"
	}

	// 5. 选组持久化
	profile := map[string]any{}
	if m, ok := cfg["profile"].(map[string]any); ok {
		for k, v := range m {
			profile[k] = v
		}
	}
	profile["store-selected"] = true
	cfg["profile"] = profile

	// 6. TUN：由本部署层整段覆写
	cfg["tun"] = buildTun(customize)

	// 7. DNS：订阅自带则保留；缺失才注入默认（TUN 模式需要可用 DNS）
	if m, ok := cfg["dns"].(map[string]any); !ok || len(m) == 0 {
		cfg["dns"] = defaultDNS(customize)
	}

	return cfg, nil
}

// Summarize 运行时配置 → 概要信息（节点 / 策略组 / 规则数 / TUN）。
func Summarize(config map[string]any) map[string]any {
	tun := false
	if m, ok := config["tun"].(map[string]any); ok {
		tun = truthyVal(m["enable"])
	}
	return map[string]any{
		"proxies":      lenList(config["proxies"]),
		"proxy_groups": lenList(config["proxy-groups"]),
		"rules":        lenList(config["rules"]),
		"tun":          tun,
	}
}

// Build Clash 配置 map → (运行时配置, 概要信息)。
func Build(clash, customize map[string]any, uiDir string) (map[string]any, map[string]any, error) {
	hasDNS := false
	if m, ok := clash["dns"].(map[string]any); ok && len(m) > 0 {
		hasDNS = true
	}
	cfg, err := Apply(clash, customize, uiDir)
	if err != nil {
		return nil, nil, err
	}
	info := Summarize(cfg)
	info["dns_from_subscription"] = hasDNS
	return cfg, info, nil
}

// FromClashYAML Clash YAML 文本 → (运行时配置, 概要信息)。
func FromClashYAML(text string, customize map[string]any, uiDir string) (map[string]any, map[string]any, error) {
	var data any
	if err := yaml.Unmarshal([]byte(text), &data); err != nil {
		return nil, nil, fmt.Errorf(i18n.T("解析订阅 YAML: %w"), err)
	}
	m, ok := data.(map[string]any)
	if !ok {
		return nil, nil, &PatchError{i18n.T("订阅 YAML 根必须是映射。")}
	}
	return Build(m, customize, uiDir)
}

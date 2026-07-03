// Package config 定制层（对应 Python 版 customize.py）：
// customize.json 的默认值、加载/保存与字段元数据。
//
// 字段分两组：部署字段（始终生效，决定订阅如何被改写为运行时配置）与
// 分流叠加字段（仅 enable_overlay 时生效）。消费方以 Get 系列容错取值，
// 不强依赖键的完整性。
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

// TUN 默认排除网段：本地 / 私网 / 常见 overlay（须与 subscription/patch 保持一致）。
var DefaultTunRouteExclude = []string{
	"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"169.254.0.0/16", "100.64.0.0/10", "::1/128", "fc00::/7", "fe80::/10",
}

var AIDomainSuffixes = []string{
	"openai.com", "chatgpt.com", "oaistatic.com", "oaiusercontent.com",
	"anthropic.com", "claude.ai", "gemini.google.com", "huggingface.co",
}

var StreamingDomainSuffixes = []string{
	"netflix.com", "nflxvideo.net", "disneyplus.com", "dssott.com",
	"hbomax.com", "max.com", "primevideo.com", "youtube.com",
	"googlevideo.com", "spotify.com",
}

var DefaultPreferKeywords = []string{"Singapore", "SG", "新加坡", "狮城"}
var DefaultHKPreferKeywords = []string{"Hong Kong", "HongKong", "HK", "香港"}

const DefaultSubconverterBackend = "https://sub.v1.mk"

// defaultsOrder 既是已知键集合，也是 customize.json 的写盘顺序（与 Python 版一致）。
var defaultsOrder = []string{
	// —— 部署字段（始终生效）——
	"enable_tun",
	"tun_stack",
	"tun_route_exclude_cidrs",
	"tun_exclude_uids",
	"lan_proxy",
	"lan_panel",
	"secret",
	"bootstrap_dns_server",
	"bootstrap_dns_port",
	"subconverter_backend",
	"base64_local_fallback",
	"github_mirror",
	"github_token",
	"download_proxy",
	"webui_port",
	"language",
	// —— 地区自动测速聚合组（各地区独立开关，不依赖 overlay）——
	"generate_sg_groups",
	"generate_hk_groups",
	// —— 分流叠加字段（仅 enable_overlay 时生效）——
	"enable_overlay",
	"ai_domain_suffixes",
	"streaming_domain_suffixes",
	"direct_domain_suffixes",
	"prefer_keywords",
	"hk_prefer_keywords",
}

// Defaults 返回一份全新的默认配置（列表均为拷贝，可放心修改）。
func Defaults() map[string]any {
	return map[string]any{
		"enable_tun":                true,
		"tun_stack":                 "gvisor",
		"tun_route_exclude_cidrs":   append([]string(nil), DefaultTunRouteExclude...),
		"tun_exclude_uids":          []int{},
		"lan_proxy":                 false,
		"lan_panel":                 false,
		"secret":                    "",
		"bootstrap_dns_server":      "223.5.5.5",
		"bootstrap_dns_port":        53,
		"subconverter_backend":      DefaultSubconverterBackend,
		"base64_local_fallback":     false,
		"github_mirror":             "",
		"github_token":              "",
		"download_proxy":            "",
		"webui_port":                9091,
		"language":                  "en",
		"generate_sg_groups":        false,
		"generate_hk_groups":        false,
		"enable_overlay":            false,
		"ai_domain_suffixes":        append([]string(nil), AIDomainSuffixes...),
		"streaming_domain_suffixes": append([]string(nil), StreamingDomainSuffixes...),
		"direct_domain_suffixes":    []string{},
		"prefer_keywords":           append([]string(nil), DefaultPreferKeywords...),
		"hk_prefer_keywords":        append([]string(nil), DefaultHKPreferKeywords...),
	}
}

// Load 读 customize.json：缺失字段以默认补全，未知键丢弃；读失败回退默认。
func Load(p paths.Paths) map[string]any {
	merged := Defaults()
	data, err := os.ReadFile(p.CustomizeFile)
	if err != nil {
		if !os.IsNotExist(err) {
			execx.Warn(i18n.T("customize.json 读取失败，使用默认值：") + err.Error())
		}
		return merged
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		execx.Warn(i18n.T("customize.json 解析失败，使用默认值：") + err.Error())
		return merged
	}
	for k, v := range raw {
		if _, known := merged[k]; known {
			merged[k] = v
		}
	}
	return merged
}

// Save 按固定键序写盘（2 空格缩进、非 ASCII 不转义，与 Python 版输出习惯一致）。
func Save(p paths.Paths, cfg map[string]any) error {
	if err := p.EnsureStateDirs(); err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString("{\n")
	first := true
	for _, k := range defaultsOrder {
		v, ok := cfg[k]
		if !ok {
			continue
		}
		val, err := marshalNoEscape(v, "  ", "  ")
		if err != nil {
			return fmt.Errorf(i18n.T("序列化字段 %s: %w"), k, err)
		}
		if !first {
			buf.WriteString(",\n")
		}
		first = false
		buf.WriteString("  " + strconv.Quote(k) + ": ")
		buf.Write(val)
	}
	buf.WriteString("\n}\n")
	return os.WriteFile(p.CustomizeFile, buf.Bytes(), 0o644)
}

// EnsureExists 首次运行时落地默认 customize.json。
func EnsureExists(p paths.Paths) (map[string]any, error) {
	cfg := Load(p)
	if _, err := os.Stat(p.CustomizeFile); os.IsNotExist(err) {
		if err := Save(p, cfg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func marshalNoEscape(v any, prefix, indent string) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent(prefix, indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// --------------------------------------------------------------------------
// 字段元数据（交互式编辑器与展示用）
// --------------------------------------------------------------------------

var ListFields = map[string]string{
	"tun_route_exclude_cidrs":   "TUN 排除网段",
	"tun_exclude_uids":          "TUN 排除 UID",
	"ai_domain_suffixes":        "AI 域名后缀（叠加）",
	"streaming_domain_suffixes": "流媒体域名后缀（叠加）",
	"direct_domain_suffixes":    "直连域名后缀（叠加）",
	"prefer_keywords":           "新加坡关键词（叠加）",
	"hk_prefer_keywords":        "香港关键词（叠加）",
}

var BoolFields = map[string]string{
	"enable_tun":            "TUN 模式（全局透明代理）",
	"lan_proxy":             "局域网代理（其他主机可用本机代理）",
	"lan_panel":             "LAN 面板暴露",
	"generate_sg_groups":    "生成新加坡自动测速聚合组（SG-Auto，可直接选用）",
	"generate_hk_groups":    "生成香港自动测速聚合组（HK-Auto，可直接选用）",
	"enable_overlay":        "启用自定义分流叠加（AI / 流媒体）",
	"base64_local_fallback": "base64 应急本地解析",
}

var ScalarFields = map[string]string{
	"tun_stack":            "TUN 协议栈（gvisor/system/mixed）",
	"secret":               "面板密钥 secret",
	"webui_port":           "独立 Web 面板端口（根路径直开）",
	"bootstrap_dns_server": "引导 DNS 服务器",
	"bootstrap_dns_port":   "引导 DNS 端口",
	"subconverter_backend": "subconverter 后端",
	"github_mirror":        "GitHub 加速前缀",
	"github_token":         "GitHub Token（提升 API 限额）",
	"download_proxy":       "下载代理",
}

// FieldOrder 编辑器展示顺序：常用部署项在前，叠加分流项在后。
var FieldOrder = []string{
	"enable_tun",
	"tun_stack",
	"lan_proxy",
	"lan_panel",
	"secret",
	"webui_port",
	"download_proxy",
	"github_mirror",
	"github_token",
	"subconverter_backend",
	"bootstrap_dns_server",
	"bootstrap_dns_port",
	"tun_route_exclude_cidrs",
	"tun_exclude_uids",
	"base64_local_fallback",
	"generate_sg_groups",
	"generate_hk_groups",
	"prefer_keywords",
	"hk_prefer_keywords",
	"enable_overlay",
	"ai_domain_suffixes",
	"streaming_domain_suffixes",
	"direct_domain_suffixes",
}

// SensitiveFields 涉密字段：菜单展示与编辑提示里都不出现明文。
var SensitiveFields = map[string]bool{"secret": true, "github_token": true}

// MaskSecret 已设置密钥的脱敏展示（保留末 4 位）。
func MaskSecret(v string) string {
	r := []rune(v)
	if len(r) > 4 {
		return fmt.Sprintf(i18n.T("已设置（***%s）"), string(r[len(r)-4:]))
	}
	return i18n.T("已设置（***）")
}

// Summary 字段值的单行摘要（列表→条数，布尔→开/关，涉密→脱敏）。
func Summary(cfg map[string]any, key string) string {
	v, ok := cfg[key]
	if !ok {
		v = Defaults()[key]
	}
	switch x := v.(type) {
	case nil:
		return i18n.T("未设置")
	case bool:
		if x {
			return i18n.T("开")
		}
		return i18n.T("关")
	case []any:
		if len(x) == 0 {
			return i18n.T("空")
		}
		return fmt.Sprintf(i18n.T("%d 条"), len(x))
	case []string:
		if len(x) == 0 {
			return i18n.T("空")
		}
		return fmt.Sprintf(i18n.T("%d 条"), len(x))
	case []int:
		if len(x) == 0 {
			return i18n.T("空")
		}
		return fmt.Sprintf(i18n.T("%d 条"), len(x))
	}
	s := toString(v)
	if s == "" {
		return i18n.T("未设置")
	}
	if SensitiveFields[key] {
		return MaskSecret(s)
	}
	return s
}

// FieldLabel 编辑器里的整行标签（名称 + 摘要）。
func FieldLabel(cfg map[string]any, key string) string {
	if label, ok := ListFields[key]; ok {
		return fmt.Sprintf(i18n.T("%s（%s）"), i18n.T(label), Summary(cfg, key))
	}
	if label, ok := BoolFields[key]; ok {
		return fmt.Sprintf(i18n.T("%s：%s"), i18n.T(label), Summary(cfg, key))
	}
	return fmt.Sprintf(i18n.T("%s：%s"), i18n.T(ScalarFields[key]), Summary(cfg, key))
}

// --------------------------------------------------------------------------
// 容错取值（JSON 反序列化后数字为 float64，此处统一收敛类型）
// --------------------------------------------------------------------------

func Bool(cfg map[string]any, key string) bool {
	v, _ := cfg[key].(bool)
	return v
}

func Str(cfg map[string]any, key string) string {
	v, ok := cfg[key]
	if !ok || v == nil {
		return ""
	}
	return toString(v)
}

func Int(cfg map[string]any, key string) int {
	switch x := cfg[key].(type) {
	case int:
		return x
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}

func StrList(cfg map[string]any, key string) []string {
	switch x := cfg[key].(type) {
	case []string:
		return append([]string(nil), x...)
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			out = append(out, toString(e))
		}
		return out
	}
	return nil
}

func IntList(cfg map[string]any, key string) []int {
	switch x := cfg[key].(type) {
	case []int:
		return append([]int(nil), x...)
	case []any:
		out := make([]int, 0, len(x))
		for _, e := range x {
			switch n := e.(type) {
			case float64:
				out = append(out, int(n))
			case int:
				out = append(out, n)
			}
		}
		return out
	}
	return nil
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	case int:
		return strconv.Itoa(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// 来源类型识别：校验拉取内容与用户所选类型是否相符（也支持启发式判断）。
package subscription

import (
	"encoding/base64"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Trilives/clashdock/internal/i18n"
)

// Detect 启发式判断订阅类型：返回 clash | base64 | unknown。
//
// mihomo 直用机场订阅：clash/mihomo 订阅均为 Clash YAML（同一类型）；
// 通用节点订阅为 base64。
func Detect(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "unknown"
	}

	// clash：YAML 且含 proxies 列表
	if strings.Contains(text, "proxies:") || strings.Contains(text, "proxy-groups:") {
		var d any
		if err := yaml.Unmarshal([]byte(text), &d); err == nil {
			if m, ok := d.(map[string]any); ok {
				if _, ok := m["proxies"].([]any); ok {
					return "clash"
				}
			}
		}
	}

	// base64：可解码且含节点分享链接
	if looksBase64(text) {
		compact := strings.Join(strings.Fields(text), "")
		if pad := len(compact) % 4; pad != 0 {
			compact += strings.Repeat("=", 4-pad)
		}
		if decoded, err := base64.StdEncoding.DecodeString(compact); err == nil {
			if strings.Contains(string(decoded), "://") {
				return "base64"
			}
		}
	}

	return "unknown"
}

func looksBase64(text string) bool {
	sample := strings.Join(strings.Fields(text), "")
	if len(sample) < 16 {
		return false
	}
	const allowed = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=-_"
	for _, c := range sample {
		if !strings.ContainsRune(allowed, c) {
			return false
		}
	}
	return true
}

// WarnIfMismatch 若检测类型与声明不符，返回提示文本；相符或无法判断返回空串。
func WarnIfMismatch(declared string, raw []byte) string {
	found := Detect(raw)
	if found != "unknown" && found != declared {
		return fmt.Sprintf(i18n.T("内容看起来更像「%s」而非你选择的「%s」。"), found, declared)
	}
	return ""
}

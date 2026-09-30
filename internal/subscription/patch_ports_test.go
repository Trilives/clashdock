package subscription

import "testing"

// 「部署设置」里编辑端口走文本输入，历史上会以字符串写进 customize.json；
// 生成运行时配置时必须仍按该端口监听，而不是静默回退默认值。
func TestProxyPortAcceptsStoredString(t *testing.T) {
	cfg := mustApply(t, patchSample(t), map[string]any{"enable_tun": false, "proxy_port": "7891"})
	if cfg["mixed-port"] != 7891 {
		t.Fatalf("mixed-port = %v, want 7891 from string proxy_port", cfg["mixed-port"])
	}
}

func TestControllerPortFollowsCustomize(t *testing.T) {
	tests := []struct {
		name      string
		customize map[string]any
		want      string
	}{
		{"default", map[string]any{}, "127.0.0.1:9090"},
		{"custom int", map[string]any{"controller_port": 19090}, "127.0.0.1:19090"},
		{"custom json number", map[string]any{"controller_port": float64(19091)}, "127.0.0.1:19091"},
		{"out of range falls back", map[string]any{"controller_port": 70000}, "127.0.0.1:9090"},
		{"lan panel keeps port", map[string]any{"controller_port": 19092, "lan_panel": true, "secret": "s"}, "0.0.0.0:19092"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mustApply(t, patchSample(t), tt.customize)
			if cfg["external-controller"] != tt.want {
				t.Fatalf("external-controller = %v, want %s", cfg["external-controller"], tt.want)
			}
		})
	}
}

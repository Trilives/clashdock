package subscription

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// 与 Python 版 tests/test_patch.py 对应的行为测试。
// 一份「机场原配置」样本：含会被覆写的端口/控制器，与应保留的业务字段。
const patchSampleJSON = `{
  "port": 7890,
  "socks-port": 7891,
  "mixed-port": 7893,
  "allow-lan": true,
  "external-controller": "0.0.0.0:9090",
  "mode": "rule",
  "dns": {"enable": true, "nameserver": ["223.5.5.5"]},
  "proxies": [
    {"name": "hk-01", "type": "ss", "server": "1.2.3.4", "port": 8388,
     "cipher": "aes-256-gcm", "password": "pw"}
  ],
  "proxy-groups": [
    {"name": "Proxies", "type": "select", "proxies": ["hk-01", "DIRECT"]}
  ],
  "rules": ["GEOIP,CN,DIRECT", "MATCH,Proxies"]
}`

const testUIDir = "/test-root/state/ui"

func patchSample(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(patchSampleJSON), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func mustApply(t *testing.T, clash, customize map[string]any) map[string]any {
	t.Helper()
	cfg, err := Apply(clash, customize, testUIDir)
	if err != nil {
		t.Fatal("Apply 失败:", err)
	}
	return cfg
}

func TestMinimalRewrite(t *testing.T) {
	sample := patchSample(t)
	cfg := mustApply(t, sample, map[string]any{"enable_tun": true})

	if cfg["mixed-port"] != MixedPort {
		t.Error("mixed-port 应统一为 7890")
	}
	if _, ok := cfg["port"]; ok {
		t.Error("应删除冲突的 port")
	}
	if _, ok := cfg["socks-port"]; ok {
		t.Error("应删除冲突的 socks-port")
	}
	if cfg["external-controller"] != "127.0.0.1:9090" {
		t.Error("控制器默认应收回本机")
	}
	if cfg["allow-lan"] != false {
		t.Error("allow-lan 默认应关")
	}
	if !strings.HasSuffix(cfg["external-ui"].(string), "/ui") {
		t.Error("external-ui 应指向 ui 目录")
	}
	profile := cfg["profile"].(map[string]any)
	if profile["store-selected"] != true {
		t.Error("应开启选组持久化")
	}
	// 业务字段原样保留
	if lenList(cfg["proxies"]) != 1 {
		t.Error("proxies 应原样保留")
	}
	if lenList(cfg["proxy-groups"]) != 1 || lenList(cfg["rules"]) != 2 {
		t.Error("proxy-groups / rules 应原样保留")
	}
	dns := cfg["dns"].(map[string]any)
	if dns["enable"] != true || lenList(dns["nameserver"]) != 1 {
		t.Error("订阅自带 dns 应原样保留")
	}
}

func TestTunToggle(t *testing.T) {
	on := mustApply(t, patchSample(t), map[string]any{"enable_tun": true})
	tun := on["tun"].(map[string]any)
	if tun["enable"] != true {
		t.Error("开启时 tun.enable 应为 true")
	}
	if tun["device"] != TunDevice {
		t.Error("tun 设备名应为 mihomo")
	}
	if tun["stack"] != "gvisor" {
		t.Error("默认 stack 应为 gvisor")
	}

	off := mustApply(t, patchSample(t), map[string]any{"enable_tun": false})
	offTun := off["tun"].(map[string]any)
	if len(offTun) != 1 || offTun["enable"] != false {
		t.Error("关闭时应仅 enable:false")
	}
}

func TestTunExcludeUIDs(t *testing.T) {
	cfg := mustApply(t, patchSample(t), map[string]any{"enable_tun": true, "tun_exclude_uids": []int{0, 1000}})
	tun := cfg["tun"].(map[string]any)
	if lenList(tun["exclude-uid"]) != 2 {
		t.Fatalf("应写入 2 个 exclude-uid，实际 %v", tun["exclude-uid"])
	}
}

func TestTunExcludeProcess(t *testing.T) {
	cfg := mustApply(t, patchSample(t), map[string]any{
		"enable_tun": true, "tun_exclude_process": []string{"sshd", " ", "mosh"}})
	rules := cfg["rules"].([]any)
	// 两条进程直连规则应插到订阅原有 2 条规则最前面（空项被跳过）。
	if len(rules) != 4 {
		t.Fatalf("期望 4 条规则，实际 %d：%v", len(rules), rules)
	}
	if rules[0] != "PROCESS-NAME,sshd,DIRECT" || rules[1] != "PROCESS-NAME,mosh,DIRECT" {
		t.Errorf("进程直连规则应在最前，实际 %v", rules[:2])
	}
	if rules[2] != "GEOIP,CN,DIRECT" {
		t.Errorf("原订阅规则应保留在其后，实际 %v", rules[2])
	}
}

func TestTunExcludeProcessSkippedWhenTunOff(t *testing.T) {
	cfg := mustApply(t, patchSample(t), map[string]any{
		"enable_tun": false, "tun_exclude_process": []string{"sshd"}})
	if lenList(cfg["rules"]) != 2 {
		t.Errorf("关 TUN 时不应注入进程直连规则，实际 %v", cfg["rules"])
	}
}

func TestLanPanelGuard(t *testing.T) {
	_, err := Apply(patchSample(t), map[string]any{"lan_panel": true}, testUIDir)
	var pe *PatchError
	if !errors.As(err, &pe) {
		t.Fatal("lan_panel 无 secret 应返回 PatchError")
	}

	ok := mustApply(t, patchSample(t), map[string]any{"lan_panel": true, "secret": "s3cr3t"})
	if ok["external-controller"] != "0.0.0.0:9090" {
		t.Error("lan_panel 应放开控制器")
	}
	if ok["secret"] != "s3cr3t" {
		t.Error("应写入 secret")
	}
}

func TestLanProxy(t *testing.T) {
	on := mustApply(t, patchSample(t), map[string]any{"lan_proxy": true})
	if on["allow-lan"] != true || on["bind-address"] != "*" {
		t.Error("lan_proxy 应开放监听")
	}
}

func TestDefaultDNSWhenMissing(t *testing.T) {
	sample := patchSample(t)
	delete(sample, "dns")
	cfg := mustApply(t, sample, map[string]any{
		"enable_tun": true,
		"fake_ip_filter": []string{"*.lan", "+.example.com"},
	})
	dns := cfg["dns"].(map[string]any)
	if dns["enable"] != true {
		t.Error("应注入默认 dns")
	}
	if dns["enhanced-mode"] != "fake-ip" {
		t.Error("默认应为 fake-ip")
	}
	filters := strListOf(dns["fake-ip-filter"])
	if len(filters) != 2 || filters[0] != "*.lan" || filters[1] != "+.example.com" {
		t.Fatalf("默认 DNS 应使用定制 fake-ip-filter，实际 %v", filters)
	}
}

func TestFakeIPFilterOverridesSubscriptionDNSFieldOnly(t *testing.T) {
	cfg := mustApply(t, patchSample(t), map[string]any{
		"enable_tun": true,
		"fake_ip_filter": []string{"*.lan", "+.internal.example"},
	})
	dns := cfg["dns"].(map[string]any)
	filters := strListOf(dns["fake-ip-filter"])
	if len(filters) != 2 || filters[1] != "+.internal.example" {
		t.Fatalf("应覆写订阅 fake-ip-filter，实际 %v", filters)
	}
	if nameservers := strListOf(dns["nameserver"]); len(nameservers) != 1 || nameservers[0] != "223.5.5.5" {
		t.Fatalf("订阅 DNS 其它字段应保留，实际 %v", dns)
	}
}

func TestRejectEmptyProxies(t *testing.T) {
	_, err := Apply(map[string]any{"proxies": []any{}}, map[string]any{}, testUIDir)
	var pe *PatchError
	if !errors.As(err, &pe) {
		t.Fatal("空 proxies 应返回 PatchError")
	}
}

func TestFromClashYAML(t *testing.T) {
	yamlText := `
proxies:
  - {name: hk-01, type: ss, server: 1.2.3.4, port: 8388}
proxy-groups:
  - name: Proxies
    type: select
    proxies: [hk-01, DIRECT]
rules:
  - GEOIP,CN,DIRECT
`
	cfg, info, err := FromClashYAML(yamlText, map[string]any{"enable_tun": true}, testUIDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg["mixed-port"] != MixedPort {
		t.Error("YAML 路径也应统一 mixed-port")
	}
	if info["proxies"] != 1 || info["tun"] != true {
		t.Errorf("概要信息不符: %v", info)
	}
	if info["dns_from_subscription"] != false {
		t.Error("无 dns 订阅应标记 dns_from_subscription=false")
	}
}

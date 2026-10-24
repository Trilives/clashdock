package subscription

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/configfile"
)

// 真实内核门禁：设置 MIHOMO_BIN 时，把代表性生成配置交给 `mihomo -t`（CI 由测试工作流下载）。
// geo 资源取自 MIHOMO_GEO_DIR，默认 packaging/deps/rules（scripts/fetch-deb-deps.sh 下载）。

const defaultGeoDir = "../../packaging/deps/rules"

// realKernelGeoAssets 校验目录内的 geo 文件名 → 源文件；缺任何一个返回 nil。
func realKernelGeoAssets() map[string]string {
	dir := os.Getenv("MIHOMO_GEO_DIR")
	if dir == "" {
		dir = defaultGeoDir
	}
	assets := map[string]string{
		"country.mmdb": filepath.Join(dir, "country.mmdb"),
		"geosite.dat":  filepath.Join(dir, "geosite.dat"),
	}
	for _, src := range assets {
		if _, err := os.Stat(src); err != nil {
			return nil
		}
	}
	return assets
}

// offlineGeoURLs geo 数据缺失时内核会联网补下载；指向本机不可达端口，缺了什么立即失败，
// 用例不依赖网络，也不会被下载结果掩盖。
var offlineGeoURLs = map[string]any{
	"geoip":   "http://127.0.0.1:9/geoip.dat",
	"geosite": "http://127.0.0.1:9/geosite.dat",
	"mmdb":    "http://127.0.0.1:9/country.mmdb",
	"asn":     "http://127.0.0.1:9/asn.mmdb",
}

func offline(t *testing.T, cfg map[string]any) map[string]any {
	t.Helper()
	out := deepCopy(t, cfg)
	out["geox-url"] = offlineGeoURLs
	return out
}

// dnsNeedsGeoIP mihomo 的 fallback-filter 默认 geoip: true：配置了 fallback 的 DNS 不写
// fallback-filter 也依赖 GeoIP 数据（如 Build 补的默认 DNS），去掉 geo 规则也离不开 geo 资源。
func dnsNeedsGeoIP(cfg map[string]any) bool {
	dns, _ := cfg["dns"].(map[string]any)
	if fallback, _ := dns["fallback"].([]any); len(fallback) == 0 {
		return false
	}
	filter, _ := dns["fallback-filter"].(map[string]any)
	geoip, set := filter["geoip"].(bool)
	return !set || geoip
}

// withoutGeoRules 去掉依赖 geo 数据的规则，代表不用 geo 规则的订阅。
func withoutGeoRules(t *testing.T, cfg map[string]any) map[string]any {
	t.Helper()
	out := offline(t, cfg)
	rules, _ := out["rules"].([]any)
	kept := make([]any, 0, len(rules))
	for _, r := range rules {
		s, _ := r.(string)
		if strings.HasPrefix(s, "GEOIP,") || strings.HasPrefix(s, "GEOSITE,") {
			continue
		}
		kept = append(kept, r)
	}
	out["rules"] = kept
	return out
}

// generatedVariants 每个可生成的 golden 订阅：直用改写，以及叠加 overlay 与地区聚合组。
func generatedVariants(t *testing.T) map[string]map[string]any {
	t.Helper()
	g := loadGolden(t)
	overlay := g.Overlay[0].Customize
	out := map[string]map[string]any{}
	for _, c := range g.Patch {
		if c.Error != "" {
			continue
		}
		cfg, _, err := Build(deepCopy(t, c.Input), c.Customize, "/var/lib/clashdock/ui")
		if err != nil {
			t.Fatalf("[%s] Build: %v", c.Name, err)
		}
		out[c.Name] = cfg
		merged := map[string]any{}
		for k, v := range c.Customize {
			merged[k] = v
		}
		for k, v := range overlay {
			merged[k] = v
		}
		full, _ := ApplyOverlay(deepCopy(t, cfg), merged)
		full, _ = ApplyRegionGroups(full, merged)
		out[c.Name+"+overlay+regions"] = full
	}
	return out
}

func TestGeneratedConfigsPassRealKernel(t *testing.T) {
	bin := os.Getenv("MIHOMO_BIN")
	if bin == "" {
		t.Skip("MIHOMO_BIN not set")
	}
	geo := realKernelGeoAssets()
	for name, cfg := range generatedVariants(t) {
		t.Run(name+"/no-geo", func(t *testing.T) {
			stripped := withoutGeoRules(t, cfg)
			if dnsNeedsGeoIP(stripped) {
				t.Skip("DNS fallback-filter relies on GeoIP; covered by the geo variant")
			}
			if err := configfile.Precheck(bin, t.TempDir(), stripped, nil); err != nil {
				t.Fatal(err)
			}
		})
		t.Run(name+"/geo", func(t *testing.T) {
			if geo == nil {
				t.Skip("geo assets not found; set MIHOMO_GEO_DIR or run scripts/fetch-deb-deps.sh")
			}
			if err := configfile.Precheck(bin, t.TempDir(), offline(t, cfg), geo); err != nil {
				t.Fatal(err)
			}
		})
	}
}

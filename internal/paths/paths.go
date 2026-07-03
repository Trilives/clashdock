// Package paths 统一路径常量。
//
// 运行期所有数据使用**固定工作目录** /var/lib/clashdock（不随用户/HOME 变化，
// root 运行的定时器与用户会话看到同一份数据）；环境变量 CLASHDOCK_HOME 可覆盖
// （主要用于测试）。首次使用由 flows.EnsureStateRoot 负责创建并交回调用者属主。
package paths

import (
	"os"
	"path/filepath"
)

// DefaultStateRoot 固定数据目录。
const DefaultStateRoot = "/var/lib/clashdock"

// EtcDir 系统侧运行时目录（与旧版一致，老部署无缝接管）。
const EtcDir = "/etc/mihomo"

// Paths 全部运行期路径；由 Detect 依据环境变量解析一次后传递使用。
type Paths struct {
	State         string
	Bin           string
	MihomoBin     string
	MihomoVersion string
	UI            string
	Ruleset       string
	Downloads     string
	Subscriptions string
	ActiveFile    string
	ConfigFile    string // 生效配置：内容为 JSON（合法 YAML，mihomo 直接解析）
	CustomizeFile string
	GeositeDat    string
	GeoipMetadb   string
	CountryMmdb   string // DB-IP Lite 种子（deb 附带，可再分发）；metadb 缺失时的兜底
}

func stateRoot() string {
	if v := os.Getenv("CLASHDOCK_HOME"); v != "" {
		return v
	}
	return DefaultStateRoot
}

// Detect 依据环境变量解析全部路径。
func Detect() Paths {
	s := stateRoot()
	bin := filepath.Join(s, "bin")
	rs := filepath.Join(s, "ruleset")
	return Paths{
		State:         s,
		Bin:           bin,
		MihomoBin:     filepath.Join(bin, "mihomo"),
		MihomoVersion: filepath.Join(bin, "mihomo.version"),
		UI:            filepath.Join(s, "ui"),
		Ruleset:       rs,
		Downloads:     filepath.Join(s, "downloads"),
		Subscriptions: filepath.Join(s, "subscriptions"),
		ActiveFile:    filepath.Join(s, "active"),
		ConfigFile:    filepath.Join(s, "config.yaml"),
		CustomizeFile: filepath.Join(s, "customize.json"),
		GeositeDat:    filepath.Join(rs, "geosite.dat"),
		GeoipMetadb:   filepath.Join(rs, "geoip.metadb"),
		CountryMmdb:   filepath.Join(rs, "country.mmdb"),
	}
}

// EnsureStateDirs 创建所有运行期目录（幂等）。
func (p Paths) EnsureStateDirs() error {
	for _, d := range []string{p.State, p.Bin, p.UI, p.Ruleset, p.Downloads, p.Subscriptions} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// SubscriptionDir 某订阅的存储目录。
func (p Paths) SubscriptionDir(name string) string {
	return filepath.Join(p.Subscriptions, name)
}

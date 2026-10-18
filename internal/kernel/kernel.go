// Package kernel 核心资源下载/更新（对应 core.py）：mihomo 内核 + Web UI + geo 数据。
//
// 下载相关设置（download_proxy / github_mirror / github_token）读 customize.json，
// 未配置回退环境变量（DOWNLOAD_PROXY、http_proxy 系、GITHUB_TOKEN/GH_TOKEN）。
// 另提供 deb 种子接管：/usr/libexec/clashdock 与 /usr/share/clashdock/ruleset 里
// 打包附带的 mihomo 与基础规则文件，在 state 缺失时复制为初始资源，安装后离线即可启动。
package kernel

import (
	"fmt"
	"os"
	"strings"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/fetchx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

const (
	MihomoRepo = "MetaCubeX/mihomo"
	UIRepo     = "MetaCubeX/metacubexd"

	geoBase    = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest"
	GeoipURL   = geoBase + "/geoip.metadb"
	GeositeURL = geoBase + "/geosite.dat"
)

// deb 包内种子位置（见 .goreleaser nfpm 配置；许可见 /usr/share/doc/clashdock/copyright）。
const (
	SeedBinDir     = "/usr/libexec/clashdock"
	SeedRulesetDir = "/usr/share/clashdock/ruleset"
)

// Settings 下载相关设置。
type Settings struct {
	DownloadProxy string
	GithubMirror  string
	GithubToken   string
}

// LoadSettings 读 customize.json 的下载字段，缺失回退环境变量。
func LoadSettings(p paths.Paths) Settings {
	cfg := config.Load(p)
	proxy := config.Str(cfg, "download_proxy")
	if proxy == "" {
		proxy = os.Getenv("DOWNLOAD_PROXY")
	}
	if proxy == "" {
		proxy = ambientProxy()
	}
	token := config.Str(cfg, "github_token")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	return Settings{
		DownloadProxy: strings.TrimSpace(proxy),
		GithubMirror:  strings.TrimSpace(config.Str(cfg, "github_mirror")),
		GithubToken:   strings.TrimSpace(token),
	}
}

// ambientProxy 当前 shell 的代理环境变量（proxyenv 写入 bashrc 的那套），
// 仅作 download_proxy 的隐式回退；fetchx 直连可达时会彻底绕过它。
func ambientProxy() string {
	for _, v := range []string{"https_proxy", "HTTPS_PROXY", "all_proxy", "ALL_PROXY", "http_proxy", "HTTP_PROXY"} {
		if s := strings.TrimSpace(os.Getenv(v)); s != "" {
			return s
		}
	}
	return ""
}

// Mirror 对 GitHub 下载/raw 链接套加速前缀；api.github.com 不套（多数镜像不代理 API）。
func Mirror(rawURL, mirror string) string {
	if mirror == "" || strings.Contains(rawURL, "api.github.com") {
		return rawURL
	}
	if strings.HasPrefix(rawURL, "https://github.com/") || strings.HasPrefix(rawURL, "https://raw.githubusercontent.com/") {
		return strings.TrimRight(mirror, "/") + "/" + rawURL
	}
	return rawURL
}

// NewFetcher 按设置构造下载器（Token 缺失时的交互式补充由上层流程负责）。
func NewFetcher(p paths.Paths) (*fetchx.Fetcher, Settings) {
	s := LoadSettings(p)
	if s.DownloadProxy != "" {
		execx.Info(i18n.T("下载代理（直连不可用时回退）: ") + s.DownloadProxy)
	}
	return fetchx.New(s.DownloadProxy, s.GithubToken), s
}

// localMixedPortProxy 本机 mihomo mixed-port 的 HTTP 代理地址（服务已启动时可用）；
// 端口取 customize 的 proxy_port（默认 7890）。
func localMixedPortProxy(p paths.Paths) string {
	return fmt.Sprintf("http://127.0.0.1:%d", config.ProxyPort(config.Load(p)))
}

// newLocalProxyFirstFetcher 服务已启动场景专用：优先走本机 mixed-port（走已生效
// 订阅的节点，出海更稳），失败再回退配置的 download_proxy，最后才直连。
func newLocalProxyFirstFetcher(p paths.Paths) (*fetchx.Fetcher, Settings) {
	s := LoadSettings(p)
	return fetchx.NewOrdered([]string{localMixedPortProxy(p), s.DownloadProxy}, s.GithubToken), s
}

// Options DownloadAll 的选项。
type Options struct {
	Compatible bool
	Force      bool
	WithUI     bool

	// SkipCore 跳过内核下载，只更新 geo 数据（+ 可选 Web UI）。内核随安装包捆绑，
	// 初始化不再下载内核；内核更新由用户在「工具 → 更新 → 内核」显式触发。
	SkipCore bool

	// LocalProxyFirst 优先走本机 mixed-port（127.0.0.1:7890）下载，失败再回退
	// download_proxy、最后直连——仅适用于主服务已启动之后的资源更新场景。
	LocalProxyFirst bool
}

// DownloadAll 下载内核 + geo 数据（+ 可选 Web UI），返回内核版本；SkipCore 时
// 跳过内核，只更新 geo/UI，返回本地已记录的内核版本（best-effort）。
func DownloadAll(p paths.Paths, opts Options) (string, error) {
	var f *fetchx.Fetcher
	var s Settings
	if opts.LocalProxyFirst {
		f, s = newLocalProxyFirstFetcher(p)
	} else {
		f, s = NewFetcher(p)
	}
	var version string
	if opts.SkipCore {
		version = localCoreVersion(p)
	} else {
		v, err := UpdateCore(p, f, s, opts.Compatible, opts.Force)
		if err != nil {
			return "", err
		}
		version = v
	}
	if err := UpdateGeodata(p, f, s, opts.Force); err != nil {
		return version, err
	}
	if opts.WithUI {
		// Web UI 是可选附加项（内核 + geo 数据已足够启动服务）：下载/解包失败
		// 只警告，不让整个更新/初始化操作因这一个可选子步骤失败而回退。
		if err := UpdateUI(p, f, s, opts.Force); err != nil {
			execx.Warn(fmt.Sprintf(i18n.T("Web UI 更新失败（不影响内核与 geo 数据，可稍后重试）：%v"), err))
		}
	}
	return version, nil
}

// localCoreVersion 读取本地记录的内核版本（<state>/.../mihomo.version），
// 缺失时返回空串。仅用于 SkipCore 场景下的信息展示。
func localCoreVersion(p paths.Paths) string {
	b, err := os.ReadFile(p.MihomoVersion)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

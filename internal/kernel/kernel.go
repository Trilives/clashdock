// Package kernel 核心资源下载/更新（对应 core.py）：mihomo 内核 + Web UI + geo 数据。
//
// 下载相关设置（download_proxy / github_mirror / github_token）读 customize.json，
// 未配置回退环境变量（DOWNLOAD_PROXY、http_proxy 系、GITHUB_TOKEN/GH_TOKEN）。
// 另提供 deb 种子接管：/usr/libexec/clashdock 与 /usr/share/clashdock/ruleset 里
// 打包附带的 mihomo 与基础规则文件，在 state 缺失时复制为初始资源，安装后离线即可启动。
package kernel

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/fetchx"
	"github.com/Trilives/clashdock/internal/firewall"
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

// archName Go arch → mihomo 资产命名（本二进制按目标架构编译，GOARCH 即目标机架构）。
func archName() string {
	if runtime.GOARCH == "arm" {
		return "armv7"
	}
	return runtime.GOARCH
}

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func latestRelease(f *fetchx.Fetcher, repo string) (release, error) {
	var rel release
	err := f.ReadJSON("https://api.github.com/repos/"+repo+"/releases/latest", &rel)
	return rel, err
}

func assetURLs(rel release) []string {
	urls := make([]string, 0, len(rel.Assets))
	for _, a := range rel.Assets {
		urls = append(urls, a.BrowserDownloadURL)
	}
	return urls
}

func pickAsset(urls []string, pattern string) string {
	rx := regexp.MustCompile("(?i)" + pattern)
	for _, u := range urls {
		if rx.MatchString(u) {
			return u
		}
	}
	return ""
}

// pickMihomoAsset 选 mihomo Linux 内核 .gz 资产：默认标准包（最大兼容），
// compatible=true 优先老 CPU 兜底包；兜底匹配任意 arch 变体的 .gz。
func pickMihomoAsset(urls []string, arch, version string, compatible bool) string {
	v := regexp.QuoteMeta(version)
	std := fmt.Sprintf(`mihomo-linux-%s-%s\.gz$`, arch, v)
	compat := fmt.Sprintf(`mihomo-linux-%s-compatible-%s\.gz$`, arch, v)
	order := []string{std, compat}
	if compatible {
		order = []string{compat, std}
	}
	order = append(order,
		fmt.Sprintf(`mihomo-linux-%s[^/]*-%s\.gz$`, arch, v),
		fmt.Sprintf(`mihomo-linux-%s[^/]*\.gz$`, arch),
	)
	for _, pat := range order {
		if u := pickAsset(urls, pat); u != "" {
			return u
		}
	}
	return ""
}

// --------------------------------------------------------------------------
// 下载 + 缓存校验
// --------------------------------------------------------------------------

func cacheValid(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		return false
	}
	name := filepath.Base(path)
	switch {
	case strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz"):
		f, err := os.Open(path)
		if err != nil {
			return false
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return false
		}
		tr := tar.NewReader(gz)
		for {
			if _, err := tr.Next(); err == io.EOF {
				return true
			} else if err != nil {
				return false
			}
		}
	case strings.HasSuffix(name, ".zip"):
		z, err := zip.OpenReader(path)
		if err != nil {
			return false
		}
		z.Close()
		return true
	case strings.HasSuffix(name, ".gz"):
		f, err := os.Open(path)
		if err != nil {
			return false
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return false
		}
		_, err = io.Copy(io.Discard, gz)
		return err == nil
	}
	return true
}

func downloadTo(f *fetchx.Fetcher, rawURL, out string, force bool) error {
	part := out + ".part"
	if !force && cacheValid(out) {
		execx.Info(i18n.T("使用缓存: ") + filepath.Base(out))
		return nil
	}
	if _, err := os.Stat(out); err == nil {
		execx.Info(i18n.T("丢弃无效缓存: ") + filepath.Base(out))
		os.Remove(out)
		os.Remove(part)
	}
	execx.Info(i18n.T("下载: ") + rawURL)
	if err := f.FetchFile(rawURL, part); err != nil {
		return err
	}
	isArchive := strings.HasSuffix(part, ".tar.gz.part") || strings.HasSuffix(part, ".tgz.part") ||
		strings.HasSuffix(part, ".zip.part") || strings.HasSuffix(part, ".gz.part")
	if isArchive {
		// 校验以 part 实际内容为准（按最终名判断格式）
		tmp := strings.TrimSuffix(part, ".part") + ".check"
		if err := os.Rename(part, tmp); err != nil {
			return err
		}
		if !cacheValid(tmp) {
			os.Remove(tmp)
			return fmt.Errorf(i18n.T("下载文件完整性校验失败: %s"), filepath.Base(out))
		}
		return os.Rename(tmp, out)
	}
	if st, err := os.Stat(part); err != nil || st.Size() == 0 {
		os.Remove(part)
		return fmt.Errorf(i18n.T("下载文件为空: %s"), filepath.Base(out))
	}
	return os.Rename(part, out)
}

// --------------------------------------------------------------------------
// 部署各组件
// --------------------------------------------------------------------------

// UpdateCore 下载并部署 mihomo 内核（gzip 单文件），返回版本号。
func UpdateCore(p paths.Paths, f *fetchx.Fetcher, s Settings, compatible, force bool) (string, error) {
	if err := p.EnsureStateDirs(); err != nil {
		return "", err
	}
	execx.Info(i18n.T("查找最新 mihomo 版本…"))
	rel, err := latestRelease(f, MihomoRepo)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(rel.TagName)
	u := pickMihomoAsset(assetURLs(rel), archName(), version, compatible)
	if u == "" {
		return "", fmt.Errorf(i18n.T("未找到架构 %s 的 Linux mihomo 资源"), archName())
	}

	archive := filepath.Join(p.Downloads, filepath.Base(u))
	if err := downloadTo(f, Mirror(u, s.GithubMirror), archive, force); err != nil {
		return "", err
	}

	src, err := os.Open(archive)
	if err != nil {
		return "", err
	}
	defer src.Close()
	gz, err := gzip.NewReader(src)
	if err != nil {
		return "", err
	}
	tmp := p.MihomoBin + ".new"
	dst, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(dst, gz); err != nil {
		dst.Close()
		return "", err
	}
	if err := dst.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, p.MihomoBin); err != nil {
		return "", err
	}
	if err := os.WriteFile(p.MihomoVersion, []byte(version+"\n"), 0o644); err != nil {
		return "", err
	}
	execx.Ok(i18n.T("内核已部署: ") + version)
	return version, nil
}

// UpdateGeodata 下载 geo 数据（geoip.metadb + geosite.dat）。
// 机场订阅 rules 普遍内联 GEOIP/GEOSITE，缺 geo 数据 mihomo 校验/运行会失败。
func UpdateGeodata(p paths.Paths, f *fetchx.Fetcher, s Settings, force bool) error {
	if err := p.EnsureStateDirs(); err != nil {
		return err
	}
	for _, item := range []struct{ url, dest string }{
		{GeoipURL, p.GeoipMetadb},
		{GeositeURL, p.GeositeDat},
	} {
		cache := filepath.Join(p.Downloads, filepath.Base(item.dest))
		if err := downloadTo(f, Mirror(item.url, s.GithubMirror), cache, force); err != nil {
			return err
		}
		if err := copyFile(cache, item.dest, 0o644); err != nil {
			return err
		}
	}
	execx.Ok(i18n.T("geo 数据已更新"))
	return nil
}

// UpdateUI 下载并部署 Web UI（metacubexd）。
func UpdateUI(p paths.Paths, f *fetchx.Fetcher, s Settings, force bool) error {
	if err := p.EnsureStateDirs(); err != nil {
		return err
	}
	execx.Info(i18n.T("查找最新 Web UI 版本…"))
	rel, err := latestRelease(f, UIRepo)
	if err != nil {
		return err
	}
	urls := assetURLs(rel)
	u := pickAsset(urls, `(gh-pages|dist|compressed-dist).*(\.zip|\.tar\.gz|\.tgz)$`)
	if u == "" {
		u = pickAsset(urls, `(\.zip|\.tar\.gz|\.tgz)$`)
	}
	if u == "" {
		return fmt.Errorf(i18n.T("未从 %s releases 找到 UI 资源"), UIRepo)
	}
	archive := filepath.Join(p.Downloads, filepath.Base(u))
	if err := downloadTo(f, Mirror(u, s.GithubMirror), archive, force); err != nil {
		return err
	}

	td, err := os.MkdirTemp("", "clashdock-ui-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(td)
	if err := extract(archive, td); err != nil {
		return err
	}
	uiRoot := findUIRoot(td)
	if uiRoot == "" {
		return fmt.Errorf(i18n.T("未能定位 UI 根目录: %s"), filepath.Base(archive))
	}
	if err := os.RemoveAll(p.UI); err != nil {
		return err
	}
	if err := copyTreeDir(uiRoot, p.UI); err != nil {
		return err
	}
	execx.Ok(i18n.T("Web UI 已部署"))
	return nil
}

func extract(archive, outDir string) error {
	name := filepath.Base(archive)
	switch {
	case strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz"):
		f, err := os.Open(archive)
		if err != nil {
			return err
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			target, err := safeJoin(outDir, hdr.Name)
			if err != nil {
				return err
			}
			switch hdr.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
			case tar.TypeReg:
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode&0o777))
				if err != nil {
					return err
				}
				if _, err := io.Copy(out, tr); err != nil {
					out.Close()
					return err
				}
				out.Close()
			}
		}
	case strings.HasSuffix(name, ".zip"):
		z, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer z.Close()
		for _, zf := range z.File {
			target, err := safeJoin(outDir, zf.Name)
			if err != nil {
				return err
			}
			if zf.FileInfo().IsDir() {
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				rc.Close()
				return err
			}
			if _, err := io.Copy(out, rc); err != nil {
				out.Close()
				rc.Close()
				return err
			}
			out.Close()
			rc.Close()
		}
		return nil
	}
	return fmt.Errorf(i18n.T("不支持的压缩格式: %s"), name)
}

// safeJoin 防 zip-slip：解包目标必须落在 outDir 内（或就是 outDir 本身——
// 很多归档把根目录自身打包为 "./" 条目，属正常写法，不应拒绝）。
func safeJoin(outDir, name string) (string, error) {
	target := filepath.Join(outDir, name)
	cleanOut := filepath.Clean(outDir)
	if target != cleanOut && !strings.HasPrefix(target, cleanOut+string(os.PathSeparator)) {
		return "", fmt.Errorf(i18n.T("非法压缩条目路径: %s"), name)
	}
	return target, nil
}

func findUIRoot(dir string) string {
	var indexes []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "index.html" {
			indexes = append(indexes, path)
		}
		return nil
	})
	for _, idx := range indexes {
		d := filepath.Dir(idx)
		if isDir(filepath.Join(d, "assets")) || isDir(filepath.Join(d, "_nuxt")) {
			return d
		}
	}
	if len(indexes) > 0 {
		return filepath.Dir(indexes[0])
	}
	return ""
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// NewFetcher 按设置构造下载器（Token 缺失时的交互式补充由上层流程负责）。
func NewFetcher(p paths.Paths) (*fetchx.Fetcher, Settings) {
	s := LoadSettings(p)
	if s.DownloadProxy != "" {
		execx.Info(i18n.T("下载代理（直连不可用时回退）: ") + s.DownloadProxy)
	}
	return fetchx.New(s.DownloadProxy, s.GithubToken), s
}

// localMixedPortProxy 本机 mihomo mixed-port 的 HTTP 代理地址（服务已启动时可用）。
func localMixedPortProxy() string {
	return fmt.Sprintf("http://127.0.0.1:%d", firewall.ProxyPort)
}

// newLocalProxyFirstFetcher 服务已启动场景专用：优先走本机 mixed-port（走已生效
// 订阅的节点，出海更稳），失败再回退配置的 download_proxy，最后才直连。
func newLocalProxyFirstFetcher(p paths.Paths) (*fetchx.Fetcher, Settings) {
	s := LoadSettings(p)
	return fetchx.NewOrdered([]string{localMixedPortProxy(), s.DownloadProxy}, s.GithubToken), s
}

// Options DownloadAll 的选项。
type Options struct {
	Compatible bool
	Force      bool
	WithUI     bool

	// SkipCore 跳过内核下载，只更新 geo 数据（+ 可选 Web UI）。内核随安装包捆绑，
	// 初始化不再下载内核；内核更新由用户在「运行时管理 → 更新内核」显式触发。
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

// --------------------------------------------------------------------------
// deb 种子接管
// --------------------------------------------------------------------------

// SeedFromSystem 把 deb 附带的种子资源（mihomo 二进制 + 基础规则文件）复制到
// state（仅当 state 对应文件缺失时），使 apt 安装后无需联网即可启动。
// 返回实际接管的文件列表。
func SeedFromSystem(p paths.Paths) ([]string, error) {
	var seeded []string
	if err := p.EnsureStateDirs(); err != nil {
		return nil, err
	}
	seedBin := filepath.Join(SeedBinDir, "mihomo")
	if _, err := os.Stat(p.MihomoBin); os.IsNotExist(err) {
		if _, err := os.Stat(seedBin); err == nil {
			if err := copyFile(seedBin, p.MihomoBin, 0o755); err != nil {
				return seeded, err
			}
			os.WriteFile(p.MihomoVersion, []byte("bundled\n"), 0o644)
			seeded = append(seeded, p.MihomoBin)
		}
	}
	for _, name := range []string{"geosite.dat", "geoip.metadb", "country.mmdb"} {
		dest := filepath.Join(p.Ruleset, name)
		src := filepath.Join(SeedRulesetDir, name)
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			continue
		}
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFile(src, dest, 0o644); err != nil {
			return seeded, err
		}
		seeded = append(seeded, dest)
	}
	if len(seeded) > 0 {
		execx.Info(fmt.Sprintf(i18n.T("已从系统包接管 %d 个种子文件（离线可用；后续可在线更新）。"), len(seeded)))
	}
	return seeded, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyTreeDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target, 0o644)
	})
}

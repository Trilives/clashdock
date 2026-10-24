package kernel

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/fetchx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

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
	if err := fetchVerified(f, Mirror(u, s.GithubMirror), archive, assetDigest(rel, u), force); err != nil {
		return "", err
	}

	if err := installGzipBinary(archive, p.MihomoBin); err != nil {
		return "", err
	}
	if err := os.WriteFile(p.MihomoVersion, []byte(version+"\n"), 0o644); err != nil {
		return "", err
	}
	execx.Ok(i18n.T("内核已部署: ") + version)
	return version, nil
}

// installGzipBinary 把 gzip 单文件解压为可执行文件 dst：先写 dst.new，完成后改名替换，
// 运行中的旧进程不受影响。
func installGzipBinary(archive, dst string) error {
	src, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer src.Close()
	gz, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, gz); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
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
		if err := CopyFile(cache, item.dest, 0o644); err != nil {
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

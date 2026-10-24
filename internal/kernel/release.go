package kernel

import (
	"fmt"
	"regexp"
	"runtime"

	"github.com/Trilives/clashdock/internal/fetchx"
)

// archName Go arch → mihomo 资产命名（本二进制按目标架构编译，GOARCH 即目标机架构）。
func archName() string {
	if runtime.GOARCH == "arm" {
		return "armv7"
	}
	return runtime.GOARCH
}

type release struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	BrowserDownloadURL string `json:"browser_download_url"`
	// Digest GitHub 为发行资产计算的摘要（"sha256:<hex>"），较早的发行版可能没有。
	Digest string `json:"digest"`
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

// assetDigest 返回 url 对应资产的摘要；没有记录时为空。
func assetDigest(rel release, url string) string {
	for _, a := range rel.Assets {
		if a.BrowserDownloadURL == url {
			return a.Digest
		}
	}
	return ""
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

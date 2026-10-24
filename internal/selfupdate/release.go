package selfupdate

import (
	"fmt"
	"runtime"

	"github.com/Trilives/clashdock/internal/fetchx"
	"github.com/Trilives/clashdock/internal/i18n"
)

type ghRelease struct {
	TagName    string    `json:"tag_name"`
	Prerelease bool      `json:"prerelease"`
	Assets     []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	// Digest GitHub 为资产计算的摘要（"sha256:<hex>"），取自 api.github.com，不经镜像。
	Digest string `json:"digest"`
}

func fetchRelease(f *fetchx.Fetcher, channel Channel) (ghRelease, error) {
	if channel == Preview {
		var rels []ghRelease
		if err := f.ReadJSON("https://api.github.com/repos/"+Repo+"/releases?per_page=1", &rels); err != nil {
			return ghRelease{}, err
		}
		if len(rels) == 0 {
			return ghRelease{}, fmt.Errorf("%s", i18n.T("仓库还没有任何发行版"))
		}
		return rels[0], nil
	}
	var rel ghRelease
	err := f.ReadJSON("https://api.github.com/repos/"+Repo+"/releases/latest", &rel)
	return rel, err
}

// fetchReleaseByTag 查询指定版本的发行版（回退到记录的稳定版时用）。
func fetchReleaseByTag(f *fetchx.Fetcher, version string) (ghRelease, error) {
	var rel ghRelease
	err := f.ReadJSON("https://api.github.com/repos/"+Repo+"/releases/tags/v"+version, &rel)
	return rel, err
}

func assetName(version string) string {
	return fmt.Sprintf("clashdock_%s_linux_%s.tar.gz", version, releaseArch())
}

func releaseArch() string {
	return releaseArchFor(runtime.GOARCH)
}

func releaseArchFor(goarch string) string {
	if goarch == "arm" {
		return "armv7"
	}
	return goarch
}

func findAssetURL(rel ghRelease, name string) string {
	a, _ := findAsset(rel, name)
	return a.BrowserDownloadURL
}

func findAsset(rel ghRelease, name string) (ghAsset, bool) {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return ghAsset{}, false
}

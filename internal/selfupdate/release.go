package selfupdate

import (
	"fmt"
	"runtime"

	"github.com/Trilives/clashdock/internal/fetchx"
	"github.com/Trilives/clashdock/internal/i18n"
)

type ghRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
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
	for _, a := range rel.Assets {
		if a.Name == name {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

package kernel

import (
	"reflect"
	"testing"
)

const relBase = "https://github.com/MetaCubeX/mihomo/releases/download/v1.19.0/"

func TestPickMihomoAsset(t *testing.T) {
	urls := []string{
		relBase + "mihomo-windows-amd64-v1.19.0.zip",
		relBase + "mihomo-linux-amd64-compatible-v1.19.0.gz",
		relBase + "mihomo-linux-amd64-v1.19.0.gz",
		relBase + "mihomo-linux-arm64-v1.19.0.gz",
	}
	tests := []struct {
		name       string
		urls       []string
		arch       string
		compatible bool
		want       string
	}{
		{name: "standard by default", urls: urls, arch: "amd64", want: relBase + "mihomo-linux-amd64-v1.19.0.gz"},
		{name: "compatible first when asked", urls: urls, arch: "amd64", compatible: true, want: relBase + "mihomo-linux-amd64-compatible-v1.19.0.gz"},
		{name: "other arch", urls: urls, arch: "arm64", want: relBase + "mihomo-linux-arm64-v1.19.0.gz"},
		{
			name: "falls back to any variant of the arch",
			urls: []string{relBase + "mihomo-linux-amd64-v3-v1.19.0.gz"},
			arch: "amd64",
			want: relBase + "mihomo-linux-amd64-v3-v1.19.0.gz",
		},
		{name: "no linux asset for arch", urls: urls, arch: "armv7", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickMihomoAsset(tt.urls, tt.arch, "v1.19.0", tt.compatible); got != tt.want {
				t.Fatalf("pickMihomoAsset = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPickAssetIsCaseInsensitiveAndReturnsFirstMatch(t *testing.T) {
	urls := []string{"https://x/Compressed-Dist.TGZ", "https://x/dist.zip"}

	if got := pickAsset(urls, `(gh-pages|dist|compressed-dist).*(\.zip|\.tar\.gz|\.tgz)$`); got != urls[0] {
		t.Fatalf("pickAsset = %q, want %q", got, urls[0])
	}
	if got := pickAsset(urls, `\.rar$`); got != "" {
		t.Fatalf("pickAsset = %q, want empty", got)
	}
}

func TestAssetURLs(t *testing.T) {
	rel := release{Assets: []releaseAsset{{BrowserDownloadURL: "https://x/a"}, {BrowserDownloadURL: "https://x/b"}}}

	if got := assetURLs(rel); !reflect.DeepEqual(got, []string{"https://x/a", "https://x/b"}) {
		t.Fatalf("assetURLs = %v", got)
	}
}

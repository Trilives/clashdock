package kernel

import (
	"os"
	"testing"

	"github.com/Trilives/clashdock/internal/paths"
)

var proxyEnvVars = []string{"https_proxy", "HTTPS_PROXY", "all_proxy", "ALL_PROXY", "http_proxy", "HTTP_PROXY"}

func clearDownloadEnv(t *testing.T) {
	t.Helper()
	for _, v := range append([]string{"DOWNLOAD_PROXY", "GITHUB_TOKEN", "GH_TOKEN"}, proxyEnvVars...) {
		t.Setenv(v, "")
	}
}

func tempPaths(t *testing.T) paths.Paths {
	t.Helper()
	t.Setenv("CLASHDOCK_HOME", t.TempDir())
	return paths.Detect()
}

func TestMirror(t *testing.T) {
	tests := []struct {
		name, url, mirror, want string
	}{
		{name: "no mirror", url: "https://github.com/a/b", mirror: "", want: "https://github.com/a/b"},
		{name: "github download", url: "https://github.com/a/b", mirror: "https://m.example/", want: "https://m.example/https://github.com/a/b"},
		{name: "raw content", url: "https://raw.githubusercontent.com/a/b", mirror: "https://m.example", want: "https://m.example/https://raw.githubusercontent.com/a/b"},
		{name: "api untouched", url: "https://api.github.com/repos/a/b", mirror: "https://m.example", want: "https://api.github.com/repos/a/b"},
		{name: "other host untouched", url: "https://example.com/x", mirror: "https://m.example", want: "https://example.com/x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Mirror(tt.url, tt.mirror); got != tt.want {
				t.Fatalf("Mirror = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadSettingsFallsBackToEnvironment(t *testing.T) {
	clearDownloadEnv(t)
	p := tempPaths(t)
	t.Setenv("DOWNLOAD_PROXY", " http://proxy:1 ")
	t.Setenv("GH_TOKEN", "gh-token")

	s := LoadSettings(p)

	if s.DownloadProxy != "http://proxy:1" || s.GithubToken != "gh-token" || s.GithubMirror != "" {
		t.Fatalf("LoadSettings = %+v", s)
	}
}

func TestLoadSettingsPrefersGithubTokenOverGhToken(t *testing.T) {
	clearDownloadEnv(t)
	p := tempPaths(t)
	t.Setenv("GITHUB_TOKEN", "primary")
	t.Setenv("GH_TOKEN", "secondary")

	if got := LoadSettings(p).GithubToken; got != "primary" {
		t.Fatalf("GithubToken = %q, want primary", got)
	}
}

func TestAmbientProxyUsesFirstSetVariable(t *testing.T) {
	clearDownloadEnv(t)
	t.Setenv("ALL_PROXY", "socks5://a:1")
	t.Setenv("http_proxy", "http://b:2")

	if got := ambientProxy(); got != "socks5://a:1" {
		t.Fatalf("ambientProxy = %q", got)
	}
}

func TestLocalCoreVersion(t *testing.T) {
	p := tempPaths(t)
	if got := localCoreVersion(p); got != "" {
		t.Fatalf("missing version file: got %q", got)
	}
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.MihomoVersion, []byte(" v1.19.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := localCoreVersion(p); got != "v1.19.0" {
		t.Fatalf("localCoreVersion = %q", got)
	}
}

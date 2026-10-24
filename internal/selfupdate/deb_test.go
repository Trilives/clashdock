package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/fetchx"
	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
)

// debFixture 一个发行版：.deb 与 checksums.txt 由本地 HTTP 服务提供。
type debFixture struct {
	p       paths.Paths
	rel     ghRelease
	fetcher *fetchx.Fetcher
	exe     string
}

func newDebFixture(t *testing.T, version string, sums func(debSum string) string) debFixture {
	t.Helper()
	t.Setenv("CLASHDOCK_HOME", filepath.Join(t.TempDir(), "state"))
	p := paths.Detect()
	if err := p.EnsureStateDirs(); err != nil {
		t.Fatal(err)
	}
	name := debAssetName(version)
	debBody := "fake deb " + version
	h := sha256.Sum256([]byte(debBody))
	checksums := sums(hex.EncodeToString(h[:]) + "  " + name)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + name:
			fmt.Fprint(w, debBody)
		case "/checksums.txt":
			fmt.Fprintln(w, checksums)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	rel := ghRelease{TagName: "v" + version}
	for _, a := range []string{name, "checksums.txt"} {
		rel.Assets = append(rel.Assets, ghAsset{Name: a, BrowserDownloadURL: srv.URL + "/" + a})
	}
	return debFixture{p: p, rel: rel, fetcher: fetchx.NewOrdered(nil, ""), exe: filepath.Join(t.TempDir(), "clashdock")}
}

// stubDpkg 替换 dpkg -i：「安装」就是把 exe 换成输出 installed 版本号的普通文件。
func stubDpkg(t *testing.T, exe, installed string) *[]string {
	t.Helper()
	var debs []string
	orig := deb
	deb = debDeps{
		owns: func(string) bool { return true },
		install: func(debPath string) error {
			debs = append(debs, filepath.Base(debPath))
			os.Remove(exe)
			return os.WriteFile(exe, []byte("#!/bin/sh\necho clashdock "+installed+"\n"), 0o755)
		},
	}
	t.Cleanup(func() { deb = orig })
	return &debs
}

func checksumsOK(line string) string { return line }

func TestUpdateDebManagedInstallsVerifiedDebAndRetiresLegacyVersions(t *testing.T) {
	fx := newDebFixture(t, "1.2.3", checksumsOK)
	// 旧版自更新留下的布局：exe → current → 1.0.0，last-stable 也指向 1.0.0
	legacy := versionBin(fx.p, "1.0.0")
	fakeExecutable(t, legacy)
	if err := atomicSymlink(legacy, currentLink(fx.p)); err != nil {
		t.Fatal(err)
	}
	if err := atomicSymlink(legacy, lastStableLink(fx.p)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(currentLink(fx.p), fx.exe); err != nil {
		t.Fatal(err)
	}
	debs := stubDpkg(t, fx.exe, "1.2.3")

	version, already, err := updateDebManaged(fx.p, fx.fetcher, kernel.Settings{}, fx.rel, "1.2.3", "1.0.0", fx.exe, Stable)

	if err != nil || already || version != "1.2.3" {
		t.Fatalf("updateDebManaged = (%q, %v, %v), want (1.2.3, false, nil)", version, already, err)
	}
	if len(*debs) != 1 || (*debs)[0] != debAssetName("1.2.3") {
		t.Fatalf("dpkg installed %v, want exactly %s", *debs, debAssetName("1.2.3"))
	}
	if isSymlink(fx.exe) {
		t.Fatal("exe must be dpkg's regular file again")
	}
	if _, err := os.Lstat(versionsDir(fx.p)); !os.IsNotExist(err) {
		t.Fatalf("legacy versions dir must be retired, lstat err = %v", err)
	}
	if v, ok := LastStableVersion(fx.p); !ok || v != "1.2.3" {
		t.Fatalf("LastStableVersion = (%q, %v), want the newly installed stable 1.2.3", v, ok)
	}
	if entries, _ := os.ReadDir(fx.p.Downloads); len(entries) != 0 {
		t.Fatalf("downloaded .deb / checksums left behind: %v", entries)
	}
}

func TestUpdateDebManagedRepairsSymlinkEvenOnSameVersion(t *testing.T) {
	fx := newDebFixture(t, "1.2.3", checksumsOK)
	target := filepath.Join(t.TempDir(), "clashdock")
	fakeExecutable(t, target)
	if err := os.Symlink(target, fx.exe); err != nil {
		t.Fatal(err)
	}
	debs := stubDpkg(t, fx.exe, "1.2.3")

	if _, already, err := updateDebManaged(fx.p, fx.fetcher, kernel.Settings{}, fx.rel, "1.2.3", "1.2.3", fx.exe, Stable); err != nil || already {
		t.Fatalf("same version with a symlinked exe must reinstall the .deb, got already=%v err=%v", already, err)
	}
	if len(*debs) != 1 {
		t.Fatalf("dpkg calls = %v, want one reinstall", *debs)
	}
}

func TestUpdateDebManagedSkipsWhenCurrentAndHealthy(t *testing.T) {
	fx := newDebFixture(t, "1.2.3", checksumsOK)
	fakeExecutable(t, fx.exe)
	debs := stubDpkg(t, fx.exe, "1.2.3")

	_, already, err := updateDebManaged(fx.p, fx.fetcher, kernel.Settings{}, fx.rel, "1.2.3", "v1.2.3", fx.exe, Stable)

	if err != nil || !already || len(*debs) != 0 {
		t.Fatalf("already latest: already=%v err=%v dpkg=%v", already, err, *debs)
	}
}

func TestInstallDebReleaseRejectsChecksumMismatch(t *testing.T) {
	fx := newDebFixture(t, "1.2.3", func(line string) string {
		return strings.Repeat("0", 64) + line[64:]
	})
	debs := stubDpkg(t, fx.exe, "1.2.3")

	err := installDebRelease(fx.p, fx.fetcher, kernel.Settings{}, fx.rel, "1.2.3", fx.exe)

	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("installDebRelease = %v, want a SHA-256 failure", err)
	}
	if len(*debs) != 0 {
		t.Fatalf("an unverified .deb reached dpkg: %v", *debs)
	}
}

func TestInstallDebReleaseFailsWhenInstalledVersionDiffers(t *testing.T) {
	fx := newDebFixture(t, "1.2.3", checksumsOK)
	stubDpkg(t, fx.exe, "1.0.0")

	if err := installDebRelease(fx.p, fx.fetcher, kernel.Settings{}, fx.rel, "1.2.3", fx.exe); err == nil {
		t.Fatal("a binary reporting another version must fail the post-install probe")
	}
}

func TestRetireLegacyVersionsKeepsDirWhileExeStillLinksIntoIt(t *testing.T) {
	fx := newDebFixture(t, "1.2.3", checksumsOK)
	legacy := versionBin(fx.p, "1.0.0")
	fakeExecutable(t, legacy)
	if err := os.Symlink(legacy, fx.exe); err != nil {
		t.Fatal(err)
	}

	if err := retireLegacyVersions(fx.p, fx.exe); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("versions dir removed while the running exe still resolves through it: %v", err)
	}
}

// withDigest 给发行版里的 .deb 资产附上 API 摘要；dropSums 时去掉 checksums.txt。
func withDigest(rel ghRelease, digest string, dropSums bool) ghRelease {
	out := ghRelease{TagName: rel.TagName}
	for _, a := range rel.Assets {
		if a.Name == "checksums.txt" {
			if !dropSums {
				out.Assets = append(out.Assets, a)
			}
			continue
		}
		a.Digest = digest
		out.Assets = append(out.Assets, a)
	}
	return out
}

func TestDownloadVerifiedTrustsApiDigestWithoutChecksums(t *testing.T) {
	fx := newDebFixture(t, "1.2.3", checksumsOK)
	h := sha256.Sum256([]byte("fake deb 1.2.3"))
	rel := withDigest(fx.rel, "sha256:"+hex.EncodeToString(h[:]), true)
	name := debAssetName("1.2.3")
	dir := t.TempDir()

	err := downloadVerified(fx.fetcher, kernel.Settings{}, rel, name, filepath.Join(dir, name), filepath.Join(dir, "sums"))

	if err != nil {
		t.Fatalf("API digest matches, checksums.txt is not needed: %v", err)
	}
}

func TestDownloadVerifiedPrefersApiDigestOverMirroredChecksums(t *testing.T) {
	// 包与 checksums.txt 出自同一（可能被篡改的）来源且彼此相符，但与 API 摘要不符。
	fx := newDebFixture(t, "1.2.3", checksumsOK)
	rel := withDigest(fx.rel, "sha256:"+strings.Repeat("0", 64), false)
	name := debAssetName("1.2.3")
	dir := t.TempDir()

	err := downloadVerified(fx.fetcher, kernel.Settings{}, rel, name, filepath.Join(dir, name), filepath.Join(dir, "sums"))

	if err == nil {
		t.Fatal("a package matching only the co-served checksums.txt must be rejected when the API digest disagrees")
	}
}

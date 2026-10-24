package kernel

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/fetchx"
)

func gzipBytes(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Digest(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

// serveAsset 返回提供 body 的 URL 与请求计数。
func serveAsset(t *testing.T, body []byte) (string, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/mihomo-linux-amd64-v1.gz", &hits
}

func TestFetchVerifiedAcceptsMatchingDigest(t *testing.T) {
	body := gzipBytes(t, "ELF kernel")
	url, _ := serveAsset(t, body)
	out := filepath.Join(t.TempDir(), "mihomo.gz")

	if err := fetchVerified(fetchx.NewOrdered(nil, ""), url, out, sha256Digest(body), true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(out); !bytes.Equal(got, body) {
		t.Fatal("downloaded archive content differs")
	}
}

func TestFetchVerifiedRejectsTamperedDownload(t *testing.T) {
	url, _ := serveAsset(t, gzipBytes(t, "malicious kernel"))
	out := filepath.Join(t.TempDir(), "mihomo.gz")

	err := fetchVerified(fetchx.NewOrdered(nil, ""), url, out, sha256Digest(gzipBytes(t, "ELF kernel")), true)

	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("fetchVerified = %v, want a SHA-256 mismatch", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("a tampered archive must not be left for installation")
	}
}

func TestFetchVerifiedRedownloadsStaleCache(t *testing.T) {
	body := gzipBytes(t, "ELF kernel v2")
	url, hits := serveAsset(t, body)
	out := filepath.Join(t.TempDir(), "mihomo.gz")
	if err := os.WriteFile(out, gzipBytes(t, "ELF kernel v1"), 0o644); err != nil { // 合法但过期的缓存
		t.Fatal(err)
	}

	if err := fetchVerified(fetchx.NewOrdered(nil, ""), url, out, sha256Digest(body), false); err != nil {
		t.Fatal(err)
	}
	if *hits != 1 {
		t.Fatalf("server hits = %d, want one re-download after the cache failed verification", *hits)
	}
	if got, _ := os.ReadFile(out); !bytes.Equal(got, body) {
		t.Fatal("stale cache kept")
	}
}

func TestFetchVerifiedWithoutDigestKeepsDownload(t *testing.T) {
	body := gzipBytes(t, "ELF kernel")
	url, _ := serveAsset(t, body)
	out := filepath.Join(t.TempDir(), "mihomo.gz")

	if err := fetchVerified(fetchx.NewOrdered(nil, ""), url, out, "", true); err != nil {
		t.Fatalf("a release without digests must still install (with a warning): %v", err)
	}
}

func TestVerifyDigestRejectsUnknownAlgorithm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDigest(path, "md5:abc"); err == nil {
		t.Fatal("unsupported digest algorithms must be refused, not silently accepted")
	}
}

func TestAssetDigestMatchesURL(t *testing.T) {
	rel := release{Assets: []releaseAsset{
		{BrowserDownloadURL: "https://x/a.gz", Digest: "sha256:aa"},
		{BrowserDownloadURL: "https://x/b.gz", Digest: "sha256:bb"},
	}}

	if got := assetDigest(rel, "https://x/b.gz"); got != "sha256:bb" {
		t.Fatalf("assetDigest = %q, want sha256:bb", got)
	}
	if got := assetDigest(rel, "https://x/c.gz"); got != "" {
		t.Fatalf("assetDigest for unknown url = %q, want empty", got)
	}
}

package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/fetchx"
	"github.com/Trilives/clashdock/internal/i18n"
)

// fetchVerified 下载 rawURL 到 out，并按 GitHub API 返回的资产摘要校验。摘要取自
// api.github.com（不经镜像），镜像只负责传输，被篡改的文件无法通过校验。缓存与摘要
// 不符（旧版本残留或损坏）时丢弃并重下一次；发行版未提供摘要时提示后沿用原行为。
func fetchVerified(f *fetchx.Fetcher, rawURL, out, digest string, force bool) error {
	if err := downloadTo(f, rawURL, out, force); err != nil {
		return err
	}
	if digest == "" {
		execx.Warn(i18n.T("发行版未提供 SHA-256 摘要，跳过完整性校验。"))
		return nil
	}
	err := VerifyDigest(out, digest)
	if err == nil {
		execx.Ok(i18n.T("SHA-256 校验通过。"))
		return nil
	}
	os.Remove(out)
	if force {
		return err
	}
	if err := downloadTo(f, rawURL, out, true); err != nil {
		return err
	}
	if err := VerifyDigest(out, digest); err != nil {
		os.Remove(out)
		return err
	}
	execx.Ok(i18n.T("SHA-256 校验通过。"))
	return nil
}

// VerifyDigest 校验 path 的内容与 GitHub API 资产摘要（"sha256:<hex>"）一致。
func VerifyDigest(path, digest string) error {
	algo, want, ok := strings.Cut(digest, ":")
	if !ok || algo != "sha256" {
		return fmt.Errorf(i18n.T("不支持的摘要格式: %s"), digest)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, want) {
		return fmt.Errorf(i18n.T("SHA-256 校验失败：期望 %s，实际 %s"), want, got)
	}
	return nil
}

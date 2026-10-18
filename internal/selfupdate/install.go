package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

// prepareExtractedBinary normalizes GoReleaser archives into the self-update
// managed layout. Portable tarballs are wrapped in a top-level directory, while
// self-update expects <versions>/<version>/clashdock for stable symlink targets.
func prepareExtractedBinary(p paths.Paths, version string) (string, error) {
	newBin := versionBin(p, version)
	if st, err := os.Stat(newBin); err == nil && !st.IsDir() {
		return newBin, nil
	}
	candidate, err := findExtractedBinary(filepath.Dir(newBin))
	if err != nil {
		return "", err
	}
	if err := copyFileMode(candidate, newBin, 0o755); err != nil {
		return "", err
	}
	return newBin, nil
}

func findExtractedBinary(root string) (string, error) {
	var matches []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "clashdock" {
			return nil
		}
		if st, err := d.Info(); err == nil && st.Mode().IsRegular() {
			matches = append(matches, path)
		}
		return nil
	}); err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("%s", i18n.T("解压后未找到 clashdock 可执行文件"))
	}
	sort.Slice(matches, func(i, j int) bool {
		if strings.Count(matches[i], string(os.PathSeparator)) == strings.Count(matches[j], string(os.PathSeparator)) {
			return matches[i] < matches[j]
		}
		return strings.Count(matches[i], string(os.PathSeparator)) < strings.Count(matches[j], string(os.PathSeparator))
	})
	return matches[0], nil
}

// probeBinary 试跑新二进制，确认它能正常执行（"version" 子命令，无需 root/网络）。
func probeBinary(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func verifySHA256(archivePath, sumsPath, name string) error {
	sums, err := os.ReadFile(sumsPath)
	if err != nil {
		return err
	}
	var want string
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			want = strings.ToLower(fields[0])
			break
		}
	}
	if want == "" {
		return fmt.Errorf(i18n.T("checksums.txt 里没有 %s 的记录"), name)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf(i18n.T("SHA-256 校验失败：期望 %s，实际 %s"), want, got)
	}
	return nil
}

func extractTarGz(archive, outDir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	cleanOut := filepath.Clean(outDir)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(outDir, hdr.Name)
		if target != cleanOut && !strings.HasPrefix(target, cleanOut+string(os.PathSeparator)) {
			return fmt.Errorf(i18n.T("非法压缩条目路径: %s"), hdr.Name)
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
}

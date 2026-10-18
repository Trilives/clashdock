package kernel

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/fetchx"
	"github.com/Trilives/clashdock/internal/i18n"
)

func cacheValid(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		return false
	}
	name := filepath.Base(path)
	switch {
	case strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz"):
		f, err := os.Open(path)
		if err != nil {
			return false
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return false
		}
		tr := tar.NewReader(gz)
		for {
			if _, err := tr.Next(); err == io.EOF {
				return true
			} else if err != nil {
				return false
			}
		}
	case strings.HasSuffix(name, ".zip"):
		z, err := zip.OpenReader(path)
		if err != nil {
			return false
		}
		z.Close()
		return true
	case strings.HasSuffix(name, ".gz"):
		f, err := os.Open(path)
		if err != nil {
			return false
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return false
		}
		_, err = io.Copy(io.Discard, gz)
		return err == nil
	}
	return true
}

func downloadTo(f *fetchx.Fetcher, rawURL, out string, force bool) error {
	part := out + ".part"
	if !force && cacheValid(out) {
		execx.Info(i18n.T("使用缓存: ") + filepath.Base(out))
		return nil
	}
	if _, err := os.Stat(out); err == nil {
		execx.Info(i18n.T("丢弃无效缓存: ") + filepath.Base(out))
		os.Remove(out)
		os.Remove(part)
	}
	execx.Info(i18n.T("下载: ") + rawURL)
	if err := f.FetchFile(rawURL, part); err != nil {
		return err
	}
	isArchive := strings.HasSuffix(part, ".tar.gz.part") || strings.HasSuffix(part, ".tgz.part") ||
		strings.HasSuffix(part, ".zip.part") || strings.HasSuffix(part, ".gz.part")
	if isArchive {
		// 校验以 part 实际内容为准（按最终名判断格式）
		tmp := strings.TrimSuffix(part, ".part") + ".check"
		if err := os.Rename(part, tmp); err != nil {
			return err
		}
		if !cacheValid(tmp) {
			os.Remove(tmp)
			return fmt.Errorf(i18n.T("下载文件完整性校验失败: %s"), filepath.Base(out))
		}
		return os.Rename(tmp, out)
	}
	if st, err := os.Stat(part); err != nil || st.Size() == 0 {
		os.Remove(part)
		return fmt.Errorf(i18n.T("下载文件为空: %s"), filepath.Base(out))
	}
	return os.Rename(part, out)
}

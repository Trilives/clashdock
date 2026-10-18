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

	"github.com/Trilives/clashdock/internal/i18n"
)

// extract 按扩展名把 tar.gz / tgz / zip 归档解包到 outDir，条目路径经 safeJoin 校验。
func extract(archive, outDir string) error {
	name := filepath.Base(archive)
	switch {
	case strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz"):
		return extractTarGz(archive, outDir)
	case strings.HasSuffix(name, ".zip"):
		return extractZip(archive, outDir)
	}
	return fmt.Errorf(i18n.T("不支持的压缩格式: %s"), name)
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
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(outDir, hdr.Name)
		if err != nil {
			return err
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

func extractZip(archive, outDir string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, zf := range z.File {
		target, err := safeJoin(outDir, zf.Name)
		if err != nil {
			return err
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}

// safeJoin 防 zip-slip：解包目标必须落在 outDir 内（或就是 outDir 本身——
// 很多归档把根目录自身打包为 "./" 条目，属正常写法，不应拒绝）。
func safeJoin(outDir, name string) (string, error) {
	target := filepath.Join(outDir, name)
	cleanOut := filepath.Clean(outDir)
	if target != cleanOut && !strings.HasPrefix(target, cleanOut+string(os.PathSeparator)) {
		return "", fmt.Errorf(i18n.T("非法压缩条目路径: %s"), name)
	}
	return target, nil
}

package portable

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Trilives/clashdock/internal/kernel"
	"github.com/Trilives/clashdock/internal/paths"
)

// RuntimeDir 便携模式下 mihomo 实际的工作目录（`-d` 指向此处）：state 下的子目录，
// 内含 geo 文件（根级命名）、config.yaml 与可选 ui/，与 systemd 服务的运行时布局
// 一致，只是全部落在用户可写的本地目录、无需 sudo。
func RuntimeDir(p paths.Paths) string {
	return filepath.Join(p.State, "runtime")
}

// RuntimeConfig 便携运行时里的配置文件路径。
func RuntimeConfig(p paths.Paths) string {
	return filepath.Join(RuntimeDir(p), "config.yaml")
}

// StageRuntime 把 state 里的生效配置与 geo 数据（及可选 Web UI）铺进便携运行时目录，
// mihomo 在工作目录根查找 geo 文件，故 geoip.metadb / country.mmdb / geosite.dat 均放
// 到运行时根级——与 internal/sysd 的服务运行时布局一致，差别仅在于此处用普通文件
// 复制、不提权。config.yaml 必须已由订阅流程写好。
func StageRuntime(p paths.Paths) error {
	rt := RuntimeDir(p)
	if err := os.MkdirAll(rt, 0o755); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	if _, err := os.Stat(p.ConfigFile); err != nil {
		return fmt.Errorf("missing config.yaml (add a subscription first): %w", err)
	}
	if err := kernel.CopyFile(p.ConfigFile, filepath.Join(rt, "config.yaml"), 0o644); err != nil {
		return fmt.Errorf("stage config: %w", err)
	}

	// geo 数据：metadb / country.mmdb 有哪个装哪个（至少其一）；geosite.dat 必需。
	if err := copyIfExists(p.GeoipMetadb, filepath.Join(rt, "geoip.metadb")); err != nil {
		return err
	}
	if err := copyIfExists(p.CountryMmdb, filepath.Join(rt, "country.mmdb")); err != nil {
		return err
	}
	if _, err := os.Stat(p.GeositeDat); err != nil {
		return fmt.Errorf("missing geosite.dat: %w", err)
	}
	if err := kernel.CopyFile(p.GeositeDat, filepath.Join(rt, "geosite.dat"), 0o644); err != nil {
		return fmt.Errorf("stage geosite.dat: %w", err)
	}

	// Web UI 可选：便携模式默认不携带 metacubexd；有就铺，没有就跳过（节点切换走
	// Clash API，不依赖面板静态资源）。
	if _, err := os.Stat(p.UI); err == nil {
		if err := copyTree(p.UI, filepath.Join(rt, "ui")); err != nil {
			return fmt.Errorf("stage ui: %w", err)
		}
	}
	return nil
}

// copyIfExists 源存在才复制；不存在不报错（geo 兜底文件二选一）。
func copyIfExists(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	if err := kernel.CopyFile(src, dst, 0o644); err != nil {
		return fmt.Errorf("stage %s: %w", filepath.Base(dst), err)
	}
	return nil
}

// copyTree 递归复制目录（用于 Web UI 静态资源）。
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return kernel.CopyFile(path, target, 0o644)
	})
}

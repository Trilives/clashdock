package portable

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Trilives/clashdock/internal/configfile"
	"github.com/Trilives/clashdock/internal/jsonx"
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

	// Web UI 可选：便携模式默认不携带 metacubexd；有就铺到运行时根级 ui/，没有就跳过
	// （节点切换走 Clash API，不依赖面板静态资源）。是否铺成功决定 config 里怎么处理
	// external-ui。
	// 只有 UI 目录里确有内容才铺（EnsureStateDirs 会预建空的 ui/，便携模式默认不下载
	// 面板资源，空目录不算有 UI）。
	uiStaged := false
	if dirHasContent(p.UI) {
		if err := copyTree(p.UI, filepath.Join(rt, "ui")); err != nil {
			return fmt.Errorf("stage ui: %w", err)
		}
		uiStaged = true
	}

	// 配置：mihomo 限制 external-ui 等文件路径必须落在工作目录（-d）之内，而生效
	// 配置里的 external-ui 指向 state/ui（在 runtime 之外），直接搬会被拒。铺 UI 时
	// 改写为运行时目录下的 ui/，未铺 UI 时移除该键（不开面板）——与 sysd 服务侧
	// stageRuntimeConfig 的改写策略一致。
	if err := stageConfig(p, rt, uiStaged); err != nil {
		return err
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
	return nil
}

// stageConfig 把生效配置写进运行时目录，并按 UI 是否就绪改写 external-ui：铺了 UI
// 就指向 <runtime>/ui（工作目录之内，mihomo 允许），否则删除该键。
func stageConfig(p paths.Paths, rt string, uiStaged bool) error {
	raw, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	data, err := configfile.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if uiStaged {
		data["external-ui"] = filepath.Join(rt, "ui")
	} else {
		delete(data, "external-ui")
	}
	out, err := jsonx.MarshalPretty(data)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(rt, "config.yaml"), out, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// dirHasContent 目录存在且至少含一个条目（空目录/不存在均视为无内容）。
func dirHasContent(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
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

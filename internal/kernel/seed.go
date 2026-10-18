package kernel

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

// SeedFromSystem 把 deb 附带的种子资源（mihomo 二进制 + 基础规则文件）复制到
// state（仅当 state 对应文件缺失时），使 apt 安装后无需联网即可启动。
// 返回实际接管的文件列表。
func SeedFromSystem(p paths.Paths) ([]string, error) {
	return SeedFrom(p, SeedBinDir, SeedRulesetDir)
}

// SeedFrom 通用种子接管：从给定的内核目录与规则目录把 mihomo + 基础规则复制到
// state（仅当 state 对应文件缺失时）。deb 走 /usr 下的种子目录（SeedFromSystem），
// 用户模式走便携包解压目录里的 deps/ 兄弟目录（见 internal/usermode）。
func SeedFrom(p paths.Paths, binDir, rulesetDir string) ([]string, error) {
	var seeded []string
	if err := p.EnsureStateDirs(); err != nil {
		return nil, err
	}
	seedBin := filepath.Join(binDir, "mihomo")
	if _, err := os.Stat(p.MihomoBin); os.IsNotExist(err) {
		if _, err := os.Stat(seedBin); err == nil {
			if err := CopyFile(seedBin, p.MihomoBin, 0o755); err != nil {
				return seeded, err
			}
			os.WriteFile(p.MihomoVersion, []byte("bundled\n"), 0o644)
			seeded = append(seeded, p.MihomoBin)
		}
	}
	for _, name := range []string{"geosite.dat", "geoip.metadb", "country.mmdb"} {
		dest := filepath.Join(p.Ruleset, name)
		src := filepath.Join(rulesetDir, name)
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			continue
		}
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := CopyFile(src, dest, 0o644); err != nil {
			return seeded, err
		}
		seeded = append(seeded, dest)
	}
	if len(seeded) > 0 {
		execx.Info(fmt.Sprintf(i18n.T("已从安装包接管 %d 个种子文件（离线可用；后续可在线更新）。"), len(seeded)))
	}
	return seeded, nil
}

// CopyFile 复制单个文件并按 mode 设置权限，必要时创建父目录。
func CopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyTreeDir(src, dst string) error {
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
		return CopyFile(path, target, 0o644)
	})
}

package configfile

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/jsonx"
)

// precheckTimeout 单次 `mihomo -t` 的上限：资源缺失时内核会尝试联网补下载，不能无限等待。
const precheckTimeout = 2 * time.Minute

// PrecheckError 部署前校验未通过：此时运行时文件与运行中的服务都还没被改动。
type PrecheckError struct {
	Output string
	Err    error
}

func (e *PrecheckError) Error() string {
	detail := e.Output
	if detail == "" {
		detail = e.Err.Error()
	}
	return fmt.Sprintf(i18n.T("部署前校验未通过（运行中的服务与运行时文件未改动）：%s"), detail)
}

func (e *PrecheckError) Unwrap() error { return e.Err }

// Precheck 在 parent 下的私有临时目录里用 bin 执行 `-t` 校验 cfg，不触碰运行时目录。
// assets 为「校验目录内的文件名 → 源文件」（geo 数据，mihomo 在工作目录根查找），以指向
// 绝对路径的符号链接放入，源文件不存在则跳过；external-ui 改写到校验目录内（mihomo 要求
// 面板路径落在 -d 之内）。cfg 不被修改。
func Precheck(bin, parent string, cfg map[string]any, assets map[string]string) error {
	dir, err := os.MkdirTemp(parent, ".precheck-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	confPath, err := writeCheckConfig(dir, cfg)
	if err != nil {
		return err
	}
	for name, src := range assets {
		if _, err := os.Stat(src); err != nil {
			continue
		}
		// 相对路径的链接相对校验目录解析，会悬空
		abs, err := filepath.Abs(src)
		if err != nil {
			return err
		}
		if err := os.Symlink(abs, filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), precheckTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-t", "-d", dir, "-f", confPath).CombinedOutput()
	if err != nil {
		return &PrecheckError{Output: strings.TrimSpace(string(out)), Err: err}
	}
	return nil
}

func writeCheckConfig(dir string, cfg map[string]any) (string, error) {
	data := maps.Clone(cfg)
	if v, ok := data["external-ui"]; ok && v != nil && v != "" {
		ui := filepath.Join(dir, "ui")
		if err := os.Mkdir(ui, 0o700); err != nil {
			return "", err
		}
		data["external-ui"] = ui
	}
	out, err := jsonx.MarshalPretty(data)
	if err != nil {
		return "", err
	}
	confPath := filepath.Join(dir, "config.yaml")
	return confPath, os.WriteFile(confPath, out, 0o600)
}

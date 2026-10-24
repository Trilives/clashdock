package sysd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

// companionEntry 调用 clashdock 自身的伴生单元：单元文件与 ExecStart 子命令。
type companionEntry struct {
	unit, subcommand string
}

func companionEntries() []companionEntry {
	return []companionEntry{
		{wdService(), "healthcheck"},
		{timerServiceFile(), "update"},
	}
}

// RepairCompanionUnits 修复旧版本写下的伴生单元入口：旧版本以 os.Executable() 记录
// ExecStart，自更新后指向具体版本目录，随旧版本清理而失效（watchdog 与周更静默停摆）。
// ExecStart 指向已不存在的文件或版本目录时，改写为当前稳定调用路径并 daemon-reload；
// 其余情况不动（不把单元劫持到用户临时运行的其它二进制上）。
func RepairCompanionUnits(p paths.Paths) error {
	exe, err := execx.TrustedExecutablePath()
	if err != nil {
		return err
	}
	if isUnder(exe, p.VersionsDir()) {
		return nil // 直接运行了版本目录里的文件，没有稳定路径可写
	}
	changed := false
	for _, e := range companionEntries() {
		raw, err := os.ReadFile(e.unit)
		if err != nil {
			continue // 未安装
		}
		text, ok := repairedUnitText(string(raw), e.subcommand, exe, func(path string) bool {
			return staleExecPath(path, p.VersionsDir())
		})
		if !ok {
			continue
		}
		if err := execx.WriteRoot(e.unit, text, "0644", i18n.T("修复伴生单元入口")); err != nil {
			return err
		}
		execx.Info(fmt.Sprintf(i18n.T("已把 %s 的入口改为 %s。"), filepath.Base(e.unit), exe))
		changed = true
	}
	if !changed {
		return nil
	}
	_, err = execx.RunRoot([]string{"systemctl", "daemon-reload"}, "", nil)
	return err
}

// repairedUnitText 把 `ExecStart=<path> <subcommand>` 中失效的 path 换成 exe；
// 无需修改时 ok=false。
func repairedUnitText(text, subcommand, exe string, stale func(string) bool) (string, bool) {
	lines := strings.Split(text, "\n")
	for i, ln := range lines {
		rest, found := strings.CutPrefix(ln, "ExecStart=")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) != 2 || fields[1] != subcommand || fields[0] == exe || !stale(fields[0]) {
			return "", false
		}
		lines[i] = "ExecStart=" + exe + " " + subcommand
		return strings.Join(lines, "\n"), true
	}
	return "", false
}

// staleExecPath 入口文件已不存在，或落在会被清理的自更新版本目录里。
func staleExecPath(path, versionsDir string) bool {
	if isUnder(path, versionsDir) {
		return true
	}
	_, err := os.Stat(path)
	return err != nil
}

func isUnder(path, dir string) bool {
	return strings.HasPrefix(path, strings.TrimRight(dir, "/")+"/")
}

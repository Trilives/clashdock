// Package sysd systemd 单元管理（对应 service.py + resilience.py + timer.py）：
// 在 /var/lib/clashdock-runtime 暂存自包含运行时并注册主服务，以及两类伴生单元
// （网络自愈 watchdog / 每周更新定时器）。mihomo 自带的 Web UI 只走内置
// 控制器路径（http://host:9090/ui/），不再提供独立根路径面板服务
// （多占一个端口，转发/放行更麻烦，收益有限）。
//
// 把内核、配置、geo 数据、UI 暂存到 /var/lib/clashdock-runtime（mihomo 的工作目录 -d），
// 并把配置内的 external-ui 改写为该目录下的绝对路径，使服务与状态目录（可能在 /home）解耦。
// 所有 root 操作经 execx.RunRoot（非 root 自动 sudo，凭证会话内缓存）。
//
// 注意：本目录曾用 /etc/mihomo（旧版遗留），自本次改名起不再兼容旧路径——
// 旧版本升级上来的部署需要先完整卸载旧版本（或运行
// scripts/migrate-runtime-dir.sh 清理），再重新初始化。
package sysd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Trilives/clashdock/internal/configfile"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/jsonx"
	"github.com/Trilives/clashdock/internal/paths"
)

//go:embed assets/mihomo.service.tmpl
var mihomoUnitTmpl string

const (
	DefaultName     = "mihomo"
	conflictingName = "sing-box"
)

type runtimePaths struct {
	Dir     string
	Bin     string
	Config  string
	UI      string
	Geoip   string
	Geosite string
	Country string
	Unit    string
}

func rtPaths(name string) runtimePaths {
	d := paths.RuntimeDir
	return runtimePaths{
		Dir:     d,
		Bin:     filepath.Join(d, "mihomo"),
		Config:  filepath.Join(d, name+".yaml"),
		UI:      filepath.Join(d, "ui"),
		Geoip:   filepath.Join(d, "geoip.metadb"),
		Geosite: filepath.Join(d, "geosite.dat"),
		Country: filepath.Join(d, "country.mmdb"),
		Unit:    "/etc/systemd/system/" + name + ".service",
	}
}

// stageRuntimeConfig 读 state/config.yaml（JSON 内容），把 external-ui 改写为
// 运行时绝对路径，写临时文件返回。输出仍为 JSON 内容、.yaml 后缀。
func stageRuntimeConfig(p paths.Paths, rt runtimePaths) (string, error) {
	raw, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		return "", err
	}
	data, err := configfile.Parse(raw)
	if err != nil {
		return "", fmt.Errorf(i18n.T("解析 state/config.yaml: %w"), err)
	}
	if v, ok := data["external-ui"]; ok && v != nil && v != "" {
		data["external-ui"] = rt.UI
	}
	out, err := jsonx.MarshalPretty(data)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(p.State, ".runtime-config.*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func preflight(p paths.Paths) error {
	if _, err := os.Stat(p.MihomoBin); err != nil {
		return fmt.Errorf("%s", i18n.T("未找到 mihomo 内核，请先执行『下载内核/UI/geo 数据』"))
	}
	if _, err := os.Stat(p.ConfigFile); err != nil {
		return fmt.Errorf("%s", i18n.T("未找到生效配置 config.yaml，请先添加订阅"))
	}
	_, metadbErr := os.Stat(p.GeoipMetadb)
	_, mmdbErr := os.Stat(p.CountryMmdb)
	_, geositeErr := os.Stat(p.GeositeDat)
	if geositeErr != nil || (metadbErr != nil && mmdbErr != nil) {
		return fmt.Errorf("%s", i18n.T("未找到 geo 数据（geosite.dat 且 geoip.metadb/country.mmdb 之一），请先执行『下载内核/UI/geo 数据』"))
	}
	if !execx.Have("systemctl") {
		return fmt.Errorf("%s", i18n.T("未找到 systemctl，注册服务需要 systemd"))
	}
	return nil
}

// Install 注册并（可选）启动主服务。会先移除同名及冲突的 sing-box 服务。
func Install(p paths.Paths, name string, start bool) error {
	if name == "" {
		name = DefaultName
	}
	if err := preflight(p); err != nil {
		return err
	}
	rt := rtPaths(name)
	if err := execx.EnsureSudo(i18n.T("注册系统服务")); err != nil {
		return err
	}

	staged, err := stageRuntimeConfig(p, rt)
	if err != nil {
		return err
	}
	defer os.Remove(staged)

	steps := [][]string{
		{"mkdir", "-p", rt.Dir},
		{"chmod", "0755", rt.Dir},
		// 二进制：临时名 + 原子改名，避免运行中替换报 ETXTBSY
		{"install", "-m", "0755", p.MihomoBin, rt.Bin + ".new"},
		{"mv", "-f", rt.Bin + ".new", rt.Bin},
	}
	// geo 数据（mihomo 在工作目录根查找；metadb / country.mmdb 有哪个装哪个）
	if _, err := os.Stat(p.GeoipMetadb); err == nil {
		steps = append(steps, []string{"install", "-m", "0644", p.GeoipMetadb, rt.Geoip})
	}
	if _, err := os.Stat(p.CountryMmdb); err == nil {
		steps = append(steps, []string{"install", "-m", "0644", p.CountryMmdb, rt.Country})
	}
	steps = append(steps, []string{"install", "-m", "0644", p.GeositeDat, rt.Geosite})
	for _, cmd := range steps {
		if _, err := execx.RunRoot(cmd, i18n.T("部署运行时"), nil); err != nil {
			return err
		}
	}
	// UI
	if _, err := os.Stat(p.UI); err == nil {
		execx.RunRoot([]string{"rm", "-rf", rt.UI}, "", nil)
		if _, err := execx.RunRoot([]string{"cp", "-a", p.UI, rt.UI}, "", nil); err != nil {
			return err
		}
	} else {
		execx.Warn(i18n.T("未找到 Web UI，面板将不可用；可稍后执行更新补齐。"))
	}
	// 配置 + 运行时校验
	if _, err := execx.RunRoot([]string{"install", "-m", "0644", staged, rt.Config}, "", nil); err != nil {
		return err
	}
	if _, err := execx.RunRoot([]string{rt.Bin, "-t", "-d", rt.Dir, "-f", rt.Config}, i18n.T("校验配置"), nil); err != nil {
		return err
	}

	// 移除旧的同名 / 冲突服务
	removeUnit(name, true)
	if name != conflictingName {
		removeUnit(conflictingName, true)
	}

	// 写 unit
	unitText, err := renderUnit(name, rt)
	if err != nil {
		return err
	}
	if err := execx.WriteRoot(rt.Unit, unitText, "0644", i18n.T("写服务单元")); err != nil {
		return err
	}
	if _, err := execx.RunRoot([]string{"systemctl", "daemon-reload"}, "", nil); err != nil {
		return err
	}
	if _, err := execx.RunRoot([]string{"systemctl", "enable", name + ".service"}, "", nil); err != nil {
		return err
	}
	if start {
		if _, err := execx.RunRoot([]string{"systemctl", "restart", name + ".service"}, "", nil); err != nil {
			return err
		}
		execx.Ok(i18n.T("服务已启动: ") + name + ".service")
	} else {
		execx.Ok(i18n.T("服务已设为开机自启（未启动）: ") + name + ".service")
	}
	if err := RecordDeployedAssets(p); err != nil {
		execx.Warn(i18n.T("记录资源部署指纹失败（不影响服务本身）：") + err.Error())
	}
	return nil
}

func renderUnit(name string, rt runtimePaths) (string, error) {
	t, err := template.New("unit").Parse(mihomoUnitTmpl)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	err = t.Execute(&sb, struct{ Name, RuntimeDir, Bin, Config string }{name, rt.Dir, rt.Bin, rt.Config})
	return sb.String(), err
}

// SyncAndRestart 把当前 state/config.yaml 同步到运行时并重启服务。
func SyncAndRestart(p paths.Paths, name string) error {
	if name == "" {
		name = DefaultName
	}
	if !IsInstalled(name) {
		execx.Warn(i18n.T("服务 ") + name + i18n.T(" 未安装，跳过同步。"))
		return nil
	}
	if err := preflight(p); err != nil {
		return err
	}
	rt := rtPaths(name)
	if err := execx.EnsureSudo(i18n.T("更新服务配置")); err != nil {
		return err
	}
	staged, err := stageRuntimeConfig(p, rt)
	if err != nil {
		return err
	}
	defer os.Remove(staged)
	if _, err := execx.RunRoot([]string{"install", "-m", "0644", staged, rt.Config}, "", nil); err != nil {
		return err
	}
	if _, err := execx.RunRoot([]string{rt.Bin, "-t", "-d", rt.Dir, "-f", rt.Config}, i18n.T("校验配置"), nil); err != nil {
		return err
	}
	if _, err := execx.RunRoot([]string{"systemctl", "restart", name + ".service"}, "", nil); err != nil {
		return err
	}
	execx.Ok(i18n.T("已同步配置并重启: ") + name + ".service")
	return nil
}

// Remove 停止/禁用/删除服务，并清理运行时文件。
func Remove(p paths.Paths, name string, purgeRuntime bool) error {
	if name == "" {
		name = DefaultName
	}
	if err := execx.EnsureSudo(i18n.T("删除系统服务")); err != nil {
		return err
	}
	removeUnit(name, false)
	if purgeRuntime {
		rt := rtPaths(name)
		execx.RunRoot([]string{"rm", "-f", rt.Config}, "", nil)
		remaining, _ := execx.RunRoot(
			[]string{"bash", "-c", fmt.Sprintf("ls %s/*.yaml 2>/dev/null | wc -l", paths.RuntimeDir)},
			"", &execx.Opt{Capture: true})
		if strings.TrimSpace(remaining.Stdout) == "0" {
			execx.RunRoot([]string{"rm", "-rf", paths.RuntimeDir}, "", nil)
		}
	}
	execx.Ok(i18n.T("服务已删除: ") + name + ".service")
	return nil
}

func removeUnit(name string, quiet bool) {
	opt := &execx.Opt{Capture: quiet}
	execx.RunRoot([]string{"systemctl", "stop", name + ".service"}, "", opt)
	execx.RunRoot([]string{"systemctl", "disable", name + ".service"}, "", opt)
	execx.RunRoot([]string{"rm", "-f", rtPaths(name).Unit}, "", nil)
	execx.RunRoot([]string{"systemctl", "daemon-reload"}, "", opt)
	execx.RunRoot([]string{"systemctl", "reset-failed", name + ".service"}, "", opt)
}

func IsInstalled(name string) bool {
	if name == "" {
		name = DefaultName
	}
	_, err := os.Stat(rtPaths(name).Unit)
	return err == nil
}

func Status(name string) {
	if name == "" {
		name = DefaultName
	}
	execx.Run([]string{"systemctl", "status", "--no-pager", name + ".service"}, nil)
}

func unitActive(unit string) bool {
	r, _ := execx.Run([]string{"systemctl", "is-active", unit}, &execx.Opt{Capture: true})
	return strings.TrimSpace(r.Stdout) == "active"
}

func IsActive(name string) bool {
	if name == "" {
		name = DefaultName
	}
	return unitActive(name + ".service")
}

// CompanionUnits 已安装的伴生 systemd 单元（暂停/启动须一并带上，
// 否则 watchdog 会把刚停掉的主服务又拉起来）。
func CompanionUnits() []string {
	var units []string
	if ResilienceInstalled() {
		units = append(units, WatchdogName+".timer")
	}
	if TimerInstalled() {
		units = append(units, TimerName+".timer")
	}
	return units
}

// Pause 暂停主服务及全部伴生单元（运行时停止；单元保持开机自启）。
func Pause(name string) error {
	if name == "" {
		name = DefaultName
	}
	if !IsInstalled(name) {
		execx.Warn(i18n.T("服务 ") + name + i18n.T(" 未安装，无需暂停。"))
		return nil
	}
	companions := CompanionUnits()
	if err := execx.EnsureSudo(i18n.T("暂停服务")); err != nil {
		return err
	}
	// 先停伴生 watchdog，否则刚停掉主服务它又会拉起来
	for _, unit := range companions {
		execx.RunRoot([]string{"systemctl", "stop", unit}, "", &execx.Opt{Capture: true})
	}
	execx.RunRoot([]string{"systemctl", "stop", name + ".service"}, "", nil)
	suffix := ""
	if len(companions) > 0 {
		suffix = fmt.Sprintf(i18n.T(" + %d 个伴生单元"), len(companions))
	}
	execx.Ok(i18n.T("已暂停：") + name + ".service" + suffix)
	execx.Info(i18n.T("提示：暂停为运行时停止，重启系统后会自动恢复运行。"))
	return nil
}

// Resume 启动主服务及全部已安装的伴生单元。
func Resume(name string) error {
	if name == "" {
		name = DefaultName
	}
	if !IsInstalled(name) {
		execx.Warn(i18n.T("服务 ") + name + i18n.T(" 未安装，请先执行『初始化』。"))
		return nil
	}
	companions := CompanionUnits()
	if err := execx.EnsureSudo(i18n.T("启动服务")); err != nil {
		return err
	}
	execx.RunRoot([]string{"systemctl", "start", name + ".service"}, "", nil)
	for _, unit := range companions {
		execx.RunRoot([]string{"systemctl", "start", unit}, "", &execx.Opt{Capture: true})
	}
	suffix := ""
	if len(companions) > 0 {
		suffix = fmt.Sprintf(i18n.T(" + %d 个伴生单元"), len(companions))
	}
	execx.Ok(i18n.T("已启动：") + name + ".service" + suffix)
	return nil
}

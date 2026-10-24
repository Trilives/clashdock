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
// 旧版本升级上来的部署需要先完整卸载旧版本（v0.3.0 及更早的源码树中附有
// scripts/migrate-runtime-dir.sh 清理脚本，可从 Git 历史取回），再重新初始化。
package sysd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Trilives/clashdock/internal/configfile"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/jsonx"
	"github.com/Trilives/clashdock/internal/paths"
)

//go:embed assets/mihomo.service.tmpl
var mihomoUnitTmpl string

const DefaultName = "mihomo"

type runtimePaths struct {
	Dir     string
	Home    string // 服务账户可写的 mihomo 工作目录（-d），见 service_user.go
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
		Home:    filepath.Join(d, "home"),
		Bin:     filepath.Join(d, "mihomo"),
		Config:  filepath.Join(d, name+".yaml"),
		UI:      filepath.Join(d, "ui"),
		Geoip:   filepath.Join(d, "geoip.metadb"),
		Geosite: filepath.Join(d, "geosite.dat"),
		Country: filepath.Join(d, "country.mmdb"),
		Unit:    "/etc/systemd/system/" + name + ".service",
	}
}

// runtimeConfigData 读 state/config.yaml（JSON 内容），把 external-ui 改写为运行时绝对路径。
func runtimeConfigData(p paths.Paths, rt runtimePaths) (map[string]any, error) {
	raw, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		return nil, err
	}
	data, err := configfile.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("解析 state/config.yaml: %w"), err)
	}
	if v, ok := data["external-ui"]; ok && v != nil && v != "" {
		data["external-ui"] = rt.UI
	}
	return data, nil
}

// stageRuntimeConfig 把 data 写成临时文件返回（仅属主可读：含节点凭据）。输出为 JSON
// 内容、.yaml 后缀。
func stageRuntimeConfig(p paths.Paths, data map[string]any) (string, error) {
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

// stateAssets 即将部署的 geo 数据（校验目录内的文件名 → state 源文件）。
func stateAssets(p paths.Paths) map[string]string {
	return map[string]string{
		"geoip.metadb": p.GeoipMetadb,
		"country.mmdb": p.CountryMmdb,
		"geosite.dat":  p.GeositeDat,
	}
}

// runtimeAssets 运行中的服务所用的 geo 数据。
func runtimeAssets(rt runtimePaths) map[string]string {
	return map[string]string{
		"geoip.metadb": rt.Geoip,
		"country.mmdb": rt.Country,
		"geosite.dat":  rt.Geosite,
	}
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

// Install 注册并（可选）启动主服务。会先移除同名旧服务；sing-box 等冲突服务不再自动
// 停用（可能是用户自行部署的），仍在运行时只提示——是否停用由初始化流程询问用户。
func Install(p paths.Paths, name string, start bool) error {
	if name == "" {
		name = DefaultName
	}
	if err := preflight(p); err != nil {
		return err
	}
	rt := rtPaths(name)
	data, err := runtimeConfigData(p, rt)
	if err != nil {
		return err
	}
	// 先用即将部署的内核与 geo 数据校验，通过后才提权、覆盖运行时文件、移除旧 unit。
	if err := precheckConfig(p.MihomoBin, p.State, data, stateAssets(p)); err != nil {
		return err
	}
	if err := execx.EnsureSudo(i18n.T("注册系统服务")); err != nil {
		return err
	}
	if err := ensureServiceUser(); err != nil {
		return err
	}
	staged, err := stageRuntimeConfig(p, data)
	if err != nil {
		return err
	}
	defer os.Remove(staged)
	if err := deployRuntime(p, rt, staged); err != nil {
		return err
	}

	// 移除旧的同名服务；冲突服务只提示，不碰
	removeUnit(name, true)
	warnIfConflictRunning(systemctlValue)

	// 写 unit
	unitText, err := renderUnit(name, rt)
	if err != nil {
		return err
	}
	if err := execx.WriteRoot(rt.Unit, unitText, "0644", i18n.T("写服务单元")); err != nil {
		return err
	}
	if _, err := runRoot([]string{"systemctl", "daemon-reload"}, "", nil); err != nil {
		return err
	}
	if _, err := runRoot([]string{"systemctl", "enable", name + ".service"}, "", nil); err != nil {
		return err
	}
	if start {
		if _, err := runRoot([]string{"systemctl", "restart", name + ".service"}, "", nil); err != nil {
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

// deployRuntime 把内核、geo 数据、UI 与已暂存的配置铺进运行时目录，再用运行时文件校验一次。
func deployRuntime(p paths.Paths, rt runtimePaths, staged string) error {
	steps := append(runtimeLayoutSteps(rt),
		// 二进制：临时名 + 原子改名，避免运行中替换报 ETXTBSY
		[]string{"install", "-m", "0755", p.MihomoBin, rt.Bin + ".new"},
		[]string{"mv", "-f", rt.Bin + ".new", rt.Bin},
	)
	// geo 数据（mihomo 在工作目录根查找；metadb / country.mmdb 有哪个装哪个）
	if _, err := os.Stat(p.GeoipMetadb); err == nil {
		steps = append(steps, []string{"install", "-m", "0644", p.GeoipMetadb, rt.Geoip})
	}
	if _, err := os.Stat(p.CountryMmdb); err == nil {
		steps = append(steps, []string{"install", "-m", "0644", p.CountryMmdb, rt.Country})
	}
	steps = append(steps, []string{"install", "-m", "0644", p.GeositeDat, rt.Geosite})
	for _, cmd := range steps {
		if _, err := runRoot(cmd, i18n.T("部署运行时"), nil); err != nil {
			return err
		}
	}
	// UI：不沿用状态目录的属组与 setgid（管理员私有组），运行时只归 root，权限按 umask
	if _, err := os.Stat(p.UI); err == nil {
		runRoot([]string{"rm", "-rf", rt.UI}, "", nil)
		if _, err := runRoot([]string{"cp", "-R", "--preserve=timestamps", p.UI, rt.UI}, "", nil); err != nil {
			return err
		}
	} else {
		execx.Warn(i18n.T("未找到 Web UI，面板将不可用；可稍后执行更新补齐。"))
	}
	// 配置 + 运行时校验
	if _, err := runRoot(installConfigCmd(staged, rt), "", nil); err != nil {
		return err
	}
	_, err := runRoot([]string{rt.Bin, "-t", "-d", rt.Dir, "-f", rt.Config}, i18n.T("校验配置"), nil)
	return err
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
	if err := syncRuntime(p, rt, name, precheckConfig, runRoot); err != nil {
		return err
	}
	execx.Ok(i18n.T("已同步配置并重启: ") + name + ".service")
	return nil
}

type rootCommandRunner func([]string, string, *execx.Opt) (execx.Result, error)

type precheckFunc func(bin, parent string, cfg map[string]any, assets map[string]string) error

// 可在测试中替换：root 命令与部署前校验。
var (
	runRoot        rootCommandRunner = execx.RunRoot
	precheckConfig precheckFunc      = configfile.Precheck
)

// syncRuntime 用运行中的内核与 geo 数据先校验，未通过时运行时配置与服务都不动；
// 通过后才安装配置并重启。
func syncRuntime(p paths.Paths, rt runtimePaths, name string, check precheckFunc, run rootCommandRunner) error {
	data, err := runtimeConfigData(p, rt)
	if err != nil {
		return err
	}
	if err := check(rt.Bin, p.State, data, runtimeAssets(rt)); err != nil {
		return err
	}
	staged, err := stageRuntimeConfig(p, data)
	if err != nil {
		return err
	}
	defer os.Remove(staged)
	return syncStagedAndRestart(staged, rt, name, run)
}

// syncStagedAndRestart 保证重启只能发生在最新配置已安装且通过校验之后。
func syncStagedAndRestart(staged string, rt runtimePaths, name string, run rootCommandRunner) error {
	if _, err := run(installConfigCmd(staged, rt), "", nil); err != nil {
		return fmt.Errorf(i18n.T("同步运行时配置: %w"), err)
	}
	if _, err := run([]string{rt.Bin, "-t", "-d", rt.Dir, "-f", rt.Config}, i18n.T("校验配置"), nil); err != nil {
		return fmt.Errorf(i18n.T("校验运行时配置: %w"), err)
	}
	if _, err := run([]string{"systemctl", "restart", name + ".service"}, "", nil); err != nil {
		return fmt.Errorf(i18n.T("重启服务: %w"), err)
	}
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
		runRoot([]string{"rm", "-f", rt.Config}, "", nil)
		remaining, _ := runRoot(
			[]string{"bash", "-c", fmt.Sprintf("ls %s/*.yaml 2>/dev/null | wc -l", paths.RuntimeDir)},
			"", &execx.Opt{Capture: true})
		if strings.TrimSpace(remaining.Stdout) == "0" {
			runRoot([]string{"rm", "-rf", paths.RuntimeDir}, "", nil)
		}
	}
	execx.Ok(i18n.T("服务已删除: ") + name + ".service")
	return nil
}

func removeUnit(name string, quiet bool) {
	opt := &execx.Opt{Capture: quiet}
	runRoot([]string{"systemctl", "stop", name + ".service"}, "", opt)
	runRoot([]string{"systemctl", "disable", name + ".service"}, "", opt)
	runRoot([]string{"rm", "-f", rtPaths(name).Unit}, "", nil)
	runRoot([]string{"systemctl", "daemon-reload"}, "", opt)
	runRoot([]string{"systemctl", "reset-failed", name + ".service"}, "", opt)
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
		runRoot([]string{"systemctl", "stop", unit}, "", &execx.Opt{Capture: true})
	}
	runRoot([]string{"systemctl", "stop", name + ".service"}, "", nil)
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
	runRoot([]string{"systemctl", "start", name + ".service"}, "", nil)
	for _, unit := range companions {
		runRoot([]string{"systemctl", "start", unit}, "", &execx.Opt{Capture: true})
	}
	suffix := ""
	if len(companions) > 0 {
		suffix = fmt.Sprintf(i18n.T(" + %d 个伴生单元"), len(companions))
	}
	execx.Ok(i18n.T("已启动：") + name + ".service" + suffix)
	return nil
}

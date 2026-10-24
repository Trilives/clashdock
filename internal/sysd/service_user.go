package sysd

// 系统模式的 mihomo 以专用系统账户运行：被篡改的配置（provider / 面板路径等）只能读写
// 该账户碰得到的文件。运行时目录仍只归 root——内核、配置（root:mihomo 0640）、geo 数据与
// 面板对服务只读，root 执行的 `mihomo -t` 因而不会运行服务账户改过的文件；服务账户只拥有
// <运行时>/home（mihomo 的 -d：cache.db、provider 下载），geo 数据由 ExecStartPre 以服务
// 身份链接进去，面板所在的运行时目录经 SAFE_PATHS 放行（mihomo 要求面板路径落在 -d 或
// SAFE_PATHS 之内）。
//
// 加固只用不依赖挂载命名空间的选项：ProtectSystem= 一类在无特权 LXC 容器里会让服务以
// 226/NAMESPACE 启动失败，而这类 VPS 很常见。

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Trilives/clashdock/internal/i18n"
)

// ServiceUser 运行 mihomo 的系统账户（同名私有组可读运行时配置）。
const ServiceUser = "mihomo"

// serviceHomeMode 服务家目录（cache.db、provider 下载）只归服务账户。
const serviceHomeMode = "0700"

// runtimeConfigMode 运行时配置含节点凭据与控制器 secret：root 可写，服务账户组可读。
const runtimeConfigMode = "0640"

// 可在测试中替换。
var (
	lookupServiceUser = user.Lookup
	lookPath          = exec.LookPath
)

// ensureServiceUser 服务账户不存在时创建：系统账户、同名私有组、无家目录、不可登录。
func ensureServiceUser() error {
	if _, err := lookupServiceUser(ServiceUser); err == nil {
		return nil
	}
	_, err := runRoot([]string{"useradd", "--system", "--user-group", "--no-create-home",
		"--home-dir", "/nonexistent", "--shell", nologinShell(), ServiceUser},
		i18n.T("创建服务账户 ")+ServiceUser, nil)
	return err
}

func nologinShell() string {
	for _, sh := range []string{"/usr/sbin/nologin", "/sbin/nologin"} {
		if _, err := os.Stat(sh); err == nil {
			return sh
		}
	}
	return "/bin/false"
}

// migrateCacheScript 旧版本以 root 运行时 cache.db（已选节点等）在运行时目录根；仅在旧文件
// 还在、家目录里还没有时搬进去一次。家目录归服务账户：mv -T 与 chown -h 都不跟随其中的符号链接。
const migrateCacheScript = `[ -f "$1" ] && [ ! -e "$2" ] && [ ! -L "$2" ] || exit 0
mv -T "$1" "$2" && chown -h ` + ServiceUser + `:` + ServiceUser + ` "$2"`

// runtimeLayoutSteps 运行时目录归 root；服务家目录归服务账户（已存在则纠正属主与权限）。
func runtimeLayoutSteps(rt runtimePaths) [][]string {
	return [][]string{
		{"install", "-d", "-m", "0755", rt.Dir},
		{"install", "-d", "-o", ServiceUser, "-g", ServiceUser, "-m", serviceHomeMode, rt.Home},
		{"sh", "-c", migrateCacheScript, "sh", filepath.Join(rt.Dir, "cache.db"), filepath.Join(rt.Home, "cache.db")},
	}
}

// installConfigCmd 以 root 属主、服务账户组安装运行时配置。
func installConfigCmd(staged string, rt runtimePaths) []string {
	return []string{"install", "-m", runtimeConfigMode, "-o", "root", "-g", ServiceUser, staged, rt.Config}
}

// unitData 主服务单元模板的参数。
type unitData struct {
	Name, User, RuntimeDir, Home, Bin, Config, Ln string
	Geo                                           []string // 运行时里现有的 geo 文件，链接进服务家目录
}

func newUnitData(name string, rt runtimePaths) unitData {
	ln, err := lookPath("ln")
	if err != nil {
		ln = "/bin/ln"
	}
	var geo []string
	for _, f := range []string{rt.Geoip, rt.Country, rt.Geosite} {
		if _, err := os.Stat(f); err == nil {
			geo = append(geo, f)
		}
	}
	return unitData{Name: name, User: ServiceUser, RuntimeDir: rt.Dir, Home: rt.Home,
		Bin: rt.Bin, Config: rt.Config, Ln: ln, Geo: geo}
}

func renderUnit(name string, rt runtimePaths) (string, error) {
	t, err := template.New("unit").Parse(mihomoUnitTmpl)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	err = t.Execute(&sb, newUnitData(name, rt))
	return sb.String(), err
}

// UnitOutdated 已安装的主服务单元与当前版本会写出的不一致（如旧版本以 root 运行 mihomo），
// 需要重新部署（Install）才能生效。
func UnitOutdated(name string) bool {
	if name == "" {
		name = DefaultName
	}
	rt := rtPaths(name)
	raw, err := os.ReadFile(rt.Unit)
	if err != nil {
		return false
	}
	want, err := renderUnit(name, rt)
	return err == nil && string(raw) != want
}

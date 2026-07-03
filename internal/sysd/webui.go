// 独立 Web 面板服务（可选）：把已下载的面板托管在根路径，浏览器打开根地址即用。
//
// mihomo 自带的 external-ui 只挂在控制器的 /ui 子路径；本文件用 clashdock 自身的
// `webui-serve` 静态服务子命令（取代 Python 版的 python3 -m http.server，零外部依赖）
// 把同一份面板挂在独立端口的根路径，并把面板 config.js 的 defaultBackendURL 指向控制器。
// 面板文件单独暂存到 /etc/mihomo/webui，与主服务的 /etc/mihomo/ui 互不影响。
package sysd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/paths"
)

const (
	WebUIName        = "mihomo-webui"
	DefaultWebUIPort = 9091
	controllerPort   = 9090
)

func webuiRuntimeDir() string { return filepath.Join(paths.EtcDir, "webui") }

func webuiUnit() string { return "/etc/systemd/system/" + WebUIName + ".service" }

func WebUIInstalled() bool {
	_, err := os.Stat(webuiUnit())
	return err == nil
}

func configJS(backendURL string) string {
	return "window.__METACUBEXD_CONFIG__ = {\n" +
		fmt.Sprintf("  defaultBackendURL: '%s',\n", backendURL) +
		"}\n"
}

// ConfigureBackend 把面板 config.js 的 defaultBackendURL 写到源 state/ui。
func ConfigureBackend(p paths.Paths, backendURL string) error {
	if _, err := os.Stat(p.UI); err != nil {
		return nil
	}
	return os.WriteFile(filepath.Join(p.UI, "config.js"), []byte(configJS(backendURL)), 0o644)
}

// defaultBackend 本机/SSH 隧道场景直连本地控制器；LAN 场景留空由用户首次填入。
func defaultBackend(lan bool) string {
	if lan {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d", controllerPort)
}

func webuiUnitText(port int, host, uiDir, selfExe string) string {
	return fmt.Sprintf(`[Unit]
Description=mihomo Web UI static server (root-path panel, %s)
After=network-online.target mihomo.service
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=%s webui-serve --port %d --bind %s --dir %s
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
`, WebUIName, selfExe, port, host, uiDir)
}

// WebUIOptions InstallWebUI 的选项；BackendURL 为 nil 表示按 lan 取默认。
type WebUIOptions struct {
	Port       int
	Lan        bool
	BackendURL *string
}

// InstallWebUI 安装/重装独立面板服务并启动。需先下载 Web UI。
func InstallWebUI(p paths.Paths, opts WebUIOptions) error {
	if !execx.Have("systemctl") {
		return fmt.Errorf("未找到 systemctl，独立面板服务需要 systemd")
	}
	if _, err := os.Stat(filepath.Join(p.UI, "index.html")); err != nil {
		return fmt.Errorf("未找到 Web UI，请先执行『下载内核 / UI / geo 数据』")
	}
	if opts.Port == 0 {
		opts.Port = DefaultWebUIPort
	}
	host := "127.0.0.1"
	if opts.Lan {
		host = "0.0.0.0"
	}
	backend := defaultBackend(opts.Lan)
	if opts.BackendURL != nil {
		backend = *opts.BackendURL
	}
	selfExe, err := os.Executable()
	if err != nil {
		return err
	}

	if err := execx.EnsureSudo("安装独立 Web 面板服务"); err != nil {
		return err
	}
	if err := ConfigureBackend(p, backend); err != nil {
		return err
	}
	if _, err := execx.RunRoot([]string{"mkdir", "-p", paths.EtcDir}, "", nil); err != nil {
		return err
	}
	execx.RunRoot([]string{"rm", "-rf", webuiRuntimeDir()}, "", nil)
	if _, err := execx.RunRoot([]string{"cp", "-a", p.UI, webuiRuntimeDir()}, "", nil); err != nil {
		return err
	}
	if err := execx.WriteRoot(webuiUnit(), webuiUnitText(opts.Port, host, webuiRuntimeDir(), selfExe), "0644", "写面板单元"); err != nil {
		return err
	}
	if _, err := execx.RunRoot([]string{"systemctl", "daemon-reload"}, "", nil); err != nil {
		return err
	}
	if _, err := execx.RunRoot([]string{"systemctl", "enable", "--now", WebUIName + ".service"}, "", nil); err != nil {
		return err
	}
	disp := "127.0.0.1"
	if opts.Lan {
		disp = host
	}
	execx.Ok(fmt.Sprintf("独立 Web 面板已启动：http://%s:%d/", disp, opts.Port))
	if opts.Lan && backend == "" {
		execx.Info(fmt.Sprintf("首次打开请在面板里填后端地址 http://<服务器IP>:%d（如设了 secret 一并填）。", controllerPort))
	}
	return nil
}

// RemoveWebUI 卸载独立面板服务并清理暂存。
func RemoveWebUI() error {
	if err := execx.EnsureSudo("卸载独立 Web 面板服务"); err != nil {
		return err
	}
	quiet := &execx.Opt{Capture: true}
	execx.RunRoot([]string{"systemctl", "stop", WebUIName + ".service"}, "", quiet)
	execx.RunRoot([]string{"systemctl", "disable", WebUIName + ".service"}, "", quiet)
	execx.RunRoot([]string{"rm", "-f", webuiUnit()}, "", nil)
	execx.RunRoot([]string{"rm", "-rf", webuiRuntimeDir()}, "", nil)
	execx.RunRoot([]string{"systemctl", "daemon-reload"}, "", nil)
	execx.Ok("独立 Web 面板已卸载。")
	return nil
}

// RefreshWebUI UI 更新后按现有设置重新暂存并重启（保持 config.js 后端设置）。
func RefreshWebUI(p paths.Paths, port int, lan bool) error {
	if !WebUIInstalled() {
		return nil
	}
	return InstallWebUI(p, WebUIOptions{Port: port, Lan: lan})
}

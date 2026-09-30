package usermode

import (
	"os"
	"path/filepath"
)

// UnitName 用户级 systemd 单元名（与完整模式的系统单元 mihomo.service 区分）。
const UnitName = "clashdock-mihomo"

// LegacyWorkdirName 旧便携模式放在便携包目录旁的工作目录名；仅用于迁移旧数据。
const LegacyWorkdirName = "clashdock-data"

// detachedMarkerName 无 systemd 用户实例时，后台进程后端的「已部署」标记文件名
// （位于 state 下，供 Installed / 模式判定使用）。
const detachedMarkerName = "usermode-detached"

// homeDir 当前用户主目录；取不到时回退 $HOME，再退到当前目录，保证路径可拼。
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "."
}

// xdgDir 读取 XDG 目录变量；未设置或不是绝对路径（规范要求忽略）时用 fallback。
func xdgDir(env, fallback string) string {
	if v := os.Getenv(env); v != "" && filepath.IsAbs(v) {
		return v
	}
	return fallback
}

// StateDir 用户模式的数据目录：$XDG_DATA_HOME/clashdock（默认 ~/.local/share/clashdock）。
// 固定在用户主目录下，不随可执行文件或启动时的当前目录变化，用户服务单元引用的
// 路径因此始终有效。
func StateDir() string {
	return filepath.Join(xdgDir("XDG_DATA_HOME", filepath.Join(homeDir(), ".local", "share")), "clashdock")
}

// BinPath 用户模式安装的 clashdock 本体：~/.local/bin/clashdock（多数发行版默认在 PATH 中）。
func BinPath() string {
	return filepath.Join(homeDir(), ".local", "bin", "clashdock")
}

// UnitDir 用户级 systemd 单元目录：$XDG_CONFIG_HOME/systemd/user（默认 ~/.config/systemd/user）。
func UnitDir() string {
	return filepath.Join(xdgDir("XDG_CONFIG_HOME", filepath.Join(homeDir(), ".config")), "systemd", "user")
}

// UnitPath 用户级服务单元文件路径。
func UnitPath() string {
	return filepath.Join(UnitDir(), UnitName+".service")
}

// LegacyWorkdir 旧便携模式的数据目录（可执行文件旁的 clashdock-data），迁移用。
func LegacyWorkdir(execPath string) string {
	return filepath.Join(filepath.Dir(execPath), LegacyWorkdirName)
}

func detachedMarker(state string) string {
	return filepath.Join(state, detachedMarkerName)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

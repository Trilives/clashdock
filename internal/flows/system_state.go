package flows

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

// systemStateMode 系统模式状态目录：只有 root 可写，其他人可进入（凭据另有权限，见
// paths.SecretFileMode）。clashdock 自更新的版本目录在其中，/usr/bin/clashdock 可能经它
// 解析，普通用户也要能执行只读子命令。
const systemStateMode = 0o755

// ResumeFlag 提权重启时附带的隐藏参数：root 进程打开主菜单后直接进入该项（见 RequireRoot）。
const ResumeFlag = "--resume="

// ResumeUpdate 「工具 → 更新」提权后恢复的菜单项。
const ResumeUpdate = "update"

// 可在测试中替换。
var (
	secureTree  = paths.SecureTree
	lookupUser  = user.LookupId
	lookupGroup = user.LookupGroupId
)

// RequireRoot 系统模式下普通用户进入写状态的菜单项前调用：先经 sudo -v 验证（取消或
// 失败则留在当前菜单），再经 sudo 以 root 重启 clashdock 并直接进入 target，成功时
// 不返回。已是 root 或处于用户模式时返回 nil，调用方照常继续。只读操作不调用它，
// 打开主菜单因此不必每次输入密码。
func RequireRoot(p paths.Paths, target string) error {
	if p.UserMode || execx.IsRoot() {
		return nil
	}
	if err := execx.EnsureSudo(i18n.T("修改系统模式的设置与服务")); err != nil {
		return err
	}
	exe, err := execx.StableExecutablePath()
	if err != nil {
		return err
	}
	execx.Info(i18n.T("系统模式由 root 管理，正在通过 sudo 重新启动 clashdock…"))
	return execx.ReexecAsRoot(exe, []string{ResumeFlag + target})
}

// EnsureStateRoot 确保系统模式数据目录存在且只有 root 可写：要求以 root 运行，旧版本
// 交给调用用户的目录在此收归，绝不再交还普通用户；属组见 stateGroup。
func EnsureStateRoot(p paths.Paths) error {
	return ensureSystemState(p, os.Geteuid())
}

func ensureSystemState(p paths.Paths, euid int) error {
	if euid != 0 {
		return fmt.Errorf(i18n.T("系统模式的数据目录 %s 只能由 root 管理，请用 sudo 运行"), p.State)
	}
	if err := os.MkdirAll(p.State, systemStateMode); err != nil {
		return err
	}
	return secureSystemState(p)
}

// SecureSystemState 供 root 运行的只读入口（watchdog 的 healthcheck 等）在读写状态前
// 调用：目录不存在时什么都不做，存在则确保整棵树只有 root 可写。返回 nil 表示 root
// 可以放心读写其中的文件。
func SecureSystemState(p paths.Paths) error {
	if p.UserMode || !execx.IsRoot() {
		return nil
	}
	if _, err := os.Lstat(p.State); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return secureSystemState(p)
}

// secureSystemState 旧版本把系统模式状态目录交给了调用用户，root 定时器与服务却会
// 读取、执行其中的文件；整棵树收归 root 属主与 stateGroup 属组（幂等，已合规时只做
// 一遍 lstat），再收紧凭据权限。
func secureSystemState(p paths.Paths) error {
	gid, err := stateGroup(p.State, os.Getenv)
	if err != nil {
		return err
	}
	if err := secureTree(p.State, gid); err != nil {
		return fmt.Errorf(i18n.T("收归数据目录 %s 为 root 属主失败：%w"), p.State, err)
	}
	return tightenCredentials(p)
}

// stateGroup 状态目录应有的属组：root 已选定的（目录属 root 且属组非 0）沿用；否则取
// 管理员——仍持有目录的旧版本调用用户，或经 sudo 运行的 SUDO_UID——的私有组，让其不经
// sudo 也能读取设置、配置与日志。认不出私有组时返回 0：只有 root 可读，普通用户打开
// 主菜单时整体提权（见 cmd/clashdock/privilege.go）。
func stateGroup(dir string, getenv func(string) string) (int, error) {
	info, err := os.Lstat(dir)
	if err != nil {
		return 0, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	switch {
	case !ok:
		return 0, nil
	case st.Uid != 0:
		return privateGroup(strconv.Itoa(int(st.Uid)), strconv.Itoa(int(st.Gid))), nil
	case st.Gid != 0:
		return int(st.Gid), nil
	default:
		return privateGroup(getenv("SUDO_UID"), getenv("SUDO_GID")), nil
	}
}

// privateGroup gid 是 uid 的主组且组名与用户名相同（各发行版 useradd 的默认做法）时
// 返回 gid：组内只有管理员本人，组可读不会把凭据泄露给其他本地用户。否则返回 0。
func privateGroup(uid, gid string) int {
	if uid == "" || uid == "0" || gid == "" {
		return 0
	}
	u, err := lookupUser(uid)
	if err != nil || u.Gid != gid {
		return 0
	}
	g, err := lookupGroup(gid)
	if err != nil || g.Name != u.Username {
		return 0
	}
	n, err := strconv.Atoi(gid)
	if err != nil {
		return 0
	}
	return n
}

// tightenCredentials 订阅原文、生成配置、customize.json 与日志含节点凭据或 Token，
// 旧版本以 0644 写入；仅对已存在的条目收紧（保留目录的 setgid）。
func tightenCredentials(p paths.Paths) error {
	targets := []struct {
		path string
		mode os.FileMode
	}{
		{p.Subscriptions, paths.SecretDirMode},
		{p.ConfigFile, paths.SecretFileMode},
		{p.CustomizeFile, paths.SecretFileMode},
		{execx.LogPath(p.State), paths.SecretFileMode},
	}
	for _, t := range targets {
		st, err := os.Lstat(t.path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm() == t.mode {
			continue
		}
		if err := os.Chmod(t.path, t.mode|st.Mode()&os.ModeSetgid); err != nil {
			return err
		}
	}
	return nil
}

// 定制层交互式编辑（对应 customize.py 的 edit 部分）：缓冲式——
// 「保存并退出」才写盘，^R 放弃全部本次修改。
package flows

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/errs"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/firewall"
	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/tui"
)

// fieldGroups 编辑定制层按用途拆成的子分组：字段一多堆在同一屏既难找，又会让
// 序号超出带圈数字范围、同一菜单里出现"前面带圈、后面变阿拉伯数字"的不统一
// 观感（见 tui.numFor）。每个分组各自远小于带圈数字上限，天然保持统一。
func fieldGroups() []struct {
	title  string
	fields []string
} {
	return []struct {
		title  string
		fields []string
	}{
		{"部署设置（TUN / 面板 / 下载）", config.DeploymentFields},
		{"自定义分流叠加（AI / 流媒体 / 地区组）", config.OverlayFields},
	}
}

// EditCustomize 交互式编辑 customize.json（缓冲式）：先选分组，再编辑分组内
// 字段；esc 在分组选择这一层才会真正保存并退出，^R 放弃全部本次修改（含跨
// 分组的改动）。分组内 esc/^R 都只是返回上一层分组选择，不提交也不放弃。
func EditCustomize(p paths.Paths) (bool, error) {
	original := config.Load(p)
	cfg := deepCopyMap(original)
	changed := false
	groups := fieldGroups()
	idx := 0
	for {
		labels := make([]string, len(groups))
		for i, g := range groups {
			labels[i] = i18n.T(g.title)
		}
		i, err := tui.Select(i18n.T("编辑定制层"), labels,
			tui.SelectOpts{BackLabel: i18n.T("放弃修改并退出"), SaveLabel: i18n.T("保存并退出"), Initial: idx})
		if err != nil {
			if errors.Is(err, errs.ErrSaveExit) {
				if !changed {
					execx.Info(i18n.T("未做修改。"))
					return false, nil
				}
				if serr := config.Save(p, cfg); serr != nil {
					return false, serr
				}
				execx.Ok(i18n.T("定制层已保存。"))
				if ferr := syncLanProxyFirewall(original, cfg); ferr != nil {
					return true, ferr
				}
				syncLogging(p, cfg)
				return true, nil
			}
			if changed {
				execx.Warn(i18n.T("已放弃本次修改（未写盘）。"))
			}
			return false, nil
		}
		idx = i
		changed = editFieldGroup(cfg, i18n.T(groups[i].title), groups[i].fields) || changed
	}
}

// editFieldGroup 单个分组内的字段编辑循环；esc/^R 都只是返回分组选择菜单。
func editFieldGroup(cfg map[string]any, title string, fields []string) bool {
	changed := false
	idx := 0
	for {
		labels := make([]string, len(fields))
		for i, k := range fields {
			labels[i] = config.FieldLabel(cfg, k)
		}
		i, err := tui.Select(title, labels, tui.SelectOpts{BackLabel: i18n.T("返回上层"), Initial: idx})
		if err != nil {
			return changed
		}
		idx = i
		key := fields[i]
		switch {
		case config.ListFields[key] != "":
			changed = editList(cfg, key, config.ListFields[key]) || changed
		case config.BoolFields[key] != "":
			cfg[key] = !config.Bool(cfg, key)
			changed = true
		default:
			changed = editScalar(cfg, key, config.ScalarFields[key]) || changed
		}
	}
}

func deepCopyMap(m map[string]any) map[string]any {
	raw, _ := json.Marshal(m)
	var out map[string]any
	json.Unmarshal(raw, &out)
	return out
}

// syncLanProxyFirewall lan_proxy 开关变化时，按需更新防火墙放行 7890 端口。
func syncLanProxyFirewall(original, cfg map[string]any) error {
	before, after := config.Bool(original, "lan_proxy"), config.Bool(cfg, "lan_proxy")
	if before == after {
		return nil
	}
	prompt := i18n.T("已开启局域网代理，更新防火墙放行 7890 端口？")
	if !after {
		prompt = i18n.T("已关闭局域网代理，撤销防火墙放行 7890 端口？")
	}
	ok, err := tui.Confirm(prompt, true)
	if err != nil || !ok {
		return nil
	}
	if after {
		firewall.Allow(firewall.ProxyPort)
	} else {
		firewall.Revoke(firewall.ProxyPort)
	}
	return nil
}

// syncLogging enable_log 开关变化时立即生效，不必等下次启动 clashdock。
func syncLogging(p paths.Paths, cfg map[string]any) {
	if !config.Bool(cfg, "enable_log") {
		execx.DisableLog()
		return
	}
	if err := execx.EnableLog(execx.LogPath(p.State), 0); err != nil {
		execx.Warn(i18n.T("日志启用失败：") + err.Error())
	}
}

func editList(cfg map[string]any, key, label string) bool {
	isInt := key == "tun_exclude_uids" || key == "direct_ports"
	changed := false
	act := 0
	for {
		items := config.StrList(cfg, key)
		summary := ""
		if len(items) > 0 {
			summary = "：" + strings.Join(items, ", ")
		}
		execx.Info(fmt.Sprintf(i18n.T("%s：当前 %d 条%s"), i18n.T(label), len(items), summary))
		a, err := tui.Select(i18n.T("编辑 · ")+i18n.T(label),
			[]string{i18n.T("添加一条"), i18n.T("删除一条"), i18n.T("批量粘贴替换（逗号/空格分隔）"), i18n.T("恢复默认"), i18n.T("清空")},
			tui.SelectOpts{Initial: act})
		if err != nil {
			return changed
		}
		act = a

		ok := true
		switch a {
		case 0:
			val, err := tui.Ask(i18n.T("新增值"), tui.AskOpts{AllowEmpty: false})
			if err != nil || (isInt && !isIntStr(val)) {
				ok = false
				break
			}
			items = append(items, val)
		case 1:
			if len(items) == 0 {
				continue
			}
			di, err := tui.Select(i18n.T("删除哪一条"), items, tui.SelectOpts{})
			if err != nil {
				ok = false
				break
			}
			items = append(items[:di], items[di+1:]...)
		case 2:
			raw, err := tui.Ask(i18n.T("粘贴（逗号或空格分隔）"), tui.AskOpts{AllowEmpty: true})
			if err != nil {
				ok = false
				break
			}
			toks := strings.Fields(strings.ReplaceAll(raw, ",", " "))
			if isInt {
				for _, t := range toks {
					if !isIntStr(t) {
						ok = false
						break
					}
				}
			}
			if ok {
				items = toks
			}
		case 3:
			items = config.StrList(config.Defaults(), key)
		case 4:
			items = []string{}
		}
		if !ok {
			execx.Warn(i18n.T("输入无效，已跳过。"))
			continue
		}
		if isInt {
			ints := make([]int, 0, len(items))
			for _, s := range items {
				n, _ := strconv.Atoi(s)
				ints = append(ints, n)
			}
			cfg[key] = ints
		} else {
			cfg[key] = items
		}
		changed = true
	}
}

func isIntStr(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func editScalar(cfg map[string]any, key, label string) bool {
	cur := config.Str(cfg, key)
	display := ""
	if config.SensitiveFields[key] && cur != "" {
		display = config.MaskSecret(cur)
	}
	val, err := tui.Ask(i18n.T(label)+i18n.T("（留空清除）"), tui.AskOpts{Default: cur, DisplayDefault: display, AllowEmpty: true})
	if err != nil {
		return false
	}
	if key == "bootstrap_dns_port" || key == "webui_port" {
		n, err := strconv.Atoi(val)
		if err != nil {
			execx.Warn(i18n.T("端口需为整数，未修改。"))
			return false
		}
		cfg[key] = n
	} else {
		cfg[key] = val
	}
	return true
}

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
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/tui"
)

func editLabels(cfg map[string]any) []string {
	labels := make([]string, len(config.FieldOrder))
	for i, k := range config.FieldOrder {
		labels[i] = config.FieldLabel(cfg, k)
	}
	return labels
}

// EditCustomize 交互式编辑 customize.json（缓冲式）。返回是否实际保存了改动。
func EditCustomize(p paths.Paths) (bool, error) {
	original := config.Load(p)
	cfg := deepCopyMap(original)
	changed := false
	idx := 0
	for {
		i, err := tui.Select("编辑定制层", editLabels(cfg),
			tui.SelectOpts{BackLabel: "放弃修改并退出", SaveLabel: "保存并退出", Initial: idx})
		if err != nil {
			if errors.Is(err, errs.ErrSaveExit) {
				if !changed {
					execx.Info("未做修改。")
					return false, nil
				}
				if serr := config.Save(p, cfg); serr != nil {
					return false, serr
				}
				execx.Ok("定制层已保存。")
				if ferr := syncLanProxyFirewall(original, cfg); ferr != nil {
					return true, ferr
				}
				return true, nil
			}
			if changed {
				execx.Warn("已放弃本次修改（未写盘）。")
			}
			return false, nil
		}
		idx = i
		key := config.FieldOrder[i]
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
	prompt := "已开启局域网代理，更新防火墙放行 7890 端口？"
	if !after {
		prompt = "已关闭局域网代理，撤销防火墙放行 7890 端口？"
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

func editList(cfg map[string]any, key, label string) bool {
	isInt := key == "tun_exclude_uids"
	changed := false
	act := 0
	for {
		items := config.StrList(cfg, key)
		summary := ""
		if len(items) > 0 {
			summary = "：" + strings.Join(items, ", ")
		}
		execx.Info(fmt.Sprintf("%s：当前 %d 条%s", label, len(items), summary))
		a, err := tui.Select("编辑 · "+label,
			[]string{"添加一条", "删除一条", "批量粘贴替换（逗号/空格分隔）", "恢复默认", "清空"},
			tui.SelectOpts{Initial: act})
		if err != nil {
			return changed
		}
		act = a

		ok := true
		switch a {
		case 0:
			val, err := tui.Ask("新增值", tui.AskOpts{AllowEmpty: false})
			if err != nil || (isInt && !isIntStr(val)) {
				ok = false
				break
			}
			items = append(items, val)
		case 1:
			if len(items) == 0 {
				continue
			}
			di, err := tui.Select("删除哪一条", items, tui.SelectOpts{})
			if err != nil {
				ok = false
				break
			}
			items = append(items[:di], items[di+1:]...)
		case 2:
			raw, err := tui.Ask("粘贴（逗号或空格分隔）", tui.AskOpts{AllowEmpty: true})
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
			execx.Warn("输入无效，已跳过。")
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
	val, err := tui.Ask(label+"（留空清除）", tui.AskOpts{Default: cur, DisplayDefault: display, AllowEmpty: true})
	if err != nil {
		return false
	}
	if key == "bootstrap_dns_port" || key == "webui_port" {
		n, err := strconv.Atoi(val)
		if err != nil {
			execx.Warn("端口需为整数，未修改。")
			return false
		}
		cfg[key] = n
	} else {
		cfg[key] = val
	}
	return true
}

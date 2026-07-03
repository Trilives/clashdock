// 命名订阅管理（对应 manager.py）：增 / 删 / 改名 / 切换 active / 刷新 / 列表。
//
// 每个订阅存于 state/subscriptions/<name>/：meta.json + raw.* + config.yaml。
// active 指针（state/active）决定哪份部署生效；切换会同步 state/config.yaml 并重启服务。
//
// mihomo 直用机场订阅：clash/mihomo 来源经 patch 最小改写即可；base64 来源先经
// subconverter 转 Clash 再 patch。自定义分流叠加（overlay）为可选，默认关闭。
package subscription

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Trilives/clashdock/internal/config"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/jsonx"
	"github.com/Trilives/clashdock/internal/paths"
	"github.com/Trilives/clashdock/internal/sysd"
)

// Subscription 元数据；JSON 字段与 Python 版 meta.json 完全一致（老数据直读）。
type Subscription struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	SourceType    string `json:"source_type"`
	ApplyOverlay  bool   `json:"apply_overlay"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	LastNodeCount int    `json:"last_node_count"`
}

var rawExt = map[string]string{"clash": "yaml", "base64": "txt"}

// now 对应 Python isoformat(timespec="seconds")：2026-07-03T12:34:56+00:00。
func now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05+00:00")
}

// Slug 订阅名清洗：折叠空白、去路径分隔符与 ".."，空名回退 "sub"。
func Slug(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, "..", "-")
	name = strings.Join(strings.Fields(name), "-")
	name = strings.Trim(name, ". ")
	if name == "" {
		return "sub"
	}
	return name
}

func metaFile(p paths.Paths, name string) string {
	return filepath.Join(p.SubscriptionDir(name), "meta.json")
}

func configFile(p paths.Paths, name string) string {
	return filepath.Join(p.SubscriptionDir(name), "config.yaml")
}

func rawFile(p paths.Paths, sub *Subscription) string {
	ext, ok := rawExt[sub.SourceType]
	if !ok {
		ext = "txt"
	}
	return filepath.Join(p.SubscriptionDir(sub.Name), "raw."+ext)
}

// --------------------------------------------------------------------------
// 读取
// --------------------------------------------------------------------------

func ListAll(p paths.Paths) []Subscription {
	var subs []Subscription
	entries, err := os.ReadDir(p.Subscriptions)
	if err != nil {
		return subs
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if sub := Get(p, e.Name()); sub != nil {
			subs = append(subs, *sub)
		}
	}
	return subs
}

func Get(p paths.Paths, name string) *Subscription {
	raw, err := os.ReadFile(metaFile(p, name))
	if err != nil {
		return nil
	}
	var sub Subscription
	if err := json.Unmarshal(raw, &sub); err != nil {
		return nil
	}
	return &sub
}

func GetActive(p paths.Paths) *Subscription {
	raw, err := os.ReadFile(p.ActiveFile)
	if err != nil {
		return nil
	}
	return Get(p, strings.TrimSpace(string(raw)))
}

// --------------------------------------------------------------------------
// 增 / 改
// --------------------------------------------------------------------------

// Add 新增订阅（拉取 → 生成配置）；setActive 时切换生效。
func Add(p paths.Paths, name, subURL, sourceType string, applyOverlay, setActive bool) (*Subscription, error) {
	name = Slug(name)
	if _, err := os.Stat(metaFile(p, name)); err == nil {
		return nil, fmt.Errorf("订阅「%s」已存在，请改名或先删除", name)
	}
	sub := &Subscription{
		Name: name, URL: subURL, SourceType: sourceType, ApplyOverlay: applyOverlay,
		CreatedAt: now(), UpdatedAt: now(),
	}
	if err := build(p, sub); err != nil {
		return nil, err
	}
	if setActive {
		if err := applyActive(p, name); err != nil {
			return sub, err
		}
	}
	return sub, nil
}

// Refresh 联网重新拉取订阅原文并重建（用于「刷新订阅」/ 定时更新）。
func Refresh(p paths.Paths, name string) (*Subscription, error) {
	sub := Get(p, name)
	if sub == nil {
		return nil, fmt.Errorf("订阅不存在: %s", name)
	}
	sub.UpdatedAt = now()
	if err := build(p, sub); err != nil {
		return nil, err
	}
	if active := GetActive(p); active != nil && active.Name == name {
		if err := applyActive(p, name); err != nil {
			return sub, err
		}
	}
	return sub, nil
}

// Rebuild 基于本地已存订阅原文重新生成（不联网），用于应用定制层等本地改动；
// 本地无原文（异常情况）时回退为联网刷新。
func Rebuild(p paths.Paths, name string) (*Subscription, error) {
	sub := Get(p, name)
	if sub == nil {
		return nil, fmt.Errorf("订阅不存在: %s", name)
	}
	raw, err := os.ReadFile(rawFile(p, sub))
	if err != nil {
		execx.Warn("本地缺少订阅原文，改为联网刷新。")
		return Refresh(p, name)
	}
	sub.UpdatedAt = now()
	execx.Info(fmt.Sprintf("用本地原文重新生成「%s」（不重新拉取）…", sub.Name))
	if err := convertAndWrite(p, sub, raw, config.Load(p)); err != nil {
		return nil, err
	}
	if active := GetActive(p); active != nil && active.Name == name {
		if err := applyActive(p, name); err != nil {
			return sub, err
		}
	}
	return sub, nil
}

// build 拉取 → 写 raw → 生成配置写盘。
func build(p paths.Paths, sub *Subscription) error {
	cfg := config.Load(p)
	proxy := config.Str(cfg, "download_proxy")
	execx.Info(fmt.Sprintf("拉取订阅「%s」…", sub.Name))
	raw, err := Fetch(sub.URL, sub.SourceType, proxy)
	if err != nil {
		return err
	}
	if msg := WarnIfMismatch(sub.SourceType, raw); msg != "" {
		execx.Warn(msg)
	}
	if err := os.MkdirAll(p.SubscriptionDir(sub.Name), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(rawFile(p, sub), raw, 0o644); err != nil {
		return err
	}
	return convertAndWrite(p, sub, raw, cfg)
}

// convertAndWrite 把订阅原文生成 mihomo 配置（直用订阅 + 最小改写），写 config.yaml/meta。
func convertAndWrite(p paths.Paths, sub *Subscription, raw []byte, cfg map[string]any) error {
	text := string(raw)

	var clash map[string]any
	if sub.SourceType == "base64" {
		execx.Info("经 subconverter 将 base64 转为 Clash…")
		var err error
		clash, err = ToClashDict(text, cfg)
		if err != nil {
			return err
		}
	} else {
		var data any
		if err := yaml.Unmarshal([]byte(text), &data); err != nil {
			return &PatchError{"订阅 YAML 解析失败或根不是映射。"}
		}
		m, ok := data.(map[string]any)
		if !ok {
			return &PatchError{"订阅 YAML 解析失败或根不是映射。"}
		}
		clash = m
	}

	execx.Info("生成 mihomo 配置（直用订阅 + 最小改写）…")
	cfgOut, info, err := Build(clash, cfg, p.UI)
	if err != nil {
		return err
	}

	if sub.ApplyOverlay {
		execx.Info("叠加自定义分流（overlay）…")
		var ovInfo map[string]any
		cfgOut, ovInfo = ApplyOverlay(cfgOut, cfg)
		for k, v := range ovInfo {
			info[k] = v
		}
	}

	// 地区自动测速聚合组：各地区独立开关，不依赖 overlay / apply_overlay
	if truthy(cfg, "generate_sg_groups", false) || truthy(cfg, "generate_hk_groups", false) {
		var rgInfo map[string]any
		cfgOut, rgInfo = ApplyRegionGroups(cfgOut, cfg)
		for k, v := range rgInfo {
			info[k] = v
		}
		if names := strListOf(rgInfo["region_groups"]); len(names) > 0 {
			execx.Info("已生成地区自动测速聚合组：" + strings.Join(names, ", "))
		} else {
			execx.Warn("启用了地区聚合组，但未匹配到对应地区节点（检查关键词与开关）。")
		}
	}

	if n, ok := info["proxies"].(int); ok {
		sub.LastNodeCount = n
	}

	// mihomo 吃 Clash YAML；JSON 内容亦为合法 YAML
	cfgJSON, err := jsonx.MarshalPretty(cfgOut)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configFile(p, sub.Name), cfgJSON, 0o644); err != nil {
		return err
	}
	metaJSON, err := jsonx.MarshalPretty(sub)
	if err != nil {
		return err
	}
	if err := os.WriteFile(metaFile(p, sub.Name), metaJSON, 0o644); err != nil {
		return err
	}
	execx.Ok(fmt.Sprintf("订阅「%s」就绪：%v 节点 / %v 策略组 / %v 规则",
		sub.Name, info["proxies"], info["proxy_groups"], info["rules"]))
	return nil
}

// --------------------------------------------------------------------------
// 切换 / 删除 / 改名
// --------------------------------------------------------------------------

// Switch 切换生效订阅。
func Switch(p paths.Paths, name string) error {
	if _, err := os.Stat(metaFile(p, name)); err != nil {
		return fmt.Errorf("订阅不存在: %s", name)
	}
	if err := applyActive(p, name); err != nil {
		return err
	}
	execx.Ok("已切换生效订阅: " + name)
	return nil
}

func applyActive(p paths.Paths, name string) error {
	if err := p.EnsureStateDirs(); err != nil {
		return err
	}
	data, err := os.ReadFile(configFile(p, name))
	if err != nil {
		return err
	}
	if err := os.WriteFile(p.ConfigFile, data, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(p.ActiveFile, []byte(name+"\n"), 0o644); err != nil {
		return err
	}
	if sysd.IsInstalled(sysd.DefaultName) {
		if err := sysd.SyncAndRestart(p, sysd.DefaultName); err != nil {
			var ce *execx.CommandError
			if errors.As(err, &ce) || err != nil {
				execx.Warn(fmt.Sprintf("配置已切换，但同步到服务失败：%v", err))
			}
		}
	}
	return nil
}

// RemoveSub 删除订阅目录；删除生效订阅时清掉 active 指针并提醒。
func RemoveSub(p paths.Paths, name string) error {
	d := p.SubscriptionDir(name)
	if _, err := os.Stat(d); err != nil {
		return fmt.Errorf("订阅不存在: %s", name)
	}
	active := GetActive(p)
	wasActive := active != nil && active.Name == name
	os.RemoveAll(d)
	if wasActive {
		os.Remove(p.ActiveFile)
		execx.Warn("已删除当前生效订阅；请切换到其它订阅或重新添加。")
	}
	execx.Ok("已删除订阅: " + name)
	return nil
}

// Rename 订阅改名（目录改名 + meta/active 同步）。
func Rename(p paths.Paths, oldName, newName string) error {
	newName = Slug(newName)
	if _, err := os.Stat(metaFile(p, oldName)); err != nil {
		return fmt.Errorf("订阅不存在: %s", oldName)
	}
	if _, err := os.Stat(metaFile(p, newName)); err == nil {
		return fmt.Errorf("目标名已存在: %s", newName)
	}
	if err := os.Rename(p.SubscriptionDir(oldName), p.SubscriptionDir(newName)); err != nil {
		return err
	}
	if sub := Get(p, newName); sub != nil {
		sub.Name = newName
		sub.UpdatedAt = now()
		if metaJSON, err := jsonx.MarshalPretty(sub); err == nil {
			os.WriteFile(metaFile(p, newName), metaJSON, 0o644)
		}
	}
	if active := GetActive(p); active == nil {
		// active 指针指向旧名时 GetActive 已经找不到 meta —— 直接检查指针文件
		if raw, err := os.ReadFile(p.ActiveFile); err == nil && strings.TrimSpace(string(raw)) == oldName {
			os.WriteFile(p.ActiveFile, []byte(newName+"\n"), 0o644)
		}
	}
	execx.Ok(fmt.Sprintf("已改名: %s → %s", oldName, newName))
	return nil
}

// 节点测速与当前状态展示：运行时当前节点、配置首选与延迟标签。
package flows

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"

	"github.com/Trilives/clashdock/internal/clashapi"
	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
)

// measure 并发实测延迟，带 TTY 进度。
func measure(api *clashapi.Client, names []string) map[string]int {
	if len(names) == 0 {
		return nil
	}
	tty := term.IsTerminal(int(os.Stdout.Fd()))
	if !tty {
		execx.Info(fmt.Sprintf(i18n.T("测速中（%d 个节点）…"), len(names)))
	}
	results := make(map[string]int, len(names))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, min(16, len(names)))
	done := 0
	for _, name := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ms, ok := api.Delay(n)
			mu.Lock()
			if ok {
				results[n] = ms
			}
			done++
			if tty {
				fmt.Printf(i18n.T("\r\033[K  测速中… %d/%d"), done, len(names))
			}
			mu.Unlock()
		}(name)
	}
	wg.Wait()
	if tty {
		fmt.Print("\r\033[K")
	}
	execx.Ok(fmt.Sprintf(i18n.T("测速完成：%d/%d 可用"), len(results), len(names)))
	return results
}

func fmtDelay(results map[string]int, name string) string {
	if ms, ok := results[name]; ok {
		return fmt.Sprintf("%dms", ms)
	}
	return i18n.T("超时")
}

func preferredNode(group map[string]any) string {
	members, _ := group["proxies"].([]any)
	if len(members) == 0 {
		return ""
	}
	return fmt.Sprint(members[0])
}

// currentNodeLabels 同时展示运行时当前节点与配置中的固定首选。runtimeOK=false
// 表示无法读取 API 状态，此时配置首选必须明确标成非运行时状态。
func currentNodeLabels(names []string, delays map[string]int, runtimeCurrent, pinned string, runtimeOK bool) []string {
	labels := make([]string, len(names))
	for i, name := range names {
		parts := []string{name}
		if delays != nil {
			parts = append(parts, fmtDelay(delays, name))
		}
		var states []string
		if runtimeOK && name == runtimeCurrent {
			states = append(states, i18n.T("当前运行"))
		}
		if name == pinned {
			states = append(states, i18n.T("配置首选"))
			if !runtimeOK {
				states = append(states, i18n.T("非运行时状态"))
			}
		}
		label := strings.Join(parts, "   ")
		if len(states) > 0 {
			label += "（" + strings.Join(states, "，") + "）"
		}
		labels[i] = label
	}
	return labels
}

func currentNodeSummary(cfg map[string]any, targetName string, selections map[string]string, runtimeOK bool) []string {
	for _, group := range groupsOf(cfg) {
		name := fmt.Sprint(group["name"])
		if name != targetName {
			continue
		}
		if runtimeOK {
			if current := selections[name]; current != "" {
				return []string{fmt.Sprintf("  • %s → %s（%s）", name, current, i18n.T("当前运行"))}
			}
			return nil
		}
		if group["type"] == "select" {
			if pinned := preferredNode(group); pinned != "" {
				return []string{fmt.Sprintf("  • %s → %s（%s，%s）", name, pinned,
					i18n.T("配置首选"), i18n.T("非运行时状态"))}
			}
		}
		return nil
	}
	return nil
}

func printCurrentNodeSummary(cfg map[string]any, targetName string, selections map[string]string, runtimeOK bool) {
	lines := currentNodeSummary(cfg, targetName, selections, runtimeOK)
	if len(lines) == 0 {
		return
	}
	execx.Info(i18n.T("主选择组当前节点："))
	for _, line := range lines {
		fmt.Println(line)
	}
}

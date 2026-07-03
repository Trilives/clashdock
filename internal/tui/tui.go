// Package tui 交互组件（对应 Python 版 menu.py + keys.py）：Bubble Tea 实现的
// 四类阻塞式提示 Select / MultiSelect / Ask / Confirm。
//
// 每次调用运行一个内联 tea.Program（退出后视图留在终端，与 Python 版自绘盒子的
// 观感一致），因此上层 flows 保持命令式结构，可与 Python 版一比一对应。
// 键位契约：↑/↓ 移动、⏎ 确认、esc 保存返回（ErrSaveExit）、^R 回退返回
// （ErrCancelled）、数字键跳选；非 TTY（管道/重定向）自动回退编号列表 + 文本输入。
package tui

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"

	"github.com/Trilives/clashdock/internal/errs"
)

// 键位契约错误（re-export，方便调用方少 import 一个包）。
var (
	ErrSaveExit  = errs.ErrSaveExit
	ErrCancelled = errs.ErrCancelled
)

const (
	ansiReset = "\033[0m"
	ansiDim   = "\033[2m"
	ansiBold  = "\033[1m"
	ansiCyan  = "\033[36m"
)

var circled = []rune("①②③④⑤⑥⑦⑧⑨⑩⑪⑫⑬⑭⑮⑯⑰⑱⑲⑳")

func num(i int) string {
	if i < len(circled) {
		return string(circled[i])
	}
	return strconv.Itoa(i + 1)
}

var useColor = term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == ""

// UseTUI TTY 且未禁色时用 Bubble Tea 盒子；否则纯文本回退。
func UseTUI() bool {
	return useColor && term.IsTerminal(int(os.Stdin.Fd()))
}

func dispWidth(s string) int { return runewidth.StringWidth(s) }

// truncate 按显示宽度截断，超出部分以省略号收尾（对应 menu._truncate）。
func truncate(s string, maxW int) string {
	if dispWidth(s) <= maxW {
		return s
	}
	var out strings.Builder
	width, limit := 0, maxW-1
	for _, ch := range s {
		cw := runewidth.RuneWidth(ch)
		if width+cw > limit {
			break
		}
		out.WriteRune(ch)
		width += cw
	}
	return out.String() + "…"
}

var ansiRe = regexp.MustCompile(`\033\[[0-9;?]*[A-Za-z]`)

func stripAnsi(s string) string { return ansiRe.ReplaceAllString(s, "") }

// rowPad 补齐到宽度 w，忽略已含 ANSI 控制码对宽度的影响。
func rowPad(s string, w int) string {
	pad := w - dispWidth(stripAnsi(s))
	if pad < 0 {
		pad = 0
	}
	return s + strings.Repeat(" ", pad)
}

func dim(s string) string {
	if !useColor {
		return s
	}
	return ansiDim + s + ansiReset
}

func maxBoxWidth(termCols int) int {
	if termCols <= 0 {
		termCols = 80
	}
	return max(20, termCols-2)
}

func maxVisibleRows(termLines int) int {
	if termLines <= 0 {
		termLines = 24
	}
	return max(5, termLines-8)
}

// scrollTop 无状态地算出滚动窗口起点，使选中项尽量居中且不越界。
func scrollTop(n, idx, visible int) int {
	if n <= visible {
		return 0
	}
	return max(0, min(idx-visible/2, n-visible))
}

func maxOf(ns []int) int {
	m := 0
	for _, n := range ns {
		if n > m {
			m = n
		}
	}
	return m
}

// 非 TTY 模式共享 stdin 读取器（避免多次调用丢缓冲）。
var stdinReader = bufio.NewReader(os.Stdin)

func readPlainLine(prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		fmt.Println()
		return "", errs.ErrCancelled
	}
	return strings.TrimRight(line, "\r\n"), nil
}

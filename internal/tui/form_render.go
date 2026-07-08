package tui

import (
	"fmt"
	"strings"

	"github.com/Trilives/clashdock/internal/i18n"
)

// View 渲染表单盒子：标题 + 分节 + 字段行（可滚动窗口）+ 提交/取消按钮 + 底部键位。
func (m *formModel) View() string {
	ss := m.slots()
	if m.cursor >= len(ss) {
		m.cursor = len(ss) - 1
	}
	maxW := maxBoxWidth(m.width)
	labelW := m.labelWidth(maxW)

	// 1. 组装可滚动主体（分节标题 + 字段行），记录焦点字段所在行。
	bodyLines, focusLine := m.bodyLines(ss, labelW, maxW)

	// 2. 终端过矮时纵向滚动，保持焦点行可见（按钮/页脚固定占位另算）。
	visible := max(3, maxVisibleRows(m.height)-4) // 预留标题/按钮/页脚/滚动提示
	top := scrollTop(len(bodyLines), focusLine, visible)
	end := min(top+visible, len(bodyLines))

	// 3. 计算盒宽：主体所有行 + 标题 + 按钮 + 页脚里最宽的一行。
	footer := truncate("  "+i18n.T("↑/↓ 移动   空格 切换/勾选   ←/→ 选项   ⏎ 下一项/确认   esc 取消"), maxW)
	buttons := m.buttonsLine(ss, maxW)
	label := truncate(fmt.Sprintf("─ %s ", m.title), maxW)
	widths := []int{dispWidth(label), dispWidth(footer), dispWidth(stripAnsi(buttons))}
	for _, l := range bodyLines {
		widths = append(widths, dispWidth(stripAnsi(l)))
	}
	w := min(maxOf(widths)+2, maxW)

	// 4. 拼盒。
	rows := []string{"┌" + label + strings.Repeat("─", max(0, w-dispWidth(label))) + "┐"}
	if top > 0 {
		rows = append(rows, "│"+dim(rowPad(truncate(fmt.Sprintf(i18n.T("  ▲ 上方还有 %d 项"), top), maxW), w))+"│")
	} else {
		rows = append(rows, "│"+rowPad("", w)+"│")
	}
	for _, l := range bodyLines[top:end] {
		rows = append(rows, "│"+rowPad(l, w)+"│")
	}
	if end < len(bodyLines) {
		rows = append(rows, "│"+dim(rowPad(truncate(fmt.Sprintf(i18n.T("  ▼ 下方还有 %d 项"), len(bodyLines)-end), maxW), w))+"│")
	} else {
		rows = append(rows, "│"+rowPad("", w)+"│")
	}
	rows = append(rows, "│"+rowPad("", w)+"│")
	rows = append(rows, "│"+rowPad(buttons, w)+"│")
	rows = append(rows, "│"+dim(rowPad(footer, w))+"│")
	rows = append(rows, "└"+strings.Repeat("─", w)+"┘")
	return strings.Join(rows, "\n") + "\n"
}

// labelWidth 标签列宽 = 可见字段标签里最宽的显示宽度（上限盒宽的一半）。
func (m *formModel) labelWidth(maxW int) int {
	w := 0
	for _, f := range m.state.fields {
		if f.visible(m.state) {
			if lw := dispWidth(f.label(m.state)); lw > w {
				w = lw
			}
		}
	}
	return min(w, maxW/2)
}

// bodyLines 生成主体行（分节标题 + 每个可见字段一行），返回行切片与焦点字段行号。
func (m *formModel) bodyLines(ss []slot, labelW, maxW int) (lines []string, focusLine int) {
	focusLine = 0
	lastSection := ""
	for i, s := range ss {
		if s.kind != slotField {
			continue
		}
		f := s.field
		if f.Section != "" && f.Section != lastSection {
			lines = append(lines, dim(truncate("  【"+f.Section+"】", maxW)))
			lastSection = f.Section
		}
		focused := i == m.cursor
		row := m.fieldRow(f, labelW, focused, maxW)
		if focused {
			focusLine = len(lines)
		}
		lines = append(lines, row)
	}
	return lines, focusLine
}

// fieldRow 渲染单个字段行：焦点标记 + 对齐标签 + 控件（开关/单选/文本）。整行先按
// 纯文本排版并截断，焦点行再整体套色（与 buildSelect 一致，避免 ANSI 干扰宽度计算）。
func (m *formModel) fieldRow(f *Field, labelW int, focused bool, maxW int) string {
	marker := "  "
	if focused {
		marker = "❯ "
	}
	label := f.label(m.state)
	label += strings.Repeat(" ", max(0, labelW-dispWidth(label)))
	line := truncate(marker+label+"  "+m.widget(f, focused), maxW)
	if focused && useColor {
		line = ansiCyan + ansiBold + line + ansiReset
	}
	return line
}

// widget 字段控件的纯文本渲染（不含 ANSI；焦点由整行套色体现）。
func (m *formModel) widget(f *Field, focused bool) string {
	switch f.Kind {
	case FieldToggle:
		if f.Bool {
			return "[x]"
		}
		return "[ ]"
	case FieldChoice:
		parts := make([]string, len(f.Choices))
		for i, c := range f.Choices {
			if i == f.ChoiceIdx {
				parts[i] = "[" + c + "]" // 当前项加方括号，无色终端也能区分
			} else {
				parts[i] = c
			}
		}
		return "< " + strings.Join(parts, " / ") + " >"
	case FieldText:
		val := m.inputs[f.Key].Value()
		if val == "" && !focused {
			val = f.Placeholder
		}
		if focused {
			val += "▏" // 简易光标指示（textinput 仍负责实际编辑）
		}
		return "[ " + val + " ]"
	}
	return ""
}

// buttonsLine 渲染「提交 / 取消」按钮行，焦点按钮加方括号高亮。
func (m *formModel) buttonsLine(ss []slot, maxW int) string {
	submit := "  " + m.submitLabel + "  "
	cancel := "  " + m.cancelLabel + "  "
	focusSubmit, focusCancel := false, false
	if m.cursor < len(ss) {
		switch ss[m.cursor].kind {
		case slotSubmit:
			focusSubmit = true
		case slotCancel:
			focusCancel = true
		}
	}
	line := truncate("  "+bracket(submit, focusSubmit)+"    "+bracket(cancel, focusCancel), maxW)
	if useColor {
		if focusSubmit {
			line = strings.Replace(line, bracket(submit, true), ansiCyan+ansiBold+bracket(submit, true)+ansiReset, 1)
		}
		if focusCancel {
			line = strings.Replace(line, bracket(cancel, true), ansiCyan+ansiBold+bracket(cancel, true)+ansiReset, 1)
		}
	}
	return line
}

// bracket 焦点按钮用方括号，非焦点用空心括号，保证无色终端也可辨识。
func bracket(s string, focused bool) string {
	if focused {
		return "❮" + s + "❯"
	}
	return "[" + s + "]"
}

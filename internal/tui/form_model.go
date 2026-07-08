package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// slotKind 焦点槽类型：字段行 / 提交按钮 / 取消按钮。
type slotKind int

const (
	slotField slotKind = iota
	slotSubmit
	slotCancel
)

type slot struct {
	kind  slotKind
	field *Field
}

// formModel 单屏表单的 Bubble Tea 模型。焦点在「可见字段 + 提交 + 取消」之间移动，
// 每次按键都按当前值重算可见字段列表（动态显示/隐藏由此生效）。
type formModel struct {
	title       string
	state       *FormState
	inputs      map[string]*textinput.Model // 各文本字段的输入组件（按 Key）
	submitLabel string
	cancelLabel string
	cursor      int
	width       int
	height      int
	canceled    bool
}

func newFormModel(title string, state *FormState, opts FormOpts) *formModel {
	inputs := make(map[string]*textinput.Model)
	for _, f := range state.fields {
		if f.Kind == FieldText {
			ti := textinput.New()
			ti.SetValue(f.Text)
			ti.Placeholder = f.Placeholder
			ti.Prompt = ""
			inputs[f.Key] = &ti
		}
	}
	m := &formModel{
		title: title, state: state, inputs: inputs,
		submitLabel: opts.SubmitLabel, cancelLabel: opts.CancelLabel,
		width: 80, height: 24,
	}
	m.syncFocus(m.slots())
	return m
}

func (m *formModel) Init() tea.Cmd { return textinput.Blink }

// slots 当前可聚焦项：可见字段（按定义顺序）+ 提交 + 取消。
func (m *formModel) slots() []slot {
	ss := make([]slot, 0, len(m.state.fields)+2)
	for _, f := range m.state.fields {
		if f.visible(m.state) {
			ss = append(ss, slot{slotField, f})
		}
	}
	return append(ss, slot{kind: slotSubmit}, slot{kind: slotCancel})
}

func (m *formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *formModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ss := m.slots()
	if m.cursor >= len(ss) {
		m.cursor = len(ss) - 1
	}
	cur := ss[m.cursor]
	switch msg.String() {
	case "up", "shift+tab":
		m.move(-1)
		return m, nil
	case "down", "tab":
		m.move(1)
		return m, nil
	case "esc", "ctrl+c":
		m.canceled = true
		return m, tea.Quit
	case "enter":
		switch cur.kind {
		case slotSubmit:
			return m, tea.Quit
		case slotCancel:
			m.canceled = true
			return m, tea.Quit
		default:
			m.move(1) // 字段上回车 = 移到下一项（末字段则到「提交」）
			return m, nil
		}
	}
	if cur.kind == slotField {
		return m.editField(cur.field, msg)
	}
	return m, nil
}

// editField 把按键交给具体字段处理：开关空格翻转、单选左右切换、文本走 textinput。
func (m *formModel) editField(f *Field, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch f.Kind {
	case FieldToggle:
		if key == " " {
			f.Bool = !f.Bool
		}
	case FieldChoice:
		n := len(f.Choices)
		if n == 0 {
			return m, nil
		}
		switch key {
		case "left":
			f.ChoiceIdx = (f.ChoiceIdx - 1 + n) % n
		case "right", " ":
			f.ChoiceIdx = (f.ChoiceIdx + 1) % n
		}
	case FieldText:
		ti := m.inputs[f.Key]
		nm, cmd := ti.Update(msg)
		*ti = nm
		f.Text = ti.Value()
		return m, cmd
	}
	return m, nil
}

// move 循环移动焦点并同步文本字段的聚焦状态。
func (m *formModel) move(d int) {
	ss := m.slots()
	n := len(ss)
	if n == 0 {
		return
	}
	m.cursor = (m.cursor + d + n) % n
	m.syncFocus(ss)
}

// syncFocus 只让当前聚焦的文本字段处于 Focus（光标闪烁），其余 Blur。
func (m *formModel) syncFocus(ss []slot) {
	for _, ti := range m.inputs {
		ti.Blur()
	}
	if m.cursor >= 0 && m.cursor < len(ss) {
		if cur := ss[m.cursor]; cur.kind == slotField && cur.field.Kind == FieldText {
			m.inputs[cur.field.Key].Focus()
		}
	}
}

// 单屏表单组件（Select/MultiSelect/Ask/Confirm 之外的第五类阻塞式提示）：把多个
// 字段汇总到同一个盒子里一次性输入，最后统一提交，用于「初始化」这类需要连续填多项
// 设置的场景。字段可随其它字段的当前值动态显示/隐藏、动态改标签（例如本地文件类型时
// 把「订阅链接」改成「文件路径」）；终端过矮时纵向滚动，保持焦点行可见。
// 盒子外观沿用 buildSelect 的 ┌─┐ 风格，与既有菜单一致。
//
// 非 TTY（管道/重定向/测试）自动回退到逐字段的编号提问，可脚本喂答案。
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Trilives/clashdock/internal/i18n"
)

// FormOpts Form 的可选项：提交 / 取消按钮文案（留空取默认）。
type FormOpts struct {
	SubmitLabel string
	CancelLabel string
}

// FieldKind 字段类型。
type FieldKind int

const (
	FieldText   FieldKind = iota // 文本输入
	FieldToggle                  // 复选开关（[x]/[ ]）
	FieldChoice                  // 单选段（< A / B / C >）
)

// Field 表单单字段定义。Visible / LabelFn 会在每次重绘时被调用，可读取表单里其它
// 字段的当前值来动态决定可见性与标签（通过传入的 *FormState）。
type Field struct {
	Key     string // 结果读取键（唯一）
	Label   string // 静态标签；LabelFn 非空时优先用后者
	LabelFn func(s *FormState) string
	Section string // 分节标题（相邻同名字段归为一节；空=无分节）
	Kind    FieldKind

	// FieldText
	Text        string
	Placeholder string
	AllowEmpty  bool // 供调用方参考；渲染层不强制

	// FieldToggle
	Bool bool

	// FieldChoice
	Choices   []string
	ChoiceIdx int

	// Visible nil = 恒可见。
	Visible func(s *FormState) bool
}

// label 当前应显示的标签（LabelFn 优先）。
func (f *Field) label(s *FormState) string {
	if f.LabelFn != nil {
		return f.LabelFn(s)
	}
	return f.Label
}

// visible 当前是否可见（Visible nil = 恒可见）。
func (f *Field) visible(s *FormState) bool {
	return f.Visible == nil || f.Visible(s)
}

// FormState 只读视图：供 Visible/LabelFn 读取同表单其它字段的当前值。
type FormState struct{ fields []*Field }

func (s *FormState) get(key string) *Field {
	for _, f := range s.fields {
		if f.Key == key {
			return f
		}
	}
	return nil
}

// Bool 读某开关字段当前值（不存在=false）。
func (s *FormState) Bool(key string) bool {
	if f := s.get(key); f != nil {
		return f.Bool
	}
	return false
}

// Text 读某文本字段当前值（不存在=""）。
func (s *FormState) Text(key string) string {
	if f := s.get(key); f != nil {
		return f.Text
	}
	return ""
}

// Choice 读某单选字段当前选项文案（不存在/越界=""）。
func (s *FormState) Choice(key string) string {
	if f := s.get(key); f != nil && f.ChoiceIdx >= 0 && f.ChoiceIdx < len(f.Choices) {
		return f.Choices[f.ChoiceIdx]
	}
	return ""
}

// ChoiceIndex 读某单选字段当前下标（不存在=-1）。
func (s *FormState) ChoiceIndex(key string) int {
	if f := s.get(key); f != nil {
		return f.ChoiceIdx
	}
	return -1
}

// FormResult 表单结果。Submitted=false 表示用户取消（esc / Cancel）。
type FormResult struct {
	state     *FormState
	Submitted bool
}

// Text 读结果里某文本字段最终值。
func (r *FormResult) Text(key string) string { return r.state.Text(key) }

// Bool 读结果里某开关字段最终值。
func (r *FormResult) Bool(key string) bool { return r.state.Bool(key) }

// Choice 读结果里某单选字段最终选项文案。
func (r *FormResult) Choice(key string) string { return r.state.Choice(key) }

// ChoiceIndex 读结果里某单选字段最终下标。
func (r *FormResult) ChoiceIndex(key string) int { return r.state.ChoiceIndex(key) }

// Form 运行单屏表单，阻塞至提交或取消。取消（esc / Cancel 按钮）返回
// Submitted=false 的结果（不是错误），与其它提示的 esc 语义区分开——初始化被取消
// 应回退整个事务，由调用方按 Submitted 判定。^C 同 esc。
func Form(title string, fields []Field, opts FormOpts) (*FormResult, error) {
	if opts.SubmitLabel == "" {
		opts.SubmitLabel = i18n.T("提交")
	}
	if opts.CancelLabel == "" {
		opts.CancelLabel = i18n.T("取消")
	}
	fs := make([]*Field, len(fields))
	for i := range fields {
		cp := fields[i]
		fs[i] = &cp
	}
	state := &FormState{fields: fs}
	if !UseTUI() {
		return formPlain(title, state, opts)
	}

	m := newFormModel(title, state, opts)
	out, err := tea.NewProgram(m).Run()
	if err != nil {
		return nil, err
	}
	fm := out.(*formModel)
	if fm.canceled {
		return &FormResult{state: state, Submitted: false}, nil
	}
	return &FormResult{state: state, Submitted: true}, nil
}

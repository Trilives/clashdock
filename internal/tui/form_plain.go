package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Trilives/clashdock/internal/errs"
	"github.com/Trilives/clashdock/internal/i18n"
)

// formPlain 非 TTY 回退：逐个可见字段提问（可见性随已答字段实时重算，与盒装表单
// 一致），末尾问一次提交/取消。脚本可按行喂答案。EOF/取消 → Submitted=false。
func formPlain(title string, state *FormState, opts FormOpts) (*FormResult, error) {
	fmt.Printf("== %s ==\n", title)
	lastSection := ""
	for _, f := range state.fields {
		if !f.visible(state) {
			continue
		}
		if f.Section != "" && f.Section != lastSection {
			fmt.Printf("-- %s --\n", f.Section)
			lastSection = f.Section
		}
		if err := askPlainField(f, state); err != nil {
			if err == errs.ErrCancelled {
				return &FormResult{state: state, Submitted: false}, nil
			}
			return nil, err
		}
	}
	if opts.Note != "" {
		fmt.Println(opts.Note)
	}
	line, err := readPlainLine(fmt.Sprintf("%s? [Y/n]: ", opts.SubmitLabel))
	if err != nil {
		return &FormResult{state: state, Submitted: false}, nil
	}
	line = strings.ToLower(strings.TrimSpace(line))
	submitted := line == "" || line == "y" || line == "yes" || line == "是"
	return &FormResult{state: state, Submitted: submitted}, nil
}

// askPlainField 逐字段提问并写回字段当前值。
func askPlainField(f *Field, state *FormState) error {
	label := f.label(state)
	switch f.Kind {
	case FieldToggle:
		def := "n"
		if f.Bool {
			def = "y"
		}
		raw, err := readPlainLine(fmt.Sprintf("%s [y/n] (%s): ", label, def))
		if err != nil {
			return errs.ErrCancelled
		}
		raw = strings.ToLower(strings.TrimSpace(raw))
		if raw != "" {
			f.Bool = raw == "y" || raw == "yes" || raw == "是"
		}
	case FieldChoice:
		opts := make([]string, len(f.Choices))
		for i, c := range f.Choices {
			opts[i] = fmt.Sprintf("%d) %s", i+1, c)
		}
		raw, err := readPlainLine(fmt.Sprintf("%s [%s] (%d): ", label, strings.Join(opts, " "), f.ChoiceIdx+1))
		if err != nil {
			return errs.ErrCancelled
		}
		raw = strings.TrimSpace(raw)
		if raw != "" {
			if n, cerr := strconv.Atoi(raw); cerr == nil && n >= 1 && n <= len(f.Choices) {
				f.ChoiceIdx = n - 1
			}
		}
	case FieldText:
		suffix := ""
		if f.Text != "" {
			suffix = fmt.Sprintf(" [%s]", f.Text)
		}
		raw, err := readPlainLine(fmt.Sprintf("%s%s: ", label, suffix))
		if err != nil {
			return errs.ErrCancelled
		}
		raw = strings.TrimSpace(raw)
		if raw != "" {
			f.Text = raw
		} else if f.Text == "" && !f.AllowEmpty {
			fmt.Println(i18n.T("不能为空。"))
			return askPlainField(f, state)
		}
	}
	return nil
}

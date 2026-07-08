package tui

import (
	"strings"
	"testing"
)

func TestFormViewRendersVisibleFieldsOnly(t *testing.T) {
	state := buildState([]Field{
		{Key: "tun", Label: "Enable TUN", Section: "Basic", Kind: FieldToggle, Bool: true},
		{Key: "bashrc", Label: "Write bashrc", Section: "Basic", Kind: FieldToggle,
			Visible: func(s *FormState) bool { return !s.Bool("tun") }},
		{Key: "name", Label: "Name", Section: "Sub", Kind: FieldText, Text: "sub-1"},
	})
	m := newFormModel("Init", state, FormOpts{SubmitLabel: "Start", CancelLabel: "Cancel"})
	out := m.View()
	if !strings.Contains(out, "Init") || !strings.Contains(out, "Start") || !strings.Contains(out, "Cancel") {
		t.Fatalf("view missing title/buttons:\n%s", out)
	}
	if !strings.Contains(out, "Enable TUN") || !strings.Contains(out, "Name") {
		t.Fatalf("view missing visible fields:\n%s", out)
	}
	if strings.Contains(out, "Write bashrc") {
		t.Fatalf("hidden field should not render when TUN is on:\n%s", out)
	}
}

// buildState 复刻 Form() 的字段拷贝逻辑，供纯逻辑测试用（不进 Bubble Tea 循环）。
func buildState(fields []Field) *FormState {
	fs := make([]*Field, len(fields))
	for i := range fields {
		cp := fields[i]
		fs[i] = &cp
	}
	return &FormState{fields: fs}
}

func TestFormStateReadsSiblingValues(t *testing.T) {
	s := buildState([]Field{
		{Key: "tun", Kind: FieldToggle, Bool: true},
		{Key: "name", Kind: FieldText, Text: "sub-1"},
		{Key: "type", Kind: FieldChoice, Choices: []string{"clash", "base64", "local"}, ChoiceIdx: 2},
	})
	if !s.Bool("tun") {
		t.Fatalf("Bool(tun) = false, want true")
	}
	if got := s.Text("name"); got != "sub-1" {
		t.Fatalf("Text(name) = %q, want sub-1", got)
	}
	if got := s.Choice("type"); got != "local" {
		t.Fatalf("Choice(type) = %q, want local", got)
	}
	if got := s.ChoiceIndex("type"); got != 2 {
		t.Fatalf("ChoiceIndex(type) = %d, want 2", got)
	}
	if s.Bool("missing") || s.Text("missing") != "" || s.ChoiceIndex("missing") != -1 {
		t.Fatalf("missing keys should return zero values")
	}
}

func TestFieldVisibilityIsDynamic(t *testing.T) {
	// bashrc 仅在 TUN 关闭时可见；allow_port 仅在 lan 开启时可见。
	bashrc := Field{Key: "bashrc", Kind: FieldToggle,
		Visible: func(s *FormState) bool { return !s.Bool("tun") }}
	allowPort := Field{Key: "allow_port", Kind: FieldToggle,
		Visible: func(s *FormState) bool { return s.Bool("lan") }}

	on := buildState([]Field{{Key: "tun", Kind: FieldToggle, Bool: true}, {Key: "lan", Kind: FieldToggle, Bool: false}})
	if bashrc.visible(on) {
		t.Errorf("bashrc should be hidden when TUN is on")
	}
	if allowPort.visible(on) {
		t.Errorf("allow_port should be hidden when LAN is off")
	}

	off := buildState([]Field{{Key: "tun", Kind: FieldToggle, Bool: false}, {Key: "lan", Kind: FieldToggle, Bool: true}})
	if !bashrc.visible(off) {
		t.Errorf("bashrc should be visible when TUN is off")
	}
	if !allowPort.visible(off) {
		t.Errorf("allow_port should be visible when LAN is on")
	}
}

func TestFieldLabelSwitchesOnChoice(t *testing.T) {
	url := Field{Key: "url", Kind: FieldText, LabelFn: func(s *FormState) string {
		if s.ChoiceIndex("type") == 2 {
			return "File path"
		}
		return "Subscription URL"
	}}
	local := buildState([]Field{{Key: "type", Kind: FieldChoice, Choices: []string{"clash", "base64", "local"}, ChoiceIdx: 2}})
	if got := url.label(local); got != "File path" {
		t.Fatalf("label (local) = %q, want File path", got)
	}
	remote := buildState([]Field{{Key: "type", Kind: FieldChoice, Choices: []string{"clash", "base64", "local"}, ChoiceIdx: 0}})
	if got := url.label(remote); got != "Subscription URL" {
		t.Fatalf("label (clash) = %q, want Subscription URL", got)
	}
}

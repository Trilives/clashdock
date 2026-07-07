package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Trilives/clashdock/internal/i18n"
	"github.com/Trilives/clashdock/internal/paths"
)

// 语言选择已从首次运行提示中拆出，改为启动第一步 flows.EnsureLanguage（仅当配置
// 文件未设置过语言时触发）。maybeOfferFirstRunInit 现在只负责「未注册服务时是否初始化」。

func TestMaybeOfferFirstRunInitOffersInitWhenNotInstalled(t *testing.T) {
	var calls []string
	deps := firstRunDeps{
		isInstalled: func(name string) bool {
			calls = append(calls, "is-installed:"+name)
			return false
		},
		confirm: func(prompt string, def bool) (bool, error) {
			calls = append(calls, "confirm")
			if prompt != i18n.T("未检测到已注册的服务，是否现在进行初始化？") {
				t.Fatalf("unexpected init prompt: %q", prompt)
			}
			if !def {
				t.Fatal("first-run initialization prompt should default to yes")
			}
			return true, nil
		},
		initFlow: func(paths.Paths) error {
			calls = append(calls, "init")
			return nil
		},
		reportError: func(string) {},
	}

	maybeOfferFirstRunInit(paths.Paths{}, deps)

	want := []string{"is-installed:mihomo", "confirm", "init"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("call order mismatch:\nwant %#v\n got %#v", want, calls)
	}
}

func TestMaybeOfferFirstRunInitSkipsWhenInstalled(t *testing.T) {
	var calls []string
	deps := firstRunDeps{
		isInstalled: func(string) bool {
			calls = append(calls, "is-installed")
			return true
		},
		confirm: func(string, bool) (bool, error) {
			calls = append(calls, "confirm")
			return true, nil
		},
		initFlow: func(paths.Paths) error {
			calls = append(calls, "init")
			return nil
		},
		reportError: func(string) {},
	}

	maybeOfferFirstRunInit(paths.Paths{}, deps)

	if want := []string{"is-installed"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("installed machine should skip init offer: want %#v, got %#v", want, calls)
	}
}

func TestMaybeOfferFirstRunInitReportsInitError(t *testing.T) {
	var reported string
	deps := firstRunDeps{
		isInstalled: func(string) bool { return false },
		confirm:     func(string, bool) (bool, error) { return true, nil },
		initFlow:    func(paths.Paths) error { return errors.New("init boom") },
		reportError: func(msg string) { reported = msg },
	}

	maybeOfferFirstRunInit(paths.Paths{}, deps)

	if reported != "init boom" {
		t.Fatalf("expected init error reported, got %q", reported)
	}
}

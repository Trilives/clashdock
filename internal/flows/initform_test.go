package flows

import (
	"testing"

	"github.com/Trilives/clashdock/internal/config"
)

func TestInitFormReplacesProcessNameWithFakeIPFilter(t *testing.T) {
	fields := initFormFields(config.Defaults(), false)
	uidIndex, filterIndex := -1, -1
	for i, field := range fields {
		switch field.Key {
		case "tun_exclude_uids":
			uidIndex = i
		case "fake_ip_filter":
			filterIndex = i
		case "tun_exclude_process":
			t.Fatal("初始化表单不应再包含直连进程名")
		}
	}
	if uidIndex < 0 || filterIndex != uidIndex+1 {
		t.Fatalf("fake_ip_filter 应在初始化表单中紧跟直连 UID，实际 UID=%d filter=%d", uidIndex, filterIndex)
	}
	if got := fields[filterIndex].Text; got != "*.lan,*.local,localhost.ptlogin2.qq.com" {
		t.Fatalf("fake_ip_filter 初值 = %q", got)
	}
	if fields[filterIndex].Visible != nil {
		t.Fatal("fake_ip_filter 不应随 TUN 关闭而隐藏")
	}
}

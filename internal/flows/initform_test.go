package flows

import (
	"testing"

	"github.com/Trilives/clashdock/internal/config"
)

func TestInitFormIncludesFakeIPFilterAndDirectProcessName(t *testing.T) {
	cfg := config.Defaults()
	cfg["tun_exclude_process"] = []string{"sshd", "mosh-server"}
	fields := initFormFields(cfg, false)
	uidIndex, filterIndex, processIndex := -1, -1, -1
	for i, field := range fields {
		switch field.Key {
		case "tun_exclude_uids":
			uidIndex = i
		case "fake_ip_filter":
			filterIndex = i
		case "tun_exclude_process":
			processIndex = i
		}
	}
	if uidIndex < 0 || filterIndex != uidIndex+1 {
		t.Fatalf("fake_ip_filter 应在初始化表单中紧跟直连 UID，实际 UID=%d filter=%d", uidIndex, filterIndex)
	}
	if processIndex != filterIndex+1 {
		t.Fatalf("tun_exclude_process 应紧跟 fake_ip_filter，实际 filter=%d process=%d", filterIndex, processIndex)
	}
	if got := fields[filterIndex].Text; got != "*.lan,*.local,localhost.ptlogin2.qq.com" {
		t.Fatalf("fake_ip_filter 初值 = %q", got)
	}
	if fields[filterIndex].Visible != nil {
		t.Fatal("fake_ip_filter 不应随 TUN 关闭而隐藏")
	}
	if got := fields[processIndex].Text; got != "sshd,mosh-server" {
		t.Fatalf("tun_exclude_process 初值 = %q", got)
	}
	if fields[processIndex].Visible == nil {
		t.Fatal("tun_exclude_process 应仅在 TUN 开启时显示")
	}
}

func TestApplyInitSettingsPersistsDirectProcessNames(t *testing.T) {
	base := config.Defaults()
	got := applyInitSettings(base, &initSettings{
		enableTun:      true,
		excludeUIDs:    []int{1000},
		fakeIPFilter:   []string{"*.lan"},
		excludeProcess: []string{"sshd", "mosh-server"},
	})
	if processes := config.StrList(got, "tun_exclude_process"); len(processes) != 2 || processes[0] != "sshd" {
		t.Fatalf("tun_exclude_process 未持久化：%v", processes)
	}
	if processes := config.StrList(base, "tun_exclude_process"); len(processes) != 0 {
		t.Fatalf("不应原地修改原配置：%v", processes)
	}
}

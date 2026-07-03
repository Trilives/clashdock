package config

import (
	"os"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/paths"
)

func testPaths(t *testing.T) paths.Paths {
	t.Setenv("CLASHDOCK_HOME", t.TempDir())
	return paths.Detect()
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	p := testPaths(t)
	cfg := Load(p)
	if !Bool(cfg, "enable_tun") {
		t.Fatal("默认 enable_tun 应为 true")
	}
	if Int(cfg, "webui_port") != 9091 {
		t.Fatalf("默认 webui_port = %d", Int(cfg, "webui_port"))
	}
	if Str(cfg, "subconverter_backend") != DefaultSubconverterBackend {
		t.Fatal("默认 subconverter_backend 不符")
	}
	if got := StrList(cfg, "prefer_keywords"); len(got) != 4 || got[2] != "新加坡" {
		t.Fatalf("默认 prefer_keywords = %v", got)
	}
}

func TestLoadMergesKnownDropsUnknown(t *testing.T) {
	p := testPaths(t)
	os.MkdirAll(p.State, 0o755)
	os.WriteFile(p.CustomizeFile,
		[]byte(`{"enable_tun": false, "no_such_key": 1, "webui_port": 8080}`), 0o644)
	cfg := Load(p)
	if Bool(cfg, "enable_tun") {
		t.Fatal("文件中的 enable_tun=false 应生效")
	}
	if Int(cfg, "webui_port") != 8080 {
		t.Fatalf("webui_port = %d, 期望 8080", Int(cfg, "webui_port"))
	}
	if _, ok := cfg["no_such_key"]; ok {
		t.Fatal("未知键应被丢弃")
	}
	if Str(cfg, "tun_stack") != "gvisor" {
		t.Fatal("缺失字段应以默认补全")
	}
}

func TestLoadCorruptFallsBackToDefaults(t *testing.T) {
	p := testPaths(t)
	os.MkdirAll(p.State, 0o755)
	os.WriteFile(p.CustomizeFile, []byte("{invalid json"), 0o644)
	cfg := Load(p)
	if !Bool(cfg, "enable_tun") {
		t.Fatal("解析失败应回退默认值")
	}
}

func TestSaveLoadRoundtripOrderAndUnicode(t *testing.T) {
	p := testPaths(t)
	cfg := Defaults()
	cfg["secret"] = "abcd1234"
	cfg["webui_port"] = 9092
	if err := Save(p, cfg); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(p.CustomizeFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `"新加坡"`) {
		t.Fatal("非 ASCII 字符不应被转义")
	}
	if strings.Index(s, `"enable_tun"`) > strings.Index(s, `"webui_port"`) {
		t.Fatal("写盘键序应与默认声明顺序一致")
	}

	got := Load(p)
	if Str(got, "secret") != "abcd1234" {
		t.Fatal("roundtrip 后 secret 不符")
	}
	if Int(got, "webui_port") != 9092 {
		t.Fatal("roundtrip 后 webui_port 不符")
	}
}

func TestEnsureExistsWritesOnce(t *testing.T) {
	p := testPaths(t)
	if _, err := EnsureExists(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.CustomizeFile); err != nil {
		t.Fatal("首次运行应落地默认 customize.json:", err)
	}
}

func TestMaskSecret(t *testing.T) {
	if got := MaskSecret("abcdef"); got != "已设置（***cdef）" {
		t.Fatalf("MaskSecret 长值 = %q", got)
	}
	if got := MaskSecret("abc"); got != "已设置（***）" {
		t.Fatalf("MaskSecret 短值 = %q", got)
	}
}

func TestSummary(t *testing.T) {
	cfg := Defaults()
	cases := []struct{ key, want string }{
		{"enable_tun", "开"},
		{"lan_proxy", "关"},
		{"direct_domain_suffixes", "空"},
		{"prefer_keywords", "4 条"},
		{"secret", "未设置"},
		{"webui_port", "9091"},
	}
	for _, c := range cases {
		if got := Summary(cfg, c.key); got != c.want {
			t.Errorf("Summary(%s) = %q, 期望 %q", c.key, got, c.want)
		}
	}
	cfg["secret"] = "verysecret"
	if got := Summary(cfg, "secret"); got != "已设置（***cret）" {
		t.Errorf("涉密字段摘要 = %q", got)
	}
}

func TestFieldMetadataConsistency(t *testing.T) {
	if len(FieldOrder) != len(defaultsOrder) {
		t.Fatalf("FieldOrder(%d) 与 defaultsOrder(%d) 数量不一致", len(FieldOrder), len(defaultsOrder))
	}
	defaults := Defaults()
	for _, k := range FieldOrder {
		if _, ok := defaults[k]; !ok {
			t.Errorf("FieldOrder 含未知键 %s", k)
		}
		n := 0
		if _, ok := ListFields[k]; ok {
			n++
		}
		if _, ok := BoolFields[k]; ok {
			n++
		}
		if _, ok := ScalarFields[k]; ok {
			n++
		}
		if n != 1 {
			t.Errorf("键 %s 应恰好属于一类字段元数据, 实际 %d 类", k, n)
		}
	}
}
